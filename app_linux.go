//go:build linux

package main

import (
	"unbound/engine"
)

func checkAdminPrivileges() (bool, error) {
	// Standard Linux way: check if UID is 0
	return true, nil // Simplified, or check os.Getuid()
}

func registerOSProviders(a *App, assets *engine.AssetPaths) {
	// Currently no Linux-specific providers implemented.
}

func GetDefaultEngineName() string {
	return "CLI Mode (Linux)"
}

func GetDiscordCacheDirsToClean() []string {
	return []string{}
}
