package main

import (
	"github.com/getlantern/systray"
	"github.com/wailsapp/wails/v2/pkg/runtime"
)

func (a *App) setupTray() {
	go systray.Run(a.onTrayReady, a.onTrayExit)
}

func (a *App) onTrayReady() {
	systray.SetTitle("ClearFlow")
	systray.SetTooltip("ClearFlow Unbound Engine")

	mShow := systray.AddMenuItem("Show ClearFlow", "Show application window")
	systray.AddSeparator()
	mQuit := systray.AddMenuItem("Quit", "Stop engine and quit application")

	go func() {
		for {
			select {
			case <-mShow.ClickedCh:
				runtime.WindowShow(a.ctx)
			case <-mQuit.ClickedCh:
				a.manager.Stop()
				systray.Quit()
				runtime.Quit(a.ctx)
			}
		}
	}()
}

func (a *App) onTrayExit() {
	// Cleanup if needed
}

func (a *App) HideToTray() {
	runtime.WindowHide(a.ctx)
}

func (a *App) ShowFromTray() {
	runtime.WindowShow(a.ctx)
}
