//go:build darwin
// +build darwin

package providers

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"syscall"

	"github.com/wailsapp/wails/v2/pkg/runtime"
)

type ZapretMacOSProvider struct {
	status         Status
	logs           []string
	cmd            *exec.Cmd
	mu             sync.Mutex
	binPath        string
	currentProfile string
	pfAnchor       string
	statusCallback func(Status)
	logCallback    func(string)
}

func NewZapretMacOSProvider(binPath string) BypassProvider {
	return &ZapretMacOSProvider{
		status:   StatusStopped,
		binPath:  binPath,
		pfAnchor: "com.unbound.zapret",
		logs:     []string{"Zapret Engine (macOS/nfqws) initialized."},
	}
}

func (e *ZapretMacOSProvider) Name() string {
	return "Zapret (nfqws)"
}

func (e *ZapretMacOSProvider) CheckPrivileges() (bool, error) {
	return os.Geteuid() == 0, nil
}

func (e *ZapretMacOSProvider) GetProfiles() []string {
	return []string{
		"Ultimate Bypass (Multi-Strategy)",
		"Discord Voice Optimized",
		"YouTube QUIC Aggressive",
		"Telegram API Bypass",
		"Standard HTTPS/QUIC",
		"HTTP + HTTPS Split",
	}
}

func (e *ZapretMacOSProvider) getProfileConfig(profileName string) ([]string, string) {
	var nfqwsArgs []string
	var pfRules string

	basePfRules := `
anchor "com.unbound.zapret"
load anchor "com.unbound.zapret" from "/tmp/unbound_pf_rules.conf"
`

	switch profileName {
	case "Ultimate Bypass (Multi-Strategy)":
		pfRules = `
pass out quick proto tcp to any port {80, 443, 5222, 5223, 5228} divert-packet port 700
pass out quick proto udp to any port {443, 3478, 50000:65535} divert-packet port 700
`
		nfqwsArgs = []string{
			"--port=700", "--daemon",
			"--filter-tcp=80", "--dpi-desync=fake,multisplit", "--dpi-desync-split-pos=method+2", "--dpi-desync-fooling=md5sig", "--new",
			"--filter-tcp=443", "--dpi-desync=fake,multidisorder", "--dpi-desync-split-pos=1,midsld", "--dpi-desync-fooling=badseq,md5sig", "--new",
			"--filter-tcp=5222,5223,5228", "--dpi-desync=split", "--dpi-desync-split-pos=2", "--dpi-desync=disorder", "--new",
			"--filter-udp=443", "--dpi-desync=fake", "--dpi-desync-repeats=10", "--dpi-desync-udplen-increment=2", "--new",
			"--filter-udp=3478,50000-65535", "--dpi-desync=fake", "--dpi-desync-repeats=8",
		}

	case "Discord Voice Optimized":
		pfRules = `
pass out quick proto tcp to any port 443 divert-packet port 700
pass out quick proto udp to any port {443, 3478, 50000:65535} divert-packet port 700
`
		nfqwsArgs = []string{
			"--port=700", "--daemon",
			"--filter-tcp=443", "--dpi-desync=fake,split", "--dpi-desync-split-pos=1", "--dpi-desync-fooling=md5sig", "--new",
			"--filter-udp=443", "--dpi-desync=fake", "--dpi-desync-repeats=10", "--new",
			"--filter-udp=3478", "--dpi-desync=fake", "--dpi-desync-repeats=8", "--new",
			"--filter-udp=50000-65535", "--dpi-desync=fake", "--dpi-desync-repeats=8",
		}

	case "YouTube QUIC Aggressive":
		pfRules = `
pass out quick proto tcp to any port {80, 443} divert-packet port 700
pass out quick proto udp to any port 443 divert-packet port 700
`
		nfqwsArgs = []string{
			"--port=700", "--daemon",
			"--filter-tcp=80", "--dpi-desync=fake,multisplit", "--dpi-desync-split-pos=method+2", "--dpi-desync-fooling=md5sig", "--new",
			"--filter-tcp=443", "--dpi-desync=fake,multisplit", "--dpi-desync-split-pos=1,midsld", "--dpi-desync-fooling=md5sig", "--new",
			"--filter-udp=443", "--dpi-desync=fake", "--dpi-desync-repeats=12", "--dpi-desync-udplen-increment=2",
		}

	case "Telegram API Bypass":
		pfRules = `
pass out quick proto tcp to any port {443, 5222, 5223, 5228} divert-packet port 700
pass out quick proto udp to any port 443 divert-packet port 700
`
		nfqwsArgs = []string{
			"--port=700", "--daemon",
			"--filter-tcp=443", "--dpi-desync=fake,split", "--dpi-desync-split-pos=1", "--dpi-desync-fooling=md5sig", "--new",
			"--filter-tcp=5222,5223,5228", "--dpi-desync=split", "--dpi-desync-split-pos=2", "--dpi-desync=disorder", "--new",
			"--filter-udp=443", "--dpi-desync=fake", "--dpi-desync-repeats=8",
		}

	case "Standard HTTPS/QUIC":
		pfRules = `
pass out quick proto tcp to any port 443 divert-packet port 700
pass out quick proto udp to any port 443 divert-packet port 700
`
		nfqwsArgs = []string{
			"--port=700", "--daemon",
			"--filter-tcp=443", "--dpi-desync=fake,split", "--dpi-desync-split-pos=1", "--dpi-desync-fooling=md5sig", "--new",
			"--filter-udp=443", "--dpi-desync=fake", "--dpi-desync-repeats=6",
		}

	case "HTTP + HTTPS Split":
		pfRules = `
pass out quick proto tcp to any port {80, 443} divert-packet port 700
`
		nfqwsArgs = []string{
			"--port=700", "--daemon",
			"--filter-tcp=80", "--dpi-desync=split", "--dpi-desync-split-pos=method+2", "--new",
			"--filter-tcp=443", "--dpi-desync=split", "--dpi-desync-split-pos=2", "--dpi-desync=disorder",
		}
	}

	return nfqwsArgs, basePfRules + pfRules
}

func (e *ZapretMacOSProvider) Start(ctx context.Context, profileName string) error {
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

	nfqwsArgs, pfRules := e.getProfileConfig(profileName)

	e.addLog(fmt.Sprintf("[%s] Configuring pf (Packet Filter)...", e.Name()))

	pfConfPath := "/tmp/unbound_pf_rules.conf"
	if err := os.WriteFile(pfConfPath, []byte(pfRules), 0644); err != nil {
		e.status = StatusError
		e.addLog("pf config write error: " + err.Error())
		return fmt.Errorf("failed to write pf config: %w", err)
	}

	if err := exec.Command("pfctl", "-e").Run(); err != nil {
		e.addLog("pf enable warning (may already be enabled): " + err.Error())
	}

	if err := exec.Command("pfctl", "-f", pfConfPath).Run(); err != nil {
		e.status = StatusError
		e.addLog("pf load error: " + err.Error())
		return fmt.Errorf("pf configuration failed: %w", err)
	}

	e.addLog(fmt.Sprintf("[%s] Starting nfqws with profile: %s", e.Name(), profileName))

	e.cmd = exec.Command(nfqwsPath, nfqwsArgs...)
	e.cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}

	if err := e.cmd.Start(); err != nil {
		e.cleanupPf()
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
			e.cleanupPf()
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

func (e *ZapretMacOSProvider) Stop() error {
	e.mu.Lock()
	defer e.mu.Unlock()

	if e.cmd != nil && e.cmd.Process != nil {
		e.addLog("Terminating nfqws process...")
		exec.Command("pkill", "-9", "nfqws").Run()
		e.cmd.Process.Kill()
		e.cmd = nil
	}

	e.cleanupPf()
	e.status = StatusStopped
	e.currentProfile = ""
	return nil
}

func (e *ZapretMacOSProvider) cleanupPf() {
	e.addLog("Cleaning up pf rules...")

	exec.Command("pfctl", "-a", e.pfAnchor, "-F", "all").Run()

	os.Remove("/tmp/unbound_pf_rules.conf")
}

func (e *ZapretMacOSProvider) GetStatus() Status {
	return e.status
}

func (e *ZapretMacOSProvider) GetLogs() []string {
	return e.logs
}

func (e *ZapretMacOSProvider) addLog(msg string) {
	e.logs = append(e.logs, msg)
	if len(e.logs) > 100 {
		e.logs = e.logs[1:]
	}
	if e.logCallback != nil {
		e.logCallback(msg)
	}
}

func (e *ZapretMacOSProvider) SetStatusCallback(cb func(Status)) {
	e.statusCallback = cb
}

func (e *ZapretMacOSProvider) SetLogCallback(cb func(string)) {
	e.logCallback = cb
}

func (e *ZapretMacOSProvider) RegisterProfile(name string, args []string) {
	// Not fully implemented for macOS yet (manual mapping in getProfileConfig)
	e.addLog("Dynamic profile registration not fully supported on macOS yet.")
}
