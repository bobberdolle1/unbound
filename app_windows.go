//go:build windows

package main

import (
	"unbound/engine"
	"unbound/engine/providers"
)

func registerOSProviders(a *App, assets *engine.AssetPaths) {
	a.manager.Register(providers.NewZapret2WindowsProvider(assets.BinDir, assets.LuaDir, assets.ListDir, a.debugMode))
}
