//go:build windows

package main

import (
	"os"
	"syscall"

	"unbound/engine"
	"unbound/engine/providers"
)

// attachConsole attaches to the parent process console on Windows.
func attachConsole() {
	kernel32 := syscall.NewLazyDLL("kernel32.dll")
	attachConsole := kernel32.NewProc("AttachConsole")
	attachConsole.Call(uintptr(0xFFFFFFFF)) // ATTACH_PARENT_PROCESS = -1

	// Reopen stdout and stderr to ensure output works
	stdout, _ := os.OpenFile("CONOUT$", os.O_WRONLY, 0)
	if stdout != nil {
		os.Stdout = stdout
		os.Stderr = stdout
	}
}

// registerHeadlessProvider registers the Windows Zapret provider for CLI mode.
func registerHeadlessProvider(manager *providers.ProviderManager, assets *engine.AssetPaths, listsDir string, debugMode bool) {
	settings, _ := engine.GetSettings()
	gameFilter := true
	if settings != nil {
		gameFilter = settings.GameFilter
	}

	provider := providers.NewZapret2WindowsProvider(assets.BinDir, assets.LuaDir, listsDir, debugMode, gameFilter)

	// Register profiles
	registered := make(map[string]bool)
	for _, p := range engine.GetProfiles(assets.LuaDir) {
		provider.RegisterProfile(p.Name, p.Args)
		registered[p.Name] = true
	}
	for _, p := range engine.GetAdvancedProfiles(assets.LuaDir) {
		if !registered[p.Name] {
			provider.RegisterProfile(p.Name, p.Args)
			registered[p.Name] = true
		}
	}

	manager.Register(provider)
}
