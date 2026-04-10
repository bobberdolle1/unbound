//go:build windows

package main

import (
	"os/exec"
	"strings"
	"syscall"
)

func checkConflictsImpl() []string {
	conflicts := []string{}

	type conflictProc struct {
		Exe  string
		Desc string
	}
	procs := []conflictProc{
		{"winws.exe",         "старый Zapret (winws)"},
		{"goodbyedpi.exe",    "GoodbyeDPI"},
		{"nfqws.exe",         "nfqws"},
		{"zapret.exe",        "Zapret"},
		{"ciadpi.exe",        "ciadpi"},
		{"byedpi.exe",        "ByeDPI"},
		{"openvpn.exe",       "OpenVPN"},
		{"warp-svc.exe",      "Cloudflare WARP"},
		{"expressvpn.exe",    "ExpressVPN"},
		{"nordvpn-service.exe", "NordVPN"},
	}

	for _, p := range procs {
		cmd := exec.Command("tasklist", "/FI", "IMAGENAME eq "+p.Exe, "/NH")
		cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true, CreationFlags: CREATE_NO_WINDOW}
		out, _ := cmd.Output()
		if strings.Contains(string(out), p.Exe) {
			conflicts = append(conflicts, "⚠️ "+p.Desc+" запущен")
		}
	}
	return conflicts
}

func killConflictsImpl() error {
	// Terminate external DPI bypassers (not our winws2.exe)
	procs := []string{
		"winws.exe", "goodbyedpi.exe", "nfqws.exe", "zapret.exe",
		"ciadpi.exe", "byedpi.exe",
	}

	for _, p := range procs {
		cmd := exec.Command("taskkill", "/F", "/IM", p)
		cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true, CreationFlags: CREATE_NO_WINDOW}
		cmd.Run()
	}

	// Reset WinDivert driver
	cmdReset := exec.Command("sc", "stop", "WinDivert")
	cmdReset.SysProcAttr = &syscall.SysProcAttr{HideWindow: true, CreationFlags: CREATE_NO_WINDOW}
	cmdReset.Run()

	return nil
}
