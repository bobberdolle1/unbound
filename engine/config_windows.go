//go:build windows

package engine

import (
	"golang.org/x/sys/windows/registry"
)

const (
	RegistryRunKey  = `Software\Microsoft\Windows\CurrentVersion\Run`
	RegistryAppName = "Unbound"
)

func applyAutoStartSetting(enable bool) error {
	// Clean up any legacy registry autorun entry to prevent un-elevated launch conflicts
	if k, err := registry.OpenKey(registry.CURRENT_USER, RegistryRunKey, registry.SET_VALUE); err == nil {
		_ = k.DeleteValue(RegistryAppName)
		_ = k.Close()
	}

	// On Windows, WinDivert driver requires administrative privileges (/RL HIGHEST),
	// which is managed via Windows Task Scheduler in EnableAutoStart / DisableAutoStart.
	if enable {
		return EnableAutoStart()
	}
	return DisableAutoStart()
}
