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
		return fmt.Errorf("administrator/root privileges required")
	}

	err = a.manager.Start(a.ctx, engineName, profileName)
	if err == nil {
		wailsruntime.EventsEmit(a.ctx, "status_changed", a.manager.GetStatus())
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
