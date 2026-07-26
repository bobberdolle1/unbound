//go:build windows

package main

import (
	"os/exec"
	"strings"

	"unbound/engine"
)

// conflictingProcesses lists other DPI-bypass tools and VPN clients that fight
// Unbound over the same packets. winws2.exe is deliberately absent: that is our
// own engine.
var conflictingProcesses = []struct {
	Exe  string
	Desc string
	Kill bool // VPN clients are reported but never killed out from under the user
}{
	{"winws.exe", "старый Zapret (winws)", true},
	{"goodbyedpi.exe", "GoodbyeDPI", true},
	{"nfqws.exe", "nfqws", true},
	{"zapret.exe", "Zapret", true},
	{"ciadpi.exe", "ciadpi", true},
	{"byedpi.exe", "ByeDPI", true},
	{"openvpn.exe", "OpenVPN", false},
	{"warp-svc.exe", "Cloudflare WARP", false},
	{"expressvpn.exe", "ExpressVPN", false},
	{"nordvpn-service.exe", "NordVPN", false},
}

func checkConflictsImpl() []string {
	conflicts := []string{}
	for _, p := range conflictingProcesses {
		cmd := exec.Command("tasklist", "/FI", "IMAGENAME eq "+p.Exe, "/NH")
		cmd.SysProcAttr = engine.GetHiddenSysProcAttr()
		out, _ := cmd.Output()
		if strings.Contains(string(out), p.Exe) {
			conflicts = append(conflicts, "⚠️ "+p.Desc+" запущен")
		}
	}
	return conflicts
}

func killConflictsImpl() error {
	for _, p := range conflictingProcesses {
		if !p.Kill {
			continue
		}
		cmd := exec.Command("taskkill", "/F", "/IM", p.Exe)
		cmd.SysProcAttr = engine.GetHiddenSysProcAttr()
		// A non-zero exit just means the process was not running.
		_ = cmd.Run()
	}

	// Release the WinDivert driver so a stale handle left by a crashed bypass
	// tool does not stop our own engine from opening it.
	reset := exec.Command("sc", "stop", "WinDivert")
	reset.SysProcAttr = engine.GetHiddenSysProcAttr()
	_ = reset.Run()

	return nil
}
