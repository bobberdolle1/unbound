//go:build darwin

package main

import (
	"os"
	"os/exec"
	"strings"

	"unbound/engine"
	"unbound/engine/providers"

	"github.com/wailsapp/wails/v2/pkg/runtime"
)

// checkAdminPrivileges reports whether the process is root or the user has admin rights.
func checkAdminPrivileges() (bool, error) {
	if os.Geteuid() == 0 {
		return true, nil
	}
	out, err := exec.Command("id", "-Gn").Output()
	if err == nil && strings.Contains(string(out), "admin") {
		return true, nil
	}
	return false, nil
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
