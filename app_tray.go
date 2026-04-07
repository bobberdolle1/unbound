package main

import (
	_ "embed"
	"os"
	"time"

	"github.com/getlantern/systray"
	"github.com/wailsapp/wails/v2/pkg/runtime"
	"unbound/engine/providers"
)

//go:embed build/appicon.png
var iconData []byte

func (a *App) setupTray() {
	go systray.Run(a.onTrayReady, a.onTrayExit)
}

func (a *App) onTrayReady() {
	systray.SetIcon(iconData)
	systray.SetTitle("Unbound")
	systray.SetTooltip("Unbound — двигатель обхода DPI")

	mStatus := systray.AddMenuItem("Статус: Отключено", "Текущий статус двигателя")
	mStatus.Disable()

	systray.AddSeparator()

	mShow := systray.AddMenuItem("Развернуть Unbound", "Показать окно приложения")
	mConnect := systray.AddMenuItem("Подключить", "Запустить обход DPI")
	mDisconnect := systray.AddMenuItem("Отключить", "Остановить обход DPI")
	mDisconnect.Hide()

	systray.AddSeparator()
	mQuit := systray.AddMenuItem("Выход", "Остановить двигатель и выйти из приложения")

	// Логика обновления меню в зависимости от статуса
	go func() {
		for {
			status := a.manager.GetStatus()
			if status == providers.StatusRunning {
				mStatus.SetTitle("Статус: Подключено")
				mConnect.Hide()
				mDisconnect.Show()
			} else {
				mStatus.SetTitle("Статус: Отключено")
				mConnect.Show()
				mDisconnect.Hide()
			}
			time.Sleep(1 * time.Second)
		}
	}()

	go func() {
		for {
			select {
			case <-mShow.ClickedCh:
				runtime.WindowUnminimise(a.ctx)
				runtime.WindowShow(a.ctx)
			
			case <-mConnect.ClickedCh:
				// Запуск дефолтного профиля или автотюна
				settings, _ := a.GetSettings()
				profile := "Unbound Ultimate (God Mode)"
				if settings != nil && settings.DefaultProfile != "" {
					profile = settings.DefaultProfile
				}
				a.StartEngine("Zapret 2 (winws)", profile)

			case <-mDisconnect.ClickedCh:
				a.StopEngine()

			case <-mQuit.ClickedCh:
				a.manager.Stop()
				time.Sleep(200 * time.Millisecond)
				systray.Quit()
				runtime.Quit(a.ctx)
				time.Sleep(1 * time.Second)
				os.Exit(0)
			}
		}
	}()
}

func (a *App) onTrayExit() {}

func (a *App) onBeforeClose() bool {
	// When user clicks X, hide to tray instead of quitting
	a.HideToTray()
	return false // false = prevent default close behavior
}

func (a *App) HideToTray() {
	runtime.WindowHide(a.ctx)
}

func (a *App) ShowFromTray() {
	runtime.WindowUnminimise(a.ctx)
	runtime.WindowShow(a.ctx)
}
