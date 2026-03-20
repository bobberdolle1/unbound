package main

import (
	"github.com/wailsapp/wails/v2/pkg/menu"
	"github.com/wailsapp/wails/v2/pkg/menu/keys"
	"github.com/wailsapp/wails/v2/pkg/runtime"
)

func (a *App) setupTray() {
	systray := menu.NewMenu()

	showItem := systray.AddText("Show", keys.CmdOrCtrl("s"), func(_ *menu.CallbackData) {
		runtime.WindowShow(a.ctx)
	})

	hideItem := systray.AddText("Hide", keys.CmdOrCtrl("h"), func(_ *menu.CallbackData) {
		runtime.WindowHide(a.ctx)
	})

	systray.AddSeparator()

	statusItem := systray.AddText("Status: Stopped", nil, nil)
	statusItem.Disabled = true

	systray.AddSeparator()

	systray.AddText("Quit", keys.CmdOrCtrl("q"), func(_ *menu.CallbackData) {
		runtime.Quit(a.ctx)
	})

	runtime.MenuSetApplicationMenu(a.ctx, systray)
}

func (a *App) HideToTray() {
	runtime.WindowHide(a.ctx)
}

func (a *App) ShowFromTray() {
	runtime.WindowShow(a.ctx)
}
