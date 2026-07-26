//go:build linux

package main

import (
	"context"

	"github.com/wailsapp/wails/v2/pkg/menu"
	"github.com/wailsapp/wails/v2/pkg/menu/keys"
	"github.com/wailsapp/wails/v2/pkg/runtime"
)

// Linux has no system tray here: getlantern/systray needs
// libayatana-appindicator at build and run time, and the indicator protocol is
// unsupported on stock GNOME. Rather than ship a tray that silently does
// nothing on the most common desktop, the same actions are exposed through the
// application menu, which GTK renders everywhere.
func (a *App) setupTray() {
	runtime.LogInfo(a.ctx, "Tray not used on Linux; engine controls live in the application menu")
}

// onBeforeClose lets the window close normally.
//
// Windows hides to the tray here. Doing that on Linux would strand the user:
// with no tray icon there would be no way to bring the window back, and the
// process would keep running with no visible UI.
func (a *App) onBeforeClose(ctx context.Context) bool {
	return false
}

func (a *App) ShowFromTray() {
	runtime.WindowUnminimise(a.ctx)
	runtime.WindowShow(a.ctx)
}

// defaultEngineAndProfile picks what the menu's Connect action should start:
// the first registered engine, and the user's saved profile if it is still
// offered by that engine.
func (a *App) defaultEngineAndProfile() (string, string) {
	engines := a.manager.GetEngineNames()
	if len(engines) == 0 {
		return "", ""
	}
	engineName := engines[0]

	profiles := a.manager.GetProfiles(engineName)
	if len(profiles) == 0 {
		return engineName, ""
	}

	if settings, err := a.GetSettings(); err == nil && settings != nil {
		for _, p := range profiles {
			if p == settings.DefaultProfile {
				return engineName, p
			}
		}
	}
	return engineName, profiles[0]
}

func getAppMenu(a *App) *menu.Menu {
	appMenu := menu.NewMenu()

	engineMenu := appMenu.AddSubmenu("Движок")
	engineMenu.AddText("Подключить", keys.CmdOrCtrl("r"), func(*menu.CallbackData) {
		engineName, profile := a.defaultEngineAndProfile()
		if engineName == "" {
			runtime.MessageDialog(a.ctx, runtime.MessageDialogOptions{
				Type:    runtime.ErrorDialog,
				Title:   "Движок недоступен",
				Message: "Не найден движок обхода. Установите пакет zapret (nfqws).",
			})
			return
		}
		if err := a.StartEngine(engineName, profile); err != nil {
			runtime.MessageDialog(a.ctx, runtime.MessageDialogOptions{
				Type:    runtime.ErrorDialog,
				Title:   "Не удалось запустить",
				Message: err.Error(),
			})
		}
	})
	engineMenu.AddText("Отключить", keys.CmdOrCtrl("t"), func(*menu.CallbackData) {
		_ = a.StopEngine()
	})
	engineMenu.AddSeparator()
	engineMenu.AddText("Остановить конфликтующие процессы", nil, func(*menu.CallbackData) {
		_ = a.KillConflicts()
	})
	engineMenu.AddSeparator()
	engineMenu.AddText("Выход", keys.CmdOrCtrl("q"), func(*menu.CallbackData) {
		runtime.Quit(a.ctx)
	})

	helpMenu := appMenu.AddSubmenu("Справка")
	helpMenu.AddText("Диагностика", nil, func(*menu.CallbackData) {
		summary := ""
		for _, r := range a.RunDiagnostics() {
			summary += r.Component + ": " + r.Status + " — " + r.Details + "\n"
		}
		runtime.MessageDialog(a.ctx, runtime.MessageDialogOptions{
			Type:    runtime.InfoDialog,
			Title:   "Диагностика системы",
			Message: summary,
		})
	})
	helpMenu.AddText("О программе", nil, func(*menu.CallbackData) {
		runtime.MessageDialog(a.ctx, runtime.MessageDialogOptions{
			Type:    runtime.InfoDialog,
			Title:   "О программе",
			Message: "Unbound " + a.GetAppVersion() + "\nДвижок обхода DPI\nСтатус: " + a.GetStatus(),
		})
	})

	return appMenu
}
