package engine

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
)

type ValidationResult struct {
	Valid    bool
	Errors   []string
	Warnings []string
}

type StartupValidator struct {
	assets *AssetPaths
}

func NewStartupValidator(assets *AssetPaths) *StartupValidator {
	return &StartupValidator{assets: assets}
}

// ValidateStartup performs comprehensive startup validation
func (v *StartupValidator) ValidateStartup() *ValidationResult {
	result := &ValidationResult{
		Valid:    true,
		Errors:   []string{},
		Warnings: []string{},
	}

	// 1. Check critical binaries
	v.validateBinaries(result)

	// 2. Check Lua scripts
	v.validateLuaScripts(result)

	// 3. Check WinDivert driver (Windows only)
	if runtime.GOOS == "windows" {
		v.validateWinDivertDriver(result)
	}

	// 4. Check lists directory
	v.validateLists(result)

	// 5. Check write permissions
	v.validatePermissions(result)

	if len(result.Errors) > 0 {
		result.Valid = false
	}

	return result
}

func (v *StartupValidator) validateBinaries(result *ValidationResult) {
	var requiredBinaries []string

	switch runtime.GOOS {
	case "windows":
		requiredBinaries = []string{"winws2.exe", "WinDivert.dll", "WinDivert64.sys"}
	case "linux":
		requiredBinaries = []string{"nfqws"}
	case "darwin":
		requiredBinaries = []string{"dvtws"}
	}

	for _, binary := range requiredBinaries {
		binPath := filepath.Join(v.assets.BinDir, binary)
		if _, err := os.Stat(binPath); os.IsNotExist(err) {
			result.Errors = append(result.Errors, fmt.Sprintf("Critical binary missing: %s", binary))
		}
	}
}

func (v *StartupValidator) validateLuaScripts(result *ValidationResult) {
	requiredScripts := []string{
		"zapret-lib.lua",
		"zapret-antidpi.lua",
		"init_vars.lua",
	}

	for _, script := range requiredScripts {
		scriptPath := filepath.Join(v.assets.LuaDir, script)
		if _, err := os.Stat(scriptPath); os.IsNotExist(err) {
			result.Errors = append(result.Errors, fmt.Sprintf("Critical Lua script missing: %s", script))
		}
	}
}

func (v *StartupValidator) validateWinDivertDriver(result *ValidationResult) {
	driverPath := filepath.Join(v.assets.BinDir, "WinDivert64.sys")
	if _, err := os.Stat(driverPath); os.IsNotExist(err) {
		result.Errors = append(result.Errors, "WinDivert driver (WinDivert64.sys) is missing")
		return
	}

	// Check if driver is signed (optional warning)
	// This is a basic check - full signature validation would require more complex code
	info, err := os.Stat(driverPath)
	if err == nil && info.Size() < 10000 {
		result.Warnings = append(result.Warnings, "WinDivert driver file size is suspiciously small")
	}
}

func (v *StartupValidator) validateLists(result *ValidationResult) {
	if _, err := os.Stat(v.assets.ListDir); os.IsNotExist(err) {
		result.Warnings = append(result.Warnings, "Lists directory not found - bypass lists may not work")
		return
	}

	// Check for at least some list files
	entries, err := os.ReadDir(v.assets.ListDir)
	if err != nil || len(entries) == 0 {
		result.Warnings = append(result.Warnings, "No bypass lists found - some profiles may not work correctly")
	}
}

func (v *StartupValidator) validatePermissions(result *ValidationResult) {
	// Test write access to temp directory
	testFile := filepath.Join(v.assets.BinDir, ".write_test")
	if err := os.WriteFile(testFile, []byte("test"), 0644); err != nil {
		result.Errors = append(result.Errors, fmt.Sprintf("No write permission to temp directory: %v", err))
	} else {
		os.Remove(testFile)
	}
}

// ValidateAdminPrivileges checks if the application is running with admin rights
func ValidateAdminPrivileges() (bool, error) {
	if runtime.GOOS == "windows" {
		return checkWindowsAdmin()
	}
	// For Linux/macOS, check if running as root
	return os.Geteuid() == 0, nil
}

func checkWindowsAdmin() (bool, error) {
	// This will be implemented in app_windows.go to avoid import cycles
	// For now, return a placeholder
	return false, fmt.Errorf("admin check not implemented")
}
