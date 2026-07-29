//go:build windows

package engine

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"golang.org/x/sys/windows"
)

const adminRuntimeDACL = "D:P(A;OICI;FA;;;SY)(A;OICI;FA;;;BA)"

var systemProgramDataPath = func() (string, error) {
	return windows.KnownFolderPath(windows.FOLDERID_ProgramData, 0)
}

func prepareAssetRuntime() (*assetRuntimeWorkspace, error) {
	if !windows.GetCurrentProcessToken().IsElevated() {
		// Non-elevated commands cannot load WinDivert. A unique private directory
		// remains useful for read-only commands and tests without weakening the
		// elevated execution path below.
		return newPrivateTempWorkspace()
	}

	programData, err := systemProgramDataPath()
	if err != nil {
		return nil, fmt.Errorf("resolve system ProgramData: %w", err)
	}

	baseDir := filepath.Join(programData, "Unbound")
	if err := os.MkdirAll(baseDir, 0700); err != nil {
		return nil, fmt.Errorf("create protected runtime parent: %w", err)
	}
	if reparse, err := isWindowsReparsePoint(baseDir); err != nil {
		return nil, fmt.Errorf("inspect protected runtime parent: %w", err)
	} else if reparse {
		return nil, fmt.Errorf("refusing reparse-point runtime parent %q", baseDir)
	}
	if err := protectWindowsDirectory(baseDir); err != nil {
		return nil, fmt.Errorf("protect runtime parent: %w", err)
	}
	cleanupStaleWindowsRuntimes(baseDir)

	runtimeDir, err := os.MkdirTemp(baseDir, fmt.Sprintf("runtime-%d-", os.Getpid()))
	if err != nil {
		return nil, fmt.Errorf("create unique runtime directory: %w", err)
	}
	cleanup := func() { _ = os.RemoveAll(runtimeDir) }
	if err := protectWindowsDirectory(runtimeDir); err != nil {
		cleanup()
		return nil, fmt.Errorf("protect runtime directory: %w", err)
	}

	return &assetRuntimeWorkspace{
		stagingDir: runtimeDir,
		finalDir:   runtimeDir,
		cleanup:    cleanup,
		commit:     func() error { return nil },
	}, nil
}
func cleanupStaleWindowsRuntimes(baseDir string) {
	entries, err := os.ReadDir(baseDir)
	if err != nil {
		return
	}
	for _, entry := range entries {
		if !entry.IsDir() || !strings.HasPrefix(entry.Name(), "runtime-") {
			continue
		}
		pidText := strings.SplitN(strings.TrimPrefix(entry.Name(), "runtime-"), "-", 2)[0]
		pid, err := strconv.ParseUint(pidText, 10, 32)
		if err != nil || windowsProcessAlive(uint32(pid)) {
			continue
		}
		_ = os.RemoveAll(filepath.Join(baseDir, entry.Name()))
	}
}

func windowsProcessAlive(pid uint32) bool {
	handle, err := windows.OpenProcess(windows.PROCESS_QUERY_LIMITED_INFORMATION, false, pid)
	if err != nil {
		// Access denied is safer to treat as a live process; only the explicit
		// "process does not exist" error permits cleanup.
		return err != windows.ERROR_INVALID_PARAMETER
	}
	_ = windows.CloseHandle(handle)
	return true
}

func protectWindowsDirectory(path string) error {
	sd, err := windows.SecurityDescriptorFromString(adminRuntimeDACL)
	if err != nil {
		return err
	}
	dacl, _, err := sd.DACL()
	if err != nil {
		return err
	}
	adminSID, err := windows.CreateWellKnownSid(windows.WinBuiltinAdministratorsSid)
	if err != nil {
		return err
	}
	if err := windows.SetNamedSecurityInfo(
		path,
		windows.SE_FILE_OBJECT,
		windows.OWNER_SECURITY_INFORMATION|windows.DACL_SECURITY_INFORMATION|windows.PROTECTED_DACL_SECURITY_INFORMATION,
		adminSID,
		nil,
		dacl,
		nil,
	); err != nil {
		return err
	}
	return verifyWindowsDirectoryProtection(path, adminSID)
}

func verifyWindowsDirectoryProtection(path string, expectedOwner *windows.SID) error {
	sd, err := windows.GetNamedSecurityInfo(
		path,
		windows.SE_FILE_OBJECT,
		windows.OWNER_SECURITY_INFORMATION|windows.DACL_SECURITY_INFORMATION,
	)
	if err != nil {
		return err
	}
	if sd == nil {
		return fmt.Errorf("directory has no security descriptor")
	}
	owner, _, err := sd.Owner()
	if err != nil {
		return err
	}
	if owner == nil || !owner.Equals(expectedOwner) {
		return fmt.Errorf("runtime owner is not BUILTIN\\Administrators")
	}
	dacl, _, err := sd.DACL()
	if err != nil {
		return err
	}
	if dacl == nil || dacl.AceCount != 2 {
		return fmt.Errorf("runtime DACL is missing or unexpected")
	}
	control, _, err := sd.Control()
	if err != nil {
		return err
	}
	if control&windows.SE_DACL_PROTECTED == 0 {
		return fmt.Errorf("runtime DACL still inherits permissions")
	}
	return nil
}

func isWindowsReparsePoint(path string) (bool, error) {
	pathPtr, err := windows.UTF16PtrFromString(path)
	if err != nil {
		return false, err
	}
	attrs, err := windows.GetFileAttributes(pathPtr)
	if err != nil {
		return false, err
	}
	return attrs&windows.FILE_ATTRIBUTE_REPARSE_POINT != 0, nil
}
