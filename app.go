package main

import (
	"context"
	"fmt"
	"runtime"

	"unbound/engine"
	"unbound/engine/providers"

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

	switch runtime.GOOS {
	case "windows":
		a.manager.Register(providers.NewGoodbyeDPIProvider(assets.BinDir))
		a.manager.Register(providers.NewZapret2WindowsProvider(assets.BinDir, assets.LuaDir))
	case "linux":
		a.manager.Register(providers.NewZapretLinuxProvider(assets.BinDir))
	case "darwin":
		a.manager.Register(providers.NewZapretMacOSProvider(assets.BinDir))
	}

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
