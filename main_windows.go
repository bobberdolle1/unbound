//go:build windows

package main

import (
	"os"
	"syscall"
	"unbound/engine"
	"unbound/engine/providers"
)

func attachConsole() {
	kernel32 := syscall.NewLazyDLL("kernel32.dll")
	attachConsoleProc := kernel32.NewProc("AttachConsole")
	attachConsoleProc.Call(uintptr(0xFFFFFFFF)) // ATTACH_PARENT_PROCESS = -1
	
	// Reopen stdout and stderr to ensure output works
	stdout, _ := os.OpenFile("CONOUT$", os.O_WRONLY, 0)
	if stdout != nil {
		os.Stdout = stdout
		os.Stderr = stdout
	}
}

func registerHeadlessProvider(manager *providers.ProviderManager, assets *engine.AssetPaths, listsDir string, debugMode bool) {
	provider := providers.NewZapret2WindowsProvider(assets.BinDir, assets.LuaDir, listsDir, debugMode, true)
	
	// Register built-in profiles (includes all reference profiles)
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
