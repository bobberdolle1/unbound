//go:build windows

package main

import (
	"unbound/engine"
	"unbound/engine/providers"
)

func registerOSProviders(a *App, assets *engine.AssetPaths) {
	settings, _ := engine.GetSettings()
	gameFilter := true
	if settings != nil {
		gameFilter = settings.GameFilter
	}
	
	listsDir, err := engine.GetListsDir()
	if err != nil {
		listsDir = assets.ListDir
	}
	
	a.manager.Register(providers.NewZapret2WindowsProvider(assets.BinDir, assets.LuaDir, listsDir, a.debugMode, gameFilter))
}
