package engine

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"sort"
	"strings"
	"time"

	"unbound/engine/providers"
)

// StrategyLabTargetConfig defines the target host and parameters for Strategy Lab.
type StrategyLabTargetConfig struct {
	TargetHost            string   `json:"targetHost"`
	ServicePreset         string   `json:"servicePreset"` // "YouTube", "Discord", "Steam", "Custom"
	Protocol              string   `json:"protocol"`      // "TLS1.3", "TLS1.2", "HTTP", "QUIC", "ANY"
	CustomCandidateArgs   []string `json:"customCandidateArgs,omitempty"`
	ForceProbeIfReachable bool     `json:"forceProbeIfReachable"`
}

// ExecutionStatus models the orchestration and execution outcome of a candidate test.
type ExecutionStatus string

const (
	ExecutionStatusPass               ExecutionStatus = "PASS"
	ExecutionStatusFail               ExecutionStatus = "FAIL"
	ExecutionStatusSkippedUnsupported ExecutionStatus = "SKIPPED_UNSUPPORTED"
	ExecutionStatusSkippedConfigError ExecutionStatus = "SKIPPED_CONFIG_ERROR"
	ExecutionStatusStartFailed        ExecutionStatus = "START_FAILED"
	ExecutionStatusCaptureNotReady    ExecutionStatus = "CAPTURE_NOT_READY"
	ExecutionStatusEngineExited       ExecutionStatus = "ENGINE_EXITED"
	ExecutionStatusTimeout            ExecutionStatus = "TIMEOUT"
	ExecutionStatusCancelled          ExecutionStatus = "CANCELLED"
	ExecutionStatusCleanupFailed      ExecutionStatus = "CLEANUP_FAILED"
	ExecutionStatusNoBypassNeeded     ExecutionStatus = "NO_BYPASS_NEEDED"
)

// RestorationStatus models the transactional outcome of restoring the previous engine state.
type RestorationStatus string

const (
	RestorationStatusRestored  RestorationStatus = "RESTORED"
	RestorationStatusNotNeeded RestorationStatus = "NOT_NEEDED"
	RestorationStatusFailed    RestorationStatus = "FAILED"
)

// CandidateTestResult represents the evaluation result of a single strategy candidate.
type CandidateTestResult struct {
	Candidate       StrategyCandidate `json:"candidate"`
	Status          ProbeStatus       `json:"status"`          // PASS, FAIL, WARNING, NOT_VERIFIED (network probe outcome)
	ExecutionStatus ExecutionStatus   `json:"executionStatus"` // Structured orchestration outcome
	PassCount       int               `json:"passCount"`
	TotalAttempts   int               `json:"totalAttempts"`
	AvgLatency      time.Duration     `json:"avgLatency"`
	Score           int               `json:"score"`
	Details         string            `json:"details"`
	Error           string            `json:"error,omitempty"`
}

// ValidationStatus represents the verified outcome of Strategy Lab service validation.
type ValidationStatus string

const (
	ValidationStatusVerified    ValidationStatus = "VERIFIED"
	ValidationStatusPartial     ValidationStatus = "PARTIAL"
	ValidationStatusNotVerified ValidationStatus = "NOT_VERIFIED"
	ValidationStatusFailed      ValidationStatus = "FAILED"
)

// StrategyLabReport contains the comprehensive findings of the discovery session.
type StrategyLabReport struct {
	RunID                 string                 `json:"runId"`
	TargetHost            string                 `json:"targetHost"`
	Protocol              string                 `json:"protocol"`
	BaselineReachable     bool                   `json:"baselineReachable"`
	BaselineStatus        ProbeResult            `json:"baselineStatus"`
	BaselineProtocolLabel string                 `json:"baselineProtocolLabel"`
	BaselineMap           map[string]ProbeResult `json:"baselineMap,omitempty"`
	RestorationStatus     RestorationStatus      `json:"restorationStatus"`
	RestorationError      string                 `json:"restorationError,omitempty"`
	CandidateResults      []CandidateTestResult  `json:"candidateResults"`
	WorkingCandidates     []CandidateTestResult  `json:"workingCandidates"`
	TestedCandidates      int                    `json:"testedCandidates"`
	TotalCandidates       int                    `json:"totalCandidates"`
	BestCandidate         *CandidateTestResult   `json:"bestCandidate,omitempty"`
	ValidationStatus      ValidationStatus       `json:"validationStatus"`
	ValidationDetails     string                 `json:"validationDetails,omitempty"`
	ServiceVerified       bool                   `json:"serviceVerified"` // Kept for UI backwards compatibility (true if VERIFIED)
	Duration              time.Duration          `json:"duration"`
	Timestamp             time.Time              `json:"timestamp"`
	TargetIPs             []string               `json:"targetIps,omitempty"`
}

// StrategyLabProgress reports real-time iteration metrics to the frontend UI.
type StrategyLabProgress struct {
	RunID                 string `json:"runId"`
	TargetHost            string `json:"targetHost"`
	CurrentCandidateIndex int    `json:"currentCandidateIndex"`
	TotalCandidates       int    `json:"totalCandidates"`
	CurrentCandidateName  string `json:"currentCandidateName"`
	BaselineStatus        string `json:"baselineStatus"`
	Percent               int    `json:"percent"`
	ElapsedMs             int64  `json:"elapsedMs"`
	LastResult            string `json:"lastResult"`
}

type StrategyLabProgressFn func(p StrategyLabProgress)

// RunStrategyLab executes the strategy discovery session with isolation and safety guarantees.
func RunStrategyLab(
	ctx context.Context,
	pc ProviderController,
	cfg StrategyLabTargetConfig,
	onProgress StrategyLabProgressFn,
) (*StrategyLabReport, error) {
	runner, err := NewDefaultCandidateRunner()
	if err != nil {
		return nil, fmt.Errorf("failed to initialize candidate runner: %w", err)
	}
	return RunStrategyLabWithRunner(ctx, pc, runner, cfg, onProgress)
}

// RunStrategyLabWithRunner executes strategy discovery with an explicit CandidateRunner (for testing/mocking).
func RunStrategyLabWithRunner(
	ctx context.Context,
	pc ProviderController,
	runner CandidateRunner,
	cfg StrategyLabTargetConfig,
	onProgress StrategyLabProgressFn,
) (*StrategyLabReport, error) {
	if cfg.TargetHost == "" {
		return nil, errors.New("target host is required for Strategy Lab")
	}
	if !IsValidProtocol(cfg.Protocol) {
		return nil, fmt.Errorf("invalid or unsupported protocol %q for Strategy Lab (allowed: HTTP, TLS1.2, TLS1.3, QUIC, ANY)", cfg.Protocol)
	}
	if runner == nil {
		var err error
		runner, err = NewDefaultCandidateRunner()
		if err != nil {
			return nil, fmt.Errorf("failed to initialize candidate runner: %w", err)
		}
	}

	logger := GetLogger()
	logger.Infof("Lab", "[LAB] starting strategy discovery session for %s (%s)", cfg.TargetHost, cfg.Protocol)

	// 1. Acquire exclusive lock via OperationCoordinator
	release, err := GetCoordinator().Acquire(OpStrategyLab, "user", true)
	if err != nil {
		return nil, fmt.Errorf("cannot start Strategy Lab: %w", err)
	}

	startTime := time.Now()
	runID := fmt.Sprintf("lab_%d", startTime.UnixNano())

	// Save active engine state for transactional restoration
	var initialProfile string
	var wasRunning bool
	if pc != nil {
		initialProfile = pc.CurrentProfile()
		wasRunning = pc.GetStatus() == providers.StatusRunning
	}

	report := &StrategyLabReport{
		RunID:             runID,
		TargetHost:        cfg.TargetHost,
		Protocol:          cfg.Protocol,
		RestorationStatus: RestorationStatusNotNeeded,
		Timestamp:         startTime,
		CandidateResults:  make([]CandidateTestResult, 0),
		WorkingCandidates: make([]CandidateTestResult, 0),
		BaselineMap:       make(map[string]ProbeResult),
	}

	// Defer guarantees that previous profile is restored and coordinator is released
	defer func() {
		cleanupCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		if wasRunning && pc != nil && initialProfile != "" {
			if pc.GetStatus() != providers.StatusRunning || pc.CurrentProfile() != initialProfile {
				logger.Infof("Lab", "[LAB] restoring previous active profile %s", initialProfile)
				if err := pc.Start(cleanupCtx, initialProfile); err != nil {
					logger.Errorf("Lab", "[LAB] CRITICAL: failed to restore previous active profile %s: %v", initialProfile, err)
					report.RestorationStatus = RestorationStatusFailed
					report.RestorationError = fmt.Sprintf("Failed to restore previous profile %s: %v", initialProfile, err)
				} else {
					report.RestorationStatus = RestorationStatusRestored
				}
			} else {
				report.RestorationStatus = RestorationStatusRestored
			}
		} else if !wasRunning && pc != nil {
			if pc.GetStatus() == providers.StatusRunning {
				if err := pc.Stop(); err != nil {
					logger.Errorf("Lab", "[LAB] failed to stop engine during cleanup: %v", err)
					report.RestorationStatus = RestorationStatusFailed
					report.RestorationError = fmt.Sprintf("Failed to stop engine during cleanup: %v", err)
				} else {
					report.RestorationStatus = RestorationStatusNotNeeded
				}
			}
		}
		release()
	}()

	ce := NewConnectivityEngine(3 * time.Second)

	// 2. Step 1: Baseline measurement (without bypass)
	// Invariant: previous engine must be fully stopped and verified before baseline
	if wasRunning && pc != nil {
		logger.Info("Lab", "[LAB] stopping active engine for baseline measurement")
		if err := pc.Stop(); err != nil {
			logger.Errorf("Lab", "[LAB] failed to stop active engine before baseline: %v", err)
			return nil, fmt.Errorf("cannot run Strategy Lab: failed to stop active engine for clean baseline: %w", err)
		}
		deadline := time.Now().Add(2 * time.Second)
		stopped := false
		for time.Now().Before(deadline) {
			if pc.GetStatus() == providers.StatusStopped {
				stopped = true
				break
			}
			time.Sleep(50 * time.Millisecond)
		}
		if !stopped && pc.GetStatus() != providers.StatusStopped {
			return nil, fmt.Errorf("cannot run Strategy Lab: engine did not transition to StatusStopped (current: %v)", pc.GetStatus())
		}
	}

	targetURL := cfg.TargetHost
	if !strings.HasPrefix(targetURL, "http://") && !strings.HasPrefix(targetURL, "https://") {
		if strings.EqualFold(cfg.Protocol, "HTTP") {
			targetURL = "http://" + targetURL
		} else {
			targetURL = "https://" + targetURL
		}
	}
	protoUpper := strings.ToUpper(strings.TrimSpace(cfg.Protocol))
	baselineLabel := fmt.Sprintf("Baseline %s", protoUpper)
	baselineMap := make(map[string]ProbeResult)
	var baseResult ProbeResult
	var baselineReachable bool

	if protoUpper == "ANY" {
		baselineLabel = "Baseline Multi-Protocol"
		// Compute per-protocol baselines: HTTP, TLS1.2, TLS1.3, QUIC
		httpBase := ProbeStrategyLabBaseline(ctx, ce, targetURL, "HTTP")
		tls12Base := ProbeStrategyLabBaseline(ctx, ce, targetURL, "TLS1.2")
		tls13Base := ProbeStrategyLabBaseline(ctx, ce, targetURL, "TLS1.3")
		quicBase := ProbeStrategyLabBaseline(ctx, ce, targetURL, "QUIC")
		baselineMap["HTTP"] = httpBase
		baselineMap["TLS1.2"] = tls12Base
		baselineMap["TLS1.3"] = tls13Base
		baselineMap["QUIC"] = quicBase

		baselineReachable = httpBase.Status == StatusPass && tls12Base.Status == StatusPass && tls13Base.Status == StatusPass && quicBase.Status == StatusPass
		baseResult = tls13Base
		logger.Infof("Lab", "[LAB] ANY multi-protocol baselines: HTTP=%s, TLS1.2=%s, TLS1.3=%s, QUIC=%s",
			httpBase.Status, tls12Base.Status, tls13Base.Status, quicBase.Status)
	} else {
		switch protoUpper {
		case "TLS1.3":
			baselineLabel = "Baseline TLS 1.3"
		case "TLS1.2":
			baselineLabel = "Baseline TLS 1.2"
		case "QUIC":
			baselineLabel = "Baseline QUIC"
		case "HTTP":
			baselineLabel = "Baseline HTTP"
		}
		baseResult = ProbeStrategyLabBaseline(ctx, ce, targetURL, cfg.Protocol)
		baselineReachable = baseResult.Status == StatusPass
		baselineMap[protoUpper] = baseResult
		logger.Infof("Lab", "[LAB] %s check for %s: %s (latency=%v)", baselineLabel, targetURL, baseResult.Status, baseResult.Latency)
	}

	report.BaselineReachable = baselineReachable
	report.BaselineStatus = baseResult
	report.BaselineProtocolLabel = baselineLabel
	report.BaselineMap = baselineMap

	if baselineReachable && !cfg.ForceProbeIfReachable {
		logger.Infof("Lab", "[LAB] target is already reachable via %s without bypass, discovery completed early", protoUpper)
		report.Duration = time.Since(startTime)
		return report, nil
	}

	// 3. Step 2: Build Isolated WinDivert Raw Filter
	var ports []int
	protoStr := "tcp"
	switch strings.ToUpper(strings.TrimSpace(cfg.Protocol)) {
	case "HTTP":
		ports = []int{80}
		protoStr = "tcp"
	case "QUIC":
		ports = []int{443}
		protoStr = "udp"
	case "ANY":
		ports = []int{80, 443}
		protoStr = "both"
	default: // TLS1.3, TLS1.2, etc.
		ports = []int{443}
		protoStr = "tcp"
	}

	filterConfig := TargetFilterConfig{
		TargetHost: cfg.TargetHost,
		Ports:      ports,
		Protocol:   protoStr,
	}
	rawFilter, targetIPs, err := BuildIsolatedWinDivertFilter(ctx, filterConfig)
	if err != nil {
		logger.Errorf("Lab", "[LAB] failed to build isolated raw filter: %v", err)
		return nil, fmt.Errorf("failed to build isolated target filter: %w", err)
	}
	if len(targetIPs) > 0 {
		ce.PinHost(cfg.TargetHost, targetIPs[0])
		defer ce.UnpinHost(cfg.TargetHost)
	}

	for _, ip := range targetIPs {
		report.TargetIPs = append(report.TargetIPs, ip.String())
	}
	logger.Infof("Lab", "[LAB] isolated raw filter constructed for %d IPs (pinned to %s): %s",
		len(targetIPs), targetIPs[0], rawFilter)
	// 4. Step 3: Load Candidates
	var candidates []StrategyCandidate
	if len(cfg.CustomCandidateArgs) > 0 {
		candidates = append(candidates, StrategyCandidate{
			ID:             "cand_custom",
			Name:           "Custom Strategy",
			Protocol:       cfg.Protocol,
			Zapret2Args:    cfg.CustomCandidateArgs,
			Aggressiveness: AggressivenessHigh,
			Source:         "user custom",
			Experimental:   true,
		})
	}
	candidates = append(candidates, GetBlockCheck2Candidates(cfg.Protocol)...)
	report.TotalCandidates = len(candidates)

	// 5. Step 4: Iterate and Test Candidates with Isolation
	assets, _ := ExtractAssets()
	for idx, cand := range candidates {
		if ctx.Err() != nil {
			logger.Warnf("Lab", "[LAB] strategy discovery cancelled by user: %v", ctx.Err())
			return nil, ctx.Err()
		}

		pct := (idx * 100) / len(candidates)
		if onProgress != nil {
			onProgress(StrategyLabProgress{
				RunID:                 runID,
				TargetHost:            cfg.TargetHost,
				CurrentCandidateIndex: idx + 1,
				TotalCandidates:       len(candidates),
				CurrentCandidateName:  cand.Name,
				BaselineStatus:        string(baseResult.Status),
				Percent:               pct,
				ElapsedMs:             time.Since(startTime).Milliseconds(),
			})
		}

		// Determine candidate's tested protocol
		candProto := strings.ToUpper(strings.TrimSpace(cand.Protocol))
		if candProto == "" || candProto == "ANY" {
			candProto = "TLS1.3"
			for _, a := range cand.Zapret2Args {
				if strings.Contains(a, "quic") {
					candProto = "QUIC"
					break
				}
				if strings.Contains(a, "http") && !strings.Contains(a, "tls") {
					candProto = "HTTP"
					break
				}
			}
		}
		cand.TestedProtocol = candProto

		candBase, hasBase := baselineMap[candProto]
		candBaselineReachable := false
		if hasBase {
			candBaselineReachable = candBase.Status == StatusPass
		} else {
			candBaselineReachable = baselineReachable
		}

		// In ANY mode, if this candidate's protocol is ALREADY reachable without bypass:
		if protoUpper == "ANY" && candBaselineReachable && !cfg.ForceProbeIfReachable {
			logger.Infof("Lab", "[LAB] candidate %s protocol %s already reachable without bypass (baseline PASS), skipping bypass test", cand.Name, candProto)
			report.TestedCandidates++
			report.CandidateResults = append(report.CandidateResults, CandidateTestResult{
				Candidate:       cand,
				Status:          StatusPass,
				ExecutionStatus: ExecutionStatusNoBypassNeeded,
				TotalAttempts:   0,
				PassCount:       0,
				Score:           0,
				Details:         fmt.Sprintf("Protocol %s is already reachable without bypass (baseline PASS)", candProto),
			})
			continue
		}

		// Capability check before launch
		if ok, reason := CheckCandidateCapabilities(cand, assets); !ok {
			logger.Infof("Lab", "[LAB] skipping candidate [%d/%d] %s: %s", idx+1, len(candidates), cand.Name, reason)
			report.TestedCandidates++
			execStatus := ExecutionStatusSkippedUnsupported
			if strings.HasPrefix(reason, "SKIPPED_CONFIG_ERROR") {
				execStatus = ExecutionStatusSkippedConfigError
			}
			report.CandidateResults = append(report.CandidateResults, CandidateTestResult{
				Candidate:       cand,
				Status:          StatusFail,
				ExecutionStatus: execStatus,
				TotalAttempts:   0,
				Error:           reason,
				Details:         reason,
			})
			continue
		}

		logger.Infof("Lab", "[LAB] testing candidate [%d/%d]: %s (proto=%s)", idx+1, len(candidates), cand.Name, candProto)

		// 1. Launch temporary isolated winws2 process with strict raw filter
		proc, err := runner.StartCandidate(ctx, cand, rawFilter)
		if err != nil {
			logger.Warnf("Lab", "[LAB] candidate %s failed to start: %v", cand.Name, err)
			report.TestedCandidates++
			report.CandidateResults = append(report.CandidateResults, CandidateTestResult{
				Candidate:       cand,
				Status:          StatusFail,
				ExecutionStatus: ExecutionStatusStartFailed,
				TotalAttempts:   3,
				Error:           fmt.Sprintf("START_FAILED: %v", err),
				Details:         "Failed to spawn candidate process",
			})
			continue
		}

		// Invariant: probe MUST NOT begin unless process reached CaptureReady or DryRunValidated
		if proc.State() != ProcessStateCaptureReady && proc.State() != ProcessStateDryRunValidated {
			stopErr := proc.Stop()
			logger.Warnf("Lab", "[LAB] candidate %s in invalid state %v: capture not ready", cand.Name, proc.State())
			report.TestedCandidates++
			if stopErr != nil {
				report.CandidateResults = append(report.CandidateResults, CandidateTestResult{
					Candidate:       cand,
					Status:          StatusFail,
					ExecutionStatus: ExecutionStatusCleanupFailed,
					TotalAttempts:   3,
					Error:           fmt.Sprintf("CAPTURE_NOT_READY and CLEANUP_FAILED: %v", stopErr),
					Details:         "Candidate failed to reach CAPTURE_READY and failed to terminate cleanly",
				})
				return report, fmt.Errorf("aborting Strategy Lab: invalid candidate %q cleanup failed: %w", cand.Name, stopErr)
			}
			report.CandidateResults = append(report.CandidateResults, CandidateTestResult{
				Candidate:       cand,
				Status:          StatusFail,
				ExecutionStatus: ExecutionStatusCaptureNotReady,
				TotalAttempts:   3,
				Error:           fmt.Sprintf("CAPTURE_NOT_READY: process state %v", proc.State()),
				Details:         "Candidate failed to reach CAPTURE_READY",
			})
			continue
		}

		// 2. Perform protocol-specific probes while candidate process is active
		res := testCandidateWithProcess(ctx, ce, proc, cand, targetURL, candProto, candBaselineReachable)
		report.TestedCandidates++
		report.CandidateResults = append(report.CandidateResults, res)

		// 3. Stop candidate process and release WinDivert handles
		// Invariant: stop error MUST abort lab immediately to prevent overlapping candidate processes!
		if stopErr := proc.Stop(); stopErr != nil {
			logger.Errorf("Lab", "[LAB] candidate %s cleanup failed: %v — aborting discovery to prevent overlapping processes", cand.Name, stopErr)
			res.ExecutionStatus = ExecutionStatusCleanupFailed
			res.Error = fmt.Sprintf("CLEANUP_FAILED: %v", stopErr)
			return report, fmt.Errorf("aborting Strategy Lab: candidate %q cleanup failed: %w", cand.Name, stopErr)
		}

		if res.Status == StatusPass && res.PassCount >= 2 {
			logger.Infof("Lab", "[LAB] candidate %s PASSED (%d/%d, avg latency=%v)",
				cand.Name, res.PassCount, res.TotalAttempts, res.AvgLatency)
			report.WorkingCandidates = append(report.WorkingCandidates, res)
		} else {
			logger.Debugf("Lab", "[LAB] candidate %s failed (%d/%d): %s",
				cand.Name, res.PassCount, res.TotalAttempts, res.Error)
		}
	}

	// 6. Step 5: Rank Working Candidates
	if len(report.WorkingCandidates) > 0 {
		sort.Slice(report.WorkingCandidates, func(i, j int) bool {
			ci := report.WorkingCandidates[i]
			cj := report.WorkingCandidates[j]

			if ci.PassCount != cj.PassCount {
				return ci.PassCount > cj.PassCount
			}
			if ci.Candidate.Aggressiveness != cj.Candidate.Aggressiveness {
				return ci.Candidate.Aggressiveness < cj.Candidate.Aggressiveness
			}
			return ci.AvgLatency < cj.AvgLatency
		})

		best := report.WorkingCandidates[0]
		if best.Candidate.TestedProtocol == "" || best.Candidate.TestedProtocol == "ANY" {
			best.Candidate.TestedProtocol = best.Candidate.Protocol
		}
		if best.Candidate.TestedProtocol == "" || best.Candidate.TestedProtocol == "ANY" {
			best.Candidate.TestedProtocol = "TLS1.3"
		}
		report.BestCandidate = &best
		logger.Infof("Lab", "[LAB] best discovered candidate: %s (score=%d, proto=%s, aggressiveness=%s)",
			best.Candidate.Name, best.Score, best.Candidate.TestedProtocol, best.Candidate.Aggressiveness)

		// 7. Step 6: Full Service/Target Validation on Winner WHILE WINNER PROCESS IS ACTIVE!
		var validationEndpoints []string
		isCustom := cfg.ServicePreset == "" || strings.EqualFold(cfg.ServicePreset, "Custom")
		if !isCustom {
			validationEndpoints = GetServiceValidationEndpoints(cfg.ServicePreset)
		} else {
			cleanTarget := extractHost(cfg.TargetHost)
			if cleanTarget == "" {
				cleanTarget = cfg.TargetHost
			}
			validationEndpoints = []string{cleanTarget}
		}

		logger.Infof("Lab", "[LAB] preparing validation filter for %s (endpoints: %v)", cfg.ServicePreset, validationEndpoints)
		var combinedIPs []net.IP
		endpointsResolved := true
		for _, ep := range validationEndpoints {
			if ep == "" {
				continue
			}
			epIPs, err := net.DefaultResolver.LookupIP(ctx, "ip", ep)
			if err != nil || len(epIPs) == 0 {
				logger.Warnf("Lab", "[LAB] fail-closed: failed to resolve validation endpoint %s: %v", ep, err)
				endpointsResolved = false
				break
			}
			ce.PinHost(ep, epIPs[0])
			defer ce.UnpinHost(ep)
			combinedIPs = append(combinedIPs, epIPs...)
		}

		if !endpointsResolved {
			logger.Warnf("Lab", "[LAB] service validation aborted: one or more required endpoints failed resolution")
			report.ValidationStatus = ValidationStatusPartial
			report.ValidationDetails = "Validation aborted: required validation endpoint failed DNS resolution"
			report.ServiceVerified = false
		} else {
			serviceRawFilter := rawFilter
			filterGenErr := false

			if len(combinedIPs) > 0 {
				f, err := GenerateWinDivertFilterForIPs(combinedIPs, ports, protoStr)
				if err != nil {
					logger.Errorf("Lab", "[LAB] fail-closed: failed to generate combined service validation filter: %v", err)
					report.ValidationStatus = ValidationStatusFailed
					report.ValidationDetails = fmt.Sprintf("Validation filter generation failed: %v", err)
					report.ServiceVerified = false
					filterGenErr = true
				} else {
					serviceRawFilter = f
				}
			}

			if !filterGenErr {
				logger.Infof("Lab", "[LAB] launching winner %s for validation with filter: %s",
					best.Candidate.Name, serviceRawFilter)
				ce.ResetConnectionPool()
				winnerProc, err := runner.StartCandidate(ctx, best.Candidate, serviceRawFilter)
				if err == nil && winnerProc != nil {
					validationProto := best.Candidate.TestedProtocol
					if validationProto == "" || validationProto == "ANY" {
						validationProto = "TLS1.3"
					}
					valResult := validateCandidateAgainstService(ctx, ce, cfg.ServicePreset, validationProto, cfg.TargetHost)
					if !winnerProc.Alive() {
						logger.Warnf("Lab", "[LAB] winner process exited prematurely during validation")
						report.ValidationStatus = ValidationStatusFailed
						report.ValidationDetails = "Winner process terminated unexpectedly during validation"
						report.ServiceVerified = false
					} else {
						report.ValidationStatus = valResult.Status
						report.ValidationDetails = valResult.Details
						report.ServiceVerified = (valResult.Status == ValidationStatusVerified)
					}
					if stopErr := winnerProc.Stop(); stopErr != nil {
						logger.Errorf("Lab", "[LAB] winner validation cleanup failed: %v", stopErr)
						report.ValidationStatus = ValidationStatusFailed
						report.ServiceVerified = false
						report.ValidationDetails = fmt.Sprintf("Winner process cleanup failed: %v", stopErr)
						return report, fmt.Errorf("winner validation cleanup failed: %w", stopErr)
					}
				} else {
					logger.Warnf("Lab", "[LAB] failed to start winner for validation: %v", err)
					report.ValidationStatus = ValidationStatusFailed
					report.ValidationDetails = fmt.Sprintf("Failed to launch winner process for validation: %v", err)
					report.ServiceVerified = false
				}
				ce.ResetConnectionPool()
			}
		}
	} else {
		report.BestCandidate = nil
		report.ValidationStatus = ValidationStatusNotVerified
		report.ValidationDetails = "No working candidates discovered"
		report.ServiceVerified = false
	}

	report.Duration = time.Since(startTime)
	logger.Infof("Lab", "[LAB] discovery completed in %v: %d working candidates found",
		report.Duration, len(report.WorkingCandidates))

	return report, nil
}

func testCandidateWithProcess(
	ctx context.Context,
	ce *ConnectivityEngine,
	proc CandidateProcess,
	cand StrategyCandidate,
	targetURL string,
	protocol string,
	baselineReachable bool,
) CandidateTestResult {
	const attempts = 3
	passCount := 0
	var totalLatency time.Duration
	var lastErr string

	for i := range attempts {
		if ctx.Err() != nil {
			break
		}

		// Process liveness check before attempt
		if proc != nil && !proc.Alive() {
			waitErr := proc.WaitErr()
			return CandidateTestResult{
				Candidate:       cand,
				Status:          StatusFail,
				ExecutionStatus: ExecutionStatusEngineExited,
				TotalAttempts:   attempts,
				Error:           fmt.Sprintf("ENGINE_EXITED: candidate process terminated unexpectedly before attempt %d (err: %v)", i+1, waitErr),
				Details:         "ENGINE_EXITED: process crashed or terminated prematurely",
			}
		}

		probeRes := ProbeTargetProtocol(ctx, ce, targetURL, protocol)
		ce.ResetConnectionPool()

		// Process liveness check after attempt
		if proc != nil && !proc.Alive() {
			waitErr := proc.WaitErr()
			return CandidateTestResult{
				Candidate:       cand,
				Status:          StatusFail,
				ExecutionStatus: ExecutionStatusEngineExited,
				TotalAttempts:   attempts,
				Error:           fmt.Sprintf("ENGINE_EXITED: candidate process terminated during attempt %d (err: %v)", i+1, waitErr),
				Details:         "ENGINE_EXITED: process crashed during network probe",
			}
		}
		if probeRes.Status == StatusPass {
			passCount++
			totalLatency += probeRes.Latency
		} else {
			lastErr = probeRes.Error
		}
		time.Sleep(100 * time.Millisecond)
	}

	avgLat := time.Duration(0)
	if passCount > 0 {
		avgLat = totalLatency / time.Duration(passCount)
	}
	status := StatusFail
	execStatus := ExecutionStatusFail
	if ctx.Err() != nil {
		execStatus = ExecutionStatusCancelled
	} else if passCount >= 2 {
		status = StatusPass
		execStatus = ExecutionStatusPass
	}
	score := passCount * 30
	if cand.Aggressiveness == AggressivenessLow {
		score += 20
	} else if cand.Aggressiveness == AggressivenessMedium {
		score += 10
	}
	if avgLat > 0 && avgLat < 150*time.Millisecond {
		score += 10
	}

	details := fmt.Sprintf("%d/%d успешно, латентность: %v", passCount, attempts, avgLat.Round(time.Millisecond))
	if !baselineReachable && passCount >= 2 {
		details += " (⭐ Обход успешен!)"
	} else if baselineReachable && passCount >= 2 {
		details += " (Цель доступна)"
	}

	return CandidateTestResult{
		Candidate:       cand,
		Status:          status,
		ExecutionStatus: execStatus,
		PassCount:       passCount,
		TotalAttempts:   attempts,
		AvgLatency:      avgLat,
		Score:           score,
		Details:         details,
		Error:           lastErr,
	}
}

// ServiceValidationResult contains structured outcomes of the final winner verification.
type ServiceValidationResult struct {
	Status   ValidationStatus `json:"status"`
	Protocol string           `json:"protocol"`
	Details  string           `json:"details"`
	Probes   []ProbeResult    `json:"probes"`
}

func validateCandidateAgainstService(
	ctx context.Context,
	ce *ConnectivityEngine,
	servicePreset string,
	protocol string,
	targetHost string,
) ServiceValidationResult {
	preset := strings.ToLower(strings.TrimSpace(servicePreset))
	proto := strings.ToUpper(strings.TrimSpace(protocol))
	isQUIC := proto == "QUIC"

	res := ServiceValidationResult{
		Protocol: proto,
		Probes:   make([]ProbeResult, 0),
	}

	switch preset {
	case "youtube":
		if isQUIC {
			r1 := ce.ProbeQUIC(ctx, "www.youtube.com:443")
			res.Probes = append(res.Probes, r1)
			if r1.Status != StatusPass {
				res.Status = ValidationStatusFailed
				res.Details = fmt.Sprintf("YouTube QUIC handshake failed: %s", r1.Error)
				return res
			}

			r2 := ce.ProbeHTTP3(ctx, "https://www.youtube.com/generate_204")
			res.Probes = append(res.Probes, r2)
			if r2.Status == StatusPass {
				res.Status = ValidationStatusVerified
				res.Details = "YouTube QUIC transport handshake and HTTP/3 application verified"
				return res
			}

			res.Status = ValidationStatusPartial
			res.Details = fmt.Sprintf("YouTube QUIC transport handshake verified, HTTP/3 application partial (%s)", r2.Error)
			return res
		}

		r1 := ce.ProbeHTTP(ctx, "https://www.youtube.com/generate_204", http.StatusNoContent, http.StatusOK)
		r2 := ce.ProbeHTTP(ctx, "https://i.ytimg.com/generate_204", http.StatusNoContent, http.StatusOK)
		res.Probes = append(res.Probes, r1, r2)
		if r1.Status == StatusPass && r2.Status == StatusPass {
			res.Status = ValidationStatusVerified
			res.Details = "YouTube Web and CDN video streaming endpoints verified"
		} else {
			res.Status = ValidationStatusFailed
			res.Details = "YouTube endpoint validation failed"
		}
		return res

	case "discord":
		if isQUIC {
			res.Status = ValidationStatusPartial
			res.Details = "QUIC transport bypass works for target. Discord Gateway & Web API operate over TCP/TLS and are not available over QUIC."
			return res
		}

		r1 := ce.ProbeHTTP(ctx, "https://discord.com/api/v10/gateway", http.StatusOK)
		r2, wsStatus := ce.ProbeDiscordGatewayWithTarget(ctx, "gateway.discord.gg:443")
		res.Probes = append(res.Probes, r1, r2)

		if r1.Status == StatusPass && wsStatus.WebSocketUpgradeVerified && wsStatus.Opcode10Verified {
			res.Status = ValidationStatusVerified
			res.Details = fmt.Sprintf("Discord Web API and Gateway WebSocket Verified (Opcode 10 Hello, Heartbeat: %dms)", wsStatus.HeartbeatInterval)
		} else if r1.Status == StatusPass && wsStatus.WebSocketUpgradeVerified {
			res.Status = ValidationStatusPartial
			res.Details = "Discord Web API and WS 101 upgrade OK, but Gateway Opcode 10 Hello unverified"
		} else {
			res.Status = ValidationStatusFailed
			res.Details = fmt.Sprintf("Discord Gateway verification failed: %s", r2.Error)
		}
		return res

	case "steam":
		if isQUIC {
			res.Status = ValidationStatusPartial
			res.Details = "QUIC transport bypass works for target. Steam Store and Web API operate over TCP/TLS and are not available over QUIC."
			return res
		}

		r1 := ce.ProbeHTTP(ctx, "https://store.steampowered.com/", http.StatusOK)
		r2 := ce.ProbeHTTP(ctx, "https://api.steampowered.com/ISteamWebAPIUtil/GetServerInfo/v1/", http.StatusOK)
		res.Probes = append(res.Probes, r1, r2)
		if r1.Status == StatusPass && r2.Status == StatusPass {
			res.Status = ValidationStatusVerified
			res.Details = "Steam Store and Web API endpoints verified"
		} else {
			res.Status = ValidationStatusFailed
			res.Details = "Steam store/API endpoint validation failed"
		}
		return res

	default: // Custom target validation
		target := targetHost
		if strings.TrimSpace(target) == "" {
			target = servicePreset
		}
		customProbe := ProbeTargetProtocol(ctx, ce, target, protocol)
		res.Probes = append(res.Probes, customProbe)
		if customProbe.Status == StatusPass {
			res.Status = ValidationStatusVerified
			res.Details = fmt.Sprintf("Custom target %s verified for %s", target, protocol)
		} else {
			res.Status = ValidationStatusFailed
			res.Details = fmt.Sprintf("Custom target %s verification failed: %s", target, customProbe.Error)
		}
		return res
	}
}

// GetServiceValidationEndpoints returns all hostnames requiring interception during service validation.
func GetServiceValidationEndpoints(servicePreset string) []string {
	switch strings.ToLower(strings.TrimSpace(servicePreset)) {
	case "youtube":
		return []string{"www.youtube.com", "i.ytimg.com"}
	case "discord":
		return []string{"discord.com", "gateway.discord.gg"}
	case "steam":
		return []string{"store.steampowered.com", "api.steampowered.com"}
	default:
		return nil
	}
}
