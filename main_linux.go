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

// registerHeadlessProvider wires the nfqws provider for `--cli` mode.
//
// This was an empty function, so `unbound --cli` on Linux registered no
// engines and died with "engine not found: Zapret 2 (winws)" - the name of the
// Windows engine, on a Linux host.
func registerHeadlessProvider(manager *providers.ProviderManager, assets *engine.AssetPaths, listsDir string, debugMode bool) {
	binPath, err := providers.ResolveEngineBinary(providers.LinuxEngineBinary, assets.BinDir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Ошибка: %v\n", err)
		fmt.Fprintln(os.Stderr, "Установите zapret (пакет с nfqws) или положите nfqws рядом с исполняемым файлом.")
		os.Exit(1)
	}

	if debugMode {
		fmt.Printf("Движок: %s\n", binPath)
	}

	provider := providers.NewZapretLinuxProvider(binPath, listsDir)

	if cb, ok := provider.(providers.BypassProviderWithCallbacks); ok {
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

	manager.Register(provider)
}
