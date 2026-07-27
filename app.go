package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sync"
	"time"

	"unbound/engine"
	"unbound/engine/providers"

	wailsruntime "github.com/wailsapp/wails/v2/pkg/runtime"
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
		notifMgr.Error("Ошибка запуска", "Не удалось извлечь необходимые файлы")
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
		notifMgr.Error("Ошибка запуска", "Критические файлы отсутствуют. Переустановите приложение.")
		wailsruntime.LogError(ctx, "Startup validation failed - see logs for details")
		return
	}

	// Log warnings if any
	for _, warning := range validationResult.Warnings {
		logger.Warnf("App", "Validation warning: %s", warning)
	}

	if len(validationResult.Warnings) > 0 {
		notifMgr.Warning("Предупреждение", "Некоторые необязательные компоненты отсутствуют")
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
		if settings.SecureDNS {
			logger.Info("App", "Enabling secure DNS")
			a.SetSecureDNS(true)
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

	if engineName == "" || engineName == " " {
		engines := a.manager.GetEngineNames()
		if len(engines) > 0 {
			engineName = engines[0]
		}
	}

	if profileName == "" || profileName == " " {
		profiles := a.manager.GetProfiles(engineName)
		if len(profiles) > 0 {
			profileName = profiles[0]
		}
	}

	logger.Infof("App", "StartEngine called: engine=%s, profile=%s", engineName, profileName)
	wailsruntime.LogInfof(a.ctx, "StartEngine called: engine=%s, profile=%s", engineName, profileName)

	// Check admin privileges
	logger.Info("App", "Checking administrator privileges...")
	wailsruntime.LogInfo(a.ctx, "Checking admin privileges...")

	hasPriv, err := checkAdminPrivileges()
	if err != nil {
		logger.Errorf("App", "Privilege check error: %v", err)
		wailsruntime.LogErrorf(a.ctx, "Privilege check error: %v", err)
		notifMgr.Error("Ошибка прав", "Не удалось проверить права администратора")
		wailsruntime.EventsEmit(a.ctx, "privilege_error", fmt.Sprintf("Privilege check failed: %v", err))
		return err
	}

	logger.Infof("App", "Privilege check result: admin=%v", hasPriv)
	wailsruntime.LogInfof(a.ctx, "Privilege check result: %v", hasPriv)

	if !hasPriv {
		logger.Error("App", "Administrator privileges required but not granted")
		wailsruntime.LogError(a.ctx, "Administrator privileges required")
		if runtime.GOOS == "darwin" {
			notifMgr.Error("Ошибка прав", "Запустите приложение с правами sudo/root")
			wailsruntime.EventsEmit(a.ctx, "privilege_error", "Требуются права root (sudo). Перезапустите приложение с правами root для управления pf.")
		} else {
			notifMgr.Error("Ошибка прав", "Запустите приложение от имени администратора")
			wailsruntime.EventsEmit(a.ctx, "privilege_error", "Требуются права администратора. Перезапустите приложение от имени администратора.")
		}
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
		notifMgr.Success("Успешный запуск", fmt.Sprintf("Профиль: %s", profileName))
		wailsruntime.EventsEmit(a.ctx, "status_changed", "Running")
		wailsruntime.LogInfof(a.ctx, "Started: %s", profileName)
	} else {
		logger.Errorf("App", "Failed to start engine: %v", err)
		notifMgr.Error("Ошибка запуска", fmt.Sprintf("Не удалось произвести запуск: %v", err))
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
		notifMgr.Success("Очистка", "Кэш Discord успешно очищен")
	} else {
		logger.Errorf("App", "Failed to clear Discord cache: %v", err)
		notifMgr.Error("Ошибка очистки", "Не удалось очистить кэш Discord")
	}
	return err
}

func (a *App) EnableTCPTimestamps() error {
	return engine.EnableTCPTimestamps()
}

// KillWinws2 force-stops Unbound's *own* bypass engine. It backs the
// "Завершить winws2.exe" button, the recovery action for an engine that has
// wedged and no longer responds to a normal stop.
//
// It used to just call KillConflicts(), whose process list deliberately
// excludes winws2.exe ("не наш winws2.exe") — so the one button meant to kill
// our engine was the one button guaranteed not to, while the UI still reported
// "Все процессы winws2 завершены".
//
// The name is kept because it is a Wails binding the frontend imports; the
// behaviour is now what the label promises on every platform.
func (a *App) KillWinws2() error {
	logger := engine.GetLogger()
	notifMgr := engine.GetNotificationManager()

	// Ask the managed process to exit first so it can clean up its firewall
	// rules; force-killing skips that and strands them.
	if err := a.manager.Stop(); err != nil {
		logger.Warnf("App", "Graceful engine stop failed, forcing: %v", err)
	}

	if err := killOwnEngineImpl(); err != nil {
		logger.Errorf("App", "Failed to force-stop the engine: %v", err)
		notifMgr.Error("Ошибка", fmt.Sprintf("Не удалось завершить движок: %v", err))
		return err
	}

	logger.Info("App", "Own bypass engine stopped")
	wailsruntime.EventsEmit(a.ctx, "status_changed", string(providers.StatusStopped))
	return nil
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
	if err := a.SetSecureDNS(settings.SecureDNS); err != nil {
		wailsruntime.LogErrorf(a.ctx, "Failed to set secure DNS: %v", err)
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
	notifMgr.Info("Автоподбор", "Начинаем оптимизацию профиля...")
	wailsruntime.EventsEmit(a.ctx, "autotune_start", true)

	defer func() {
		a.autoTuneCancel = nil
		wailsruntime.EventsEmit(a.ctx, "autotune_start", false)
	}()

	assets, err := engine.ExtractAssets()
	if err != nil {
		logger.Errorf("App", "AutoTune could not extract assets: %v", err)
		notifMgr.Error("Ошибка автоподбора", "Не удалось извлечь файлы движка")
		return "Failed"
	}

	provider := providers.NewAutoTuneProvider(assets.BinDir, assets.LuaDir, assets.ListDir)
	if provider == nil {
		// The factory returns nil when no engine binary can be found. Handing
		// that straight to the tuner used to panic with a nil dereference.
		logger.Error("App", "AutoTune has no usable engine provider on this platform")
		notifMgr.Error("Ошибка автоподбора", "Движок обхода не найден на этой системе")
		wailsruntime.EventsEmit(a.ctx, "autotune_log", "❌ Движок обхода не найден")
		return "Failed"
	}

	// LOAD ALL PROFILES
	allProfiles := append(engine.GetProfiles(assets.LuaDir), engine.GetAdvancedProfiles(assets.LuaDir)...)
	logger.Infof("App", "Loaded %d profiles for testing", len(allProfiles))

	result, err := engine.RunAutoTuneV2WithContext(tuneCtx, provider, allProfiles)
	if err != nil {
		logger.Errorf("App", "AutoTune failed: %v", err)
		notifMgr.Error("Ошибка автоподбора", "Не удалось найти оптимальный профиль")
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
	targets := []string{
		"https://www.youtube.com",
		"https://discord.com",
		"https://x.com",
		"https://www.instagram.com",
	}

	var minLatency time.Duration = -1
	var mu sync.Mutex
	var wg sync.WaitGroup

	for _, t := range targets {
		wg.Add(1)
		go func(url string) {
			defer wg.Done()
			ctx, cancel := context.WithTimeout(a.ctx, 2*time.Second)
			defer cancel()
			lat, err := engine.SimplePing(ctx, url)
			if err == nil {
				mu.Lock()
				if minLatency == -1 || lat < minLatency {
					minLatency = lat
				}
				mu.Unlock()
			}
		}(t)
	}
	wg.Wait()

	if minLatency == -1 {
		return map[string]interface{}{"active": true, "latency": 0, "status": "blocked"}
	}

	return map[string]interface{}{"active": true, "latency": minLatency.Milliseconds(), "status": "ok"}
}

func (a *App) GetAppVersion() string {
	return engine.Version
}

func (a *App) GetOSPlatform() string {
	return runtime.GOOS
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

// QuitApp stops the engine and terminates the application process cleanly.
func (a *App) QuitApp() {
	logger := engine.GetLogger()
	logger.Info("App", "QuitApp requested by user")
	a.manager.Stop()
	time.Sleep(200 * time.Millisecond)
	wailsruntime.Quit(a.ctx)
	time.Sleep(500 * time.Millisecond)
	os.Exit(0)
}

func (a *App) CheckPrivileges() bool {
	hasPriv, err := checkAdminPrivileges()
	if err != nil {
		wailsruntime.LogErrorf(a.ctx, "CheckPrivileges error: %v", err)
		return false
	}
	return hasPriv
}

// CheckConflicts reports other DPI-bypass tools and VPN clients that would
// fight Unbound over the same packets. The per-OS implementations live in
// conflicts_{windows,linux,darwin}.go.
func (a *App) CheckConflicts() []string {
	return checkConflictsImpl()
}

// KillConflicts terminates the processes reported by CheckConflicts.
func (a *App) KillConflicts() error {
	logger := engine.GetLogger()
	notifMgr := engine.GetNotificationManager()

	logger.Info("App", "Выполняем завершение конфликтующих процессов...")

	if err := killConflictsImpl(); err != nil {
		logger.Errorf("App", "Failed to terminate conflicting processes: %v", err)
		notifMgr.Error("Ошибка", fmt.Sprintf("Не удалось остановить конфликтующие процессы: %v", err))
		return err
	}

	logger.Info("App", "Конфликтующие процессы и драйверы остановлены")
	notifMgr.Success("Конфликты устранены", "Все конфликтующие процессы и драйверы остановлены")
	return nil
}

func (a *App) ShowNotification(title string, message string) {
	notifMgr := engine.GetNotificationManager()
	notifMgr.Info(title, message)
}

// HideWindowToTray hides the window while leaving the engine running.
//
// This used to call a.manager.Stop() first, so the "Закрыть в трей" button and
// the window's ✕ (which routes here via onBeforeClose) silently tore down the
// DPI bypass. Running in the background is the entire reason the app lives in
// the tray, and nothing told the user their traffic had stopped being
// protected — the window just disappeared.
func (a *App) HideWindowToTray() {
	wailsruntime.WindowHide(a.ctx)
}

func (a *App) ShowWindowFromTray() {
	wailsruntime.WindowShow(a.ctx)
	wailsruntime.WindowUnminimise(a.ctx)
}

var editableLists = map[string]bool{
	"youtube.txt":       true,
	"discord.txt":       true,
	"other.txt":         true,
	"ipset-exclude.txt": true,
}

func (a *App) GetBypassLists() []string {
	return []string{"youtube.txt", "discord.txt", "other.txt", "ipset-exclude.txt"}
}

func (a *App) ReadBypassList(name string) (string, error) {
	if !editableLists[name] {
		return "", fmt.Errorf("file access denied")
	}
	listsDir, err := engine.GetListsDir()
	if err != nil {
		return "", err
	}
	path := filepath.Join(listsDir, name)
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return "", nil
		}
		return "", err
	}
	return string(data), nil
}

func (a *App) SaveBypassList(name string, content string) error {
	if !editableLists[name] {
		return fmt.Errorf("file access denied")
	}
	listsDir, err := engine.GetListsDir()
	if err != nil {
		return err
	}
	path := filepath.Join(listsDir, name)
	return os.WriteFile(path, []byte(content), 0644)
}

func (a *App) SaveCustomScript(content string) error {
	return engine.SaveCustomScript(content)
}

func (a *App) LoadCustomScript() (string, error) {
	return engine.LoadCustomScript()
}

func (a *App) ExportLogs(content string) (bool, error) {
	filePath, err := wailsruntime.SaveFileDialog(a.ctx, wailsruntime.SaveDialogOptions{
		Title:           "Сохранить лог",
		DefaultFilename: "unbound_log.txt",
		Filters: []wailsruntime.FileFilter{
			{
				DisplayName: "Текстовые файлы (*.txt)",
				Pattern:     "*.txt",
			},
		},
	})
	if err != nil {
		return false, err
	}
	if filePath == "" {
		return false, nil // user cancelled
	}

	err = os.WriteFile(filePath, []byte(content), 0644)
	if err != nil {
		return false, err
	}

	return true, nil
}

// SetSecureDNS points the active network interfaces at Cloudflare's resolvers
// (or restores the system defaults). The per-OS implementations live in
// securedns_{windows,linux,darwin}.go.
func (a *App) SetSecureDNS(enabled bool) error {
	return setSecureDNSImpl(enabled)
}

// IsSecureDNSEnabled reports whether the active interfaces currently resolve
// through the Cloudflare servers set by SetSecureDNS.
func (a *App) IsSecureDNSEnabled() bool {
	return isSecureDNSEnabledImpl()
}
func (a *App) VerifyEngineAssets() engine.AssetVerificationResult {
	return engine.VerifyAssets()
}
