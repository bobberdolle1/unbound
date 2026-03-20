package main

import (
	"context"
	"fmt"
	"runtime"
	"time"

	"unbound/engine"
	"unbound/engine/providers"
	"unbound/engine/tester"

	"github.com/wailsapp/wails/v2/pkg/runtime"
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
		runtime.LogErrorf(ctx, "Failed to extract assets: %v", err)
		return
	}

	if err := providers.ValidateBinaries(assets.BinDir); err != nil {
		runtime.LogWarningf(ctx, "Binary validation warning: %v", err)
	}

	switch runtime.GOOS {
	case "windows":
		a.manager.Register(providers.NewGoodbyeDPIProvider(assets.BinDir))
		a.manager.Register(providers.NewZapret2WindowsProvider(assets.BinDir, assets.LuaDir))
	case "linux":
		a.manager.Register(providers.NewZapretLinuxProvider(assets.BinDir))
	case "darwin":
		a.manager.Register(providers.NewZapretMacOSProvider(assets.BinDir))
	}

	a.setupTray()

	runtime.LogInfo(ctx, "UNBOUND initialized successfully")
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
		runtime.EventsEmit(a.ctx, "status_changed", a.manager.GetStatus())
	}
	return err
}

func (a *App) StopEngine() error {
	err := a.manager.Stop()
	if err == nil {
		runtime.EventsEmit(a.ctx, "status_changed", a.manager.GetStatus())
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
		"os":   runtime.GOOS,
		"arch": runtime.GOARCH,
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
		runtime.LogInfof(a.ctx, "Testing profile: %s", profile)

		err := a.manager.Start(a.ctx, engineName, profile)
		if err != nil {
			runtime.LogErrorf(a.ctx, "Failed to start %s: %v", profile, err)
			continue
		}

		time.Sleep(2 * time.Second)

		testCtx, cancel := context.WithTimeout(a.ctx, 15*time.Second)
		results := tester.TestProfile(testCtx, tester.TestURLs, 10*time.Second)
		cancel()

		score := tester.CalculateScore(results)
		runtime.LogInfof(a.ctx, "Profile %s score: %d", profile, score)

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
