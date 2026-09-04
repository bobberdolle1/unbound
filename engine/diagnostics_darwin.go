//go:build darwin
// +build darwin

package engine

import (
	"os/exec"
	"strings"
	"unbound/engine/providers"
)

// EnableTCPTimestamps is a no-op on macOS as TCP timestamps are enabled by default.
func EnableTCPTimestamps() error {
	// macOS enables TCP timestamps by default in the BSD networking stack.
	// No equivalent netsh command exists.
	return nil
}


// RunDiagnostics performs macOS-specific system diagnostics.
func RunDiagnostics() []DiagnosticResult {
	return []DiagnosticResult{
		checkAdminPrivilegesMac(),
		checkEngineStatusMac(),
		checkPfAnchorStatus(),
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

func checkEngineStatusMac() DiagnosticResult {
	assetsBinDir := ""
	if configDir, err := GetConfigDir(); err == nil {
		assetsBinDir = configDir + "/core_bin"
	}

	binPath, err := providers.ResolveEngineBinary(providers.MacOSEngineBinary, assetsBinDir)
	if err == nil && binPath != "" {
		return DiagnosticResult{"Engine Binary", "OK", providers.MacOSEngineBinary + " found at: " + binPath, false}
	}
	return DiagnosticResult{"Engine Binary", "Warning", providers.MacOSEngineBinary + " binary not found. Install zapret (e.g., via Homebrew: brew install zapret).", true}
}

func checkPfAnchorStatus() DiagnosticResult {
	cmd := exec.Command("pfctl", "-s", "info")
	out, err := cmd.CombinedOutput()
	if err != nil {
		return DiagnosticResult{"Packet Filter (pf)", "Warning", "pfctl check failed (requires root or pf disabled).", false}
	}
	if strings.Contains(string(out), "Enabled") {
		return DiagnosticResult{"Packet Filter (pf)", "OK", "pf packet filter is active.", false}
	}
	return DiagnosticResult{"Packet Filter (pf)", "Info", "pf is currently disabled (will be enabled on start).", false}
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
