package engine

import (
	"context"
	"crypto/tls"
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
	TargetHost           string   `json:"targetHost"`
	ServicePreset        string   `json:"servicePreset"` // "YouTube", "Discord", "Steam", "Custom"
	Protocol             string   `json:"protocol"`      // "TLS1.3", "TLS1.2", "HTTP", "QUIC", "ANY"
	CustomCandidateArgs  []string `json:"customCandidateArgs,omitempty"`
	ForceProbeIfReachable bool    `json:"forceProbeIfReachable"`
}

// CandidateTestResult represents the evaluation result of a single strategy candidate.
type CandidateTestResult struct {
	Candidate     StrategyCandidate `json:"candidate"`
	Status        ProbeStatus       `json:"status"` // PASS, FAIL, WARNING, NOT_VERIFIED
	PassCount     int               `json:"passCount"`
	TotalAttempts int               `json:"totalAttempts"`
	AvgLatency    time.Duration     `json:"avgLatency"`
	Score         int               `json:"score"`
	Details       string            `json:"details"`
	Error         string            `json:"error,omitempty"`
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
	RunID             string                `json:"runId"`
	TargetHost        string                `json:"targetHost"`
	TargetIPs         []string              `json:"targetIps"`
	Protocol          string                `json:"protocol"`
	BaselineReachable bool                  `json:"baselineReachable"`
	BaselineStatus    ProbeResult           `json:"baselineStatus"`
	TotalCandidates   int                   `json:"totalCandidates"`
	TestedCandidates  int                   `json:"testedCandidates"`
	WorkingCandidates []CandidateTestResult `json:"workingCandidates"`
	BestCandidate     *CandidateTestResult  `json:"bestCandidate,omitempty"`
	ValidationStatus  ValidationStatus      `json:"validationStatus"`
	ServiceVerified   bool                  `json:"serviceVerified"` // Kept for UI backwards compatibility (true if VERIFIED)
	Duration          time.Duration         `json:"duration"`
	Timestamp         time.Time             `json:"timestamp"`
}
// StrategyLabProgress streams live execution progress to the frontend.
type StrategyLabProgress struct {
	RunID                string `json:"runId"`
	TargetHost           string `json:"targetHost"`
	CurrentCandidateIndex int    `json:"currentCandidateIndex"`
	TotalCandidates      int    `json:"totalCandidates"`
	CurrentCandidateName string `json:"currentCandidateName"`
	BaselineStatus       string `json:"baselineStatus"`
	Percent              int    `json:"percent"`
	ElapsedMs            int64  `json:"elapsedMs"`
	LastResult           string `json:"lastResult"`
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

	// Defer guarantees that previous profile is restored and coordinator is released
	defer func() {
		cleanupCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		if wasRunning && pc != nil && initialProfile != "" {
			if pc.GetStatus() != providers.StatusRunning || pc.CurrentProfile() != initialProfile {
				logger.Infof("Lab", "[LAB] restoring previous active profile %s", initialProfile)
				_ = pc.Start(cleanupCtx, initialProfile)
			}
		} else if !wasRunning && pc != nil {
			if pc.GetStatus() == providers.StatusRunning {
				_ = pc.Stop()
			}
		}
		release()
	}()

	ce := NewConnectivityEngine(3 * time.Second)

	// 2. Step 1: Baseline measurement (without bypass)
	if wasRunning && pc != nil {
		logger.Info("Lab", "[LAB] stopping active engine for baseline measurement")
		_ = pc.Stop()
		time.Sleep(300 * time.Millisecond)
	}

	targetURL := cfg.TargetHost
	if !strings.HasPrefix(targetURL, "http://") && !strings.HasPrefix(targetURL, "https://") {
		if strings.EqualFold(cfg.Protocol, "HTTP") {
			targetURL = "http://" + targetURL
		} else {
			targetURL = "https://" + targetURL
		}
	}

	baseResult := ce.ProbeHTTP(ctx, targetURL, http.StatusOK, http.StatusNoContent, http.StatusMovedPermanently, http.StatusFound)
	baselineReachable := baseResult.Status == StatusPass
	logger.Infof("Lab", "[LAB] baseline check for %s: %s (latency=%v)", targetURL, baseResult.Status, baseResult.Latency)

	report := &StrategyLabReport{
		RunID:             runID,
		TargetHost:        cfg.TargetHost,
		Protocol:          cfg.Protocol,
		BaselineReachable: baselineReachable,
		BaselineStatus:    baseResult,
		Timestamp:         startTime,
		WorkingCandidates: make([]CandidateTestResult, 0),
	}

	if baselineReachable && !cfg.ForceProbeIfReachable {
		logger.Info("Lab", "[LAB] target is already reachable without bypass, discovery completed early")
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
	for idx, cand := range candidates {
		if ctx.Err() != nil {
			logger.Warnf("Lab", "[LAB] strategy discovery cancelled by user: %v", ctx.Err())
			return nil, ctx.Err()
		}

		pct := (idx * 100) / len(candidates)
		if onProgress != nil {
			onProgress(StrategyLabProgress{
				RunID:                runID,
				TargetHost:           cfg.TargetHost,
				CurrentCandidateIndex: idx + 1,
				TotalCandidates:      len(candidates),
				CurrentCandidateName: cand.Name,
				BaselineStatus:       string(baseResult.Status),
				Percent:              pct,
				ElapsedMs:            time.Since(startTime).Milliseconds(),
			})
		}

		logger.Infof("Lab", "[LAB] testing candidate [%d/%d]: %s", idx+1, len(candidates), cand.Name)

		// 1. Launch temporary isolated winws2 process with strict raw filter
		proc, err := runner.StartCandidate(ctx, cand, rawFilter)
		if err != nil {
			logger.Warnf("Lab", "[LAB] candidate %s failed to start: %v", cand.Name, err)
			report.TestedCandidates++
			report.WorkingCandidates = append(report.WorkingCandidates, CandidateTestResult{
				Candidate:     cand,
				Status:        StatusFail,
				TotalAttempts: 3,
				Error:         fmt.Sprintf("START_FAILED: %v", err),
			})
			continue
		}

		// 2. Perform protocol-specific probes while candidate process is active
		res := testCandidateWithProcess(ctx, ce, cand, targetURL, cfg.Protocol, baselineReachable)
		report.TestedCandidates++

		// 3. Stop candidate process and release WinDivert handles
		_ = proc.Stop()

		if res.Status == StatusPass {
			logger.Infof("Lab", "[LAB] candidate %s PASSED (%d/%d, avg latency=%v)",
				cand.Name, res.PassCount, res.TotalAttempts, res.AvgLatency)
			report.WorkingCandidates = append(report.WorkingCandidates, res)
		} else {
			logger.Debugf("Lab", "[LAB] candidate %s failed (%d/%d)",
				cand.Name, res.PassCount, res.TotalAttempts)
		}
	}

	// 6. Step 5: Rank Working Candidates
	//           2. Aggressiveness (Lower is better)
	//           3. AvgLatency (Lower is better)
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
		report.BestCandidate = &best
		logger.Infof("Lab", "[LAB] best discovered candidate: %s (score=%d, aggressiveness=%s)",
			best.Candidate.Name, best.Score, best.Candidate.Aggressiveness)

		// 7. Step 6: Full Service Validation on Winner WHILE WINNER PROCESS IS ACTIVE!
		if cfg.ServicePreset != "" && !strings.EqualFold(cfg.ServicePreset, "Custom") {
			logger.Infof("Lab", "[LAB] preparing combined service validation filter for %s", cfg.ServicePreset)
			endpoints := GetServiceValidationEndpoints(cfg.ServicePreset)
			var combinedIPs []net.IP
			endpointsResolved := true
			for _, ep := range endpoints {
				epIPs, err := net.DefaultResolver.LookupIP(ctx, "ip", ep)
				if err != nil || len(epIPs) == 0 {
					logger.Warnf("Lab", "[LAB] fail-closed: failed to resolve service endpoint %s: %v", ep, err)
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
				report.ServiceVerified = false
			} else {
				serviceRawFilter := rawFilter
				if len(combinedIPs) > 0 {
					if f, err := GenerateWinDivertFilterForIPs(combinedIPs, ports, protoStr); err == nil {
						serviceRawFilter = f
					}
				}

				logger.Infof("Lab", "[LAB] launching winner %s for service validation (%s) with filter: %s",
					best.Candidate.Name, cfg.ServicePreset, serviceRawFilter)
				ce.ResetConnectionPool()
				winnerProc, err := runner.StartCandidate(ctx, best.Candidate, serviceRawFilter)
				if err == nil {
					verified := validateCandidateAgainstService(ctx, ce, cfg.ServicePreset, cfg.Protocol)
					if verified {
						report.ValidationStatus = ValidationStatusVerified
						report.ServiceVerified = true
					} else {
						report.ValidationStatus = ValidationStatusFailed
						report.ServiceVerified = false
					}
					_ = winnerProc.Stop()
				} else {
					logger.Warnf("Lab", "[LAB] failed to start winner for service validation: %v", err)
					report.ValidationStatus = ValidationStatusFailed
					report.ServiceVerified = false
				}
				ce.ResetConnectionPool()
			}
		} else {
			// Custom target validation
			if strings.EqualFold(cfg.Protocol, "QUIC") {
				quicRes := ce.ProbeQUIC(ctx, cfg.TargetHost)
				if quicRes.Status == StatusPass {
					report.ValidationStatus = ValidationStatusVerified
					report.ServiceVerified = true
				} else {
					report.ValidationStatus = ValidationStatusFailed
					report.ServiceVerified = false
				}
			} else {
				report.ValidationStatus = ValidationStatusVerified
				report.ServiceVerified = true
			}
		}
	}
	report.Duration = time.Since(startTime)
	logger.Infof("Lab", "[LAB] discovery completed in %v: %d working candidates found",
		report.Duration, len(report.WorkingCandidates))

	return report, nil
}

func testCandidateWithProcess(
	ctx context.Context,
	ce *ConnectivityEngine,
	cand StrategyCandidate,
	targetURL string,
	protocol string,
	baselineReachable bool,
) CandidateTestResult {
	const attempts = 3
	passCount := 0
	var totalLatency time.Duration
	var lastErr string

	for range attempts {
		if ctx.Err() != nil {
			break
		}
		ce.ResetConnectionPool()
		var probeRes ProbeResult
		switch strings.ToUpper(strings.TrimSpace(protocol)) {
		case "TLS1.2":
			probeRes = ce.ProbeTLSVersion(ctx, targetURL, tls.VersionTLS12)
		case "TLS1.3":
			probeRes = ce.ProbeTLSVersion(ctx, targetURL, tls.VersionTLS13)
		case "HTTP":
			probeRes = ce.ProbeHTTP(ctx, targetURL, http.StatusOK, http.StatusNoContent, http.StatusMovedPermanently, http.StatusFound)
		case "QUIC":
			cleanHost := extractHost(targetURL)
			if cleanHost == "" {
				cleanHost = targetURL
			}
			probeRes = ce.ProbeQUIC(ctx, cleanHost+":443")
		default:
			probeRes = ce.ProbeHTTP(ctx, targetURL, http.StatusOK, http.StatusNoContent, http.StatusMovedPermanently, http.StatusFound)
		}
		ce.ResetConnectionPool()
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
	if passCount >= 2 {
		status = StatusPass
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
		Candidate:     cand,
		Status:        status,
		PassCount:     passCount,
		TotalAttempts: attempts,
		AvgLatency:    avgLat,
		Score:         score,
		Details:       details,
		Error:         lastErr,
	}
}
func validateCandidateAgainstService(ctx context.Context, ce *ConnectivityEngine, servicePreset, protocol string) bool {
	preset := strings.ToLower(strings.TrimSpace(servicePreset))
	isQUIC := strings.EqualFold(strings.TrimSpace(protocol), "QUIC")

	switch preset {
	case "youtube":
		if isQUIC {
			r1 := ce.ProbeQUIC(ctx, "www.youtube.com:443")
			r2 := ce.ProbeHTTP(ctx, "https://www.youtube.com/generate_204", http.StatusNoContent, http.StatusOK)
			return r1.Status == StatusPass && r2.Status == StatusPass
		}
		r1 := ce.ProbeHTTP(ctx, "https://www.youtube.com/generate_204", http.StatusNoContent, http.StatusOK)
		r2 := ce.ProbeHTTP(ctx, "https://i.ytimg.com/generate_204", http.StatusNoContent, http.StatusOK)
		return r1.Status == StatusPass && r2.Status == StatusPass
	case "discord":
		if isQUIC {
			return false // Discord gateway/API does not use QUIC on 443
		}
		r1 := ce.ProbeHTTP(ctx, "https://discord.com/api/v10/gateway", http.StatusOK)
		r2 := ce.ProbeDiscordGateway(ctx)
		return r1.Status == StatusPass && r2.Status == StatusPass
	case "steam":
		if isQUIC {
			return false // Steam store/API does not use QUIC on 443
		}
		r1 := ce.ProbeHTTP(ctx, "https://store.steampowered.com/", http.StatusOK)
		r2 := ce.ProbeHTTP(ctx, "https://api.steampowered.com/ISteamWebAPIUtil/GetServerInfo/v1/", http.StatusOK)
		return r1.Status == StatusPass && r2.Status == StatusPass
	default:
		return true
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

