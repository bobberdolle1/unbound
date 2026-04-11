//go:build linux

package main

import (
	"context"
	"github.com/wailsapp/wails/v2/pkg/runtime"
	"github.com/wailsapp/wails/v2/pkg/menu"
)

func (a *App) setupTray() {
	// No-op for Linux currently as we disabled CGO for easier cross-build.
	runtime.LogInfo(a.ctx, "Tray disabled on Linux (non-CGO build)")
}

func (a *App) onBeforeClose(ctx context.Context) bool {
	return false // Regular quit on Linux X click for now
}

func (a *App) ShowFromTray() {
	runtime.WindowShow(a.ctx)
}

func getAppMenu(a *App) *menu.Menu {
	return nil
}
