package engine

import (
	"encoding/json"
	"os"
	"path/filepath"

	"golang.org/x/sys/windows/registry"
)

const (
	ConfigDirName      = "Unbound"
	CustomScriptName   = "custom_profile.lua"
	SettingsFileName   = "settings.json"
	RegistryRunKey     = `Software\Microsoft\Windows\CurrentVersion\Run`
	RegistryAppName    = "Unbound"
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

type Settings struct {
	AutoStart          bool   `json:"autoStart"`
	StartMinimized     bool   `json:"startMinimized"`
	DefaultProfile     string `json:"defaultProfile"`
	StartupProfileMode string `json:"startupProfileMode"`
	GameFilter         bool   `json:"gameFilter"`
	AutoUpdateEnabled  bool   `json:"autoUpdateEnabled"`
	ShowLogs           bool   `json:"showLogs"`
}

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
	settingsPath, err := GetSettingsPath()
	if err != nil {
		return err
	}

	data, err := json.Marshal(settings)
	if err != nil {
		return err
	}

	if err := os.WriteFile(settingsPath, data, 0644); err != nil {
		return err
	}

	if err := applyAutoStartSetting(settings.AutoStart); err != nil {
		return err
	}

	return nil
}

func getDefaultSettings() *Settings {
	return &Settings{
		AutoStart:          false,
		StartMinimized:     false,
		DefaultProfile:     "Unbound Ultimate (God Mode)",
		StartupProfileMode: "Last Used",
		GameFilter:         true,
		AutoUpdateEnabled:  true,
		ShowLogs:           true,
	}
}

func applyAutoStartSetting(enable bool) error {
	k, err := registry.OpenKey(registry.CURRENT_USER, RegistryRunKey, registry.ALL_ACCESS)
	if err != nil {
		return err
	}
	defer k.Close()

	if enable {
		exePath, err := os.Executable()
		if err != nil {
			return err
		}

		cmdLine := `"` + exePath + `" --tray`
		return k.SetStringValue(RegistryAppName, cmdLine)
	} else {
		err := k.DeleteValue(RegistryAppName)
		if err != nil && err != registry.ErrNotExist {
			return err
		}
		return nil
	}
}
