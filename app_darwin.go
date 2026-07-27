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
	if err == nil {
		groups := strings.Fields(string(out))
		for _, g := range groups {
			if g == "admin" || g == "wheel" {
				return true, nil
			}
		}
	}
	return false, nil
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
	}

	a.manager.Register(provider)
}
