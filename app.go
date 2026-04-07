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
	
	logger := engine.GetLogger()
	notifMgr := engine.GetNotificationManager()
	
	// Initialize notification manager with Wails event emitter
	notifMgr.Initialize(ctx, wailsruntime.EventsEmit)
	
	logger.Info("App", "UNBOUND starting up...")
	
	// Extract assets
	assets, err := engine.ExtractAssets()
	if err != nil {
		logger.Errorf("App", "Failed to extract assets: %v", err)
		notifMgr.Error("Startup Error", "Failed to extract required files")
		wailsruntime.LogErrorf(ctx, "Failed to extract assets: %v", err)
		return
	}
	logger.Info("App", "Assets extracted successfully")

	// Validate startup requirements
	validator := engine.NewStartupValidator(assets)
	validationResult := validator.ValidateStartup()
	
	if !validationResult.Valid {
		logger.Error("App", "Startup validation failed")
		for _, err := range validationResult.Errors {
			logger.Errorf("App", "Validation error: %s", err)
		}
		notifMgr.Error("Startup Failed", "Critical files missing. Please reinstall the application.")
		wailsruntime.LogError(ctx, "Startup validation failed - see logs for details")
		return
	}
	
	// Log warnings if any
	for _, warning := range validationResult.Warnings {
		logger.Warnf("App", "Validation warning: %s", warning)
	}
	
	if len(validationResult.Warnings) > 0 {
		notifMgr.Warning("Startup Warning", "Some optional components are missing")
	}
	
	logger.Info("App", "Startup validation passed")

	// Apply system settings
	settings, _ := engine.GetSettings()
	if settings != nil {
		if settings.EnableTCPTimestamps {
			logger.Info("App", "Enabling TCP timestamps")
			a.EnableTCPTimestamps()
		}
		if settings.DiscordCacheAutoClean {
			logger.Info("App", "Cleaning Discord cache")
			a.ClearDiscordCache()
		}
	}

	// Register OS-specific providers
	registerOSProviders(a, assets)
	
	// Log registered engines
	engines := a.manager.GetEngineNames()
	logger.Infof("App", "Registered engines: %v", engines)
	
	a.setupTray()

	if a.startMinimized {
		wailsruntime.WindowMinimise(ctx)
	}
	
	logger.Info("App", "UNBOUND initialized successfully")
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
	defer func() {
		if r := recover(); r != nil {
			wailsruntime.LogErrorf(a.ctx, "PANIC in StartEngine: %v", r)
		}
	}()
	
	logger := engine.GetLogger()
	notifMgr := engine.GetNotificationManager()
	
	logger.Infof("App", "StartEngine called: engine=%s, profile=%s", engineName, profileName)
	wailsruntime.LogInfof(a.ctx, "StartEngine called: engine=%s, profile=%s", engineName, profileName)
	
	// Check admin privileges
	logger.Info("App", "Checking administrator privileges...")
	wailsruntime.LogInfo(a.ctx, "Checking admin privileges...")
	
	hasPriv, err := checkAdminPrivileges()
	if err != nil {
		logger.Errorf("App", "Privilege check error: %v", err)
		wailsruntime.LogErrorf(a.ctx, "Privilege check error: %v", err)
		notifMgr.Error("Privilege Error", "Failed to check administrator privileges")
		wailsruntime.EventsEmit(a.ctx, "privilege_error", fmt.Sprintf("Privilege check failed: %v", err))
		return err
	}
	
	logger.Infof("App", "Privilege check result: admin=%v", hasPriv)
	wailsruntime.LogInfof(a.ctx, "Privilege check result: %v", hasPriv)
	
	if !hasPriv {
		logger.Error("App", "Administrator privileges required but not granted")
		wailsruntime.LogError(a.ctx, "Administrator privileges required")
		notifMgr.Error("Admin Required", "Please restart the application as administrator")
		wailsruntime.EventsEmit(a.ctx, "privilege_error", "Administrator privileges required. Please restart the application as administrator.")
		return fmt.Errorf("administrator privileges required")
	}
	
	logger.Info("App", "Administrator privileges confirmed")

	logger.Info("App", "Stopping current engine if running...")
	wailsruntime.LogInfo(a.ctx, "Stopping current engine if running...")
	a.manager.Stop()
	time.Sleep(500 * time.Millisecond)

	logger.Infof("App", "Starting engine: %s with profile: %s", engineName, profileName)
	logger.Infof("App", "Available engines: %v", a.manager.GetEngineNames())
	wailsruntime.LogInfof(a.ctx, "Starting engine: %s with profile: %s", engineName, profileName)
	wailsruntime.LogInfof(a.ctx, "Available engines: %v", a.manager.GetEngineNames())
	
	wailsruntime.LogInfo(a.ctx, "About to call manager.Start...")
	err = a.manager.Start(a.ctx, engineName, profileName)
	logger.Infof("App", "Manager.Start returned: err=%v", err)
	wailsruntime.LogInfof(a.ctx, "Manager.Start returned: err=%v", err)
	
	if err == nil {
		logger.Infof("App", "Engine started successfully: %s", profileName)
		notifMgr.Success("Engine Started", fmt.Sprintf("Profile: %s", profileName))
		wailsruntime.EventsEmit(a.ctx, "status_changed", "Running")
		wailsruntime.LogInfof(a.ctx, "Started: %s", profileName)
	} else {
		logger.Errorf("App", "Failed to start engine: %v", err)
		notifMgr.Error("Engine Failed", fmt.Sprintf("Failed to start: %v", err))
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

func (a *App) GetStructuredLogs() []string {
	logger := engine.GetLogger()
	return logger.GetEntriesFormatted()
}

func (a *App) RunDiagnostics() []engine.DiagnosticResult {
	return engine.RunDiagnostics()
}

func (a *App) ClearDiscordCache() error {
	logger := engine.GetLogger()
	notifMgr := engine.GetNotificationManager()
	
	logger.Info("App", "Clearing Discord cache")
	err := engine.ClearDiscordCache()
	if err == nil {
		notifMgr.Success("Cleanup", "Discord cache cleared successfully")
	} else {
		logger.Errorf("App", "Failed to clear Discord cache: %v", err)
		notifMgr.Error("Cleanup Failed", "Could not clear Discord cache")
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
	if settings.EnableTCPTimestamps {
		if err := engine.EnableTCPTimestamps(); err != nil {
			wailsruntime.LogErrorf(a.ctx, "Failed to enable TCP Timestamps: %v", err)
		}
	}
	if settings.DiscordCacheAutoClean {
		if err := engine.ClearDiscordCache(); err != nil {
			wailsruntime.LogErrorf(a.ctx, "Failed to clear Discord cache: %v", err)
		}
	}
	return engine.SaveSettings(settings)
}

func (a *App) AutoTune() string {
	logger := engine.GetLogger()
	notifMgr := engine.GetNotificationManager()
	
	if a.autoTuneCancel != nil {
		logger.Warn("App", "AutoTune already running")
		return "Already running"
	}

	tuneCtx, cancel := context.WithCancel(a.ctx)
	a.autoTuneCancel = cancel
	
	logger.Info("App", "AutoTune process started")
	notifMgr.Info("AutoTune", "Starting profile optimization...")
	wailsruntime.EventsEmit(a.ctx, "autotune_start", true)
	
	defer func() {
		a.autoTuneCancel = nil
		wailsruntime.EventsEmit(a.ctx, "autotune_start", false)
	}()

	assets, _ := engine.ExtractAssets()
	provider := providers.NewZapret2WindowsProvider(assets.BinDir, assets.LuaDir, assets.ListDir, true, false)
	
	// LOAD ALL PROFILES
	allProfiles := append(engine.GetProfiles(assets.LuaDir), engine.GetAdvancedProfiles(assets.LuaDir)...)
	logger.Infof("App", "Loaded %d profiles for testing", len(allProfiles))
	
	result, err := engine.RunAutoTuneV2WithContext(tuneCtx, provider, allProfiles)
	if err != nil {
		logger.Errorf("App", "AutoTune failed: %v", err)
		notifMgr.Error("AutoTune Failed", "Could not find optimal profile")
		wailsruntime.EventsEmit(a.ctx, "autotune_log", "❌ Auto-Tune failed or cancelled")
		return "Failed"
	}

	logger.Infof("App", "AutoTune completed successfully: %s", result.ProfileName)
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
	// Проверяем реальный обход — тестируем YouTube (основная заблокированная цель)
	latency, err := engine.SimplePing(a.ctx, "https://www.youtube.com")
	if err != nil {
		// Fallback: пробуем Discord
		latency2, err2 := engine.SimplePing(a.ctx, "https://discord.com")
		if err2 != nil {
			return map[string]interface{}{"active": true, "latency": 0, "status": "blocked"}
		}
		return map[string]interface{}{"active": true, "latency": latency2.Milliseconds(), "status": "ok"}
	}
	return map[string]interface{}{"active": true, "latency": latency.Milliseconds(), "status": "ok"}
}

func (a *App) GetAppVersion() string {
	return "1.0.4"
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

func (a *App) CheckPrivileges() bool {
	hasPriv, err := checkAdminPrivileges()
	if err != nil {
		wailsruntime.LogErrorf(a.ctx, "CheckPrivileges error: %v", err)
		return false
	}
	return hasPriv
}

func (a *App) CheckConflicts() []string {
	conflicts := []string{}
	
	// Проверяем DPI-обходчики и VPN-клиенты (не наш winws2.exe)
	type conflictProc struct {
		Exe  string
		Desc string
	}
	procs := []conflictProc{
		{"winws.exe",         "старый Zapret (winws)"},
		{"goodbyedpi.exe",    "GoodbyeDPI"},
		{"nfqws.exe",         "nfqws"},
		{"zapret.exe",        "Zapret"},
		{"ciadpi.exe",        "ciadpi"},
		{"byedpi.exe",        "ByeDPI"},
		{"openvpn.exe",       "OpenVPN"},
		{"warp-svc.exe",      "Cloudflare WARP"},
		{"expressvpn.exe",    "ExpressVPN"},
		{"nordvpn-service.exe", "NordVPN"},
	}
	
	for _, p := range procs {
		cmd := exec.Command("tasklist", "/FI", "IMAGENAME eq "+p.Exe, "/NH")
		cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true, CreationFlags: CREATE_NO_WINDOW}
		out, _ := cmd.Output()
		if strings.Contains(string(out), p.Exe) {
			conflicts = append(conflicts, "⚠️ "+p.Desc+" запущен")
		}
	}
	return conflicts
}

func (a *App) KillConflicts() error {
	logger := engine.GetLogger()
	notifMgr := engine.GetNotificationManager()
	
	logger.Info("App", "Выполняем завершение конфликтующих процессов...")
	wailsruntime.LogInfo(a.ctx, "Executing Full Kill...")
	
	// Завершаем внешние DPI-обходчики и VPN (не наш winws2.exe)
	procs := []string{
		"winws.exe", "goodbyedpi.exe", "nfqws.exe", "zapret.exe",
		"ciadpi.exe", "byedpi.exe",
	}
	
	for _, p := range procs {
		cmd := exec.Command("taskkill", "/F", "/IM", p)
		cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true, CreationFlags: CREATE_NO_WINDOW}
		cmd.Run()
	}
	
	// Сброс драйвера WinDivert
	exec.Command("sc", "stop", "WinDivert").Run()
	
	logger.Info("App", "Конфликтующие процессы и драйверы остановлены")
	notifMgr.Success("Конфликты устранены", "Все конфликтующие процессы и драйверы остановлены")
	return nil
}

func (a *App) ShowNotification(title string, message string) {
	notifMgr := engine.GetNotificationManager()
	notifMgr.Info(title, message)
}
