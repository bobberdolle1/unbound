//go:build darwin
// +build darwin

package engine

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

const (
	macOSLaunchAgentsPlist = `<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
    <key>Label</key>
    <string>com.bobberdolle1.unbound</string>
    <key>ProgramArguments</key>
    <array>
        <string>%s</string>
        <string>--tray</string>
    </array>
    <key>RunAtLoad</key>
    <true/>
    <key>KeepAlive</key>
    <false/>
    <key>StandardOutPath</key>
    <string>/dev/null</string>
    <key>StandardErrorPath</key>
    <string>/dev/null</string>
</dict>
</plist>
`
)

// EnableAutoStart creates a launchd plist file in ~/Library/LaunchAgents/
func EnableAutoStart() error {
	logger := GetLogger()
	logger.Info("Startup", "Enabling auto-start via launchd")

	exePath, err := os.Executable()
	if err != nil {
		logger.Errorf("Startup", "Failed to get executable path: %v", err)
		return fmt.Errorf("failed to get executable path: %w", err)
	}

	exePath, err = filepath.Abs(exePath)
	if err != nil {
		logger.Errorf("Startup", "Failed to resolve absolute path: %v", err)
		return fmt.Errorf("failed to resolve absolute path: %w", err)
	}

	homeDir, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("failed to get home directory: %w", err)
	}

	launchAgentsDir := filepath.Join(homeDir, "Library", "LaunchAgents")
	if err := os.MkdirAll(launchAgentsDir, 0755); err != nil {
		return fmt.Errorf("failed to create LaunchAgents directory: %w", err)
	}

	plistPath := filepath.Join(launchAgentsDir, MacOSPlistFilename)
	plistContent := fmt.Sprintf(macOSLaunchAgentsPlist, exePath)

	if err := os.WriteFile(plistPath, []byte(plistContent), 0644); err != nil {
		logger.Errorf("Startup", "Failed to write plist: %v", err)
		return fmt.Errorf("failed to write plist: %w", err)
	}

	// Load the plist into launchd
	cmd := exec.Command("launchctl", "load", plistPath)
	output, err := cmd.CombinedOutput()
	if err != nil {
		// It's OK if launchctl fails (e.g., already loaded)
		logger.Debugf("Startup", "launchctl load returned: %v, output: %s", err, string(output))
	}

	logger.Info("Startup", "Auto-start enabled successfully via launchd")
	return nil
}

// DisableAutoStart removes the launchd plist file and unloads it from launchd.
func DisableAutoStart() error {
	logger := GetLogger()
	logger.Info("Startup", "Disabling auto-start via launchd")

	homeDir, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("failed to get home directory: %w", err)
	}

	plistPath := filepath.Join(homeDir, "Library", "LaunchAgents", MacOSPlistFilename)

	// Unload from launchd first
	cmd := exec.Command("launchctl", "unload", plistPath)
	output, err := cmd.CombinedOutput()
	if err != nil {
		outputStr := string(output)
		if !strings.Contains(outputStr, "Could not find specified service") {
			logger.Debugf("Startup", "launchctl unload returned: %v, output: %s", err, outputStr)
		}
	}

	// Remove the plist file
	if err := os.Remove(plistPath); err != nil {
		if os.IsNotExist(err) {
			logger.Debug("Startup", "Plist file does not exist (already disabled)")
			return nil
		}
		logger.Errorf("Startup", "Failed to remove plist: %v", err)
		return fmt.Errorf("failed to remove plist: %w", err)
	}

	logger.Info("Startup", "Auto-start disabled successfully")
	return nil
}

// IsAutoStartEnabled checks if the launchd plist file exists and is loaded.
func IsAutoStartEnabled() (bool, error) {
	logger := GetLogger()
	logger.Debug("Startup", "Checking auto-start status via launchd")

	homeDir, err := os.UserHomeDir()
	if err != nil {
		return false, fmt.Errorf("failed to get home directory: %w", err)
	}

	plistPath := filepath.Join(homeDir, "Library", "LaunchAgents", MacOSPlistFilename)

	// Check if plist file exists
	if _, err := os.Stat(plistPath); os.IsNotExist(err) {
		logger.Debug("Startup", "Auto-start is disabled (plist not found)")
		return false, nil
	}

	// Check if loaded in launchd
	cmd := exec.Command("launchctl", "list", "com.bobberdolle1.unbound")
	output, err := cmd.CombinedOutput()
	if err != nil {
		outputStr := string(output)
		if strings.Contains(outputStr, "Could not find") || strings.Contains(outputStr, "not found") {
			logger.Debug("Startup", "Auto-start is disabled (not loaded in launchd)")
			return false, nil
		}
		// File exists but not loaded - still count as enabled
		logger.Debugf("Startup", "launchctl list returned: %v", err)
		return true, nil
	}

	logger.Debug("Startup", "Auto-start is enabled (plist exists and loaded)")
	return true, nil
}
