//go:build windows

package engine

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"

	"golang.org/x/sys/windows"
)

func GetHiddenSysProcAttr() *syscall.SysProcAttr {
	return &syscall.SysProcAttr{
		HideWindow:    true,
		CreationFlags: 0x08000000,
	}
}

func quotePowerShellSingle(value string) string {
	return "'" + strings.ReplaceAll(value, "'", "''") + "'"
}

func buildDefenderExclusionScript(paths []string) string {
	quoted := make([]string, 0, len(paths))
	seen := make(map[string]struct{}, len(paths))
	for _, raw := range paths {
		cleaned := strings.TrimSpace(raw)
		if cleaned == "" {
			continue
		}
		cleaned = filepath.Clean(cleaned)
		key := strings.ToLower(cleaned)
		if _, exists := seen[key]; exists {
			continue
		}
		seen[key] = struct{}{}
		quoted = append(quoted, quotePowerShellSingle(cleaned))
	}
	return "Add-MpPreference -ExclusionPath @(" + strings.Join(quoted, ", ") + ")"
}

func defenderExclusionPaths() ([]string, error) {
	var paths []string

	if configDir, err := GetConfigDir(); err == nil && configDir != "" {
		paths = append(paths, configDir)
	}

	programData, err := systemProgramDataPath()
	if err != nil || strings.TrimSpace(programData) == "" {
		programData = os.Getenv("ProgramData")
	}
	if strings.TrimSpace(programData) != "" {
		paths = append(paths, filepath.Join(programData, ConfigDirName))
	}

	localAppData := os.Getenv("LOCALAPPDATA")
	if strings.TrimSpace(localAppData) == "" {
		if known, kErr := windows.KnownFolderPath(windows.FOLDERID_LocalAppData, 0); kErr == nil {
			localAppData = known
		}
	}
	if strings.TrimSpace(localAppData) != "" {
		paths = append(paths, filepath.Join(localAppData, ConfigDirName))
	}

	if len(paths) == 0 {
		return nil, fmt.Errorf("no Unbound runtime/config directories resolved for Defender exclusion")
	}
	return paths, nil
}

// AddDefenderExclusion adds Unbound's config and runtime directories
// (%ProgramData%\Unbound and %LOCALAPPDATA%\Unbound) to Windows Defender exclusions.
func AddDefenderExclusion() error {
	paths, err := defenderExclusionPaths()
	if err != nil {
		return err
	}
	script := buildDefenderExclusionScript(paths)
	cmd := exec.Command("powershell", "-NoProfile", "-NonInteractive", "-Command", script)
	cmd.SysProcAttr = GetHiddenSysProcAttr()
	return cmd.Run()
}
