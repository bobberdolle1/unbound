package engine

import (
	"os"
	"path/filepath"
)

const (
	ConfigDirName      = "Unbound"
	CustomScriptName   = "custom_profile.lua"
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
