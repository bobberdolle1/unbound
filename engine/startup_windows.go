//go:build windows
// +build windows

package engine

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"unicode/utf16"
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

	taskCmd := fmt.Sprintf(`"%s"`, exePath)
	settings, _ := GetSettings()
	if settings != nil && settings.StartMinimized {
		taskCmd = fmt.Sprintf(`"%s" --tray`, exePath)
	}

	args := []string{
		"/Create",
		"/TN", taskName,
		"/TR", taskCmd,
		"/SC", "ONLOGON",
		"/RL", "HIGHEST",
		"/F",
	}

	if username != "" {
		args = append(args, "/RU", username)
	}

	logger.Infof("Startup", "Creating scheduled task: %s", taskName)
	cmd := exec.Command("schtasks.exe", args...)
	cmd.SysProcAttr = GetHiddenSysProcAttr()
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
	cmd.SysProcAttr = GetHiddenSysProcAttr()
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
	cmd.SysProcAttr = GetHiddenSysProcAttr()
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
	cmd.SysProcAttr = GetHiddenSysProcAttr()
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

// QueryAutoStartTaskRegistration returns the parsed executable and arguments
// registered with Windows Task Scheduler for UnboundDPIBypass.
func QueryAutoStartTaskRegistration() (*TaskRegistrationInfo, error) {
	cmd := exec.Command("schtasks.exe", "/Query", "/TN", taskName, "/XML")
	cmd.SysProcAttr = GetHiddenSysProcAttr()
	output, err := cmd.CombinedOutput()
	if err != nil {
		outputStr := string(output)
		if strings.Contains(outputStr, "cannot find") ||
			strings.Contains(outputStr, "does not exist") ||
			strings.Contains(outputStr, "0x80070002") {
			return &TaskRegistrationInfo{Exists: false}, nil
		}
		// Fallback to LIST /V if XML query fails
		return queryTaskRegistrationViaList()
	}

	xmlStr := decodeWindowsEncoding(output)
	cmdPath, args, ok := parseTaskXML(xmlStr)
	if !ok {
		return queryTaskRegistrationViaList()
	}

	state := extractXMLTag(xmlStr, "ScheduledTaskState")
	if state == "" {
		state = "Ready"
	}

	rawCmd := cmdPath
	if args != "" {
		rawCmd = fmt.Sprintf(`"%s" %s`, cmdPath, args)
	}

	return &TaskRegistrationInfo{
		Exists:     true,
		Executable: cleanExecutablePath(cmdPath),
		Arguments:  strings.TrimSpace(args),
		RawCommand: rawCmd,
		TaskState:  state,
	}, nil
}

// queryTaskRegistrationViaList falls back to schtasks /FO LIST /V when XML is unavailable.
func queryTaskRegistrationViaList() (*TaskRegistrationInfo, error) {
	info, err := GetAutoStartTaskInfo()
	if err != nil {
		if strings.Contains(err.Error(), "cannot find") || strings.Contains(err.Error(), "does not exist") {
			return &TaskRegistrationInfo{Exists: false}, nil
		}
		return nil, err
	}

	var rawToRun string
	for k, v := range info {
		kLower := strings.ToLower(k)
		if strings.Contains(kLower, "task to run") || strings.Contains(kLower, "задача для выполнения") {
			rawToRun = v
			break
		}
	}
	if rawToRun == "" {
		// Search for any line with an executable
		for _, v := range info {
			if strings.Contains(strings.ToLower(v), ".exe") {
				rawToRun = v
				break
			}
		}
	}

	if rawToRun == "" {
		return &TaskRegistrationInfo{Exists: true, TaskState: info["Status"]}, nil
	}

	cmdPath, args := parseCommandLine(rawToRun)
	return &TaskRegistrationInfo{
		Exists:     true,
		Executable: cleanExecutablePath(cmdPath),
		Arguments:  strings.TrimSpace(args),
		RawCommand: rawToRun,
		TaskState:  info["Status"],
	}, nil
}

// parseTaskXML extracts <Command> and <Arguments> from scheduled task XML.
func parseTaskXML(xmlStr string) (cmd string, args string, ok bool) {
	cmd = extractXMLTag(xmlStr, "Command")
	if cmd == "" {
		return "", "", false
	}
	args = extractXMLTag(xmlStr, "Arguments")
	return cmd, args, true
}

func extractXMLTag(xmlStr, tag string) string {
	openTag := "<" + tag + ">"
	closeTag := "</" + tag + ">"
	idx := strings.Index(xmlStr, openTag)
	if idx == -1 {
		return ""
	}
	start := idx + len(openTag)
	end := strings.Index(xmlStr[start:], closeTag)
	if end == -1 {
		return ""
	}
	return strings.TrimSpace(xmlStr[start : start+end])
}

func decodeWindowsEncoding(data []byte) string {
	if len(data) >= 2 {
		// Check for UTF-16LE BOM or null bytes pattern
		if (data[0] == 0xFF && data[1] == 0xFE) || strings.Contains(string(data), "\x00") {
			// UTF-16LE
			start := 0
			if data[0] == 0xFF && data[1] == 0xFE {
				start = 2
			}
			u16 := make([]uint16, (len(data)-start)/2)
			for i := range u16 {
				u16[i] = uint16(data[start+i*2]) | (uint16(data[start+i*2+1]) << 8)
			}
			return string(utf16.Decode(u16))
		}
	}
	return string(data)
}

func cleanExecutablePath(p string) string {
	p = strings.TrimSpace(p)
	p = strings.Trim(p, `"`)
	p = strings.Trim(p, `'`)
	if abs, err := filepath.Abs(p); err == nil {
		return filepath.Clean(abs)
	}
	return filepath.Clean(p)
}

func parseCommandLine(cmdLine string) (exe string, args string) {
	cmdLine = strings.TrimSpace(cmdLine)
	if strings.HasPrefix(cmdLine, `"`) {
		end := strings.Index(cmdLine[1:], `"`)
		if end != -1 {
			exe = cmdLine[1 : end+1]
			args = strings.TrimSpace(cmdLine[end+2:])
			return exe, args
		}
	}
	parts := strings.SplitN(cmdLine, " ", 2)
	if len(parts) == 2 {
		return parts[0], parts[1]
	}
	return cmdLine, ""
}

// SelfHealAutoStart checks if the registered Scheduled Task matches the currently
// executing Unbound binary and arguments. If mismatched, it updates the task safely.
func SelfHealAutoStart() (bool, error) {
	logger := GetLogger()
	settings, err := GetSettings()
	if err != nil {
		return false, fmt.Errorf("failed to load settings for autostart check: %w", err)
	}
	if !settings.AutoStart {
		// Auto-start disabled in settings; do not touch OS registration
		logger.Debug("Startup", "[AUTOSTART] auto-start disabled in settings, skipping self-healing")
		return false, nil
	}

	currentExe, err := os.Executable()
	if err != nil {
		return false, fmt.Errorf("failed to determine executable path: %w", err)
	}
	currentExe, err = filepath.Abs(currentExe)
	if err != nil {
		return false, fmt.Errorf("failed to resolve absolute executable path: %w", err)
	}
	currentExe = filepath.Clean(currentExe)

	expectedArgs := ""
	if settings.StartMinimized {
		expectedArgs = "--tray"
	}

	info, err := QueryAutoStartTaskRegistration()
	if err != nil {
		logger.Warnf("Startup", "[AUTOSTART] failed to query task registration: %v", err)
		return false, err
	}

	if !info.Exists {
		logger.Warnf("Startup", "[AUTOSTART] task missing while autostart is enabled, registering task for %s", currentExe)
		if err := EnableAutoStart(); err != nil {
			logger.Errorf("Startup", "[AUTOSTART] failed to create missing task: %v", err)
			return false, fmt.Errorf("failed to create missing autostart task: %w", err)
		}
		logger.Info("Startup", "[AUTOSTART] task repaired successfully")
		return true, nil
	}

	exeMatches := strings.EqualFold(info.Executable, currentExe)
	argsMatch := strings.TrimSpace(info.Arguments) == strings.TrimSpace(expectedArgs)

	if exeMatches && argsMatch {
		logger.Debugf("Startup", "[AUTOSTART] task valid (exe=%s args=%s)", currentExe, expectedArgs)
		return false, nil
	}

	logger.Warnf("Startup", "[AUTOSTART] registered executable or arguments changed")
	logger.Warnf("Startup", "[AUTOSTART] old=%s args=%s", info.Executable, info.Arguments)
	logger.Warnf("Startup", "[AUTOSTART] new=%s args=%s", currentExe, expectedArgs)

	if err := EnableAutoStart(); err != nil {
		logger.Errorf("Startup", "[AUTOSTART] failed to repair task: %v", err)
		return false, fmt.Errorf("failed to repair autostart task: %w", err)
	}

	logger.Info("Startup", "[AUTOSTART] task repaired successfully")
	return true, nil
}
