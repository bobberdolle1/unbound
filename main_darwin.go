//go:build darwin

package main

import (
	"fmt"
	"os"

	"unbound/engine"
	"unbound/engine/providers"
)

// attachConsole is a no-op on macOS: the process already inherits a terminal.
func attachConsole() {}

// registerHeadlessProvider wires the macOS engine provider for --cli mode.
// assets.BinDir is the private runtime directory populated from the verified
// embedded Universal tpws asset; ResolveEngineBinary consults it before any
// portable, PATH, Homebrew, or distribution-prefix diagnostic fallback.
func registerHeadlessProvider(manager *providers.ProviderManager, assets *engine.AssetPaths, listsDir string, debugMode bool) {
	binPath, err := providers.ResolveEngineBinary(providers.MacOSEngineBinary, assets.BinDir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Ошибка: %v\n", err)
		fmt.Fprintln(os.Stderr, "Проверенный встроенный tpws недоступен.")
		os.Exit(1)
	}

	if debugMode {
		fmt.Printf("Движок: %s\n", binPath)
	}

	provider := providers.NewZapretMacOSProvider(binPath)

	manager.Register(provider)
}
