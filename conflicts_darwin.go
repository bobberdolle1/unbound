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

func isDarwinProcessRunning(proc string) bool {
	out, err := exec.Command("pgrep", "-x", proc).Output()
	return err == nil && len(strings.TrimSpace(string(out))) > 0
}

func isDarwinExternalProxyEnabled() bool {
	out, err := exec.Command("networksetup", "-listallglobalproxy").Output()
	return err == nil && strings.Contains(string(out), "Enabled: Yes")
}

func checkConflictsImpl() []string {
	conflicts := []string{}
	for _, p := range conflictingProcesses {
		if !isDarwinProcessRunning(p.Proc) {
			continue
		}
		if p.Kill {
			conflicts = append(conflicts, "⚠️ "+p.Desc+" запущен")
		} else {
			conflicts = append(conflicts, "⚠️ "+p.Desc+" запущен (внешний VPN — закройте вручную)")
		}
	}

	if isDarwinExternalProxyEnabled() {
		conflicts = append(conflicts, "⚠️ Системный прокси включён другим приложением (отключите вручную)")
	}

	return conflicts
}

func resolveConflictsImpl() ([]string, error) {
	for _, p := range conflictingProcesses {
		if !p.Kill {
			continue
		}
		// SIGTERM so the tool can tear down its own pf anchors; SIGKILL would
		// strand them and leave the user's networking broken.
		_ = exec.Command("pkill", "-TERM", "-x", p.Proc).Run()
	}

	var skipped []string
	for _, p := range conflictingProcesses {
		if !p.Kill && isDarwinProcessRunning(p.Proc) {
			skipped = append(skipped, p.Desc)
		}
	}
	if isDarwinExternalProxyEnabled() {
		skipped = append(skipped, "Системный прокси")
	}
	return skipped, nil
}

func killConflictsImpl() error {
	_, err := resolveConflictsImpl()
	return err
}

// killOwnEngineImpl force-stops Unbound's own engine.
func killOwnEngineImpl() error {
	// Force-kill any orphaned tpws or nfqws processes
	_ = exec.Command("pkill", "-KILL", "-x", "tpws").Run()
	_ = exec.Command("pkill", "-KILL", "-x", "nfqws").Run()

	// Flush our pf anchor if running as root
	if os.Geteuid() == 0 {
		_ = exec.Command("pfctl", "-a", "com.unbound.zapret", "-F", "all").Run()
	}
	return nil
}
