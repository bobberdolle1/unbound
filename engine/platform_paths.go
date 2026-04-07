package engine

import (
	"os"
	"path/filepath"
	"runtime"
)

// GetDiscordCacheDir returns the platform-specific Discord cache directory.
// Windows: %APPDATA%\discord
// macOS:   ~/Library/Application Support/discord
// Linux:   ~/.config/discord
func GetDiscordCacheDir() string {
	switch runtime.GOOS {
	case "windows":
		appData := os.Getenv("APPDATA")
		return filepath.Join(appData, "discord")

	case "darwin":
		homeDir, err := os.UserHomeDir()
		if err != nil {
			return ""
		}
		return filepath.Join(homeDir, "Library", "Application Support", "discord")

	default: // linux
		homeDir, err := os.UserHomeDir()
		if err != nil {
			return ""
		}
		return filepath.Join(homeDir, ".config", "discord")
	}
}

// GetDiscordCacheDirs returns all cache dirs to clean (Cache, Code Cache, GPUCache).
func GetDiscordCacheDirs() []string {
	base := GetDiscordCacheDir()
	if base == "" {
		return nil
	}
	return []string{
		filepath.Join(base, "Cache"),
		filepath.Join(base, "Code Cache"),
		filepath.Join(base, "GPUCache"),
	}
}
