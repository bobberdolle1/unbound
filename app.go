package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"

	"unbound/engine"
	"unbound/engine/providers"

	wailsruntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

type App struct {
	ctx                 context.Context
	manager             *providers.ProviderManager
	startMinimized      bool
	debugMode           bool
	autoTuneCancel      context.CancelFunc
	autoTuneWG          sync.WaitGroup
	autoReconnectCancel context.CancelFunc
	autoReconnectWG     sync.WaitGroup
	startupCancel       context.CancelFunc
	startupWG           sync.WaitGroup
	autoReconnectID     uint64
	profileChangeMu     sync.Mutex
	profileChange       bool
	mu                  sync.Mutex
	closing             bool
	quitting            bool

	// Tray lifecycle & cache
	trayCtx             context.Context
	trayCancel          context.CancelFunc
	trayUpdateTrigger   chan struct{}
	lastPingLatency     int64
	lastPingStatus      string
	lastPingMu          sync.RWMutex

	// Doctor lifecycle & cancellation
	doctorMu            sync.Mutex
	doctorCancel        context.CancelFunc
	doctorRunID         string
}

func NewApp() *App {
	return &App{
		manager:           providers.NewProviderManager(),
		trayUpdateTrigger: make(chan struct{}, 1),
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

	// Self-heal autostart registration if executable path changed (e.g. portable app unpacked in a new folder)
	if healed, healErr := engine.SelfHealAutoStart(); healErr != nil {
		logger.Warnf("Startup", "Autostart self-healing check: %v", healErr)
	} else if healed {
		logger.Info("Startup", "Autostart registration was automatically repaired to current executable")
	}

	// Register OS-specific providers
	registerOSProviders(a, assets)

	// Log registered engines and notify frontend
	engines := a.manager.GetEngineNames()
	logger.Infof("App", "Registered engines: %v", engines)
	wailsruntime.EventsEmit(ctx, "engines_changed", engines)
	// Auto-start profile: activate the user-selected strategy on every launch
	// (boot or manual). This is independent of settings.AutoStart, which only
	// registers the OS-level launch.
	if settings != nil && settings.AutoStartProfile {
		startupCtx, cancelStartup := context.WithCancel(ctx)
		a.mu.Lock()
		a.startupCancel = cancelStartup
		a.startupWG.Add(1)
		a.mu.Unlock()
		go func() {
			defer a.startupWG.Done()
			defer cancelStartup()
			timer := time.NewTimer(3 * time.Second)
			defer timer.Stop()
			select {
			case <-startupCtx.Done():
				return
			case <-timer.C:
			}
			if strings.EqualFold(strings.TrimSpace(settings.StartupProfileMode), "autotune") ||
				strings.TrimSpace(settings.StartupProfileMode) == "Автоподбор" {
				a.AutoTune()
				return
			}
			if len(engines) == 0 {
				return
			}
			engineName := engines[0]
			profiles := a.manager.GetProfiles(engineName)
			if len(profiles) == 0 {
				return
			}
			profileName := settings.DefaultProfile
			if contains(profiles, settings.StartupProfileMode) {
				profileName = settings.StartupProfileMode
			}
			if !contains(profiles, profileName) {
				profileName = profiles[0]
			}
			logger.Info("Startup", fmt.Sprintf("Auto-recovery: trying profile %s", profileName))
			if err := a.manager.Start(startupCtx, engineName, profileName); err != nil {
				logger.Warnf("Startup", "Profile %s failed: %v, running AutoTune", profileName, err)
				autoProfile := a.AutoTune()
				if autoProfile != "Failed" && autoProfile != "Already running" && autoProfile != "Shutting down" {
					logger.Info("Startup", fmt.Sprintf("Auto-recovery: switched to %s", autoProfile))
				}
				return
			}
			_ = engine.SaveLastProfile(profileName)
			if settings.AutoReconnect {
				a.AutoReconnectMonitor()
			}
			timer.Reset(5 * time.Second)
			select {
			case <-startupCtx.Done():
				return
			case <-timer.C:
			}
			ping := a.GetLivePing()
			status, _ := ping["status"].(string)
			if status == "blocked" || status == "disconnected" {
				logger.Warn("Startup", "Profile started but connectivity blocked, running AutoTune")
				_ = a.manager.Stop()
				autoProfile := a.AutoTune()
				if autoProfile != "Failed" && autoProfile != "Already running" && autoProfile != "Shutting down" {
					logger.Info("Startup", fmt.Sprintf("Auto-recovery: switched to %s", autoProfile))
				}
			}
		}()
	}

	a.setupTray()

	if a.startMinimized {
		logger.Info("App", "Starting hidden to tray (--tray); window stays hidden until restored")
	}

	logger.Info("App", "UNBOUND initialized successfully")
	wailsruntime.LogInfo(ctx, "UNBOUND initialized")
}

func (a *App) shutdown(ctx context.Context) {
	a.mu.Lock()
	a.closing = true
	autoTuneCancel := a.autoTuneCancel
	autoReconnectCancel := a.autoReconnectCancel
	startupCancel := a.startupCancel
	trayCancel := a.trayCancel
	doctorCancel := a.doctorCancel
	a.mu.Unlock()
	if trayCancel != nil {
		trayCancel()
	}
	if doctorCancel != nil {
		doctorCancel()
	}
	if autoTuneCancel != nil {
		autoTuneCancel()
	}
	if startupCancel != nil {
		startupCancel()
	}
	if autoReconnectCancel != nil {
		autoReconnectCancel()
	}
	a.profileChangeMu.Lock()
	a.profileChangeMu.Unlock()
	a.startupWG.Wait()
	a.autoTuneWG.Wait()
	a.autoReconnectWG.Wait()
	_ = a.manager.Stop()
	if err := engine.CleanupExtractedAssets(); err != nil {
		engine.GetLogger().Warnf("App", "Runtime cleanup failed: %v", err)
	}
}

func (a *App) TriggerTrayUpdate() {
	if a.trayUpdateTrigger == nil {
		return
	}
	select {
	case a.trayUpdateTrigger <- struct{}{}:
	default:
	}
}

func (a *App) updateCachedPing(lat int64, status string) {
	a.lastPingMu.Lock()
	a.lastPingLatency = lat
	a.lastPingStatus = status
	a.lastPingMu.Unlock()
	a.TriggerTrayUpdate()
}

func (a *App) getCachedPingText() string {
	a.lastPingMu.RLock()
	defer a.lastPingMu.RUnlock()
	if a.lastPingStatus == "ok" && a.lastPingLatency > 0 {
		return fmt.Sprintf("Пинг: %dмс", a.lastPingLatency)
	} else if a.lastPingStatus == "blocked" {
		return "Пинг: Заблокировано"
	}
	return "Пинг: —"
}
func (a *App) GetEngineNames() []string {
	return a.manager.GetEngineNames()
}

func (a *App) GetProfiles(engineName string) []string {
	return a.manager.GetProfiles(engineName)
}

func (a *App) beginManualProfileChange() bool {
	a.profileChangeMu.Lock()
	a.mu.Lock()
	if a.closing {
		a.mu.Unlock()
		a.profileChangeMu.Unlock()
		return false
	}
	a.profileChange = true
	autoTuneCancel := a.autoTuneCancel
	autoReconnectCancel := a.autoReconnectCancel
	a.mu.Unlock()
	if autoTuneCancel != nil {
		autoTuneCancel()
	}
	if autoReconnectCancel != nil {
		autoReconnectCancel()
	}
	a.autoTuneWG.Wait()
	a.autoReconnectWG.Wait()
	return true
}

func (a *App) endManualProfileChange() {
	a.mu.Lock()
	a.profileChange = false
	a.mu.Unlock()
	a.profileChangeMu.Unlock()
}

func (a *App) StartEngine(engineName string, profileName string) (err error) {
	manualChangeStarted := false
	defer func() {
		if r := recover(); r != nil {
			if manualChangeStarted {
				a.endManualProfileChange()
			}
			err = fmt.Errorf("StartEngine panic: %v", r)
			wailsruntime.LogErrorf(a.ctx, "%v", err)
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

	if !a.beginManualProfileChange() {
		return fmt.Errorf("application is shutting down")
	}
	defer func() {
		if manualChangeStarted {
			a.endManualProfileChange()
			manualChangeStarted = false
		}
	}()
	manualChangeStarted = true
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
		if saveErr := engine.SaveLastProfile(profileName); saveErr != nil {
			logger.Warnf("App", "Could not persist last profile: %v", saveErr)
		}
		wailsruntime.EventsEmit(a.ctx, "profile_changed", profileName)
		notifMgr.Success("Успешный запуск", fmt.Sprintf("Профиль: %s", profileName))
		wailsruntime.EventsEmit(a.ctx, "status_changed", "Running")
		wailsruntime.LogInfof(a.ctx, "Started: %s", profileName)
	} else {
		logger.Errorf("App", "Failed to start engine: %v", err)
		notifMgr.Error("Ошибка запуска", fmt.Sprintf("Не удалось произвести запуск: %v", err))
		wailsruntime.LogErrorf(a.ctx, "Start failed: %v", err)
		wailsruntime.EventsEmit(a.ctx, "engine_error", err.Error())
	}
	a.endManualProfileChange()
	manualChangeStarted = false
	if err == nil {
		if settings, settingsErr := engine.GetSettings(); settingsErr == nil && settings.AutoReconnect {
			a.AutoReconnectMonitor()
		}
	}
	return err
}
func (a *App) AddDefenderExclusion() error {
	return engine.AddDefenderExclusion()
}

func (a *App) StopEngine() (err error) {
	if !a.beginManualProfileChange() {
		return fmt.Errorf("application is shutting down")
	}
	changeEnded := false
	defer func() {
		if !changeEnded {
			a.endManualProfileChange()
		}
	}()
	err = a.manager.Stop()
	a.endManualProfileChange()
	changeEnded = true
	wailsruntime.EventsEmit(a.ctx, "status_changed", "Stopped")
	return err
}

func (a *App) GetStatus() string {
	return string(a.manager.GetStatus())
}
func (a *App) GetStatusInfo() map[string]interface{} {
	return a.manager.GetStatusInfo()
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

func (a *App) RunDoctor(mode string) (*engine.DoctorResult, error) {
	logger := engine.GetLogger()
	logger.Infof("App", "Doctor diagnostics requested (mode=%s)", mode)

	a.doctorMu.Lock()
	if a.doctorCancel != nil {
		logger.Info("App", "Cancelling previous active Doctor run")
		a.doctorCancel()
	}
	runID := fmt.Sprintf("run_%d", time.Now().UnixNano())
	a.doctorRunID = runID

	timeout := 25 * time.Second
	if mode == "extended" {
		timeout = 50 * time.Second
	}
	docCtx, cancel := context.WithTimeout(context.Background(), timeout)
	a.doctorCancel = cancel
	a.doctorMu.Unlock()

	defer func() {
		a.doctorMu.Lock()
		if a.doctorRunID == runID {
			a.doctorCancel = nil
		}
		a.doctorMu.Unlock()
	}()

	activeProfile := a.manager.CurrentProfileName("")

	onProgress := func(p engine.DoctorProgress) {
		p.RunID = runID
		wailsruntime.EventsEmit(a.ctx, "doctor_progress", p)
	}

	res, err := engine.RunDoctorWithProgress(docCtx, mode, activeProfile, a.manager.GetStatus(), onProgress)
	if err != nil {
		wailsruntime.EventsEmit(a.ctx, "doctor_error", map[string]interface{}{
			"runId": runID,
			"error": err.Error(),
		})
		return nil, err
	}

	wailsruntime.EventsEmit(a.ctx, "doctor_complete", map[string]interface{}{
		"runId":  runID,
		"result": res,
	})
	return res, nil
}

func (a *App) CancelDoctor(runID string) {
	logger := engine.GetLogger()
	logger.Infof("App", "CancelDoctor called for runId=%s", runID)

	a.doctorMu.Lock()
	defer a.doctorMu.Unlock()

	if a.doctorCancel != nil {
		if runID == "" || runID == a.doctorRunID {
			a.doctorCancel()
			a.doctorCancel = nil
			wailsruntime.EventsEmit(a.ctx, "doctor_cancelled", runID)
			logger.Info("App", "Doctor run cancelled successfully")
		}
	}
}

func (a *App) RunBypassComparison() (*engine.BypassComparisonResult, error) {
	logger := engine.GetLogger()
	logger.Info("App", "A/B bypass comparison requested")
	ctrl := &appProviderController{app: a}
	return engine.RunBypassComparison(a.ctx, ctrl, engine.GetQuickDiagnosticProbes())
}

func (a *App) GenerateDoctorReport(result *engine.DoctorResult) string {
	if result == nil {
		return ""
	}
	return result.FormatMarkdownReport()
}

func (a *App) OpenLogsFolder() error {
	return engine.OpenLogsFolder()
}

func (a *App) CheckAllUpdates() (*engine.SystemUpdateOverview, error) {
	return engine.CheckAllComponents(a.ctx, engine.Version)
}

func (a *App) RollbackEngineUpdate() error {
	ctrl := &appProviderController{app: a}
	return engine.RollbackEngine(ctrl)
}

func (a *App) GetAutoStartTaskInfo() (*engine.TaskRegistrationInfo, error) {
	return engine.QueryAutoStartTaskRegistration()
}

type appProviderController struct {
	app *App
}
func (c *appProviderController) CurrentProfile() string {
	return c.app.manager.CurrentProfileName("")
}
func (c *appProviderController) GetStatus() providers.Status {
	return c.app.manager.GetStatus()
}

func (c *appProviderController) Start(ctx context.Context, profile string) error {
	return c.app.StartEngine("", profile)
}

func (c *appProviderController) Stop() error {
	return c.app.StopEngine()
}
func (a *App) ClearDiscordCache() (*engine.DiscordCacheCleanupResult, error) {
	logger := engine.GetLogger()
	notifMgr := engine.GetNotificationManager()

	logger.Info("App", "Clearing Discord cache (structured)")
	res := engine.ClearDiscordCacheStructured()

	switch res.Status {
	case "SUCCESS":
		notifMgr.Success("Кэш очищен", res.Message)
	case "PARTIAL":
		notifMgr.Warning("Очищено частично", res.Message)
	case "NO_CACHE_FOUND":
		notifMgr.Info("Кэш Discord", res.Message)
	default:
		notifMgr.Error("Ошибка очистки", res.Message)
	}

	logger.Infof("App", "Discord cache cleanup finished: status=%s, freed=%d bytes, running=%v, failures=%d",
		res.Status, res.BytesFreed, res.DiscordRunning, len(res.Failures))

	return res, nil
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
	return persistSettings(settings)
}

// persistSettings writes settings and refreshes the OS autostart registration
// only when autostart-relevant fields actually changed. Rewriting the
// Windows scheduled task on every save required elevation, churned the task
// and broke settings saves from non-elevated contexts.
func persistSettings(next *engine.Settings) error {
	prev, _ := engine.GetSettings()
	if err := engine.SaveSettings(next); err != nil {
		return err
	}
	if prev != nil && prev.AutoStart == next.AutoStart && prev.StartMinimized == next.StartMinimized {
		return nil
	}
	if err := engine.ApplyAutoStartSetting(next.AutoStart); err != nil {
		return err
	}
	logger := engine.GetLogger()
	if next.AutoStart {
		logger.Info("App", "OS autostart registration refreshed")
	} else {
		logger.Info("App", "OS autostart registration removed")
	}
	return nil
}

func (a *App) AutoTune() string {
	logger := engine.GetLogger()
	notifMgr := engine.GetNotificationManager()

	a.mu.Lock()
	if a.closing {
		a.mu.Unlock()
		logger.Warn("App", "AutoTune rejected during shutdown")
		return "Shutting down"
	}
	if a.profileChange {
		a.mu.Unlock()
		logger.Warn("App", "AutoTune rejected during manual profile change")
		return "Already running"
	}
	if a.autoTuneCancel != nil {
		a.mu.Unlock()
		logger.Warn("App", "AutoTune already running")
		return "Already running"
	}

	tuneCtx, cancel := context.WithCancel(a.ctx)
	a.autoTuneCancel = cancel
	a.autoTuneWG.Add(1)
	a.mu.Unlock()
	defer a.autoTuneWG.Done()

	logger.Info("App", "AutoTune process started")
	notifMgr.Info("Автоподбор", "Начинаем оптимизацию профиля...")
	wailsruntime.EventsEmit(a.ctx, "autotune_start", true)

	resumeAutoReconnect := false
	defer func() {
		a.mu.Lock()
		a.autoTuneCancel = nil
		closing := a.closing
		a.mu.Unlock()
		if !resumeAutoReconnect || closing {
			return
		}
		settings, err := engine.GetSettings()
		if err == nil && settings.AutoReconnect {
			a.AutoReconnectMonitor()
		}
	}()

	statusInfo := a.manager.GetStatusInfo()
	previousEngine, _ := statusInfo["engine"].(string)
	previousProfile := a.manager.CurrentProfileName(previousEngine)
	previousWasRunning := statusInfo["status"] == string(providers.StatusRunning) && previousEngine != "" && previousProfile != ""
	managerStopped := false
	winnerActivated := false
	defer func() {
		if !managerStopped || winnerActivated || !previousWasRunning {
			return
		}
		a.mu.Lock()
		closing := a.closing
		a.mu.Unlock()
		if closing {
			return
		}
		if err := a.manager.Start(a.ctx, previousEngine, previousProfile); err != nil {
			logger.Errorf("App", "Could not restore previous profile %s: %v", previousProfile, err)
			wailsruntime.EventsEmit(a.ctx, "autotune_log", fmt.Sprintf("⚠ Не удалось восстановить прежний профиль %s: %v", previousProfile, err))
		} else {
			logger.Infof("App", "Restored previous profile after AutoTune failure: %s", previousProfile)
			wailsruntime.EventsEmit(a.ctx, "autotune_log", fmt.Sprintf("↩ Восстановлен прежний профиль: %s", previousProfile))
		}
	}()

	// Helper to emit failure and stop scanning in one place
	failAutoTune := func(errMsg string, logMsg string) string {
		wailsruntime.EventsEmit(a.ctx, "autotune_log", logMsg)
		wailsruntime.EventsEmit(a.ctx, "autotune_complete", map[string]interface{}{
			"success": false,
			"error":   errMsg,
		})
		return "Failed"
	}

	assets, err := engine.ExtractAssets()
	if err != nil {
		logger.Errorf("App", "AutoTune could not extract assets: %v", err)
		notifMgr.Error("Ошибка автоподбора", "Не удалось извлечь файлы движка")
		return failAutoTune("Не удалось извлечь файлы движка", fmt.Sprintf("❌ Ошибка: %v", err))
	}

	provider := providers.NewAutoTuneProvider(assets.BinDir, assets.LuaDir, assets.ListDir, assets.EngineSHA256)
	if provider == nil {
		logger.Error("App", "AutoTune has no usable engine provider on this platform")

		notifMgr.Error("Ошибка автоподбора", "Движок обхода не найден на этой системе")
		return failAutoTune("Движок обхода не найден. Установите nfqws/tpws/winws2.", "❌ Движок обхода не найден на этой системе")
	}

	a.mu.Lock()
	reconnectCancel := a.autoReconnectCancel
	resumeAutoReconnect = reconnectCancel != nil
	a.mu.Unlock()
	if reconnectCancel != nil {
		reconnectCancel()
		a.autoReconnectWG.Wait()
	}

	allProfiles := engine.PrepareAutoTuneProfiles(provider, assets.LuaDir)
	logger.Infof("App", "Loaded %d native profiles for testing", len(allProfiles))
	wailsruntime.EventsEmit(a.ctx, "autotune_log", fmt.Sprintf("Загружено %d профилей для тестирования", len(allProfiles)))

	if err := a.manager.Stop(); err != nil {
		logger.Errorf("App", "Could not stop active engine before AutoTune: %v", err)
		return failAutoTune(fmt.Sprintf("Не удалось остановить текущий профиль: %v", err), fmt.Sprintf("❌ AutoTune не запущен: %v", err))
	}
	managerStopped = true

	progressCb := func(step, total int, profile string, okCount, totalTargets int, msg string) {
		pct := int((float64(step) / float64(total)) * 100)
		wailsruntime.EventsEmit(a.ctx, "autotune_progress", map[string]interface{}{
			"step":         step,
			"total":        total,
			"percent":      pct,
			"profile":      profile,
			"okCount":      okCount,
			"totalTargets": totalTargets,
			"msg":          msg,
		})
	}

	result, err := engine.RunAutoTuneV2WithProgress(tuneCtx, provider, allProfiles, progressCb)
	if err != nil {
		logger.Errorf("App", "AutoTune failed: %v", err)
		notifMgr.Error("Ошибка автоподбора", "Не удалось найти оптимальный профиль")
		return failAutoTune(fmt.Sprintf("Автоподбор не удался: %v", err), fmt.Sprintf("❌ Ошибка: %v", err))
	}

	engineNames := a.manager.GetEngineNames()
	if len(engineNames) == 0 {
		return failAutoTune("Движок обхода не зарегистрирован", "❌ Невозможно запустить выбранный профиль")
	}
	if err := a.manager.Start(a.ctx, engineNames[0], result.ProfileName); err != nil {
		logger.Errorf("App", "AutoTune winner could not be activated: %v", err)
		return failAutoTune(fmt.Sprintf("Профиль найден, но не запустился: %v", err), fmt.Sprintf("❌ Ошибка запуска %s: %v", result.ProfileName, err))
	}
	winnerActivated = true
	resumeAutoReconnect = true
	if saveErr := engine.SaveLastProfile(result.ProfileName); saveErr != nil {
		logger.Warnf("App", "Could not persist AutoTune winner: %v", saveErr)
	}
	logger.Infof("App", "AutoTune completed and activated: %s", result.ProfileName)
	wailsruntime.EventsEmit(a.ctx, "autotune_log", fmt.Sprintf("✅ Запущен лучший профиль: %s (счёт: %d, восстановлено целей: %d)", result.ProfileName, result.Score, result.RecoveredTargets))
	wailsruntime.EventsEmit(a.ctx, "autotune_complete", map[string]interface{}{
		"success": true,
		"profile": result.ProfileName,
	})

	return result.ProfileName
}

func (a *App) CancelAutoTune() {
	a.mu.Lock()
	cancel := a.autoTuneCancel
	a.mu.Unlock()
	if cancel != nil {
		cancel()
	}
}

func (a *App) GetLivePing() map[string]interface{} {
	if a.manager.GetStatus() != providers.StatusRunning {
		return map[string]interface{}{"active": true, "latency": 0, "status": "disconnected", "services": map[string]int64{}}
	}
	targets := []struct{ Name, URL string }{
		{"YouTube", "https://www.youtube.com"},
		{"Discord", "https://discord.com"},
		{"Instagram", "https://www.instagram.com"},
	}

	var minLatency time.Duration = -1
	services := make(map[string]int64)
	var mu sync.Mutex
	var wg sync.WaitGroup

	for _, t := range targets {
		wg.Add(1)
		go func(name, url string) {
			defer wg.Done()
			ctx, cancel := context.WithTimeout(a.ctx, 2*time.Second)
			defer cancel()
			lat, err := engine.SimplePing(ctx, url)
			if err == nil {
				mu.Lock()
				services[name] = lat.Milliseconds()
				if minLatency == -1 || lat < minLatency {
					minLatency = lat
				}
				mu.Unlock()
			}
		}(t.Name, t.URL)
	}

	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	copyServices := func() map[string]int64 {
		mu.Lock()
		defer mu.Unlock()
		cp := make(map[string]int64, len(services))
		for k, v := range services {
			cp[k] = v
		}
		return cp
	}

	select {
	case <-done:
		resServices := copyServices()
		if minLatency == -1 {
			return map[string]interface{}{"active": true, "latency": 0, "status": "blocked", "services": resServices}
		}
		return map[string]interface{}{"active": true, "latency": minLatency.Milliseconds(), "status": "ok", "services": resServices}
	case <-time.After(2500 * time.Millisecond):
		return map[string]interface{}{"active": true, "latency": 0, "status": "blocked", "services": copyServices()}
	}
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
//
// The actual teardown lives in App.shutdown (wails OnShutdown): it waits for
// autotune/reconnect/startup goroutines, stops the engine and removes the
// per-process runtime directory. onBeforeClose lets the quit through only
// when a.quitting is set — otherwise runtime.Quit is vetoed and the window
// just hides to tray. The 5s failsafe guarantees process death even if the
// webview teardown wedges.
func (a *App) QuitApp() {
	logger := engine.GetLogger()
	logger.Info("App", "QuitApp requested by user")
	a.mu.Lock()
	a.quitting = true
	a.closing = true
	a.mu.Unlock()
	time.AfterFunc(5*time.Second, func() {
		logger.Warn("App", "graceful shutdown timed out, forcing exit")
		os.Exit(0)
	})
	wailsruntime.Quit(a.ctx)
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

// ──────────────────────────────────────────────────────────────────────────────
// QoA: Auto-reconnect
// ──────────────────────────────────────────────────────────────────────────────

// AutoReconnectMonitor starts a goroutine that monitors connectivity and
// falls back to the next profile if the current one becomes blocked.
// Only one monitor runs at a time; calling again cancels the previous one.
// Stops after maxCycles full rotations through all profiles.
func (a *App) AutoReconnectMonitor() {
	a.mu.Lock()
	if a.closing || a.autoTuneCancel != nil || a.profileChange {
		a.mu.Unlock()
		return
	}
	if a.autoReconnectCancel != nil {
		a.autoReconnectCancel()
	}
	ctx, cancel := context.WithCancel(a.ctx)
	a.autoReconnectCancel = cancel
	a.autoReconnectID++
	monitorID := a.autoReconnectID
	a.autoReconnectWG.Add(1)
	a.mu.Unlock()

	go func() {
		defer cancel()
		defer a.autoReconnectWG.Done()
		defer func() {
			a.mu.Lock()
			if a.autoReconnectID == monitorID {
				a.autoReconnectCancel = nil
			}
			a.mu.Unlock()
		}()
		blockedCount := 0
		maxBlocked := 3
		maxCycles := 3
		cyclesDone := 0
		switchCount := 0
		lastSwitch := time.Time{}
		cooldown := 30 * time.Second
		checkInterval := 15 * time.Second
		ticker := time.NewTicker(checkInterval)
		defer ticker.Stop()

		logger := engine.GetLogger()
		logger.Info("AutoReconnect", "Monitor started")

		for {
			select {
			case <-ctx.Done():
				logger.Info("AutoReconnect", "Monitor stopped")
				return
			case <-ticker.C:
			}
			// Respect cooldown after a switch
			if !lastSwitch.IsZero() && time.Since(lastSwitch) < cooldown {
				continue
			}
			if a.manager.GetStatus() != providers.StatusRunning {
				blockedCount = 0
				continue
			}
			ping := a.GetLivePing()
			status, _ := ping["status"].(string)
			if status == "blocked" || status == "disconnected" {
				blockedCount++
			} else {
				blockedCount = 0
				continue
			}
			if blockedCount >= maxBlocked {
				names := a.manager.GetEngineNames()
				if len(names) == 0 {
					blockedCount = 0
					continue
				}
				profiles := a.manager.GetProfiles(names[0])
				if len(profiles) < 2 {
					blockedCount = 0
					continue
				}
				// Track cycles: each time we wrap around, increment cyclesDone
				currentIndex := -1
				currentName := a.manager.CurrentProfileName(names[0])
				for i, p := range profiles {
					if p == currentName {
						currentIndex = i
						break
					}
				}
				nextIndex := (currentIndex + 1) % len(profiles)
				if nextIndex == 0 && switchCount > 0 {
					cyclesDone++
				}
				if cyclesDone >= maxCycles {
					logger.Warn("AutoReconnect", fmt.Sprintf("All %d profiles blocked after %d full cycles, stopping monitor.", len(profiles), maxCycles))
					wailsruntime.EventsEmit(a.ctx, "autotune_log", "❌ Все профили заблокированы. Авто-реконнект остановлен.")
					wailsruntime.EventsEmit(a.ctx, "profile_error", "Все профили заблокированы после "+fmt.Sprintf("%d", maxCycles)+" циклов")
					return
				}
				logger.Warn("AutoReconnect", fmt.Sprintf("Profile blocked for %d consecutive checks, switching...", blockedCount))
				wailsruntime.EventsEmit(a.ctx, "autotune_log", "⚠️ Профиль заблокирован, переключаем...")
				_ = a.manager.Stop()
				select {
				case <-ctx.Done():
					return
				case <-time.After(500 * time.Millisecond):
				}
				next := profiles[nextIndex]
				if err := ctx.Err(); err != nil {
					return
				}
				if err := a.manager.Start(ctx, names[0], next); err != nil {
					logger.Error("AutoReconnect", fmt.Sprintf("Failed to start %s: %v", next, err))
				} else {
					logger.Info("AutoReconnect", fmt.Sprintf("Switched to profile: %s", next))
					wailsruntime.EventsEmit(a.ctx, "autotune_log", fmt.Sprintf("✅ Переключено на профиль: %s", next))
					wailsruntime.EventsEmit(a.ctx, "profile_changed", next)
				}
				switchCount++
				lastSwitch = time.Now()
				blockedCount = 0
			}
		}
	}()
}

// StopAutoReconnect cancels the auto-reconnect monitor if running.
func (a *App) StopAutoReconnect() {
	a.mu.Lock()
	cancel := a.autoReconnectCancel
	a.mu.Unlock()
	if cancel != nil {
		cancel()
	}
}

// ──────────────────────────────────────────────────────────────────────────────
// QoA: Favorite profiles
// ──────────────────────────────────────────────────────────────────────────────

func (a *App) ToggleFavoriteProfile(profile string) ([]string, error) {
	s, err := engine.GetSettings()
	if err != nil {
		return nil, err
	}
	if s.FavoriteProfiles == nil {
		s.FavoriteProfiles = []string{}
	}
	found := -1
	for i, f := range s.FavoriteProfiles {
		if f == profile {
			found = i
			break
		}
	}
	if found >= 0 {
		s.FavoriteProfiles = append(s.FavoriteProfiles[:found], s.FavoriteProfiles[found+1:]...)
	} else {
		s.FavoriteProfiles = append(s.FavoriteProfiles, profile)
	}
	if err := engine.SaveSettings(s); err != nil {
		return nil, err
	}
	return s.FavoriteProfiles, nil
}

func (a *App) GetFavoriteProfiles() []string {
	s, err := engine.GetSettings()
	if err != nil || s.FavoriteProfiles == nil {
		return []string{}
	}
	return s.FavoriteProfiles
}

// ──────────────────────────────────────────────────────────────────────────────
// QoA: Diagnostic report
// ──────────────────────────────────────────────────────────────────────────────

func (a *App) GenerateDiagnosticReport() string {
	logger := engine.GetLogger()
	var report string
	report += "# UNBOUND Diagnostic Report\n"
	report += fmt.Sprintf("**Version**: %s\n", engine.Version)
	report += fmt.Sprintf("**OS**: %s/%s\n", runtime.GOOS, runtime.GOARCH)
	report += fmt.Sprintf("**Date**: %s\n\n", time.Now().Format("2006-01-02 15:04:05"))

	// Engine status
	report += "## Engine Status\n"
	report += fmt.Sprintf("- Status: %s\n", string(a.manager.GetStatus()))
	report += fmt.Sprintf("- Privileges: %v\n", a.CheckPrivileges())
	report += fmt.Sprintf("- SecureDNS: %v\n\n", a.IsSecureDNSEnabled())

	// Profiles
	report += "## Available Profiles\n"
	for _, name := range a.manager.GetEngineNames() {
		for _, p := range a.manager.GetProfiles(name) {
			report += fmt.Sprintf("- %s\n", p)
		}
	}
	report += "\n"

	// Conflicts
	report += "## Conflict Detection\n"
	conflicts := a.CheckConflicts()
	if len(conflicts) == 0 {
		report += "- No conflicts detected\n"
	} else {
		for _, c := range conflicts {
			report += fmt.Sprintf("- %s\n", c)
		}
	}
	report += "\n"

	// Connectivity probe
	report += "## Connectivity Probe\n"
	targets := []struct{ Name, URL string }{
		{"YouTube", "https://www.youtube.com"},
		{"Discord", "https://discord.com"},
		{"Instagram", "https://www.instagram.com"},
		{"Cloudflare", "https://1.1.1.1"},
	}
	for _, t := range targets {
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		lat, err := engine.SimplePing(ctx, t.URL)
		cancel()
		if err == nil {
			report += fmt.Sprintf("- %s: OK (%dms)\n", t.Name, lat.Milliseconds())
		} else {
			report += fmt.Sprintf("- %s: FAIL (%v)\n", t.Name, err)
		}
	}
	report += "\n"

	// Asset verification
	report += "## Asset Integrity\n"
	vr := engine.VerifyAssets()
	if vr.Verified {
		report += fmt.Sprintf("- %d files verified, all hashes match\n", vr.TotalFiles)
	} else {
		report += fmt.Sprintf("- ERROR: %s\n", vr.Error)
	}
	report += "\n"

	// Recent logs
	report += "## Recent Logs (last 30)\n"
	logs := logger.GetEntriesFormatted()
	start := 0
	if len(logs) > 30 {
		start = len(logs) - 30
	}
	for _, l := range logs[start:] {
		report += fmt.Sprintf("```\n%s\n```\n", l)
	}

	logger.Info("Diagnostic", "Report generated")
	return report
}

// ──────────────────────────────────────────────────────────────────────────────
// QoA: Hostlist auto-update
// ──────────────────────────────────────────────────────────────────────────────

func (a *App) UpdateHostlistsNow() (string, error) {
	logger := engine.GetLogger()
	logger.Info("Hostlists", "Manual hostlist update triggered")
	if err := providers.SyncHostlists(); err != nil {
		logger.Errorf("Hostlists", "Update failed: %v", err)
		return "", err
	}
	logger.Info("Hostlists", "Hostlists updated successfully")
	return "Hostlists updated", nil
}

// ──────────────────────────────────────────────────────────────────────────────
// QoA: Ping history persistence
// ──────────────────────────────────────────────────────────────────────────────

const pingHistoryFile = "ping_history.json"
const maxPingHistory = 100

type PingRecord struct {
	Timestamp int64  `json:"ts"`
	Latency   int64  `json:"lat"`
	Status    string `json:"st"`
}

func (a *App) SavePingHistory(latency int64, status string) {
	a.updateCachedPing(latency, status)
	path, err := engine.GetConfigDir()
	if err != nil {
		return
	}
	filePath := filepath.Join(path, pingHistoryFile)

	var records []PingRecord
	data, err := os.ReadFile(filePath)
	if err == nil {
		_ = json.Unmarshal(data, &records)
	}
	records = append(records, PingRecord{
		Timestamp: time.Now().Unix(),
		Latency:   latency,
		Status:    status,
	})
	if len(records) > maxPingHistory {
		records = records[len(records)-maxPingHistory:]
	}
	out, _ := json.Marshal(records)
	_ = os.WriteFile(filePath, out, 0644)
}

func (a *App) LoadPingHistory() []PingRecord {
	path, err := engine.GetConfigDir()
	if err != nil {
		return nil
	}
	filePath := filepath.Join(path, pingHistoryFile)
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil
	}
	var records []PingRecord
	_ = json.Unmarshal(data, &records)
	return records
}

// ──────────────────────────────────────────────────────────────────────────────
// Helpers
// ──────────────────────────────────────────────────────────────────────────────

func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}
