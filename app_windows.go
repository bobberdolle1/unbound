//go:build windows

package main

import (
	"unbound/engine"
	"unbound/engine/providers"
	"github.com/wailsapp/wails/v2/pkg/runtime"
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
	
	zapretProvider := providers.NewZapret2WindowsProvider(assets.BinDir, assets.LuaDir, listsDir, a.debugMode, gameFilter)
	
	// Register status callback for Wails events
	zapretProvider.SetStatusCallback(func(status providers.Status) {
		runtime.EventsEmit(a.ctx, "status_changed", status)
	})
	
	// Register built-in profiles (includes all reference profiles)
	registered := make(map[string]bool)
	for _, p := range engine.GetProfiles(assets.LuaDir) {
		zapretProvider.RegisterProfile(p.Name, p.Args)
		registered[p.Name] = true
	}
	for _, p := range engine.GetAdvancedProfiles(assets.LuaDir) {
		if !registered[p.Name] {
			zapretProvider.RegisterProfile(p.Name, p.Args)
			registered[p.Name] = true
		}
	}

	a.manager.Register(zapretProvider)
}
