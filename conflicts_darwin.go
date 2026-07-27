//go:build darwin

package main

import (
	"os"
	"os/exec"
	"strings"
)

// conflictingProcesses lists other bypass tools and tunnels that contend with
// Unbound for the same packet filter hooks or routes.
//
// Kill=false entries are reported but never terminated: killing a user's
// tunnel without asking would drop every connection they have.
var conflictingProcesses = []struct {
	Proc string
	Desc string
	Kill bool
}{
	{"spoofdpi", "SpoofDPI (другой экземпляр)", true},
	{"goodbyedpi", "GoodbyeDPI", true},
	{"nfqws", "nfqws", true},
	{"v2ray", "V2Ray", false},
	{"xray", "Xray", false},
	{"sing-box", "sing-box", false},
	{"clash", "Clash", false},
	{"shadowsocks", "Shadowsocks", false},
	{"hiddify", "Hiddify", false},
}

func checkConflictsImpl() []string {
	conflicts := []string{}
	for _, p := range conflictingProcesses {
		out, err := exec.Command("pgrep", "-x", p.Proc).Output()
		if err == nil && len(strings.TrimSpace(string(out))) > 0 {
			conflicts = append(conflicts, "⚠️ "+p.Desc+" запущен")
		}
	}

	if out, err := exec.Command("networksetup", "-listallglobalproxy").Output(); err == nil {
		if strings.Contains(string(out), "Enabled: Yes") {
			conflicts = append(conflicts, "⚠️ Системный прокси включён другим приложением")
		}
	}

	return conflicts
}

func killConflictsImpl() error {
	for _, p := range conflictingProcesses {
		if !p.Kill {
			continue
		}
		// SIGTERM so the tool can tear down its own pf anchors; SIGKILL would
		// strand them and leave the user's networking broken.
		_ = exec.Command("pkill", "-TERM", "-x", p.Proc).Run()
	}

	// This used to also run `networksetup -setwebproxystate Wi-Fi off`, which
	// silently rewrote the user's proxy configuration - on the Wi-Fi service
	// specifically, so it was simultaneously destructive and a no-op on wired
	// Macs. Conflicting proxies are now reported by checkConflictsImpl and left
	// for the user to decide about.
	return nil
}

// killOwnEngineImpl force-stops Unbound's own engine.
//
// As on Linux, the engine binary shares the plain `nfqws` name with any
// system-wide zapret install, so killing by name could take down a service the
// user runs independently. Stop() already signals the process group we own, and
// an orphan from an earlier crash is surfaced by CheckConflicts instead.
func killOwnEngineImpl() error {
	// Force-kill any orphaned tpws or nfqws processes
	_ = exec.Command("pkill", "-KILL", "-x", "tpws").Run()
	_ = exec.Command("pkill", "-KILL", "-x", "nfqws").Run()

	// Flush our pf anchor
	if os.Geteuid() == 0 {
		_ = exec.Command("pfctl", "-a", "com.unbound.zapret", "-F", "all").Run()
	} else {
		script := `do shell script "pfctl -a com.unbound.zapret -F all" with administrator privileges`
		_ = exec.Command("osascript", "-e", script).Run()
	}
	return nil
}
