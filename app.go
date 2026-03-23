package main

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
	"time"
	"syscall"

	"unbound/engine"
	"unbound/engine/providers"

	wailsruntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

const (
	CREATE_NO_WINDOW = 0x08000000
)

type App struct {
	ctx            context.Context
	manager        *providers.ProviderManager
	startMinimized bool
	debugMode      bool
	autoTuneCancel context.CancelFunc
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

	// Apply system settings
	settings, _ := engine.GetSettings()
	if settings != nil {
		if settings.EnableTCPTimestamps {
			a.EnableTCPTimestamps()
		}
		if settings.DiscordCacheAutoClean {
			a.ClearDiscordCache()
		}
	}

	registerOSProviders(a, assets)
	a.setupTray()

	if a.startMinimized {
		wailsruntime.WindowMinimise(ctx)
	}
	wailsruntime.LogInfo(ctx, "UNBOUND initialized")
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
	wailsruntime.LogInfof(a.ctx, "StartEngine called: engine=%s, profile=%s", engineName, profileName)
	
	hasPriv, err := checkAdminPrivileges()
	if err != nil {
		wailsruntime.LogErrorf(a.ctx, "Privilege check error: %v", err)
		return err
	}
	if !hasPriv {
		wailsruntime.LogError(a.ctx, "Administrator privileges required")
		wailsruntime.EventsEmit(a.ctx, "privilege_error", "Administrator privileges required. Please restart the application as administrator.")
		return fmt.Errorf("administrator privileges required")
	}

	a.manager.Stop()
	time.Sleep(500 * time.Millisecond)

	err = a.manager.Start(a.ctx, engineName, profileName)
	if err == nil {
		wailsruntime.EventsEmit(a.ctx, "status_changed", "Running")
		wailsruntime.LogInfof(a.ctx, "Started: %s", profileName)
	} else {
		wailsruntime.LogErrorf(a.ctx, "Start failed: %v", err)
		wailsruntime.EventsEmit(a.ctx, "engine_error", err.Error())
	}
	return err
}

func (a *App) StopEngine() error {
	err := a.manager.Stop()
	wailsruntime.EventsEmit(a.ctx, "status_changed", "Stopped")
	return err
}

func (a *App) GetStatus() string {
	return string(a.manager.GetStatus())
}

func (a *App) GetLogs() []string {
	return a.manager.GetLogs()
}

func (a *App) RunDiagnostics() []engine.DiagnosticResult {
	return engine.RunDiagnostics()
}

func (a *App) ClearDiscordCache() error {
	err := engine.ClearDiscordCache()
	if err == nil {
		wailsruntime.EventsEmit(a.ctx, "notification", map[string]string{
			"title": "Cleanup",
			"message": "Discord cache cleared successfully",
		})
	}
	return err
}

func (a *App) EnableTCPTimestamps() error {
	return engine.EnableTCPTimestamps()
}

func (a *App) KillWinws2() error {
	return a.KillConflicts()
}

func (a *App) GetSettings() (*engine.Settings, error) {
	return engine.GetSettings()
}

func (a *App) SaveSettings(settings *engine.Settings) error {
	return engine.SaveSettings(settings)
}

func (a *App) AutoTune() string {
	if a.autoTuneCancel != nil {
		return "Already running"
	}

	tuneCtx, cancel := context.WithCancel(a.ctx)
	a.autoTuneCancel = cancel
	
	wailsruntime.EventsEmit(a.ctx, "autotune_start", true)
	
	defer func() {
		a.autoTuneCancel = nil
		wailsruntime.EventsEmit(a.ctx, "autotune_start", false)
	}()

	assets, _ := engine.ExtractAssets()
	provider := providers.NewZapret2WindowsProvider(assets.BinDir, assets.LuaDir, assets.ListDir, true, false)
	
	// LOAD ALL PROFILES
	allProfiles := append(engine.GetProfiles(assets.LuaDir), engine.GetAdvancedProfiles(assets.LuaDir)...)
	
	result, err := engine.RunAutoTuneV2WithContext(tuneCtx, provider, allProfiles)
	if err != nil {
		wailsruntime.EventsEmit(a.ctx, "autotune_log", "❌ Auto-Tune failed or cancelled")
		return "Failed"
	}

	wailsruntime.EventsEmit(a.ctx, "autotune_complete", map[string]interface{}{
		"success": true,
		"profile": result.ProfileName,
	})
	
	return result.ProfileName
}

func (a *App) CancelAutoTune() {
	if a.autoTuneCancel != nil {
		a.autoTuneCancel()
		a.autoTuneCancel = nil
	}
}

func (a *App) GetLivePing() map[string]interface{} {
	if a.manager.GetStatus() != providers.StatusRunning {
		return map[string]interface{}{"active": false}
	}
	latency, err := engine.SimplePing(a.ctx, "https://1.1.1.1")
	if err != nil {
		return map[string]interface{}{"active": true, "latency": 0, "status": "error"}
	}
	return map[string]interface{}{"active": true, "latency": latency.Milliseconds(), "status": "ok"}
}

func (a *App) GetAppVersion() string {
	return "1.0.3"
}

func (a *App) EnableAutoStart() error {
	return engine.EnableAutoStart()
}

func (a *App) DisableAutoStart() error {
	return engine.DisableAutoStart()
}

func (a *App) IsAutoStartEnabled() bool {
	enabled, _ := engine.IsAutoStartEnabled()
	return enabled
}

func (a *App) CheckPrivileges() (bool, error) {
	return checkAdminPrivileges()
}

func (a *App) CheckConflicts() []string {
	conflicts := []string{}
	procs := []string{"winws2.exe", "winws.exe", "goodbyedpi.exe", "nfqws.exe", "zapret.exe"}
	
	for _, p := range procs {
		cmd := exec.Command("tasklist", "/FI", "IMAGENAME eq "+p, "/NH")
		cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true, CreationFlags: CREATE_NO_WINDOW}
		out, _ := cmd.Output()
		if strings.Contains(string(out), p) {
			conflicts = append(conflicts, "⚠️ "+p+" is running")
		}
	}
	return conflicts
}

func (a *App) KillConflicts() error {
	wailsruntime.LogInfo(a.ctx, "Executing Full Kill...")
	procs := []string{"winws2.exe", "winws.exe", "goodbyedpi.exe", "nfqws.exe", "zapret.exe"}
	
	for _, p := range procs {
		cmd := exec.Command("taskkill", "/F", "/IM", p)
		cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true, CreationFlags: CREATE_NO_WINDOW}
		cmd.Run()
	}
	
	// Reset WinDivert driver
	exec.Command("sc", "stop", "WinDivert").Run()
	
	wailsruntime.EventsEmit(a.ctx, "notification", map[string]string{
		"title": "Full Kill",
		"message": "All conflicting processes and drivers reset.",
	})
	return nil
}

func (a *App) ShowNotification(title string, message string) {
	wailsruntime.EventsEmit(a.ctx, "notification", map[string]string{
		"title":   title,
		"message": message,
	})
}
