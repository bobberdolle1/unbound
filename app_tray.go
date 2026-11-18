//go:build windows

package main

import (
	"context"
	_ "embed"
	"sync"
	"time"

	"github.com/getlantern/systray"
	"github.com/wailsapp/wails/v2/pkg/menu"
	"github.com/wailsapp/wails/v2/pkg/runtime"
	"unbound/engine/providers"
)

var appSystrayQuit = systray.Quit

func getAppMenu(a *App) *menu.Menu {
	return nil
}

//go:embed build/windows/icon.ico
var iconData []byte

type trayStateSnapshot struct {
	status     providers.Status
	profile    string
	pingText   string
	autoTuning bool
	managed    AutoTuneVNextManagedStatus
}

type trayController struct {
	mu           sync.Mutex
	lastSnapshot trayStateSnapshot
	mStatus      *systray.MenuItem
	mPing        *systray.MenuItem
	mShow        *systray.MenuItem
	mConnect     *systray.MenuItem
	mDisconnect  *systray.MenuItem
	mAutoTune    *systray.MenuItem
	mQuit        *systray.MenuItem
	initialized  bool
}

func (tc *trayController) sync(current trayStateSnapshot) {
	tc.mu.Lock()
	defer tc.mu.Unlock()

	if !tc.initialized {
		return
	}

	prev := tc.lastSnapshot

	if prev.status == current.status &&
		prev.profile == current.profile &&
		prev.pingText == current.pingText &&
		prev.autoTuning == current.autoTuning &&
		prev.managed.State == current.managed.State &&
		prev.managed.Active == current.managed.Active &&
		prev.managed.StrategyID == current.managed.StrategyID {
		return
	}

	// 2. Status & action transitions (Enable/Disable only; no Hide/Show structural mutations)
	if prev.status != current.status || prev.profile != current.profile || prev.autoTuning != current.autoTuning || prev.managed.State != current.managed.State || prev.managed.Active != current.managed.Active || prev.managed.StrategyID != current.managed.StrategyID {
		statusTitle, _ := formatTrayStatusLabel(current.status, current.profile, current.autoTuning, current.managed)
		switch {
		case current.autoTuning:
			tc.mConnect.Disable()
			tc.mDisconnect.Disable()
			tc.mAutoTune.Disable()
		case current.managed.State == "STATE_RESTORE_FAILED" || current.managed.Active || current.status == providers.StatusRunning:
			tc.mConnect.Disable()
			tc.mDisconnect.Enable()
			tc.mAutoTune.Disable()
		case current.status == providers.StatusStarting:
			tc.mConnect.Disable()
			tc.mDisconnect.Disable()
			tc.mAutoTune.Disable()
		default:
			tc.mConnect.Enable()
			tc.mDisconnect.Disable()
			tc.mAutoTune.Enable()
		}

		tc.mStatus.SetTitle(statusTitle)
	}

	// 3. Ping item update
	if prev.pingText != current.pingText {
		tc.mPing.SetTitle(current.pingText)
	}

	tc.lastSnapshot = current
}

func (a *App) getTraySnapshot() trayStateSnapshot {
	status := a.manager.GetStatus()
	profile := a.manager.CurrentProfileName("")
	pingText := a.getCachedPingText()
	autoTuning := a.isAutoTuneInFlight()
	managed := a.GetAutoTuneVNextManagedStatus()

	return trayStateSnapshot{
		status:     status,
		profile:    profile,
		pingText:   pingText,
		autoTuning: autoTuning,
		managed:    managed,
	}
}

func (a *App) setupTray() {
	a.mu.Lock()
	a.trayCtx, a.trayCancel = context.WithCancel(context.Background())
	a.trayStarted = true
	a.mu.Unlock()

	go systray.Run(a.onTrayReady, a.onTrayExit)
}

func (a *App) quitTray() {
	a.mu.Lock()
	started := a.trayStarted
	a.trayStarted = false
	a.mu.Unlock()
	if started && appSystrayQuit != nil {
		appSystrayQuit()
	}
}

func (a *App) onTrayReady() {
	systray.SetIcon(iconData)
	systray.SetTitle("UNBOUND")
	systray.SetTooltip("UNBOUND — обход DPI")

	tc := &trayController{
		mStatus:     systray.AddMenuItem("Статус: Отключено", "Текущий статус двигателя"),
		mPing:       systray.AddMenuItem("Пинг: —", "Задержка до целевых сервисов"),
		mShow:       systray.AddMenuItem("Развернуть Unbound", "Показать окно приложения"),
		mConnect:    systray.AddMenuItem("Подключить", "Запустить обход DPI"),
		mDisconnect: systray.AddMenuItem("Отключить", "Остановить обход DPI"),
		mAutoTune:   systray.AddMenuItem("Автоподбор стратегии", "Открыть основной интерфейс AutoTune vNext"),
		mQuit:       systray.AddMenuItem("Выход", "Остановить двигатель и выйти из приложения"),
		initialized: true,
	}

	tc.mStatus.Disable()
	tc.mPing.Disable()
	tc.mDisconnect.Disable()

	systray.AddSeparator()

	// Initial sync
	tc.sync(a.getTraySnapshot())

	// Periodic & event-driven update worker
	go func() {
		ticker := time.NewTicker(2 * time.Second)
		defer ticker.Stop()

		for {
			select {
			case <-a.trayCtx.Done():
				return
			case <-a.trayUpdateTrigger:
				tc.sync(a.getTraySnapshot())
			case <-ticker.C:
				tc.sync(a.getTraySnapshot())
			}
		}
	}()

	// Click event worker
	go func() {
		for {
			select {
			case <-a.trayCtx.Done():
				return

			case <-tc.mShow.ClickedCh:
				a.ShowFromTray()

			case <-tc.mConnect.ClickedCh:
				engines := a.manager.GetEngineNames()
				if len(engines) == 0 {
					continue
				}
				settings, _ := a.GetSettings()
				profile := ""
				if settings != nil && settings.DefaultProfile != "" {
					profile = settings.DefaultProfile
				}
				if profile == "" {
					profiles := a.manager.GetProfiles(engines[0])
					if len(profiles) > 0 {
						profile = profiles[0]
					}
				}
				_ = a.StartEngine(engines[0], profile)
				a.TriggerTrayUpdate()

			case <-tc.mDisconnect.ClickedCh:
				_ = a.StopEngine()
				a.TriggerTrayUpdate()

			case <-tc.mAutoTune.ClickedCh:
				a.ShowFromTray()
				appRuntimeEventsEmit(a.ctx, "open_autotune_vnext")

			case <-tc.mQuit.ClickedCh:
				a.QuitApp()
			}
		}
	}()
}

func (a *App) onTrayExit() {
	a.mu.Lock()
	a.trayStarted = false
	cancel := a.trayCancel
	a.mu.Unlock()
	if cancel != nil {
		cancel()
	}
}

func (a *App) onBeforeClose(ctx context.Context) bool {
	a.mu.Lock()
	isQuitting := a.quitting || a.closing
	a.mu.Unlock()

	if isQuitting {
		return false
	}

	a.HideWindowToTray()
	return true
}

func (a *App) ShowFromTray() {
	if a.ctx != nil && a.ctx.Value("frontend") != nil {
		runtime.WindowShow(a.ctx)
		runtime.WindowUnminimise(a.ctx)
	}
}
