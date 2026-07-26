//go:build darwin

package main

import (
	"os"

	"unbound/engine"
	"unbound/engine/providers"

	"github.com/wailsapp/wails/v2/pkg/runtime"
)

// checkAdminPrivileges reports whether the process can load pf rules and open
// the divert socket the engine needs.
//
// This used to return (true, nil) unconditionally, with a comment claiming
// macOS escalates via osascript at runtime - which nothing in the codebase
// actually does. The result was that an unprivileged run passed the privilege
// gate and then failed inside pfctl with a bare "permission denied", while the
// README told users to launch with sudo all along.
//
// A checkAdminPrivilegesReal() helper next to it did check group membership,
// but had no callers.
func checkAdminPrivileges() (bool, error) {
	return os.Geteuid() == 0, nil
}

func registerOSProviders(a *App, assets *engine.AssetPaths) {
	logger := engine.GetLogger()

	// Look beyond the extracted assets: the macOS build does not vendor an
	// engine, so a Homebrew or manual install has to be discoverable.
	binPath, err := providers.ResolveEngineBinary(providers.MacOSEngineBinary, assets.BinDir)
	if err != nil {
		logger.Errorf("App", "%v", err)
		engine.GetNotificationManager().Error(
			"Движок не найден",
			"Не найден бинарник nfqws. Установите zapret (например, через Homebrew).",
		)
		return
	}
	logger.Infof("App", "Using engine binary at %s", binPath)

	provider := providers.NewZapretMacOSProvider(binPath)

	cb, ok := provider.(providers.BypassProviderWithCallbacks)
	if !ok {
		// The profile registration below used to re-assert this interface
		// unchecked inside two loops, which would panic rather than degrade.
		logger.Error("App", "macOS provider does not support callbacks; profiles unavailable")
		a.manager.Register(provider)
		return
	}

	cb.SetStatusCallback(func(status providers.Status) {
		runtime.EventsEmit(a.ctx, "status_changed", status)
	})
	cb.SetLogCallback(func(line string) {
		runtime.EventsEmit(a.ctx, "engine_log", line)
	})

	registered := make(map[string]bool)
	for _, p := range engine.GetProfiles(assets.LuaDir) {
		cb.RegisterProfile(p.Name, p.Args)
		registered[p.Name] = true
	}
	for _, p := range engine.GetAdvancedProfiles(assets.LuaDir) {
		if !registered[p.Name] {
			cb.RegisterProfile(p.Name, p.Args)
			registered[p.Name] = true
		}
	}

	a.manager.Register(provider)
}
