//go:build darwin

package main

import (
	"unbound/engine"
	"unbound/engine/providers"

	"github.com/wailsapp/wails/v2/pkg/runtime"
)

// checkAdminPrivileges reports whether the process can run on macOS.
// Privileges for pfctl are requested gracefully via osascript when starting
// the bypass engine, so standard user GUI sessions start without error overlays.
func checkAdminPrivileges() (bool, error) {
	return true, nil
}

func registerOSProviders(a *App, assets *engine.AssetPaths) {
	logger := engine.GetLogger()

	binPath, err := providers.ResolveEngineBinary(providers.MacOSEngineBinary, assets.BinDir)
	if err != nil {
		logger.Warnf("App", "Engine binary resolution warning: %v", err)
		binPath = ""
	} else {
		logger.Infof("App", "Using engine binary at %s", binPath)
	}

	provider := providers.NewZapretMacOSProvider(binPath)

	if cb, ok := provider.(providers.BypassProviderWithCallbacks); ok {
		cb.SetStatusCallback(func(status providers.Status) {
			runtime.EventsEmit(a.ctx, "status_changed", status)
		})
		cb.SetLogCallback(func(line string) {
			runtime.EventsEmit(a.ctx, "engine_log", line)
		})
	}

	a.manager.Register(provider)
}
