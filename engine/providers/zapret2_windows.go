//go:build windows
// +build windows

package providers

import (
	"context"
	"fmt"
	"os/exec"
	"path/filepath"
	"sync"
	"syscall"

	"github.com/wailsapp/wails/v2/pkg/runtime"
	"golang.org/x/sys/windows"
)

type Zapret2WindowsProvider struct {
	status         Status
	logs           []string
	cmd            *exec.Cmd
	mu             sync.Mutex
	binPath        string
	luaDir         string
	currentProfile string
}

func NewZapret2WindowsProvider(binPath, luaDir string) BypassProvider {
	return &Zapret2WindowsProvider{
		status:  StatusStopped,
		binPath: binPath,
		luaDir:  luaDir,
		logs:    []string{"Zapret 2 Engine (Windows) initialized."},
	}
}

func (e *Zapret2WindowsProvider) Name() string {
	return "Zapret 2 (winws)"
}

func (e *Zapret2WindowsProvider) CheckPrivileges() (bool, error) {
	var sid *windows.SID
	err := windows.AllocateAndInitializeSid(
		&windows.SECURITY_NT_AUTHORITY,
		2,
		windows.SECURITY_BUILTIN_DOMAIN_RID,
		windows.DOMAIN_ALIAS_RID_ADMINS,
		0, 0, 0, 0, 0, 0,
		&sid)
	if err != nil {
		return false, err
	}
	defer windows.FreeSid(sid)

	token := windows.Token(0)
	member, err := token.IsMember(sid)
	if err != nil {
		return false, err
	}
	return member, nil
}

func (e *Zapret2WindowsProvider) GetProfiles() []string {
	return []string{
		"Unbound Ultimate (God Mode)", 
		"Discord Voice Optimized",
		"YouTube QUIC Aggressive",
		"Telegram API Bypass",
		"Flowseal Legacy (Discord/YT)", 
		"Fake TLS & QUIC", 
		"Split & Disorder",
		"Multi-Strategy Chaos",
	}
}

func (e *Zapret2WindowsProvider) getProfileArgs(profileName string) []string {
	luaLib := filepath.ToSlash(filepath.Join(e.luaDir, "zapret-lib.lua"))
	luaAntiDpi := filepath.ToSlash(filepath.Join(e.luaDir, "zapret-antidpi.lua"))

	args := []string{
		"--wf-tcp-empty=1",
		"--lua-init=@" + luaLib,
		"--lua-init=@" + luaAntiDpi,
	}

	switch profileName {
	case "Unbound Ultimate (God Mode)":
		// Comprehensive filter for almost all blocked services in RU (2026)
		// TCP: 80 (HTTP), 443 (HTTPS), 5222/5223/5228/4244 (Telegram/WhatsApp API)
		// UDP: 443 (QUIC), 3478 (STUN/Calls), 50000-65535 (Discord Voice)
		args = append([]string{"--wf-tcp-out=80,443,5222,5223,5228,4244", "--wf-udp-out=443,3478,50000-65535"}, args...)
		args = append(args,
			// HTTPS (YouTube, Discord API, General Web)
			"--filter-tcp=443", "--filter-l7=tls", "--payload=tls_client_hello", "--lua-desync=fake:blob=fake_default_tls:tcp_md5", "--lua-desync=split:pos=1", "--new",
			// HTTP (General Web)
			"--filter-tcp=80", "--lua-desync=fake:blob=fake_default_http:tcp_md5", "--new",
			// Custom TCP APIs (Telegram, WhatsApp)
			"--filter-tcp=5222,5223,5228,4244", "--lua-desync=split:pos=2", "--lua-desync=disorder", "--new",
			// QUIC (YouTube Fast, Discord)
			"--filter-udp=443", "--filter-l7=quic", "--payload=quic_initial", "--lua-desync=fake:blob=fake_default_quic:repeats=10", "--new",
			// Discord Voice & STUN
			"--filter-udp=3478,50000-65535", "--lua-desync=fake:blob=fake_default_quic:repeats=6",
		)

	case "Discord Voice Optimized":
		args = append([]string{"--wf-tcp-out=443", "--wf-udp-out=443,3478,50000-65535"}, args...)
		args = append(args,
			"--filter-tcp=443", "--filter-l7=tls", "--payload=tls_client_hello", "--lua-desync=fake:blob=fake_default_tls:tcp_md5", "--lua-desync=split:pos=1", "--new",
			"--filter-udp=443", "--filter-l7=quic", "--payload=quic_initial", "--lua-desync=fake:blob=fake_default_quic:repeats=10", "--new",
			"--filter-udp=3478", "--lua-desync=fake:blob=fake_default_quic:repeats=8", "--new",
			"--filter-udp=50000-65535", "--lua-desync=fake:blob=fake_default_quic:repeats=8",
		)

	case "YouTube QUIC Aggressive":
		args = append([]string{"--wf-tcp-out=80,443", "--wf-udp-out=443"}, args...)
		args = append(args,
			"--filter-tcp=80", "--lua-desync=fake:blob=fake_default_http:tcp_md5", "--lua-desync=split:pos=method+2", "--new",
			"--filter-tcp=443", "--filter-l7=tls", "--payload=tls_client_hello", "--lua-desync=fake:blob=fake_default_tls:tcp_md5", "--lua-desync=multisplit:pos=1,midsld", "--new",
			"--filter-udp=443", "--filter-l7=quic", "--payload=quic_initial", "--lua-desync=fake:blob=fake_default_quic:repeats=12", "--lua-desync=udplen:increment=2",
		)

	case "Telegram API Bypass":
		args = append([]string{"--wf-tcp-out=80,443,5222,5223,5228", "--wf-udp-out=443"}, args...)
		args = append(args,
			"--filter-tcp=443", "--filter-l7=tls", "--payload=tls_client_hello", "--lua-desync=fake:blob=fake_default_tls:tcp_md5", "--lua-desync=split:pos=1", "--new",
			"--filter-tcp=5222,5223,5228", "--lua-desync=split:pos=2", "--lua-desync=disorder", "--lua-desync=fake:blob=fake_default_tls:tcp_md5", "--new",
			"--filter-udp=443", "--filter-l7=quic", "--payload=quic_initial", "--lua-desync=fake:blob=fake_default_quic:repeats=8",
		)

	case "Flowseal Legacy (Discord/YT)":
		args = append([]string{"--wf-tcp-out=80,443", "--wf-udp-out=443,50000-65535"}, args...)
		args = append(args,
			"--filter-tcp=443", "--filter-l7=tls", "--payload=tls_client_hello", "--lua-desync=fake:blob=fake_default_tls:tcp_md5", "--lua-desync=split:pos=1", "--new",
			"--filter-udp=443", "--filter-l7=quic", "--payload=quic_initial", "--lua-desync=fake:blob=fake_default_quic:repeats=6", "--new",
			"--filter-udp=50000-65535", "--lua-desync=fake:blob=fake_default_quic:repeats=6",
		)

	case "Fake TLS & QUIC":
		args = append([]string{"--wf-tcp-out=80,443", "--wf-udp-out=443,50000-65535"}, args...)
		args = append(args,
			"--filter-tcp=80,443", "--lua-desync=fake:blob=fake_default_tls:tcp_md5", "--new",
			"--filter-udp=443,50000-65535", "--lua-desync=fake:blob=fake_default_quic:repeats=10",
		)

	case "Split & Disorder":
		args = append([]string{"--wf-tcp-out=80,443", "--wf-udp-out=443"}, args...)
		args = append(args,
			"--filter-tcp=443", "--lua-desync=split:pos=2", "--lua-desync=disorder", "--new",
			"--filter-udp=443", "--lua-desync=fake:blob=fake_default_quic",
		)

	case "Multi-Strategy Chaos":
		args = append([]string{"--wf-tcp-out=80,443", "--wf-udp-out=443,3478,50000-65535"}, args...)
		args = append(args,
			"--filter-tcp=80", "--lua-desync=fake:blob=fake_default_http:tcp_md5", "--lua-desync=multisplit:pos=method+2", "--lua-desync=disorder", "--new",
			"--filter-tcp=443", "--filter-l7=tls", "--payload=tls_client_hello", "--lua-desync=fake:blob=fake_default_tls:tcp_md5", "--lua-desync=multidisorder:pos=1,midsld", "--lua-desync=badseq", "--new",
			"--filter-udp=443", "--filter-l7=quic", "--payload=quic_initial", "--lua-desync=fake:blob=fake_default_quic:repeats=10", "--lua-desync=udplen:increment=2", "--new",
			"--filter-udp=3478,50000-65535", "--lua-desync=fake:blob=fake_default_quic:repeats=8",
		)
	}

	return args
}

func (e *Zapret2WindowsProvider) Start(ctx context.Context, profileName string) error {
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
	winwsPath := filepath.Join(e.binPath, "winws2.exe")

	args := e.getProfileArgs(profileName)
	
	e.cmd = exec.Command(winwsPath, args...)
	e.cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}

	e.addLog(fmt.Sprintf("[%s] Starting with profile: %s", e.Name(), profileName))

	if err := e.cmd.Start(); err != nil {
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

func (e *Zapret2WindowsProvider) Stop() error {
	e.mu.Lock()
	defer e.mu.Unlock()

	if e.cmd != nil && e.cmd.Process != nil {
		e.addLog("Terminating winws2 process...")
		exec.Command("taskkill", "/F", "/T", "/IM", "winws2.exe").Run()
		e.cmd.Process.Kill()
		e.cmd = nil
	}
	e.status = StatusStopped
	e.currentProfile = ""
	return nil
}

func (e *Zapret2WindowsProvider) GetStatus() Status {
	return e.status
}

func (e *Zapret2WindowsProvider) GetLogs() []string {
	return e.logs
}

func (e *Zapret2WindowsProvider) addLog(msg string) {
	e.logs = append(e.logs, msg)
	if len(e.logs) > 100 {
		e.logs = e.logs[1:]
	}
}
