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
	ctx     context.Context
	manager *providers.ProviderManager
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

	registerOSProviders(a, assets)

	a.setupTray()

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

	a.StopEngine()
	// Sleep briefly before starting main engine to ensure ports are completely free
	time.Sleep(1 * time.Second)
	a.StartEngine("Zapret 2 (winws)", profile.Name)

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

	result, err := engine.ProbeConnection(testCtx, "https://discord.com", nil)
	
	if err != nil || !result.Success {
		return map[string]interface{}{
			"active":  true,
			"latency": 0,
			"status":  "error",
			"error":   result.Error,
		}
	}

	return map[string]interface{}{
		"active":    true,
		"latency":   result.Latency.Milliseconds(),
		"status":    "ok",
		"certValid": result.CertValid,
	}
}
