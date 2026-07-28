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

// registerHeadlessProvider wires the macOS engine provider for `--cli` mode.
func registerHeadlessProvider(manager *providers.ProviderManager, assets *engine.AssetPaths, listsDir string, debugMode bool) {
	// The provider used to be handed assets.BinDir and join "nfqws" onto it,
	// so only a bundled binary could ever be found - and the macOS build does
	// not bundle one, making CLI mode unusable on a Homebrew install.
	binPath, err := providers.ResolveEngineBinary(providers.MacOSEngineBinary, assets.BinDir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Ошибка: %v\n", err)
		fmt.Fprintln(os.Stderr, "Установите zapret (например, brew install zapret) или положите tpws рядом с исполняемым файлом.")
		os.Exit(1)
	}

	if debugMode {
		fmt.Printf("Движок: %s\n", binPath)
	}

	provider := providers.NewZapretMacOSProvider(binPath)

	manager.Register(provider)
}
