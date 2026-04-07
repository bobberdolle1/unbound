//go:build darwin

package main

import (
	"os/exec"
	"strings"
)

func checkConflictsImpl() []string {
	conflicts := []string{}

	type conflictProc struct {
		Name string
		Desc string
	}
	procs := []conflictProc{
		{"spoofdpi",      "SpoofDPI (another instance)"},
		{"goodbyedpi",    "GoodbyeDPI"},
		{"v2ray",         "V2Ray"},
		{"clash",         "Clash"},
		{"shadowsocks",   "Shadowsocks"},
		{"hiddify",       "Hiddify"},
	}

	for _, p := range procs {
		cmd := exec.Command("pgrep", "-x", p.Name)
		out, _ := cmd.Output()
		if len(out) > 0 {
			conflicts = append(conflicts, "⚠️ "+p.Desc+" is running")
		}
	}

	// Check for active VPN connections
	cmd := exec.Command("networksetup", "-listallglobalproxy")
	out, err := cmd.Output()
	if err == nil && strings.Contains(string(out), "Enabled") {
		// Only warn if a non-SOCKS proxy is enabled (e.g., HTTP proxy from a VPN app)
		conflicts = append(conflicts, "⚠️ System proxy may be in use by another app")
	}

	return conflicts
}

func killConflictsImpl() error {
	// Terminate external DPI bypassers
	procs := []string{"goodbyedpi", "v2ray", "clash", "shadowsocks"}

	for _, p := range procs {
		cmd := exec.Command("pkill", "-x", p)
		cmd.Run()
	}

	// Disable any active system proxy that might interfere
	exec.Command("networksetup", "-setwebproxystate", "Wi-Fi", "off").Run()
	exec.Command("networksetup", "-setsecurewebproxystate", "Wi-Fi", "off").Run()

	return nil
}
