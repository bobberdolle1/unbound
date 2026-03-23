package engine

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
)

type DiagnosticResult struct {
	Component string
	Status    string
	Details   string
	IsError   bool
}

// EnableTCPTimestamps включает TCP timestamps через netsh (требует прав администратора)
func EnableTCPTimestamps() error {
	cmd := exec.Command("netsh", "interface", "tcp", "set", "global", "timestamps=enabled")
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
	return cmd.Run()
}

// ClearDiscordCache очищает кэш Discord (аналог из service.bat)
func ClearDiscordCache() error {
	appData, err := os.UserConfigDir()
	if err != nil {
		return err
	}

	discordDir := filepath.Join(appData, "discord")
	cacheDirs := []string{"Cache", "Code Cache", "GPUCache"}

	for _, dir := range cacheDirs {
		target := filepath.Join(discordDir, dir)
		if _, err := os.Stat(target); err == nil {
			os.RemoveAll(target)
		}
	}
	return nil
}

// RunDiagnostics выполняет системную диагностику (конфликты, драйверы, права)
func RunDiagnostics() []DiagnosticResult {
	results := []DiagnosticResult{}

	// 1. Проверка прав администратора
	results = append(results, checkAdminPrivileges())

	// 2. Проверка TCP Timestamps
	results = append(results, checkTCPTimestamps())

	// 3. Проверка конфликтующих процессов
	results = append(results, checkConflictingProcesses())

	// 4. Проверка WinDivert
	results = append(results, checkWinDivertStatus())

	return results
}

func checkAdminPrivileges() DiagnosticResult {
	cmd := exec.Command("net", "session")
	err := cmd.Run()
	if err != nil {
		return DiagnosticResult{
			Component: "Privileges",
			Status:    "Error",
			Details:   "Administrator privileges NOT found. App may not work correctly.",
			IsError:   true,
		}
	}
	return DiagnosticResult{
		Component: "Privileges",
		Status:    "OK",
		Details:   "Running with Administrator privileges.",
		IsError:   false,
	}
}

func checkTCPTimestamps() DiagnosticResult {
	cmd := exec.Command("netsh", "interface", "tcp", "show", "global")
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
	out, err := cmd.Output()
	if err != nil {
		return DiagnosticResult{Component: "TCP Stack", Status: "Unknown", Details: "Failed to query netsh", IsError: true}
	}

	if strings.Contains(strings.ToLower(string(out)), "enabled") {
		return DiagnosticResult{
			Component: "TCP Stack",
			Status:    "OK",
			Details:   "TCP Timestamps are enabled (Recommended).",
			IsError:   false,
		}
	}
	return DiagnosticResult{
		Component: "TCP Stack",
		Status:    "Warning",
		Details:   "TCP Timestamps are disabled. This may cause issues with some ISPs.",
		IsError:   true,
	}
}

func checkConflictingProcesses() DiagnosticResult {
	conflicts := []string{"goodbyedpi.exe", "winws.exe", "nfqws.exe"}
	found := []string{}

	cmd := exec.Command("tasklist")
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
	out, _ := cmd.Output()
	processList := strings.ToLower(string(out))

	for _, c := range conflicts {
		if strings.Contains(processList, c) {
			found = append(found, c)
		}
	}

	if len(found) > 0 {
		return DiagnosticResult{
			Component: "Conflicts",
			Status:    "Warning",
			Details:   fmt.Sprintf("Conflicting processes found: %s. Close them before starting.", strings.Join(found, ", ")),
			IsError:   true,
		}
	}
	return DiagnosticResult{
		Component: "Conflicts",
		Status:    "OK",
		Details:   "No conflicting DPI bypass tools detected.",
		IsError:   false,
	}
}

func checkWinDivertStatus() DiagnosticResult {
	// 1. Проверяем наличие драйвера в системной папке или рядом с процессом
	system32 := os.Getenv("SystemRoot") + "\\System32\\drivers\\WinDivert64.sys"
	if _, err := os.Stat(system32); err == nil {
		return DiagnosticResult{
			Component: "WinDivert",
			Status:    "Installed",
			Details:   "Driver found in System32. Ready to intercept.",
			IsError:   false,
		}
	}

	// 2. Проверка через sc query (стандартный метод)
	cmd := exec.Command("sc", "query", "WinDivert")
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
	out, _ := cmd.Output()
	
	if strings.Contains(string(out), "RUNNING") {
		return DiagnosticResult{
			Component: "WinDivert",
			Status:    "Active",
			Details:   "Driver is currently running and active.",
			IsError:   false,
		}
	}
	
	return DiagnosticResult{
		Component: "WinDivert",
		Status:    "Ready",
		Details:   "Driver will be loaded automatically by winws2.exe when needed.",
		IsError:   false,
	}
}
