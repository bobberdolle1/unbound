//go:build linux

package main

import (
	"unbound/engine"
	"unbound/engine/providers"
)

func attachConsole() {
	// No-op on Linux
}

func registerHeadlessProvider(manager *providers.ProviderManager, assets *engine.AssetPaths, listsDir string, debugMode bool) {
	// No-op on Linux currently
}
