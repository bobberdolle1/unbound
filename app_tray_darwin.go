//go:build darwin

package main

/*
#cgo darwin CFLAGS: -x objective-c -fobjc-arc
#cgo darwin LDFLAGS: -framework Cocoa
#include <stdlib.h>
#include "tray_darwin.h"
*/
import "C"

import (
	"context"
	_ "embed"
	"fmt"
	"strings"
	"sync"
	"time"
	"unsafe"

	"unbound/engine"
	"unbound/engine/providers"

	"github.com/wailsapp/wails/v2/pkg/menu"
	"github.com/wailsapp/wails/v2/pkg/menu/keys"
	"github.com/wailsapp/wails/v2/pkg/runtime"
)

var (
	globalAppRef   *App
	globalAppRefMu sync.RWMutex
)

//go:embed build/darwin/tray_icon@2x.png
var trayIconData []byte
func setGlobalApp(a *App) {
	globalAppRefMu.Lock()
	globalAppRef = a
	globalAppRefMu.Unlock()
}

func getGlobalApp() *App {
	globalAppRefMu.RLock()
	defer globalAppRefMu.RUnlock()
	return globalAppRef
}

//export onTrayAction
func onTrayAction(actionTag C.int) {
	a := getGlobalApp()
	if a == nil {
		return
	}

	switch int(actionTag) {
	case 1: // Show Window
		a.ShowFromTray()
	case 2: // Hide Window
		if a.ctx != nil {
			runtime.WindowHide(a.ctx)
		}
	case 3: // Connect
		eng, prof := a.defaultEngineAndProfile()
		if eng != "" {
			_ = a.StartEngine(eng, prof)
			a.TriggerTrayUpdate()
		}
	case 4: // Disconnect
		_ = a.StopEngine()
		a.TriggerTrayUpdate()
	case 5: // AutoTune
		go func() {
			a.TriggerTrayUpdate()
			_ = a.AutoTune()
			a.TriggerTrayUpdate()
		}()
	case 6: // Quit
		a.QuitApp()
	}
}

//export onTraySelectProfile
func onTraySelectProfile(profileIndex C.int) {
	a := getGlobalApp()
	if a == nil {
		return
	}

	engines := a.manager.GetEngineNames()
	if len(engines) == 0 {
		return
	}
	engName := engines[0]
	profiles := a.manager.GetProfiles(engName)
	idx := int(profileIndex)
	if idx >= 0 && idx < len(profiles) {
		_ = a.StartEngine(engName, profiles[idx])
		a.TriggerTrayUpdate()
	}
}

//export onDockReopen
func onDockReopen() {
	a := getGlobalApp()
	if a == nil {
		return
	}
	a.ShowFromTray()
}

func (a *App) setupTray() {
	setGlobalApp(a)

	a.mu.Lock()
	a.trayCtx, a.trayCancel = context.WithCancel(context.Background())
	a.mu.Unlock()

	runtime.LogInfo(a.ctx, "macOS Native Status Bar Tray initialized")

	// Set up Cocoa Dock click observer and Status Bar Item
	C.setupDockClickObserver()
	C.initNativeTray()
	if len(trayIconData) > 0 {
		C.setNativeTrayIcon(unsafe.Pointer(&trayIconData[0]), C.int(len(trayIconData)))
	}
	// Initial push to tray
	a.syncNativeStatusBar()

	// Dynamic updater for macOS Status Bar Tray & Application Menu Bar
	go func() {
		ticker := time.NewTicker(2 * time.Second)
		defer ticker.Stop()

		for {
			select {
			case <-a.trayCtx.Done():
				return
			case <-a.trayUpdateTrigger:
				a.syncNativeStatusBar()
				if a.ctx != nil {
					runtime.MenuSetApplicationMenu(a.ctx, getAppMenu(a))
				}
			case <-ticker.C:
				a.syncNativeStatusBar()
				if a.ctx != nil {
					runtime.MenuSetApplicationMenu(a.ctx, getAppMenu(a))
				}
			}
		}
	}()
}

func (a *App) syncNativeStatusBar() {
	status := a.manager.GetStatus()
	currentProfile := a.manager.CurrentProfileName("")
	pingText := a.getCachedPingText()

	var statusLabel string
	isRunning := 0
	switch status {
	case providers.StatusRunning:
		isRunning = 1
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

	engines := a.manager.GetEngineNames()
	var profiles []string
	activeProfileIndex := -1
	if len(engines) > 0 {
		profiles = a.manager.GetProfiles(engines[0])
		for i, p := range profiles {
			if strings.EqualFold(p, currentProfile) {
				activeProfileIndex = i
				break
			}
		}
	}

	cStatus := C.CString(statusLabel)
	defer C.free(unsafe.Pointer(cStatus))

	cPing := C.CString(pingText)
	defer C.free(unsafe.Pointer(cPing))

	cProfilePointers := make([]*C.char, len(profiles))
	for i, p := range profiles {
		cProfilePointers[i] = C.CString(p)
		defer C.free(unsafe.Pointer(cProfilePointers[i]))
	}

	var cProfileArray **C.char
	if len(cProfilePointers) > 0 {
		cProfileArray = &cProfilePointers[0]
	}

	C.updateNativeTray(
		cStatus,
		cPing,
		C.int(isRunning),
		C.int(activeProfileIndex),
		cProfileArray,
		C.int(len(profiles)),
	)
}

func (a *App) onBeforeClose(ctx context.Context) bool {
	a.mu.Lock()
	closing := a.closing || a.quitting
	a.mu.Unlock()

	if closing {
		return false // Allow system quit requests to terminate cleanly
	}

	// Hide window to status bar tray when 'X' is clicked
	if a.ctx != nil {
		runtime.WindowHide(a.ctx)
	}
	return true
}

func (a *App) ShowFromTray() {
	if a.ctx != nil {
		runtime.WindowShow(a.ctx)
		runtime.WindowUnminimise(a.ctx)
	}
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
