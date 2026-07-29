//go:build windows

package engine

import (
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
)

func GetHiddenSysProcAttr() *syscall.SysProcAttr {
	return &syscall.SysProcAttr{
		HideWindow:    true,
		CreationFlags: 0x08000000,
	}
}

// AddDefenderExclusion adds Unbound's config and app directories to Windows Defender exclusions
func AddDefenderExclusion() error {
	configDir, err := GetConfigDir()
	if err != nil {
		return err
	}
	tempDir := filepath.Join(os.TempDir(), "clearflow")

	script := `Add-MpPreference -ExclusionPath "` + configDir + `", "` + tempDir + `"`
	cmd := exec.Command("powershell", "-NoProfile", "-NonInteractive", "-Command", script)
	cmd.SysProcAttr = GetHiddenSysProcAttr()
	return cmd.Run()
}
