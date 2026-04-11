//go:build darwin

package main

import (
	"context"

	"github.com/wailsapp/wails/v2/pkg/menu"
	"github.com/wailsapp/wails/v2/pkg/menu/keys"
	"github.com/wailsapp/wails/v2/pkg/runtime"
)

func (a *App) setupTray() {
	// macOS native tray is handled via the Menu option in wails.Run
	// But we can also manage it here if we want dynamic updates
	// However, for Wails v2, the best way is to define it in main.go
	// and update it via events.
	
	// Since we already have a setupTray() call in app.go,
	// we'll just log it here and rely on the menu defined in wails.Run (main.go).
	runtime.LogInfo(a.ctx, "macOS Native Tray initialized via Wails Options")
}

func (a *App) onBeforeClose(ctx context.Context) bool {
	// Hide to tray on X click
	runtime.WindowHide(ctx)
	return true
}

func (a *App) ShowFromTray() {
	runtime.WindowShow(a.ctx)
	runtime.WindowUnminimise(a.ctx)
}

// macOS native menu for Wails
func getAppMenu(a *App) *menu.Menu {
	AppMenu := menu.NewMenu()
	
	fileMenu := AppMenu.AddSubmenu("File")
	fileMenu.AddText("About", nil, func(cbdata *menu.CallbackData) {
		runtime.MessageDialog(a.ctx, runtime.MessageDialogOptions{
			Type:    runtime.InfoDialog,
			Title:   "About Unbound",
			Message: "Unbound v2.0.0\nUltimate DPI Bypass Engine",
		})
	})
	fileMenu.AddSeparator()
	fileMenu.AddText("Quit", keys.CmdOrCtrl("q"), func(cbdata *menu.CallbackData) {
		runtime.Quit(a.ctx)
	})

	return AppMenu
}
