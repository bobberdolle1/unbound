//go:build linux
// +build linux

package providers

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"syscall"

	"github.com/wailsapp/wails/v2/pkg/runtime"
)

type ZapretLinuxProvider struct {
	status         Status
	logs           []string
	cmd            *exec.Cmd
	mu             sync.Mutex
	binPath        string
	currentProfile string
	iptablesRules  []string
}

func NewZapretLinuxProvider(binPath string) BypassProvider {
	return &ZapretLinuxProvider{
		status:  StatusStopped,
		binPath: binPath,
		logs:    []string{"Zapret Engine (Linux/nfqws) initialized."},
	}
}

func (e *ZapretLinuxProvider) Name() string {
	return "Zapret (nfqws)"
}

func (e *ZapretLinuxProvider) CheckPrivileges() (bool, error) {
	return os.Geteuid() == 0, nil
}

func (e *ZapretLinuxProvider) GetProfiles() []string {
	return []string{
		"Ultimate Bypass (Multi-Strategy)",
		"Discord Voice Optimized",
		"YouTube QUIC Aggressive",
		"Telegram API Bypass",
		"Standard HTTPS/QUIC",
		"HTTP + HTTPS Split",
	}
}

func (e *ZapretLinuxProvider) getProfileConfig(profileName string) ([]string, []string) {
	var nfqwsArgs []string
	var iptablesRules []string

	queueNum := "200"

	switch profileName {
	case "Ultimate Bypass (Multi-Strategy)":
		iptablesRules = []string{
			fmt.Sprintf("iptables -t mangle -I POSTROUTING -p tcp -m multiport --dports 80,443,5222,5223,5228 -m connbytes --connbytes-dir=original --connbytes-mode=packets --connbytes 1:6 -m mark ! --mark 0x40000000/0x40000000 -j NFQUEUE --queue-num %s --queue-bypass", queueNum),
			fmt.Sprintf("iptables -t mangle -I POSTROUTING -p udp -m multiport --dports 443,3478,50000:65535 -m mark ! --mark 0x40000000/0x40000000 -j NFQUEUE --queue-num %s --queue-bypass", queueNum),
		}
		nfqwsArgs = []string{
			"--qnum=" + queueNum, "--daemon", "--user=daemon",
			"--filter-tcp=80", "--dpi-desync=fake,multisplit", "--dpi-desync-split-pos=method+2", "--dpi-desync-fooling=md5sig", "--new",
			"--filter-tcp=443", "--dpi-desync=fake,multidisorder", "--dpi-desync-split-pos=1,midsld", "--dpi-desync-fooling=badseq,md5sig", "--new",
			"--filter-tcp=5222,5223,5228", "--dpi-desync=split", "--dpi-desync-split-pos=2", "--dpi-desync=disorder", "--new",
			"--filter-udp=443", "--dpi-desync=fake", "--dpi-desync-repeats=10", "--dpi-desync-udplen-increment=2", "--new",
			"--filter-udp=3478,50000-65535", "--dpi-desync=fake", "--dpi-desync-repeats=8",
		}

	case "Discord Voice Optimized":
		iptablesRules = []string{
			fmt.Sprintf("iptables -t mangle -I POSTROUTING -p tcp --dport 443 -m connbytes --connbytes-dir=original --connbytes-mode=packets --connbytes 1:6 -m mark ! --mark 0x40000000/0x40000000 -j NFQUEUE --queue-num %s --queue-bypass", queueNum),
			fmt.Sprintf("iptables -t mangle -I POSTROUTING -p udp -m multiport --dports 443,3478,50000:65535 -m mark ! --mark 0x40000000/0x40000000 -j NFQUEUE --queue-num %s --queue-bypass", queueNum),
		}
		nfqwsArgs = []string{
			"--qnum=" + queueNum, "--daemon", "--user=daemon",
			"--filter-tcp=443", "--dpi-desync=fake,split", "--dpi-desync-split-pos=1", "--dpi-desync-fooling=md5sig", "--new",
			"--filter-udp=443", "--dpi-desync=fake", "--dpi-desync-repeats=10", "--new",
			"--filter-udp=3478", "--dpi-desync=fake", "--dpi-desync-repeats=8", "--new",
			"--filter-udp=50000-65535", "--dpi-desync=fake", "--dpi-desync-repeats=8",
		}

	case "YouTube QUIC Aggressive":
		iptablesRules = []string{
			fmt.Sprintf("iptables -t mangle -I POSTROUTING -p tcp -m multiport --dports 80,443 -m connbytes --connbytes-dir=original --connbytes-mode=packets --connbytes 1:6 -m mark ! --mark 0x40000000/0x40000000 -j NFQUEUE --queue-num %s --queue-bypass", queueNum),
			fmt.Sprintf("iptables -t mangle -I POSTROUTING -p udp --dport 443 -m mark ! --mark 0x40000000/0x40000000 -j NFQUEUE --queue-num %s --queue-bypass", queueNum),
		}
		nfqwsArgs = []string{
			"--qnum=" + queueNum, "--daemon", "--user=daemon",
			"--filter-tcp=80", "--dpi-desync=fake,multisplit", "--dpi-desync-split-pos=method+2", "--dpi-desync-fooling=md5sig", "--new",
			"--filter-tcp=443", "--dpi-desync=fake,multisplit", "--dpi-desync-split-pos=1,midsld", "--dpi-desync-fooling=md5sig", "--new",
			"--filter-udp=443", "--dpi-desync=fake", "--dpi-desync-repeats=12", "--dpi-desync-udplen-increment=2",
		}

	case "Telegram API Bypass":
		iptablesRules = []string{
			fmt.Sprintf("iptables -t mangle -I POSTROUTING -p tcp -m multiport --dports 443,5222,5223,5228 -m connbytes --connbytes-dir=original --connbytes-mode=packets --connbytes 1:6 -m mark ! --mark 0x40000000/0x40000000 -j NFQUEUE --queue-num %s --queue-bypass", queueNum),
			fmt.Sprintf("iptables -t mangle -I POSTROUTING -p udp --dport 443 -m mark ! --mark 0x40000000/0x40000000 -j NFQUEUE --queue-num %s --queue-bypass", queueNum),
		}
		nfqwsArgs = []string{
			"--qnum=" + queueNum, "--daemon", "--user=daemon",
			"--filter-tcp=443", "--dpi-desync=fake,split", "--dpi-desync-split-pos=1", "--dpi-desync-fooling=md5sig", "--new",
			"--filter-tcp=5222,5223,5228", "--dpi-desync=split", "--dpi-desync-split-pos=2", "--dpi-desync=disorder", "--new",
			"--filter-udp=443", "--dpi-desync=fake", "--dpi-desync-repeats=8",
		}

	case "Standard HTTPS/QUIC":
		iptablesRules = []string{
			fmt.Sprintf("iptables -t mangle -I POSTROUTING -p tcp --dport 443 -m connbytes --connbytes-dir=original --connbytes-mode=packets --connbytes 1:6 -m mark ! --mark 0x40000000/0x40000000 -j NFQUEUE --queue-num %s --queue-bypass", queueNum),
			fmt.Sprintf("iptables -t mangle -I POSTROUTING -p udp --dport 443 -m mark ! --mark 0x40000000/0x40000000 -j NFQUEUE --queue-num %s --queue-bypass", queueNum),
		}
		nfqwsArgs = []string{
			"--qnum=" + queueNum, "--daemon", "--user=daemon",
			"--filter-tcp=443", "--dpi-desync=fake,split", "--dpi-desync-split-pos=1", "--dpi-desync-fooling=md5sig", "--new",
			"--filter-udp=443", "--dpi-desync=fake", "--dpi-desync-repeats=6",
		}

	case "HTTP + HTTPS Split":
		iptablesRules = []string{
			fmt.Sprintf("iptables -t mangle -I POSTROUTING -p tcp -m multiport --dports 80,443 -m connbytes --connbytes-dir=original --connbytes-mode=packets --connbytes 1:6 -m mark ! --mark 0x40000000/0x40000000 -j NFQUEUE --queue-num %s --queue-bypass", queueNum),
		}
		nfqwsArgs = []string{
			"--qnum=" + queueNum, "--daemon", "--user=daemon",
			"--filter-tcp=80", "--dpi-desync=split", "--dpi-desync-split-pos=method+2", "--new",
			"--filter-tcp=443", "--dpi-desync=split", "--dpi-desync-split-pos=2", "--dpi-desync=disorder",
		}
	}

	return nfqwsArgs, iptablesRules
}

func (e *ZapretLinuxProvider) Start(ctx context.Context, profileName string) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	if e.status == StatusRunning && e.currentProfile == profileName {
		return nil
	}

	if e.status == StatusRunning {
		e.mu.Unlock()
		e.Stop()
		e.mu.Lock()
	}

	e.status = StatusStarting
	nfqwsPath := filepath.Join(e.binPath, "nfqws")

	nfqwsArgs, iptablesRules := e.getProfileConfig(profileName)
	e.iptablesRules = iptablesRules

	e.addLog(fmt.Sprintf("[%s] Applying iptables rules...", e.Name()))
	for _, rule := range iptablesRules {
		parts := strings.Fields(rule)
		cmd := exec.Command(parts[0], parts[1:]...)
		if err := cmd.Run(); err != nil {
			e.status = StatusError
			e.addLog("iptables Error: " + err.Error())
			return fmt.Errorf("iptables setup failed: %w", err)
		}
	}

	e.addLog(fmt.Sprintf("[%s] Starting nfqws with profile: %s", e.Name(), profileName))

	e.cmd = exec.Command(nfqwsPath, nfqwsArgs...)
	e.cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}

	if err := e.cmd.Start(); err != nil {
		e.cleanupIptables()
		e.status = StatusError
		e.addLog("Launch Error: " + err.Error())
		return err
	}

	e.status = StatusRunning
	e.currentProfile = profileName
	e.addLog("Engine is ACTIVE.")

	go func() {
		err := e.cmd.Wait()
		e.mu.Lock()
		defer e.mu.Unlock()
		if e.currentProfile == profileName {
			e.status = StatusStopped
			e.cleanupIptables()
			if err != nil {
				e.addLog("Engine stopped unexpectedly. Code: " + err.Error())
			} else {
				e.addLog("Engine stopped gracefully.")
			}
			runtime.EventsEmit(ctx, "status_changed", e.status)
		}
	}()

	return nil
}

func (e *ZapretLinuxProvider) Stop() error {
	e.mu.Lock()
	defer e.mu.Unlock()

	if e.cmd != nil && e.cmd.Process != nil {
		e.addLog("Terminating nfqws process...")
		exec.Command("pkill", "-9", "nfqws").Run()
		e.cmd.Process.Kill()
		e.cmd = nil
	}

	e.cleanupIptables()
	e.status = StatusStopped
	e.currentProfile = ""
	return nil
}

func (e *ZapretLinuxProvider) cleanupIptables() {
	e.addLog("Cleaning up iptables rules...")
	for _, rule := range e.iptablesRules {
		deleteRule := strings.Replace(rule, "-I POSTROUTING", "-D POSTROUTING", 1)
		deleteRule = strings.Replace(deleteRule, "-A POSTROUTING", "-D POSTROUTING", 1)
		parts := strings.Fields(deleteRule)
		exec.Command(parts[0], parts[1:]...).Run()
	}
	e.iptablesRules = nil
}

func (e *ZapretLinuxProvider) GetStatus() Status {
	return e.status
}

func (e *ZapretLinuxProvider) GetLogs() []string {
	return e.logs
}

func (e *ZapretLinuxProvider) addLog(msg string) {
	e.logs = append(e.logs, msg)
	if len(e.logs) > 100 {
		e.logs = e.logs[1:]
	}
}
