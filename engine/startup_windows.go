//go:build windows
// +build windows

package engine

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

const (
	taskName        = "UnboundDPIBypass"
	taskDescription = "Unbound DPI Bypass Engine - Auto-start with elevated privileges"
)

func EnableAutoStart() error {
	exePath, err := os.Executable()
	if err != nil {
		return fmt.Errorf("failed to get executable path: %w", err)
	}

	exePath, err = filepath.Abs(exePath)
	if err != nil {
		return fmt.Errorf("failed to resolve absolute path: %w", err)
	}

	if err := DisableAutoStart(); err != nil {
		// Ignore error if task doesn't exist
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

	cmd := exec.Command("schtasks.exe", args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to create scheduled task: %w\nOutput: %s", err, string(output))
	}

	return nil
}

func DisableAutoStart() error {
	cmd := exec.Command("schtasks.exe", "/Delete", "/TN", taskName, "/F")
	output, err := cmd.CombinedOutput()
	if err != nil {
		outputStr := string(output)
		if strings.Contains(outputStr, "cannot find") || strings.Contains(outputStr, "does not exist") {
			return nil
		}
		return fmt.Errorf("failed to delete scheduled task: %w\nOutput: %s", err, outputStr)
	}

	return nil
}

func IsAutoStartEnabled() (bool, error) {
	cmd := exec.Command("schtasks.exe", "/Query", "/TN", taskName, "/FO", "LIST")
	output, err := cmd.CombinedOutput()
	if err != nil {
		outputStr := string(output)
		if strings.Contains(outputStr, "cannot find") || strings.Contains(outputStr, "does not exist") {
			return false, nil
		}
		return false, fmt.Errorf("failed to query scheduled task: %w\nOutput: %s", err, outputStr)
	}

	outputStr := string(output)
	return strings.Contains(outputStr, taskName), nil
}

func GetAutoStartTaskInfo() (map[string]string, error) {
	cmd := exec.Command("schtasks.exe", "/Query", "/TN", taskName, "/FO", "LIST", "/V")
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
