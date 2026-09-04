//go:build linux

package engine

import (
	"os"
	"path/filepath"
	"strings"
)

// Autostart on Linux uses an XDG autostart entry, the mechanism every major
// desktop environment honours. The three functions here used to live in
// diagnostics_linux.go as unconditional `return nil` stubs, so the autostart
// toggle in the UI reported success and did nothing at all.

const linuxAutostartFile = "unbound.desktop"

// autostartPath returns the XDG autostart entry path. The spec puts it under
// $XDG_CONFIG_HOME, falling back to ~/.config.
func autostartPath() (string, error) {
	configHome := os.Getenv("XDG_CONFIG_HOME")
	if configHome == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		configHome = filepath.Join(home, ".config")
	}
	return filepath.Join(configHome, "autostart", linuxAutostartFile), nil
}

func EnableAutoStart() error {
	exePath, err := os.Executable()
	if err != nil {
		return err
	}
	if exePath, err = filepath.Abs(exePath); err != nil {
		return err
	}

	path, err := autostartPath()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}

	entry := "[Desktop Entry]\n" +
		"Type=Application\n" +
		"Name=Unbound\n" +
		"Comment=DPI bypass engine\n" +
		"Exec=" + exePath + " --tray\n" +
		"Terminal=false\n" +
		"X-GNOME-Autostart-enabled=true\n"

	if err := os.WriteFile(path, []byte(entry), 0o644); err != nil {
		return err
	}

	GetLogger().Infof("Startup", "Auto-start enabled via %s", path)
	return nil
}

func DisableAutoStart() error {
	path, err := autostartPath()
	if err != nil {
		return err
	}
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return err
	}
	GetLogger().Info("Startup", "Auto-start disabled")
	return nil
}

func IsAutoStartEnabled() (bool, error) {
	path, err := autostartPath()
	if err != nil {
		return false, err
	}
	if _, err := os.Stat(path); err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

func QueryAutoStartTaskRegistration() (*TaskRegistrationInfo, error) {
	path, err := autostartPath()
	if err != nil {
		return nil, err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return &TaskRegistrationInfo{Exists: false}, nil
		}
		return nil, err
	}
	content := string(data)
	var execLine string
	for _, line := range strings.Split(content, "\n") {
		if strings.HasPrefix(line, "Exec=") {
			execLine = strings.TrimPrefix(line, "Exec=")
			break
		}
	}
	return &TaskRegistrationInfo{
		Exists:     true,
		Executable: execLine,
		TaskState:  "Enabled",
	}, nil
}

func SelfHealAutoStart() (bool, error) {
	settings, err := GetSettings()
	if err != nil || !settings.AutoStart {
		return false, nil
	}
	info, err := QueryAutoStartTaskRegistration()
	if err != nil {
		return false, err
	}
	exe, err := os.Executable()
	if err != nil {
		return false, err
	}
	absExe, _ := filepath.Abs(exe)
	if !info.Exists || !strings.Contains(info.Executable, absExe) {
		if err := EnableAutoStart(); err != nil {
			return false, err
		}
		return true, nil
	}
	return false, nil
}
