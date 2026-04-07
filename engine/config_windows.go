//go:build windows

package engine

import (
	"os"

	"golang.org/x/sys/windows/registry"
)

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
