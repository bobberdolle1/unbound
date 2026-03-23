package main

import (
	"os"
	"time"

	"github.com/getlantern/systray"
	"github.com/wailsapp/wails/v2/pkg/runtime"
)

func (a *App) setupTray() {
	go systray.Run(a.onTrayReady, a.onTrayExit)
}

func (a *App) onTrayReady() {
	systray.SetTitle("Unbound")
	systray.SetTooltip("Unbound DPI Bypass Engine")

	mShow := systray.AddMenuItem("Show Unbound", "Show application window")
	systray.AddSeparator()
	mQuit := systray.AddMenuItem("Quit", "Stop engine and quit application")

	go func() {
		for {
			select {
			case <-mShow.ClickedCh:
				runtime.WindowShow(a.ctx)
			case <-mQuit.ClickedCh:
				// Остановка движка с таймаутом
				go func() {
					a.manager.Stop()
					time.Sleep(200 * time.Millisecond)
					systray.Quit()
					runtime.Quit(a.ctx)
					// Если Wails не закрылся сам за 1 секунду - убиваем процесс принудительно
					time.Sleep(1 * time.Second)
					os.Exit(0)
				}()
			}
		}
	}()
}

func (a *App) onTrayExit() {
}

func (a *App) HideToTray() {
	runtime.WindowHide(a.ctx)
}

func (a *App) ShowFromTray() {
	runtime.WindowShow(a.ctx)
}
