//go:build windows
// +build windows

package engine

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
)



const (
	taskName        = "UnboundDPIBypass"
	taskDescription = "Unbound DPI Bypass Engine - Auto-start with elevated privileges"
)

func EnableAutoStart() error {
	logger := GetLogger()
	logger.Info("Startup", "Enabling auto-start")
	
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

	if err := DisableAutoStart(); err != nil {
		// Ignore error if task doesn't exist
		logger.Debugf("Startup", "DisableAutoStart returned: %v (ignored)", err)
	}

	username := os.Getenv("USERNAME")
	if username == "" {
		username = os.Getenv("USER")
	}

	args := []string{
		"/Create",
		"/TN", taskName,
		"/TR", fmt.Sprintf(`"%s"`, exePath),
		"/SC", "ONLOGON",
		"/RL", "HIGHEST",
		"/F",
	}

	if username != "" {
		args = append(args, "/RU", username)
	}

	logger.Infof("Startup", "Creating scheduled task: %s", taskName)
	cmd := exec.Command("schtasks.exe", args...)
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true, CreationFlags: CREATE_NO_WINDOW}
	output, err := cmd.CombinedOutput()
	if err != nil {
		logger.Errorf("Startup", "Failed to create scheduled task: %v, output: %s", err, string(output))
		return fmt.Errorf("failed to create scheduled task: %w\nOutput: %s", err, string(output))
	}

	logger.Info("Startup", "Auto-start enabled successfully")
	return nil
}

func DisableAutoStart() error {
	logger := GetLogger()
	logger.Info("Startup", "Disabling auto-start")
	
	cmd := exec.Command("schtasks.exe", "/Delete", "/TN", taskName, "/F")
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true, CreationFlags: CREATE_NO_WINDOW}
	output, err := cmd.CombinedOutput()
	if err != nil {
		outputStr := string(output)
		if strings.Contains(outputStr, "cannot find") || strings.Contains(outputStr, "does not exist") {
			logger.Debug("Startup", "Scheduled task does not exist (already disabled)")
			return nil
		}
		logger.Errorf("Startup", "Failed to delete scheduled task: %v, output: %s", err, outputStr)
		return fmt.Errorf("failed to delete scheduled task: %w\nOutput: %s", err, outputStr)
	}

	logger.Info("Startup", "Auto-start disabled successfully")
	return nil
}

func IsAutoStartEnabled() (bool, error) {
	logger := GetLogger()
	logger.Debug("Startup", "Checking auto-start status")
	
	cmd := exec.Command("schtasks.exe", "/Query", "/TN", taskName, "/FO", "LIST")
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true, CreationFlags: CREATE_NO_WINDOW}
	output, err := cmd.CombinedOutput()
	if err != nil {
		outputStr := string(output)
		if strings.Contains(outputStr, "cannot find") || strings.Contains(outputStr, "does not exist") {
			logger.Debug("Startup", "Auto-start is disabled (task not found)")
			return false, nil
		}
		logger.Warnf("Startup", "Failed to query scheduled task: %v", err)
		return false, fmt.Errorf("failed to query scheduled task: %w\nOutput: %s", err, outputStr)
	}

	outputStr := string(output)
	enabled := strings.Contains(outputStr, taskName)
	logger.Debugf("Startup", "Auto-start status: enabled=%v", enabled)
	return enabled, nil
}

func GetAutoStartTaskInfo() (map[string]string, error) {
	cmd := exec.Command("schtasks.exe", "/Query", "/TN", taskName, "/FO", "LIST", "/V")
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true, CreationFlags: CREATE_NO_WINDOW}
	output, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("failed to query task info: %w", err)
	}

	info := make(map[string]string)
	lines := strings.Split(string(output), "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		parts := strings.SplitN(line, ":", 2)
		if len(parts) == 2 {
			key := strings.TrimSpace(parts[0])
			value := strings.TrimSpace(parts[1])
			info[key] = value
		}
	}

	return info, nil
}
