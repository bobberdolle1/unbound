//go:build linux

package main

import (
	"os/exec"
	"strings"
)

// conflictingProcesses lists other DPI-bypass tools and tunnels that contend
// with Unbound for the same NFQUEUE numbers, netfilter hooks or routes.
//
// Kill=false entries are reported but never terminated: killing a user's VPN
// or WireGuard tunnel without asking would drop every connection they have.
var conflictingProcesses = []struct {
	Proc string
	Desc string
	Kill bool
}{
	{"nfqws", "nfqws (другой экземпляр Zapret)", true},
	{"tpws", "tpws (Zapret transparent proxy)", true},
	{"goodbyedpi", "GoodbyeDPI", true},
	{"ciadpi", "ciadpi", true},
	{"byedpi", "ByeDPI", true},
	{"sing-box", "sing-box", false},
	{"xray", "Xray", false},
	{"v2ray", "V2Ray", false},
	{"clash", "Clash", false},
	{"openvpn", "OpenVPN", false},
	{"wg-quick", "WireGuard", false},
	{"warp-svc", "Cloudflare WARP", false},
}

// isProcessRunning reports whether a process with the exact name is alive.
// pgrep exits 1 when nothing matches, which is not an error for us.
func isProcessRunning(name string) bool {
	out, err := exec.Command("pgrep", "-x", name).Output()
	if err != nil {
		return false
	}
	return len(strings.TrimSpace(string(out))) > 0
}

func checkConflictsImpl() []string {
	conflicts := []string{}
	for _, p := range conflictingProcesses {
		if isProcessRunning(p.Proc) {
			conflicts = append(conflicts, "⚠️ "+p.Desc+" запущен")
		}
	}

	// A leftover NFQUEUE rule from a crashed bypass tool silently swallows the
	// packets we are about to queue, so surface it even when no process
	// matches by name.
	if out, err := exec.Command("iptables", "-t", "mangle", "-S").Output(); err == nil {
		if strings.Contains(string(out), "NFQUEUE") {
			conflicts = append(conflicts, "⚠️ В таблице mangle остались чужие правила NFQUEUE")
		}
	}

	return conflicts
}

func killConflictsImpl() error {
	for _, p := range conflictingProcesses {
		if !p.Kill {
			continue
		}
		// SIGTERM first so the tool can remove its own netfilter rules; a
		// SIGKILL here would strand them and break the user's networking.
		_ = exec.Command("pkill", "-TERM", "-x", p.Proc).Run()
	}
	return nil
}

// killOwnEngineImpl force-stops Unbound's own engine.
//
// On Linux the engine binary is plain `nfqws`, the same name a system-wide
// zapret service uses, so there is no safe way to force-kill "ours" by name -
// `pkill nfqws` would take down a service the user runs independently. The
// provider already signals its own process group on Stop(), which covers the
// process we started; a genuinely orphaned nfqws from an earlier crash shows up
// in CheckConflicts, where the user can decide about it explicitly.
func killOwnEngineImpl() error {
	return nil
}
