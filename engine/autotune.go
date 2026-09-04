package engine

import (
	"context"
	"crypto/tls"
	"fmt"
	"runtime"
	"strings"
	"sync"
	"time"

	"unbound/engine/providers"
)

type AutoTuneResult struct {
	ProfileName        string
	Success            bool
	Score              int
	Latency            time.Duration
	Results            map[string]TargetStatus
	Baseline           map[string]TargetStatus
	RecoveredTargets   int
	RegressedTargets   int
	BaselineAvailable  int
	Aggressiveness     int
	Explanation        string
	AlternativeProfile string
	FailedTargets      []string
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
	Category string // "youtube", "discord", "steam", "general"
}
// testTargets model the HTTPS services that the bundled profiles are meant to
// restore. UDP/QUIC and Discord voice are intentionally not claimed here: the
// current shared prober performs a verified IPv4 TLS handshake.
var testTargets = []Target{
	{Name: "YouTube", URL: "https://www.youtube.com", Priority: 30, Category: "youtube"},
	{Name: "Discord", URL: "https://discord.com", Priority: 30, Category: "discord"},
	{Name: "Steam", URL: "https://store.steampowered.com", Priority: 20, Category: "steam"},
	{Name: "Instagram", URL: "https://www.instagram.com", Priority: 20, Category: "general"},
	{Name: "Twitter/X", URL: "https://twitter.com", Priority: 15, Category: "general"},
	{Name: "Facebook", URL: "https://www.facebook.com", Priority: 15, Category: "general"},
	{Name: "RuTracker", URL: "https://rutracker.org", Priority: 15, Category: "general"},
	{Name: "NordVPN", URL: "https://nordvpn.com", Priority: 10, Category: "general"},
	{Name: "Proton", URL: "https://proton.me", Priority: 10, Category: "general"},
}

// FilterTargets returns the subset of targets matching selected categories.
func FilterTargets(categories []string) []Target {
	if len(categories) == 0 {
		return append([]Target(nil), testTargets...)
	}
	catMap := make(map[string]bool, len(categories))
	for _, c := range categories {
		catMap[strings.ToLower(strings.TrimSpace(c))] = true
	}
	var filtered []Target
	for _, t := range testTargets {
		if catMap[t.Category] {
			filtered = append(filtered, t)
		}
	}
	if len(filtered) == 0 {
		return append([]Target(nil), testTargets...)
	}
	return filtered
}

// ProfileAggressiveness ranks strategy aggressiveness (1 = least aggressive / most specific).
func ProfileAggressiveness(name string) int {
	n := strings.ToLower(name)
	switch {
	case strings.Contains(n, "hostfakesplit") || strings.Contains(n, "recommended"):
		return 1
	case strings.Contains(n, "alternative 1") || (strings.Contains(n, "multisplit") && !strings.Contains(n, "sni") && !strings.Contains(n, "fake")):
		return 2
	case strings.Contains(n, "alternative 2") || (strings.Contains(n, "fake tls") && !strings.Contains(n, "multisplit")):
		return 3
	case strings.Contains(n, "alternative 3") || strings.Contains(n, "multisplit sni"):
		return 4
	case strings.Contains(n, "universal") || strings.Contains(n, "all-in-one"):
		return 5
	case strings.Contains(n, "alternative 4"):
		return 6
	case strings.Contains(n, "game") || strings.Contains(n, "steam"):
		return 7
	default:
		return 8
	}
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
	AllowPartial       bool
}
func DefaultAutoTuneOptions() AutoTuneOptions {
	return AutoTuneOptions{
		Targets:            append([]Target(nil), testTargets...),
		Probe:              defaultAutoTuneProbe,
		ProbeTimeout:       5 * time.Second,
		StabilizationDelay: 2 * time.Second,
		CleanupDelay:       1500 * time.Millisecond,
		MinimumOK:          2,
	}
}

// RegisterWindowsProfileCatalog installs the canonical winws2 strategies.
// Every catalog profile passes through steamSafeArgs so Steam web and
// Valve-operated ranges stay out of the generic desync sections; opt-in
// game/Steam coverage ships as its own profile (GetGamesSteamProfile) whose
// args are used verbatim — its whole purpose is desyncing that traffic.
func RegisterWindowsProfileCatalog(registrar interface{ RegisterProfile(string, []string) }, luaDir string) []Profile {
	listsDir, _ := GetListsDir()
	catalog := append(GetProfiles(luaDir), GetAdvancedProfiles(luaDir)...)
	profiles := make([]Profile, 0, len(catalog)+1)
	registered := make(map[string]struct{}, len(catalog)+1)
	for _, profile := range catalog {
		if _, exists := registered[profile.Name]; exists {
			continue
		}
		args := steamSafeArgs(profile.Args, listsDir)
		registrar.RegisterProfile(profile.Name, args)
		registered[profile.Name] = struct{}{}
		profiles = append(profiles, Profile{Name: profile.Name, Args: args})
	}
	for _, profile := range GetGamesSteamProfiles() {
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
		options.Probe = defaultAutoTuneProbe
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
	var runnerUp *AutoTuneResult
	var bestPartial *AutoTuneResult
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
		if result.Success {
			if bestResult == nil || betterAutoTuneResult(result, bestResult) {
				runnerUp = bestResult
				bestResult = result
			} else if runnerUp == nil || betterAutoTuneResult(result, runnerUp) {
				runnerUp = result
			}
		} else if result.RegressedTargets == 0 {
			if bestPartial == nil || result.Score > bestPartial.Score {
				bestPartial = result
			}
		}
	}

	if bestResult == nil {
		if options.AllowPartial && bestPartial != nil {
			bestPartial.BaselineAvailable = baselineAvailable
			bestPartial.Explanation = fmt.Sprintf("Ни один профиль не восстановил все проверки без регрессий. Лучший частичный профиль: «%s» (%d/%d доступно)",
				bestPartial.ProfileName, countStatusesOK(bestPartial.Results), len(options.Targets))
			logger.Warnf("AutoTune", "No full winner; best partial: %s (score=%d)", bestPartial.ProfileName, bestPartial.Score)
			return bestPartial, nil
		}
		logger.Error("AutoTune", "No profile improved connectivity without regressions")
		notifMgr.Error("AutoTune не удался", "Рабочий профиль не найден; подробности сохранены в журнале")
		return nil, fmt.Errorf("no profile improved connectivity without regressions")
	}
	if runnerUp != nil && runnerUp.ProfileName != bestResult.ProfileName {
		bestResult.AlternativeProfile = runnerUp.ProfileName
	}
	bestResult.Explanation = fmt.Sprintf("Все проверки пройдены (%d/%d). Выбран профиль с наименьшей агрессивностью.",
		countStatusesOK(bestResult.Results), len(options.Targets))

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
		} else {
			result.FailedTargets = append(result.FailedTargets, target.Name)
			if base.OK {
				result.RegressedTargets++
				result.Score -= target.Priority * 3
			}
		}
	}
	result.Aggressiveness = ProfileAggressiveness(profileName)
	if okCount > 0 {
		result.Latency = totalLatency / time.Duration(okCount)
	}
	improvedWhenNeeded := baselineFailures == 0 || result.RecoveredTargets > 0
	result.Success = okCount >= minimumOK && result.RegressedTargets == 0 && improvedWhenNeeded
	return result
}

func betterAutoTuneResult(candidate, current *AutoTuneResult) bool {
	if current == nil {
		return true
	}
	if candidate.Score != current.Score {
		return candidate.Score > current.Score
	}
	// Tie breaker 1: prefer LESS aggressive profile
	if candidate.Aggressiveness != current.Aggressiveness {
		return candidate.Aggressiveness < current.Aggressiveness
	}
	// Tie breaker 2: lower latency
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

func defaultAutoTuneProbe(ctx context.Context, targetURL string) (ProbeResult, error) {
	ce := NewConnectivityEngine(DefaultProbeTimeout)
	res := ce.ExecuteWithRetry(ctx, 2, func(probeCtx context.Context) ProbeResult {
		return ce.ProbeTLS(probeCtx, targetURL)
	})
	return res, nil
}
