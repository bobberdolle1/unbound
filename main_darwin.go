//go:build darwin

package main

import (
	"unbound/engine"
	"unbound/engine/providers"
)

// attachConsole is a no-op on macOS since the console is already available.
func attachConsole() {}

// registerHeadlessProvider registers the macOS SpoofDPI provider for CLI mode.
func registerHeadlessProvider(manager *providers.ProviderManager, assets *engine.AssetPaths, listsDir string, debugMode bool) {
	provider := providers.NewZapretMacOSProvider(assets.BinDir)

	// Register profiles
	if cbProvider, ok := provider.(providers.BypassProviderWithCallbacks); ok {
		for _, p := range engine.GetProfiles(assets.LuaDir) {
			cbProvider.RegisterProfile(p.Name, p.Args)
		}
		for _, p := range engine.GetAdvancedProfiles(assets.LuaDir) {
			cbProvider.RegisterProfile(p.Name, p.Args)
		}
	}

	manager.Register(provider)
}
