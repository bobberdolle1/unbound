//go:build windows

package main

import (
	"context"
	_ "embed"
	"fmt"
	"time"

	"github.com/getlantern/systray"
	"github.com/wailsapp/wails/v2/pkg/menu"
	"github.com/wailsapp/wails/v2/pkg/runtime"
	"unbound/engine/providers"
)

func getAppMenu(a *App) *menu.Menu {
	return nil
}

//go:embed build/windows/icon.ico
var iconData []byte

func (a *App) setupTray() {
	go systray.Run(a.onTrayReady, a.onTrayExit)
}

func (a *App) onTrayReady() {
	systray.SetIcon(iconData)
	systray.SetTitle("UNBOUND")
	systray.SetTooltip("UNBOUND — обход DPI")

	mStatus := systray.AddMenuItem("Статус: Отключено", "Текущий статус двигателя")
	mStatus.Disable()

	mPing := systray.AddMenuItem("Пинг: —", "Задержка до целевых сервисов")
	mPing.Disable()

	systray.AddSeparator()

	mShow := systray.AddMenuItem("Развернуть Unbound", "Показать окно приложения")
	mConnect := systray.AddMenuItem("Подключить", "Запустить обход DPI")
	mDisconnect := systray.AddMenuItem("Отключить", "Остановить обход DPI")
	mDisconnect.Hide()
	mAutoTune := systray.AddMenuItem("Автоподбор", "Запустить автоматический подбор профиля")

	systray.AddSeparator()
	mQuit := systray.AddMenuItem("Выход", "Остановить двигатель и выйти из приложения")

	// Обновление статуса и пинга в трее
	go func() {
		for {
			status := a.manager.GetStatus()
			if status == providers.StatusRunning {
				mStatus.SetTitle("Статус: Подключено")
				mConnect.Hide()
				mDisconnect.Show()
				mAutoTune.Disable()
				ping := a.GetLivePing()
				lat, _ := ping["latency"].(int64)
				pingSt, _ := ping["status"].(string)
				if pingSt == "ok" && lat > 0 {
					mPing.SetTitle(fmt.Sprintf("Пинг: %dмс", lat))
				} else if pingSt == "blocked" {
					mPing.SetTitle("Пинг: Заблокировано")
				} else {
					mPing.SetTitle("Пинг: —")
				}
			} else {
				mStatus.SetTitle("Статус: Отключено")
				mConnect.Show()
				mDisconnect.Hide()
				mAutoTune.Enable()
				mPing.SetTitle("Пинг: —")
			}
			time.Sleep(2 * time.Second)
		}
	}()

	go func() {
		for {
			select {
			case <-mShow.ClickedCh:
				runtime.WindowUnminimise(a.ctx)
				runtime.WindowShow(a.ctx)

			case <-mConnect.ClickedCh:
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
				a.StartEngine(engines[0], profile)

			case <-mDisconnect.ClickedCh:
				a.StopEngine()

			case <-mAutoTune.ClickedCh:
				go a.AutoTune()

			case <-mQuit.ClickedCh:
				a.QuitApp()
			}
		}
	}()
}

func (a *App) onTrayExit() {}

func (a *App) onBeforeClose(ctx context.Context) bool {
	a.mu.Lock()
	quitting := a.quitting
	a.mu.Unlock()
	if quitting {
		// Real quit request (QuitApp binding or tray "Выход"): let Wails run
		// its OnShutdown teardown instead of vetoing it.
		return false
	}
	// Window close (X / programmatic hide) keeps the app alive in the tray.
	a.HideWindowToTray()
	return true
}

func (a *App) ShowFromTray() {
	runtime.WindowUnminimise(a.ctx)
	runtime.WindowShow(a.ctx)
}
