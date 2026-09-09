//go:build darwin

package main

import (
	"context"
	"fmt"
	"strings"
	"time"

	"unbound/engine"
	"unbound/engine/providers"

	"github.com/wailsapp/wails/v2/pkg/menu"
	"github.com/wailsapp/wails/v2/pkg/menu/keys"
	"github.com/wailsapp/wails/v2/pkg/runtime"
)

func (a *App) setupTray() {
	a.mu.Lock()
	a.trayCtx, a.trayCancel = context.WithCancel(context.Background())
	a.mu.Unlock()

	runtime.LogInfo(a.ctx, "macOS Menu Bar controller initialized")

	// Dynamic updater for macOS Application Menu Bar
	go func() {
		ticker := time.NewTicker(2 * time.Second)
		defer ticker.Stop()

		for {
			select {
			case <-a.trayCtx.Done():
				return
			case <-a.trayUpdateTrigger:
				if a.ctx != nil {
					runtime.MenuSetApplicationMenu(a.ctx, getAppMenu(a))
				}
			case <-ticker.C:
				if a.ctx != nil {
					runtime.MenuSetApplicationMenu(a.ctx, getAppMenu(a))
				}
			}
		}
	}()
}

func (a *App) onBeforeClose(ctx context.Context) bool {
	// Returning false allows macOS system quit requests (Dock icon "Завершить", Cmd+Q, OS shutdown)
	// to terminate the application cleanly. Window 'X' button in UI calls HideWindowToTray() directly.
	return false
}

func (a *App) ShowFromTray() {
	runtime.WindowShow(a.ctx)
	runtime.WindowUnminimise(a.ctx)
}

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
		if settings.DefaultProfile != "" {
			for _, p := range profiles {
				if p == settings.DefaultProfile {
					return engineName, p
				}
			}
		}
	}
	return engineName, profiles[0]
}

// macOS native menu for Wails (Application Menu Bar)
func getAppMenu(a *App) *menu.Menu {
	appMenu := menu.NewMenu()

	status := a.manager.GetStatus()
	currentProfile := a.manager.CurrentProfileName("")
	pingText := a.getCachedPingText()

	// 1. App Submenu (UNBOUND)
	fileMenu := appMenu.AddSubmenu("UNBOUND")
	fileMenu.AddText("О программе UNBOUND", nil, func(cbdata *menu.CallbackData) {
		runtime.MessageDialog(a.ctx, runtime.MessageDialogOptions{
			Type:    runtime.InfoDialog,
			Title:   "О программе UNBOUND",
			Message: "UNBOUND v" + engine.Version + "\nUltimate DPI Bypass Engine for macOS\n\nСтатус: " + string(status),
		})
	})
	fileMenu.AddSeparator()
	fileMenu.AddText("Показать окно", nil, func(cbdata *menu.CallbackData) {
		a.ShowFromTray()
	})
	fileMenu.AddText("Скрыть окно", keys.CmdOrCtrl("h"), func(cbdata *menu.CallbackData) {
		runtime.WindowHide(a.ctx)
	})
	fileMenu.AddSeparator()
	fileMenu.AddText("Завершить UNBOUND", keys.CmdOrCtrl("q"), func(cbdata *menu.CallbackData) {
		a.QuitApp()
	})

	// 2. Движок (Engine controls & status)
	engineMenu := appMenu.AddSubmenu("Движок")

	var statusLabel string
	switch status {
	case providers.StatusRunning:
		if currentProfile != "" {
			statusLabel = fmt.Sprintf("Статус: Подключено (%s)", currentProfile)
		} else {
			statusLabel = "Статус: Подключено"
		}
	case providers.StatusStarting:
		statusLabel = "Статус: Подключение..."
	case providers.StatusError:
		statusLabel = "Статус: Ошибка"
	default:
		statusLabel = "Статус: Отключено"
	}
	engineMenu.AddText(statusLabel, nil, nil)
	engineMenu.AddText(pingText, nil, nil)
	engineMenu.AddSeparator()

	if status != providers.StatusRunning {
		engineMenu.AddText("Подключить", keys.CmdOrCtrl("r"), func(*menu.CallbackData) {
			eng, prof := a.defaultEngineAndProfile()
			if eng != "" {
				_ = a.StartEngine(eng, prof)
				a.TriggerTrayUpdate()
			}
		})
	} else {
		engineMenu.AddText("Отключить", keys.CmdOrCtrl("t"), func(*menu.CallbackData) {
			_ = a.StopEngine()
			a.TriggerTrayUpdate()
		})
	}

	engineMenu.AddText("Автоподбор", nil, func(*menu.CallbackData) {
		go func() {
			a.TriggerTrayUpdate()
			_ = a.AutoTune()
			a.TriggerTrayUpdate()
		}()
	})

	// 3. Профили (Profiles submenu)
	engines := a.manager.GetEngineNames()
	if len(engines) > 0 {
		engName := engines[0]
		profiles := a.manager.GetProfiles(engName)
		if len(profiles) > 0 {
			profMenu := appMenu.AddSubmenu("Профили")
			for _, p := range profiles {
				pName := p
				label := pName
				if status == providers.StatusRunning && strings.EqualFold(pName, currentProfile) {
					label = "✓ " + pName
				}
				profMenu.AddText(label, nil, func(*menu.CallbackData) {
					_ = a.StartEngine(engName, pName)
					a.TriggerTrayUpdate()
				})
			}
		}
	}

	// 4. Окно / Справка
	windowMenu := appMenu.AddSubmenu("Окно")
	windowMenu.AddText("Показать главное окно", nil, func(*menu.CallbackData) {
		a.ShowFromTray()
	})
	windowMenu.AddText("Скрыть в строку меню", nil, func(*menu.CallbackData) {
		runtime.WindowHide(a.ctx)
	})
	windowMenu.AddSeparator()
	windowMenu.AddText("Диагностика", nil, func(*menu.CallbackData) {
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

	return appMenu
}
