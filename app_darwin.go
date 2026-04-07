//go:build darwin

package main

import (
	"os/exec"
	"strings"
	"unbound/engine"
	"unbound/engine/providers"

	"github.com/wailsapp/wails/v2/pkg/runtime"
)

// checkAdminPrivileges on macOS always returns true because we use
// osascript with administrator privileges at runtime when needed.
func checkAdminPrivileges() (bool, error) {
	// macOS uses osascript for privilege escalation at runtime.
	// The app itself doesn't need to be launched as root.
	return true, nil
}

// checkAdminPrivilegesReal checks if the current user can execute osascript
// with administrator privileges (i.e., has admin rights).
func checkAdminPrivilegesReal() (bool, error) {
	cmd := exec.Command("id", "-Gn")
	out, err := cmd.Output()
	if err != nil {
		return false, err
	}
	// Check if user is in the admin group
	return strings.Contains(string(out), "admin"), nil
}

func registerOSProviders(a *App, assets *engine.AssetPaths) {
	macosProvider := providers.NewZapretMacOSProvider(assets.BinDir)

	// Use the callback interface
	if cbProvider, ok := macosProvider.(providers.BypassProviderWithCallbacks); ok {
		cbProvider.SetStatusCallback(func(status providers.Status) {
			runtime.EventsEmit(a.ctx, "status_changed", status)
		})
		cbProvider.SetLogCallback(func(log string) {
			runtime.EventsEmit(a.ctx, "engine_log", log)
		})
	}

	// Register built-in profiles
	for _, p := range engine.GetProfiles(assets.LuaDir) {
		macosProvider.(providers.BypassProviderWithCallbacks).RegisterProfile(p.Name, p.Args)
	}
	for _, p := range engine.GetAdvancedProfiles(assets.LuaDir) {
		macosProvider.(providers.BypassProviderWithCallbacks).RegisterProfile(p.Name, p.Args)
	}

	a.manager.Register(macosProvider)
}

// GetDefaultEngineName returns the default engine name for the tray menu on macOS.
func GetDefaultEngineName() string {
	return "SpoofDPI (macOS)"
}

// GetDiscordCacheDirsToClean returns the Discord cache directories to clean on macOS.
func GetDiscordCacheDirsToClean() []string {
	return engine.GetDiscordCacheDirs()
}

// createAutoTuneProvider creates a provider for AutoTune testing on macOS.
func (a *App) createAutoTuneProvider() (providers.BypassProvider, []engine.Profile) {
	assets, _ := engine.ExtractAssets()
	provider := providers.NewZapretMacOSProvider(assets.BinDir)

	// Register all profiles
	allProfiles := append(engine.GetProfiles(assets.LuaDir), engine.GetAdvancedProfiles(assets.LuaDir)...)
	cbProvider := provider.(providers.BypassProviderWithCallbacks)
	for _, p := range allProfiles {
		cbProvider.RegisterProfile(p.Name, p.Args)
	}

	return provider, allProfiles
}
