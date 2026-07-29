package engine

import (
	"context"
	"crypto/tls"
	"fmt"
	"runtime"
	"sync"
	"time"

	"unbound/engine/providers"
)

type AutoTuneResult struct {
	ProfileName       string
	Success           bool
	Score             int
	Latency           time.Duration
	Results           map[string]TargetStatus
	Baseline          map[string]TargetStatus
	RecoveredTargets  int
	RegressedTargets  int
	BaselineAvailable int
}

type TargetStatus struct {
	OK         bool
	Latency    time.Duration
	TLS13      bool
	CertValid  bool
	CertIssuer string
	Error      string
}

type Target struct {
	Name     string
	URL      string
	Priority int
}

// testTargets model the HTTPS services that the bundled profiles are meant to
// restore. UDP/QUIC and Discord voice are intentionally not claimed here: the
// current shared prober performs a verified IPv4 TLS handshake.
var testTargets = []Target{
	{Name: "YouTube", URL: "https://www.youtube.com", Priority: 30},
	{Name: "Discord", URL: "https://discord.com", Priority: 30},
	{Name: "Instagram", URL: "https://www.instagram.com", Priority: 20},
	{Name: "Twitter/X", URL: "https://twitter.com", Priority: 15},
	{Name: "Facebook", URL: "https://www.facebook.com", Priority: 15},
	{Name: "RuTracker", URL: "https://rutracker.org", Priority: 15},
	{Name: "NordVPN", URL: "https://nordvpn.com", Priority: 10},
	{Name: "Proton", URL: "https://proton.me", Priority: 10},
}

type AutoTuneProgressFn func(step, total int, profile string, okCount, totalTargets int, msg string)
type AutoTuneProbe func(context.Context, string) (ProbeResult, error)

type AutoTuneOptions struct {
	Targets            []Target
	Probe              AutoTuneProbe
	ProbeTimeout       time.Duration
	StabilizationDelay time.Duration
	CleanupDelay       time.Duration
	MinimumOK          int
}

func DefaultAutoTuneOptions() AutoTuneOptions {
	return AutoTuneOptions{
		Targets:            append([]Target(nil), testTargets...),
		Probe:              ProbeConnection,
		ProbeTimeout:       5 * time.Second,
		StabilizationDelay: 2 * time.Second,
		CleanupDelay:       1500 * time.Millisecond,
		MinimumOK:          2,
	}
}

// RegisterWindowsProfileCatalog installs the canonical winws2 strategies.
func RegisterWindowsProfileCatalog(registrar interface{ RegisterProfile(string, []string) }, luaDir string) []Profile {
	catalog := append(GetProfiles(luaDir), GetAdvancedProfiles(luaDir)...)
	profiles := make([]Profile, 0, len(catalog))
	registered := make(map[string]struct{}, len(catalog))
	for _, profile := range catalog {
		if _, exists := registered[profile.Name]; exists {
			continue
		}
		registrar.RegisterProfile(profile.Name, profile.Args)
		registered[profile.Name] = struct{}{}
		profiles = append(profiles, profile)
	}
	return profiles
}

// benchmark provider. Linux and macOS providers already own native catalogs,
// so importing Windows winws2 arguments into those backends would be invalid.
func PrepareAutoTuneProfiles(provider providers.BypassProvider, luaDir string) []Profile {
	if runtime.GOOS == "windows" {
		registrar, ok := provider.(interface{ RegisterProfile(string, []string) })
		if !ok {
			return nil
		}
		return RegisterWindowsProfileCatalog(registrar, luaDir)
	}

	names := provider.GetProfiles()
	profiles := make([]Profile, 0, len(names))
	for _, name := range names {
		profiles = append(profiles, Profile{Name: name})
	}
	return profiles
}

func RunAutoTuneV2WithContext(ctx context.Context, provider providers.BypassProvider, profiles []Profile) (*AutoTuneResult, error) {
	return RunAutoTuneV2WithProgress(ctx, provider, profiles, nil)
}

func RunAutoTuneV2WithProgress(ctx context.Context, provider providers.BypassProvider, profiles []Profile, progressFn AutoTuneProgressFn) (*AutoTuneResult, error) {
	return RunAutoTuneV3(ctx, provider, profiles, progressFn, DefaultAutoTuneOptions())
}

func RunAutoTuneV3(ctx context.Context, provider providers.BypassProvider, profiles []Profile, progressFn AutoTuneProgressFn, options AutoTuneOptions) (*AutoTuneResult, error) {
	if provider == nil {
		return nil, fmt.Errorf("bypass provider is not available")
	}
	if len(profiles) == 0 {
		return nil, fmt.Errorf("no profiles available for %s", provider.Name())
	}
	if len(options.Targets) == 0 {
		return nil, fmt.Errorf("no AutoTune targets configured")
	}
	if options.Probe == nil {
		options.Probe = ProbeConnection
	}
	if options.ProbeTimeout <= 0 {
		options.ProbeTimeout = 5 * time.Second
	}
	if options.MinimumOK <= 0 {
		options.MinimumOK = 1
	}
	if options.MinimumOK > len(options.Targets) {
		options.MinimumOK = len(options.Targets)
	}

	logger := GetLogger()
	notifMgr := GetNotificationManager()
	logger.Infof("AutoTune", "AutoTune V3: %d profiles, %d verified TLS targets", len(profiles), len(options.Targets))

	if err := provider.Stop(); err != nil {
		return nil, fmt.Errorf("establish clean baseline: %w", err)
	}
	if progressFn != nil {
		progressFn(0, len(profiles), "Baseline", 0, len(options.Targets), "Проверяем соединение без обхода...")
	}
	baseline := runAutoTuneProbes(ctx, options)
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	for _, target := range options.Targets {
		status := baseline[target.Name]
		if status.OK {
			logger.Infof("AutoTune", "[Baseline] %s: OK, %dms, issuer=%s", target.Name, status.Latency.Milliseconds(), status.CertIssuer)
		} else {
			logger.Warnf("AutoTune", "[Baseline] %s: FAIL, %s", target.Name, status.Error)
		}
	}
	baselineAvailable := countStatusesOK(baseline)
	logger.Infof("AutoTune", "Baseline: %d/%d targets available", baselineAvailable, len(options.Targets))

	var bestResult *AutoTuneResult
	for index, profile := range profiles {
		step := index + 1
		if err := ctx.Err(); err != nil {
			notifMgr.Warning("AutoTune", "Процесс отменён")
			return nil, err
		}
		if progressFn != nil {
			progressFn(step, len(profiles), profile.Name, 0, len(options.Targets), fmt.Sprintf("Тестируем [%d/%d]: %s...", step, len(profiles), profile.Name))
		}
		logger.Infof("AutoTune", "[%d/%d] Starting %s", step, len(profiles), profile.Name)

		if err := provider.Start(ctx, profile.Name); err != nil {
			logger.Warnf("AutoTune", "Profile %s failed to start: %v", profile.Name, err)
			if progressFn != nil {
				progressFn(step, len(profiles), profile.Name, 0, len(options.Targets), fmt.Sprintf("Ошибка запуска %s: %v", profile.Name, err))
			}
			continue
		}

		if err := waitAutoTune(ctx, options.StabilizationDelay); err != nil {
			_ = provider.Stop()
			return nil, err
		}
		statuses := runAutoTuneProbes(ctx, options)
		stopErr := provider.Stop()
		if err := waitAutoTune(ctx, options.CleanupDelay); err != nil {
			return nil, err
		}
		if stopErr != nil {
			logger.Warnf("AutoTune", "Profile %s did not stop cleanly: %v", profile.Name, stopErr)
			continue
		}
		if err := ctx.Err(); err != nil {
			return nil, err
		}

		for _, target := range options.Targets {
			status := statuses[target.Name]
			if status.OK {
				logger.Infof("AutoTune", "[%s] %s: OK, %dms, TLS1.3=%v", profile.Name, target.Name, status.Latency.Milliseconds(), status.TLS13)
			} else {
				logger.Warnf("AutoTune", "[%s] %s: FAIL, %s", profile.Name, target.Name, status.Error)
			}
		}

		result := scoreAutoTuneProfile(profile.Name, statuses, baseline, options.Targets, options.MinimumOK)
		okCount := countStatusesOK(statuses)
		logger.Infof("AutoTune", "%s: %d/%d OK, recovered=%d, regressed=%d, score=%d", profile.Name, okCount, len(options.Targets), result.RecoveredTargets, result.RegressedTargets, result.Score)
		if progressFn != nil {
			progressFn(step, len(profiles), profile.Name, okCount, len(options.Targets), fmt.Sprintf("%s: %d/%d OK, +%d восстановлено, -%d регрессий (счёт %d)", profile.Name, okCount, len(options.Targets), result.RecoveredTargets, result.RegressedTargets, result.Score))
		}
		if result.Success && betterAutoTuneResult(result, bestResult) {
			bestResult = result
		}
	}

	if bestResult == nil {
		logger.Error("AutoTune", "No profile improved connectivity without regressions")
		notifMgr.Error("AutoTune не удался", "Рабочий профиль не найден; подробности сохранены в журнале")
		return nil, fmt.Errorf("no profile improved connectivity without regressions")
	}

	bestResult.BaselineAvailable = baselineAvailable
	logger.Infof("AutoTune", "Winner: %s (score=%d, recovered=%d, latency=%dms)", bestResult.ProfileName, bestResult.Score, bestResult.RecoveredTargets, bestResult.Latency.Milliseconds())
	notifMgr.Success("AutoTune завершён", fmt.Sprintf("Лучший профиль: %s", bestResult.ProfileName))
	return bestResult, nil
}

func runAutoTuneProbes(ctx context.Context, options AutoTuneOptions) map[string]TargetStatus {
	statuses := make(map[string]TargetStatus, len(options.Targets))
	var mu sync.Mutex
	var wg sync.WaitGroup
	for _, target := range options.Targets {
		wg.Add(1)
		go func() {
			defer wg.Done()
			probeCtx, cancel := context.WithTimeout(ctx, options.ProbeTimeout)
			defer cancel()
			probeResult, err := options.Probe(probeCtx, target.URL)
			status := TargetStatus{
				OK:         err == nil && probeResult.Success && probeResult.CertValid,
				Latency:    probeResult.Latency,
				TLS13:      probeResult.TLSVersion == tls.VersionTLS13,
				CertValid:  probeResult.CertValid,
				CertIssuer: probeResult.CertIssuer,
				Error:      probeResult.Error,
			}
			if err != nil {
				status.Error = err.Error()
			}
			if status.Error == "" && !status.OK {
				status.Error = "TLS probe did not establish a verified connection"
			}
			mu.Lock()
			statuses[target.Name] = status
			mu.Unlock()
		}()
	}
	wg.Wait()
	return statuses
}

func scoreAutoTuneProfile(profileName string, statuses, baseline map[string]TargetStatus, targets []Target, minimumOK int) *AutoTuneResult {
	result := &AutoTuneResult{
		ProfileName: profileName,
		Results:     statuses,
		Baseline:    baseline,
	}
	okCount := 0
	totalLatency := time.Duration(0)
	baselineFailures := 0
	for _, target := range targets {
		status := statuses[target.Name]
		base := baseline[target.Name]
		if !base.OK {
			baselineFailures++
		}
		if status.OK {
			okCount++
			totalLatency += status.Latency
			result.Score += target.Priority
			if !base.OK {
				result.RecoveredTargets++
				result.Score += target.Priority * 2
			}
			if status.TLS13 {
				result.Score += 3
			}
			if status.Latency < 150*time.Millisecond {
				result.Score += 5
			}
		} else if base.OK {
			result.RegressedTargets++
			result.Score -= target.Priority * 3
		}
	}
	if okCount > 0 {
		result.Latency = totalLatency / time.Duration(okCount)
	}
	improvedWhenNeeded := baselineFailures == 0 || result.RecoveredTargets > 0
	result.Success = okCount >= minimumOK && result.RegressedTargets == 0 && improvedWhenNeeded
	return result
}

func betterAutoTuneResult(candidate, current *AutoTuneResult) bool {
	if current == nil || candidate.Score != current.Score {
		return current == nil || candidate.Score > current.Score
	}
	return candidate.Latency < current.Latency
}

func countStatusesOK(statuses map[string]TargetStatus) int {
	count := 0
	for _, status := range statuses {
		if status.OK {
			count++
		}
	}
	return count
}

func waitAutoTune(ctx context.Context, delay time.Duration) error {
	if delay <= 0 {
		return ctx.Err()
	}
	timer := time.NewTimer(delay)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}
