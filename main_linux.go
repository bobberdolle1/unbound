//go:build linux

package main

import (
	"fmt"
	"os"

	"unbound/engine"
	"unbound/engine/providers"
)

// attachConsole is a no-op on Linux: the process already inherits a terminal.
func attachConsole() {}

// registerHeadlessProvider wires the bundled nfqws2 provider for `--cli`.
func registerHeadlessProvider(manager *providers.ProviderManager, assets *engine.AssetPaths, _ string, debugMode bool) {
	binPath, err := providers.ResolveEngineBinary(providers.LinuxEngineBinary, assets.BinDir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Ошибка: %v\n", err)
		fmt.Fprintln(os.Stderr, "Переустановите UNBOUND: встроенный nfqws2 отсутствует или повреждён.")
		os.Exit(1)
	}

	if debugMode {
		fmt.Printf("Движок: %s\n", binPath)
	}

	provider := providers.NewZapretLinuxProvider(binPath, assets.LuaDir, assets.EngineSHA256)

	manager.Register(provider)
}
