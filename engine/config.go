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
	var settings Settings
	if err := json.Unmarshal(data, &settings); err != nil {
		return getDefaultSettings(), err
	}
	return &settings, nil
}

func SaveSettings(settings *Settings) error {
	settingsMu.Lock()
	err := writeSettings(settings)
	settingsMu.Unlock()
	if err != nil {
		return err
	}
	return applyAutoStartSetting(settings.AutoStart)
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
		DefaultProfile:     "",
		StartupProfileMode: "Последний использованный",
		GameFilter:         true,
		AutoUpdateEnabled:  true,
		ShowLogs:           true,
	}
}
