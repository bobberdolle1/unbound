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

// registerOSProviders wires the Linux nfqws provider into the manager. It used
// to be an empty function, so the Linux GUI started with no engines registered
// at all: the engine dropdown was empty and StartEngine always failed with
// "engine not found".
func registerOSProviders(a *App, assets *engine.AssetPaths) {
	logger := engine.GetLogger()

	binPath, err := providers.ResolveEngineBinary(providers.LinuxEngineBinary, assets.BinDir)
	if err != nil {
		logger.Errorf("App", "%v", err)
		engine.GetNotificationManager().Error(
			"Движок не найден",
			"Не найден бинарник nfqws. Установите пакет zapret или положите nfqws рядом с приложением.",
		)
		return
	}
	logger.Infof("App", "Using nfqws binary at %s", binPath)

	listsDir, err := engine.GetListsDir()
	if err != nil {
		listsDir = assets.ListDir
	}

	provider := providers.NewZapretLinuxProvider(binPath, listsDir)

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
