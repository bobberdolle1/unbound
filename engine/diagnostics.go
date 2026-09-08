//go:build windows

package engine

import (
	"os"
	"os/exec"
	"strings"
)

const CREATE_NO_WINDOW = 0x08000000

func EnableTCPTimestamps() error {
	cmd := exec.Command("netsh", "interface", "tcp", "set", "global", "timestamps=enabled")
	cmd.SysProcAttr = GetHiddenSysProcAttr()
	return cmd.Run()
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
	cmd.SysProcAttr = GetHiddenSysProcAttr()
	if err := cmd.Run(); err != nil {
		return DiagnosticResult{"Privileges", "Error", "Admin rights required.", true}
	}
	return DiagnosticResult{"Privileges", "OK", "Running as Admin.", false}
}

// parseNetshTimestamps scans output strictly for the RFC 1323 property line and checks its value.
// It avoids false matches against unrelated global TCP settings like Receive-Side Scaling or Fast Open.
func parseNetshTimestamps(output string) (enabled bool, matched bool) {
	for _, line := range strings.Split(output, "\n") {
		lineLower := strings.ToLower(line)
		if strings.Contains(lineLower, "1323") {
			parts := strings.SplitN(line, ":", 2)
			if len(parts) == 2 {
				val := strings.ToLower(strings.TrimSpace(parts[1]))
				// Check for known disabled values across locales
				if strings.Contains(val, "disabled") ||
					strings.Contains(val, "отключ") ||
					strings.Contains(val, "запрещ") ||
					strings.Contains(val, "désactivé") ||
					strings.Contains(val, "deaktiviert") ||
					val == "0" || val == "off" || val == "none" {
					return false, true
				}
				// Check for known enabled/allowed values across locales
				if strings.Contains(val, "allowed") ||
					strings.Contains(val, "enabled") ||
					strings.Contains(val, "разреш") ||
					strings.Contains(val, "включ") ||
					strings.Contains(val, "activé") ||
					strings.Contains(val, "autorisé") ||
					strings.Contains(val, "aktiviert") ||
					strings.Contains(val, "zulässig") ||
					val == "1" || val == "on" {
					return true, true
				}
			}
		}
	}
	return false, false
}

func checkTCPTimestamps() DiagnosticResult {
	cmd := exec.Command("netsh", "interface", "tcp", "show", "global")
	cmd.SysProcAttr = GetHiddenSysProcAttr()
	out, err := cmd.Output()
	if err == nil {
		if enabled, matched := parseNetshTimestamps(string(out)); matched {
			if enabled {
				return DiagnosticResult{"TCP Stack", "OK", "Timestamps enabled (RFC 1323 allowed/enabled).", false}
			}
			return DiagnosticResult{"TCP Stack", "Warning", "Timestamps disabled (RFC 1323 disabled).", true}
		}
	}

	// Fallback to PowerShell Get-NetTCPSetting
	psCmd := exec.Command("powershell.exe", "-NoProfile", "-NonInteractive", "-Command", "(Get-NetTCPSetting -SettingName Internet -ErrorAction SilentlyContinue).Timestamps")
	psCmd.SysProcAttr = GetHiddenSysProcAttr()
	psOut, psErr := psCmd.Output()
	if psErr == nil {
		val := strings.ToLower(strings.TrimSpace(string(psOut)))
		if strings.Contains(val, "allowed") || strings.Contains(val, "enabled") {
			return DiagnosticResult{"TCP Stack", "OK", "Timestamps enabled (PowerShell Get-NetTCPSetting).", false}
		}
		if strings.Contains(val, "disabled") {
			return DiagnosticResult{"TCP Stack", "Warning", "Timestamps disabled (PowerShell Get-NetTCPSetting).", true}
		}
	}

	return DiagnosticResult{"TCP Stack", "Warning", "Timestamps disabled or unknown.", true}
}

func checkTCPTimestampsBool() bool {
	return checkTCPTimestamps().Status == "OK"
}
func checkConflictingProcesses() DiagnosticResult {
	conflicts := []string{
		"goodbyedpi.exe", "winws.exe", "nfqws.exe",
		"xray.exe", "happ.exe", "sing-box.exe", "v2ray.exe", "v2rayn.exe",
		"clash.exe", "clash-verge.exe", "hiddify.exe", "nekoray.exe", "shadowsocks.exe",
	}
	found := []string{}
	cmd := exec.Command("tasklist")
	cmd.SysProcAttr = GetHiddenSysProcAttr()
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
