package main

import (
	"context"
	"fmt"
	stdruntime "runtime"
	"time"

	"unbound/engine"
	"unbound/engine/providers"
	"unbound/engine/tester"

	wailsruntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

type App struct {
	ctx            context.Context
	manager        *providers.ProviderManager
	startMinimized bool
	debugMode      bool
}

func NewApp() *App {
	return &App{
		manager: providers.NewProviderManager(),
	}
}

func (a *App) startup(ctx context.Context) {
	a.ctx = ctx

	assets, err := engine.ExtractAssets()
	if err != nil {
		wailsruntime.LogErrorf(ctx, "Failed to extract assets: %v", err)
		return
	}

	if err := providers.ValidateBinaries(assets.BinDir); err != nil {
		wailsruntime.LogWarningf(ctx, "Binary validation warning: %v", err)
	}

	// Ensure dynamic lists exist
	if err := engine.EnsureListsExist(); err != nil {
		wailsruntime.LogWarningf(ctx, "Failed to ensure lists exist: %v", err)
	}

	registerOSProviders(a, assets)

	a.setupTray()

	if a.startMinimized {
		wailsruntime.WindowMinimise(ctx)
	}

	if a.debugMode {
		wailsruntime.LogInfo(ctx, "Debug mode enabled - verbose logging active")
	}

	wailsruntime.LogInfo(ctx, "UNBOUND initialized successfully")
}

func (a *App) shutdown(ctx context.Context) {
	a.manager.Stop()
}

func (a *App) GetEngineNames() []string {
	return a.manager.GetEngineNames()
}

func (a *App) GetProfiles(engineName string) []string {
	return a.manager.GetProfiles(engineName)
}

func (a *App) StartEngine(engineName string, profileName string) error {
	hasPriv, err := a.manager.CheckPrivileges()
	if err != nil {
		return err
	}
	if !hasPriv {
		wailsruntime.LogError(a.ctx, "Administrator privileges required")
		wailsruntime.EventsEmit(a.ctx, "privilege_error", "Administrator privileges required. Please restart the application as administrator.")
		return fmt.Errorf("administrator privileges required")
	}

	currentStatus := a.manager.GetStatus()
	if currentStatus == providers.StatusRunning {
		wailsruntime.LogInfo(a.ctx, "Stopping current engine before starting new profile")
		if err := a.manager.Stop(); err != nil {
			wailsruntime.LogErrorf(a.ctx, "Failed to stop current engine: %v", err)
		}
		time.Sleep(1 * time.Second)
	}

	err = a.manager.Start(a.ctx, engineName, profileName)
	if err == nil {
		wailsruntime.EventsEmit(a.ctx, "status_changed", a.manager.GetStatus())
		wailsruntime.LogInfof(a.ctx, "Engine started with profile: %s", profileName)
	} else {
		wailsruntime.LogErrorf(a.ctx, "Failed to start engine: %v", err)
	}
	return err
}

func (a *App) StopEngine() error {
	err := a.manager.Stop()
	if err == nil {
		wailsruntime.EventsEmit(a.ctx, "status_changed", a.manager.GetStatus())
	}
	return err
}

func (a *App) GetStatus() string {
	return string(a.manager.GetStatus())
}

func (a *App) GetLogs() []string {
	return a.manager.GetLogs()
}

func (a *App) GetSystemInfo() map[string]string {
	return map[string]string{
		"os":   stdruntime.GOOS,
		"arch": stdruntime.GOARCH,
	}
}

func (a *App) TestProfile(engineName string, profileName string) (string, error) {
	hasPriv, err := a.manager.CheckPrivileges()
	if err != nil {
		return "", err
	}
	if !hasPriv {
		return "", fmt.Errorf("administrator/root privileges required")
	}

	err = a.manager.Start(a.ctx, engineName, profileName)
	if err != nil {
		return "", err
	}

	time.Sleep(2 * time.Second)

	testCtx, cancel := context.WithTimeout(a.ctx, 15*time.Second)
	defer cancel()

	results := tester.TestProfile(testCtx, tester.TestURLs, 10*time.Second)
	score := tester.CalculateScore(results)
	output := tester.FormatResults(results)

	a.manager.Stop()

	return fmt.Sprintf("Score: %d/600\n\n%s", score, output), nil
}

func (a *App) AutoTune() string {
	wailsruntime.LogInfo(a.ctx, "Starting Auto-Tune...")
	
	updateLog := func(msg string) {
		wailsruntime.EventsEmit(a.ctx, "autotune_log", msg)
	}

	profile, err := engine.RunAutoTune(a.ctx, updateLog)
	if err != nil {
		updateLog("Auto-Tune failed: " + err.Error())
		return "Failed"
	}

	updateLog(fmt.Sprintf("Starting engine with profile: %s", profile.Name))
	time.Sleep(1 * time.Second)
	
	if err := a.StartEngine("Zapret 2 (winws)", profile.Name); err != nil {
		updateLog("Failed to start engine: " + err.Error())
		return "Failed"
	}

	return profile.Name
}

func (a *App) AutoSelectProfile(engineName string) (string, error) {
	hasPriv, err := a.manager.CheckPrivileges()
	if err != nil {
		return "", err
	}
	if !hasPriv {
		return "", fmt.Errorf("administrator/root privileges required")
	}

	profiles := a.manager.GetProfiles(engineName)
	if len(profiles) == 0 {
		return "", fmt.Errorf("no profiles available for engine: %s", engineName)
	}

	bestProfile := ""
	bestScore := -1

	for _, profile := range profiles {
		wailsruntime.LogInfof(a.ctx, "Testing profile: %s", profile)

		err := a.manager.Start(a.ctx, engineName, profile)
		if err != nil {
			wailsruntime.LogErrorf(a.ctx, "Failed to start %s: %v", profile, err)
			continue
		}

		time.Sleep(2 * time.Second)

		testCtx, cancel := context.WithTimeout(a.ctx, 15*time.Second)
		results := tester.TestProfile(testCtx, tester.TestURLs, 10*time.Second)
		cancel()

		score := tester.CalculateScore(results)
		wailsruntime.LogInfof(a.ctx, "Profile %s score: %d", profile, score)

		a.manager.Stop()
		time.Sleep(1 * time.Second)

		if score > bestScore {
			bestScore = score
			bestProfile = profile
		}
	}

	if bestProfile == "" {
		return "", fmt.Errorf("no working profile found")
	}

	return bestProfile, nil
}

func (a *App) SaveCustomScript(scriptContent string) error {
	if err := engine.SaveCustomScript(scriptContent); err != nil {
		wailsruntime.LogErrorf(a.ctx, "Failed to save custom script: %v", err)
		return err
	}
	wailsruntime.LogInfo(a.ctx, "Custom Lua script saved successfully")
	return nil
}

func (a *App) LoadCustomScript() (string, error) {
	content, err := engine.LoadCustomScript()
	if err != nil {
		wailsruntime.LogWarningf(a.ctx, "Custom script load warning: %v", err)
	}
	return content, nil
}

func (a *App) GetCurrentPing() map[string]interface{} {
	status := a.manager.GetStatus()
	
	if status != providers.StatusRunning {
		return map[string]interface{}{
			"active":  false,
			"latency": 0,
			"status":  "stopped",
		}
	}

	testCtx, cancel := context.WithTimeout(a.ctx, 5*time.Second)
	defer cancel()

	latency, err := engine.SimplePing(testCtx, "https://discord.com")
	
	if err != nil {
		return map[string]interface{}{
			"active":  true,
			"latency": 0,
			"status":  "error",
			"error":   err.Error(),
		}
	}

	return map[string]interface{}{
		"active":  true,
		"latency": latency.Milliseconds(),
		"status":  "ok",
	}
}

func (a *App) GetSettings() (*engine.Settings, error) {
	settings, err := engine.GetSettings()
	if err != nil {
		wailsruntime.LogWarningf(a.ctx, "Failed to load settings: %v", err)
		return settings, err
	}
	return settings, nil
}

func (a *App) SaveSettings(settings engine.Settings) error {
	if err := engine.SaveSettings(&settings); err != nil {
		wailsruntime.LogErrorf(a.ctx, "Failed to save settings: %v", err)
		return err
	}
	wailsruntime.LogInfo(a.ctx, "Settings saved successfully")
	return nil
}

func (a *App) UpdateLists() error {
	if err := engine.UpdateLists(); err != nil {
		wailsruntime.LogErrorf(a.ctx, "Failed to update lists: %v", err)
		return err
	}
	wailsruntime.LogInfo(a.ctx, "Lists updated successfully")
	return nil
}

func (a *App) GetLivePing() map[string]interface{} {
	status := a.manager.GetStatus()
	
	if status != providers.StatusRunning {
		return map[string]interface{}{
			"active":  false,
			"latency": 0,
			"status":  "stopped",
		}
	}

	testCtx, cancel := context.WithTimeout(a.ctx, 3*time.Second)
	defer cancel()

	latency, err := engine.SimplePing(testCtx, "https://discord.com")
	
	if err != nil {
		return map[string]interface{}{
			"active":  true,
			"latency": 0,
			"status":  "blocked",
			"error":   err.Error(),
		}
	}

	return map[string]interface{}{
		"active":  true,
		"latency": latency.Milliseconds(),
		"status":  "ok",
	}
}

func (a *App) CheckForUpdates(currentVersion string) (engine.UpdateInfo, error) {
	updateInfo, err := engine.CheckForUpdates(currentVersion)
	if err != nil {
		wailsruntime.LogWarningf(a.ctx, "Update check failed: %v", err)
		return updateInfo, err
	}
	
	if updateInfo.Available {
		wailsruntime.LogInfof(a.ctx, "Update available: %s", updateInfo.Version)
	} else {
		wailsruntime.LogInfo(a.ctx, "No updates available")
	}
	
	return updateInfo, nil
}

func (a *App) RunDiagnostics() engine.DiagnosticsReport {
	wailsruntime.LogInfo(a.ctx, "Running system diagnostics...")
	report := engine.RunDiagnostics()
	wailsruntime.LogInfof(a.ctx, "Diagnostics complete: %s", report.Summary)
	return report
}

func (a *App) ClearDiscordCache() error {
	if err := engine.ClearDiscordCache(); err != nil {
		wailsruntime.LogErrorf(a.ctx, "Failed to clear Discord cache: %v", err)
		return err
	}
	wailsruntime.LogInfo(a.ctx, "Discord cache cleared successfully")
	return nil
}

func (a *App) RunAdvancedTests(mode string, maxConcurrent int) string {
	hasPriv, err := a.manager.CheckPrivileges()
	if err != nil || !hasPriv {
		return "Administrator privileges required"
	}

	profiles := a.manager.GetProfiles("Zapret 2 (winws)")
	if len(profiles) == 0 {
		return "No profiles available"
	}

	testMode := tester.TestModeStandard
	if mode == "dpi_checker" {
		testMode = tester.TestModeDPIChecker
	}

	config := tester.AdvancedTestConfig{
		Mode:            testMode,
		MaxConcurrent:   maxConcurrent,
		Timeout:         15 * time.Second,
		URLs:            tester.TestURLs,
		CheckTCPFreeze:  true,
		MinDownloadSize: 16 * 1024,
	}

	wailsruntime.LogInfo(a.ctx, "Starting advanced profile tests...")

	startProfile := func(profile string) error {
		return a.manager.Start(a.ctx, "Zapret 2 (winws)", profile)
	}

	stopProfile := func() error {
		return a.manager.Stop()
	}

	var results []tester.AdvancedTestResult
	if maxConcurrent > 1 {
		results = tester.RunParallelTests(a.ctx, profiles, config, startProfile, stopProfile)
	} else {
		results = tester.RunAdvancedTests(a.ctx, profiles, config, startProfile, stopProfile)
	}

	sessionID := fmt.Sprintf("%d", time.Now().Unix())
	for _, result := range results {
		persistentResults := make([]engine.TestResultPersistent, len(result.Results))
		for i, r := range result.Results {
			persistentResults[i] = engine.TestResultPersistent{
				URL:        r.URL,
				Success:    r.Success,
				Latency:    r.Latency,
				Error:      r.Error,
				StatusCode: r.StatusCode,
				TCPFreeze:  result.TCPFreezeDetected,
			}
		}
		
		session := &engine.TestSession{
			ID:          sessionID,
			StartTime:   time.Now().Add(-time.Duration(len(results)) * time.Second),
			EndTime:     time.Now(),
			Duration:    time.Duration(len(results)) * time.Second,
			ProfileName: result.ProfileName,
			TestMode:    string(result.Mode),
			Results:     persistentResults,
			Score:       result.Score,
			SuccessRate: result.SuccessRate,
		}
		
		engine.SaveTestSession(session)
	}

	output := tester.FormatAdvancedResults(results)
	
	best := tester.FindBestProfile(results)
	if best != nil {
		wailsruntime.LogInfof(a.ctx, "Best profile: %s (Score: %d)", best.ProfileName, best.Score)
		
		analytics, _ := engine.GenerateTestAnalytics()
		if analytics != nil {
			engine.SaveTestAnalytics(analytics)
		}
	}

	return output
}

func (a *App) GetTestAnalytics() (*engine.TestAnalytics, error) {
	analytics, err := engine.LoadTestAnalytics()
	if err != nil {
		wailsruntime.LogWarningf(a.ctx, "Failed to load analytics: %v", err)
		return nil, err
	}
	return analytics, nil
}

func (a *App) GetTestHistory() ([]*engine.TestSession, error) {
	sessions, err := engine.LoadAllTestSessions()
	if err != nil {
		wailsruntime.LogWarningf(a.ctx, "Failed to load test history: %v", err)
		return nil, err
	}
	return sessions, nil
}

func (a *App) CleanOldTests(daysOld int) error {
	duration := time.Duration(daysOld) * 24 * time.Hour
	if err := engine.CleanOldTestResults(duration); err != nil {
		wailsruntime.LogErrorf(a.ctx, "Failed to clean old tests: %v", err)
		return err
	}
	wailsruntime.LogInfo(a.ctx, "Old test results cleaned successfully")
	return nil
}

func (a *App) GetProfileCategories() []string {
	return engine.GetProfileCategories()
}

func (a *App) GetProfilesByCategory(category string) []engine.AdvancedProfile {
	return engine.GetProfilesByCategory(category)
}

func (a *App) GetBlobList() []engine.BlobPayload {
	return engine.ListBlobs()
}

func (a *App) GenerateCustomBlob(blobType string, sni string) (string, error) {
	switch blobType {
	case "tls_random":
		data := engine.GenerateRandomTLSClientHello(sni)
		return string(data), nil
	case "quic_random":
		data := engine.GenerateRandomQUICInitial()
		return string(data), nil
	default:
		return "", fmt.Errorf("unknown blob type: %s", blobType)
	}
}
