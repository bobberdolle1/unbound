package engine

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
)

var settingsMu sync.Mutex

const (
	ConfigDirName      = "Unbound"
	CustomScriptName   = "custom_profile.lua"
	SettingsFileName   = "settings.json"
	DefaultLuaTemplate = `-- Custom Zapret Lua Bypass Strategy
-- Enter your custom DPI bypass logic here
-- This script will be loaded with --lua flag when "Custom Profile" is selected
--
-- Example structure:
-- if packet_type == "tls_client_hello" then
--     return "fake", "split:pos=1"
-- end
--
-- Refer to zapret-lib.lua and zapret-antidpi.lua for available functions

`
)

func GetConfigDir() (string, error) {
	userConfigDir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	configPath := filepath.Join(userConfigDir, ConfigDirName)

	if err := os.MkdirAll(configPath, 0755); err != nil {
		return "", err
	}

	return configPath, nil
}

func GetCustomScriptPath() (string, error) {
	configDir, err := GetConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(configDir, CustomScriptName), nil
}

func SaveCustomScript(content string) error {
	scriptPath, err := GetCustomScriptPath()
	if err != nil {
		return err
	}
	return os.WriteFile(scriptPath, []byte(content), 0644)
}

func LoadCustomScript() (string, error) {
	scriptPath, err := GetCustomScriptPath()
	if err != nil {
		return DefaultLuaTemplate, err
	}

	data, err := os.ReadFile(scriptPath)
	if err != nil {
		if os.IsNotExist(err) {
			return DefaultLuaTemplate, nil
		}
		return DefaultLuaTemplate, err
	}

	if len(data) == 0 {
		return DefaultLuaTemplate, nil
	}

	return string(data), nil
}

func GetSettingsPath() (string, error) {
	configDir, err := GetConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(configDir, SettingsFileName), nil
}

func GetSettings() (*Settings, error) {
	settingsMu.Lock()
	defer settingsMu.Unlock()
	return loadSettings()
}

func loadSettings() (*Settings, error) {
	settingsPath, err := GetSettingsPath()
	if err != nil {
		return getDefaultSettings(), err
	}
	data, err := os.ReadFile(settingsPath)
	if err != nil {
		if os.IsNotExist(err) {
			return getDefaultSettings(), nil
		}
		return getDefaultSettings(), err
	}
	var raw map[string]json.RawMessage
	_ = json.Unmarshal(data, &raw)
	var settings Settings
	if err := json.Unmarshal(data, &settings); err != nil {
		return getDefaultSettings(), err
	}
	// Legacy migration: settings written before autoStartProfile existed tied
	// the "Автозапуск" checkbox to both OS registration and profile startup.
	// Users who relied on that keep it; fresh installs start inert.
	if _, present := raw["autoStartProfile"]; !present && settings.AutoStart {
		settings.AutoStartProfile = true
	}
	if len(settings.AutoTuneTargets) == 0 {
		settings.AutoTuneTargets = []string{"youtube", "discord", "steam", "general"}
	}
	if settings.DiagnosticsMode == "" {
		settings.DiagnosticsMode = "quick"
	}
	if settings.AutoUpdatePolicy == "" {
		settings.AutoUpdatePolicy = "check_only"
	}
	return &settings, nil
}

func SaveSettings(settings *Settings) error {
	settingsMu.Lock()
	defer settingsMu.Unlock()
	return writeSettings(settings)
}

// ApplyAutoStartSetting registers or removes the OS-level autostart entry
// (scheduled task on Windows, launchd/xdg equivalents elsewhere). It is the
// exported seam used by the app layer; engine.SaveSettings deliberately
// stays pure persistence so that saving unrelated fields never rewrites the
// OS autostart registration — which on Windows means deleting and recreating
// a scheduled task and therefore requires elevation.
func ApplyAutoStartSetting(enable bool) error {
	return applyAutoStartSetting(enable)
}

func SaveLastProfile(profileName string) error {
	settingsMu.Lock()
	defer settingsMu.Unlock()
	settings, err := loadSettings()
	if err != nil {
		return err
	}
	settings.DefaultProfile = profileName
	return writeSettings(settings)
}

func writeSettings(settings *Settings) error {
	settingsPath, err := GetSettingsPath()
	if err != nil {
		return err
	}
	data, err := json.Marshal(settings)
	if err != nil {
		return err
	}
	return os.WriteFile(settingsPath, data, 0644)
}

func getDefaultSettings() *Settings {
	return &Settings{
		AutoStart:          false,
		StartMinimized:     false,
		AutoStartProfile:   false,
		DefaultProfile:     "",
		StartupProfileMode: "Последний использованный",
		GameFilter:         true,
		AutoUpdateEnabled:  true,
		ShowLogs:           true,
		AutoTuneTargets:    []string{"youtube", "discord", "steam", "general"},
		DiagnosticsMode:    "quick",
		AutoUpdatePolicy:   "check_only",
	}
}
