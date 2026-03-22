package engine

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"unsafe"

	"golang.org/x/sys/windows"
	"golang.org/x/sys/windows/registry"
)

type DiagnosticResult struct {
	Name     string
	Status   string // "OK", "WARNING", "ERROR"
	Message  string
	Critical bool
}

type DiagnosticsReport struct {
	Results []DiagnosticResult
	Score   int
	Summary string
}

func RunDiagnostics() DiagnosticsReport {
	report := DiagnosticsReport{
		Results: make([]DiagnosticResult, 0),
	}

	report.Results = append(report.Results, checkBaseFilteringEngine())
	report.Results = append(report.Results, checkProxySettings())
	report.Results = append(report.Results, checkTCPTimestamps())
	report.Results = append(report.Results, checkConflictingSoftware()...)
	report.Results = append(report.Results, checkWinDivertConflicts())
	report.Results = append(report.Results, checkVPNServices())
	report.Results = append(report.Results, checkSecureDNS())
	report.Results = append(report.Results, checkHostsFile())

	score := 0
	criticalIssues := 0
	warnings := 0

	for _, result := range report.Results {
		if result.Status == "OK" {
			score += 10
		} else if result.Status == "WARNING" {
			warnings++
			score += 5
		} else if result.Status == "ERROR" {
			if result.Critical {
				criticalIssues++
			}
		}
	}

	report.Score = score

	if criticalIssues > 0 {
		report.Summary = fmt.Sprintf("CRITICAL: %d critical issues found. Engine may not work properly.", criticalIssues)
	} else if warnings > 0 {
		report.Summary = fmt.Sprintf("WARNING: %d potential issues detected. Engine should work but may have reduced effectiveness.", warnings)
	} else {
		report.Summary = "System configuration optimal for DPI bypass."
	}

	return report
}

func checkBaseFilteringEngine() DiagnosticResult {
	cmd := exec.Command("sc", "query", "BFE")
	output, err := cmd.CombinedOutput()

	if err != nil {
		return DiagnosticResult{
			Name:     "Base Filtering Engine",
			Status:   "ERROR",
			Message:  "BFE service not found or not accessible",
			Critical: true,
		}
	}

	outputStr := string(output)
	if strings.Contains(outputStr, "RUNNING") {
		return DiagnosticResult{
			Name:    "Base Filtering Engine",
			Status:  "OK",
			Message: "BFE service is running",
		}
	}

	return DiagnosticResult{
		Name:     "Base Filtering Engine",
		Status:   "ERROR",
		Message:  "BFE service is not running. Start it with: sc start BFE",
		Critical: true,
	}
}

func checkProxySettings() DiagnosticResult {
	k, err := registry.OpenKey(registry.CURRENT_USER, `Software\Microsoft\Windows\CurrentVersion\Internet Settings`, registry.QUERY_VALUE)
	if err != nil {
		return DiagnosticResult{
			Name:    "Proxy Settings",
			Status:  "WARNING",
			Message: "Cannot read proxy settings",
		}
	}
	defer k.Close()

	proxyEnable, _, err := k.GetIntegerValue("ProxyEnable")
	if err == nil && proxyEnable == 1 {
		proxyServer, _, _ := k.GetStringValue("ProxyServer")
		return DiagnosticResult{
			Name:    "Proxy Settings",
			Status:  "WARNING",
			Message: fmt.Sprintf("System proxy is enabled: %s. May interfere with DPI bypass.", proxyServer),
		}
	}

	return DiagnosticResult{
		Name:    "Proxy Settings",
		Status:  "OK",
		Message: "No system proxy configured",
	}
}

func checkTCPTimestamps() DiagnosticResult {
	cmd := exec.Command("netsh", "int", "tcp", "show", "global")
	output, err := cmd.CombinedOutput()

	if err != nil {
		return DiagnosticResult{
			Name:    "TCP Timestamps",
			Status:  "WARNING",
			Message: "Cannot check TCP timestamp settings",
		}
	}

	outputStr := strings.ToLower(string(output))
	if strings.Contains(outputStr, "timestamps") && strings.Contains(outputStr, "enabled") {
		return DiagnosticResult{
			Name:    "TCP Timestamps",
			Status:  "OK",
			Message: "TCP timestamps are enabled (recommended)",
		}
	}

	return DiagnosticResult{
		Name:    "TCP Timestamps",
		Status:  "WARNING",
		Message: "TCP timestamps may be disabled. Enable with: netsh int tcp set global timestamps=enabled",
	}
}

func checkConflictingSoftware() []DiagnosticResult {
	results := make([]DiagnosticResult, 0)

	conflictingServices := []struct {
		name        string
		displayName string
		critical    bool
	}{
		{"AdguardSvc", "Adguard", false},
		{"Killer Network Service", "Killer Network Manager", false},
		{"cplspcon", "Intel Connectivity Performance", false},
		{"SentinelAgent", "Check Point Endpoint Security", true},
		{"SmartByte", "SmartByte Network Service", false},
	}

	for _, svc := range conflictingServices {
		cmd := exec.Command("sc", "query", svc.name)
		output, err := cmd.CombinedOutput()

		if err == nil && strings.Contains(string(output), "RUNNING") {
			status := "WARNING"
			if svc.critical {
				status = "ERROR"
			}

			results = append(results, DiagnosticResult{
				Name:     fmt.Sprintf("Conflicting Software: %s", svc.displayName),
				Status:   status,
				Message:  fmt.Sprintf("%s is running and may conflict with WinDivert", svc.displayName),
				Critical: svc.critical,
			})
		}
	}

	if len(results) == 0 {
		results = append(results, DiagnosticResult{
			Name:    "Conflicting Software",
			Status:  "OK",
			Message: "No known conflicting software detected",
		})
	}

	return results
}

func checkWinDivertConflicts() DiagnosticResult {
	systemRoot := os.Getenv("SystemRoot")
	if systemRoot == "" {
		systemRoot = "C:\\Windows"
	}

	windivertPaths := []string{
		filepath.Join(systemRoot, "System32", "WinDivert.dll"),
		filepath.Join(systemRoot, "System32", "WinDivert64.sys"),
		filepath.Join(systemRoot, "SysWOW64", "WinDivert.dll"),
	}

	foundConflicts := make([]string, 0)
	for _, path := range windivertPaths {
		if _, err := os.Stat(path); err == nil {
			foundConflicts = append(foundConflicts, path)
		}
	}

	if len(foundConflicts) > 0 {
		return DiagnosticResult{
			Name:     "WinDivert Conflicts",
			Status:   "WARNING",
			Message:  fmt.Sprintf("Old WinDivert files found in system directories: %s. May cause version conflicts.", strings.Join(foundConflicts, ", ")),
			Critical: false,
		}
	}

	return DiagnosticResult{
		Name:    "WinDivert Conflicts",
		Status:  "OK",
		Message: "No WinDivert conflicts detected",
	}
}

func checkVPNServices() DiagnosticResult {
	vpnServices := []string{
		"OpenVPNService",
		"WireGuardTunnel",
		"NordVPN",
		"ExpressVPN",
		"ProtonVPN",
	}

	runningVPNs := make([]string, 0)
	for _, svc := range vpnServices {
		cmd := exec.Command("sc", "query", svc)
		output, err := cmd.CombinedOutput()

		if err == nil && strings.Contains(string(output), "RUNNING") {
			runningVPNs = append(runningVPNs, svc)
		}
	}

	if len(runningVPNs) > 0 {
		return DiagnosticResult{
			Name:    "VPN Services",
			Status:  "WARNING",
			Message: fmt.Sprintf("Active VPN services detected: %s. May interfere with DPI bypass.", strings.Join(runningVPNs, ", ")),
		}
	}

	return DiagnosticResult{
		Name:    "VPN Services",
		Status:  "OK",
		Message: "No active VPN services detected",
	}
}

func checkSecureDNS() DiagnosticResult {
	modiphlpsapi := windows.NewLazySystemDLL("iphlpapi.dll")
	procGetAdaptersAddresses := modiphlpsapi.NewProc("GetAdaptersAddresses")

	var bufLen uint32 = 15000
	buf := make([]byte, bufLen)

	ret, _, _ := procGetAdaptersAddresses.Call(
		uintptr(windows.AF_UNSPEC),
		uintptr(0),
		0,
		uintptr(unsafe.Pointer(&buf[0])),
		uintptr(unsafe.Pointer(&bufLen)),
	)

	if ret != 0 {
		return DiagnosticResult{
			Name:    "Secure DNS",
			Status:  "WARNING",
			Message: "Cannot check DNS configuration",
		}
	}

	k, err := registry.OpenKey(registry.LOCAL_MACHINE, `SYSTEM\CurrentControlSet\Services\Dnscache\Parameters`, registry.QUERY_VALUE)
	if err == nil {
		defer k.Close()

		dohEnabled, _, err := k.GetIntegerValue("EnableAutoDoh")
		if err == nil && dohEnabled == 2 {
			return DiagnosticResult{
				Name:    "Secure DNS",
				Status:  "WARNING",
				Message: "DNS over HTTPS (DoH) is enabled. May bypass some DPI techniques.",
			}
		}
	}

	return DiagnosticResult{
		Name:    "Secure DNS",
		Status:  "OK",
		Message: "Standard DNS configuration",
	}
}

func checkHostsFile() DiagnosticResult {
	systemRoot := os.Getenv("SystemRoot")
	if systemRoot == "" {
		systemRoot = "C:\\Windows"
	}

	hostsPath := filepath.Join(systemRoot, "System32", "drivers", "etc", "hosts")

	content, err := os.ReadFile(hostsPath)
	if err != nil {
		return DiagnosticResult{
			Name:    "Hosts File",
			Status:  "WARNING",
			Message: "Cannot read hosts file",
		}
	}

	lines := strings.Split(string(content), "\n")
	activeEntries := 0

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line != "" && !strings.HasPrefix(line, "#") {
			activeEntries++
		}
	}

	if activeEntries > 2 {
		return DiagnosticResult{
			Name:    "Hosts File",
			Status:  "WARNING",
			Message: fmt.Sprintf("Hosts file contains %d active entries. May interfere with DNS resolution.", activeEntries),
		}
	}

	return DiagnosticResult{
		Name:    "Hosts File",
		Status:  "OK",
		Message: "Hosts file is clean",
	}
}

func ClearDiscordCache() error {
	appData := os.Getenv("APPDATA")
	if appData == "" {
		return fmt.Errorf("APPDATA environment variable not set")
	}

	discordPaths := []string{
		filepath.Join(appData, "discord", "Cache"),
		filepath.Join(appData, "discord", "Code Cache"),
		filepath.Join(appData, "discord", "GPUCache"),
	}

	for _, path := range discordPaths {
		if _, err := os.Stat(path); err == nil {
			os.RemoveAll(path)
		}
	}

	return nil
}
