//go:build darwin
// +build darwin

package engine

import (
	"os"
	"os/exec"
	"strings"
)

// EnableTCPTimestamps is a no-op on macOS as TCP timestamps are enabled by default.
func EnableTCPTimestamps() error {
	// macOS enables TCP timestamps by default in the BSD networking stack.
	// No equivalent netsh command exists.
	return nil
}

// ClearDiscordCache removes Discord cache directories on macOS.
func ClearDiscordCache() error {
	discordDir := GetDiscordCacheDir()
	if discordDir == "" {
		return nil
	}

	cacheDirs := []string{"Cache", "Code Cache", "GPUCache"}
	for _, dir := range cacheDirs {
		os.RemoveAll(discordDir + "/" + dir)
	}
	return nil
}

// RunDiagnostics performs macOS-specific system diagnostics.
func RunDiagnostics() []DiagnosticResult {
	return []DiagnosticResult{
		checkAdminPrivilegesMac(),
		checkSpoofDPIStatus(),
		checkConflictingProcessesMac(),
		checkNetworkService(),
	}
}

func checkAdminPrivilegesMac() DiagnosticResult {
	cmd := exec.Command("id", "-Gn")
	out, err := cmd.Output()
	if err != nil {
		return DiagnosticResult{"Privileges", "Error", "Could not determine privileges.", true}
	}
	if strings.Contains(string(out), "admin") {
		return DiagnosticResult{"Privileges", "OK", "User has admin rights.", false}
	}
	return DiagnosticResult{"Privileges", "Warning", "User may not have admin rights.", true}
}

func checkSpoofDPIStatus() DiagnosticResult {
	// Check if spoofdpi is available in PATH or in the bin directory
	if _, err := exec.LookPath("spoofdpi"); err == nil {
		return DiagnosticResult{"SpoofDPI", "OK", "Found in PATH.", false}
	}
	// Check if it would be found in the bin path
	configDir, err := GetConfigDir()
	if err == nil {
		binPath := configDir + "/core_bin"
		if _, statErr := os.Stat(binPath + "/spoofdpi"); statErr == nil {
			return DiagnosticResult{"SpoofDPI", "OK", "Found in app directory.", false}
		}
	}
	return DiagnosticResult{"SpoofDPI", "Warning", "SpoofDPI not found. Install via 'brew install spoofdpi'.", true}
}

func checkConflictingProcessesMac() DiagnosticResult {
	conflicts := []string{"spoofdpi", "goodbyedpi", "v2ray", "clash", "shadowsocks"}
	found := []string{}

	for _, proc := range conflicts {
		cmd := exec.Command("pgrep", "-x", proc)
		out, _ := cmd.Output()
		if len(out) > 0 {
			found = append(found, proc)
		}
	}

	if len(found) > 0 {
		return DiagnosticResult{"Conflicts", "Warning", "Found: " + strings.Join(found, ", "), true}
	}
	return DiagnosticResult{"Conflicts", "OK", "No conflicts.", false}
}

func checkNetworkService() DiagnosticResult {
	cmd := exec.Command("networksetup", "-listallnetworkservices")
	out, err := cmd.Output()
	if err != nil {
		return DiagnosticResult{"Network", "Error", "Could not list network services.", true}
	}

	outStr := string(out)
	hasWifi := strings.Contains(outStr, "Wi-Fi")
	hasEthernet := strings.Contains(outStr, "Ethernet")

	if hasWifi || hasEthernet {
		return DiagnosticResult{"Network", "OK", "Active network service found.", false}
	}
	return DiagnosticResult{"Network", "Warning", "No standard network service found.", true}
}
