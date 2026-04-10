//go:build windows

package engine

import (
	"os"
	"os/exec"
	"strings"
	"syscall"
)

const CREATE_NO_WINDOW = 0x08000000

type DiagnosticResult struct {
	Component string
	Status    string
	Details   string
	IsError   bool
}

func EnableTCPTimestamps() error {
	cmd := exec.Command("netsh", "interface", "tcp", "set", "global", "timestamps=enabled")
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true, CreationFlags: CREATE_NO_WINDOW}
	return cmd.Run()
}

func ClearDiscordCache() error {
	appData := os.Getenv("APPDATA")
	discordDir := appData + "\\discord"
	cacheDirs := []string{"Cache", "Code Cache", "GPUCache"}

	for _, dir := range cacheDirs {
		os.RemoveAll(discordDir + "\\" + dir)
	}
	return nil
}

func RunDiagnostics() []DiagnosticResult {
	return []DiagnosticResult{
		checkAdminPrivileges(),
		checkTCPTimestamps(),
		checkConflictingProcesses(),
		checkWinDivertStatus(),
	}
}

func checkAdminPrivileges() DiagnosticResult {
	cmd := exec.Command("net", "session")
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true, CreationFlags: CREATE_NO_WINDOW}
	if err := cmd.Run(); err != nil {
		return DiagnosticResult{"Privileges", "Error", "Admin rights required.", true}
	}
	return DiagnosticResult{"Privileges", "OK", "Running as Admin.", false}
}

func checkTCPTimestamps() DiagnosticResult {
	cmd := exec.Command("netsh", "interface", "tcp", "show", "global")
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true, CreationFlags: CREATE_NO_WINDOW}
	out, _ := cmd.Output()
	if strings.Contains(strings.ToLower(string(out)), "enabled") {
		return DiagnosticResult{"TCP Stack", "OK", "Timestamps enabled.", false}
	}
	return DiagnosticResult{"TCP Stack", "Warning", "Timestamps disabled.", true}
}

func checkConflictingProcesses() DiagnosticResult {
	conflicts := []string{"goodbyedpi.exe", "winws.exe", "nfqws.exe"}
	found := []string{}
	cmd := exec.Command("tasklist")
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true, CreationFlags: CREATE_NO_WINDOW}
	out, _ := cmd.Output()
	for _, c := range conflicts {
		if strings.Contains(strings.ToLower(string(out)), c) {
			found = append(found, c)
		}
	}
	if len(found) > 0 {
		return DiagnosticResult{"Conflicts", "Warning", "Found: " + strings.Join(found, ", "), true}
	}
	return DiagnosticResult{"Conflicts", "OK", "No conflicts.", false}
}

func checkWinDivertStatus() DiagnosticResult {
	system32 := os.Getenv("SystemRoot") + "\\System32\\drivers\\WinDivert64.sys"
	if _, err := os.Stat(system32); err == nil {
		return DiagnosticResult{"WinDivert", "Installed", "Driver found in System32.", false}
	}
	return DiagnosticResult{"WinDivert", "Ready", "Driver will be loaded on start.", false}
}
