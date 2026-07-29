//go:build linux

package main

import (
	"os"

	"unbound/engine"
	"unbound/engine/providers"

	"github.com/wailsapp/wails/v2/pkg/runtime"
)

// checkAdminPrivileges reports whether the process can install netfilter rules
// and open an NFQUEUE.
//
// This previously returned (true, nil) unconditionally, so an unprivileged run
// sailed past the privilege gate and failed later inside iptables with an
// opaque "operation not permitted" instead of telling the user to use sudo.
func checkAdminPrivileges() (bool, error) {
	return os.Geteuid() == 0, nil
}

// registerOSProviders wires the bundled Linux nfqws2 provider into the manager.
func registerOSProviders(a *App, assets *engine.AssetPaths) {
	logger := engine.GetLogger()

	binPath, err := providers.ResolveEngineBinary(providers.LinuxEngineBinary, assets.BinDir)
	if err != nil {
		logger.Errorf("App", "%v", err)
		engine.GetNotificationManager().Error(
			"Движок не найден",
			"Встроенный nfqws2 не найден или не прошёл проверку целостности.",
		)
		return
	}
	logger.Infof("App", "Using nfqws2 binary at %s", binPath)

	provider := providers.NewZapretLinuxProvider(binPath, assets.LuaDir, assets.EngineSHA256)

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
