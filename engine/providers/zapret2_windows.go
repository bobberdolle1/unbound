//go:build windows
// +build windows

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
	"golang.org/x/sys/windows"
)

const (
	configDirName    = "Unbound"
	customScriptName = "custom_profile.lua"
)

func getCustomScriptPath() (string, error) {
	userConfigDir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	configPath := filepath.Join(userConfigDir, configDirName)
	return filepath.Join(configPath, customScriptName), nil
}

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
	InitLogger()
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
		"Fake TLS & QUIC", 
		"Multi-Strategy Chaos",
		"Standard Split",
		"Fake Packets + BadSeq",
		"Disorder",
		"Split Handshake",
		"Flowseal Legacy",
		"Custom Profile",
	}
}

func (e *Zapret2WindowsProvider) getProfileArgs(profileName string) []string {
	absLuaLib, _ := filepath.Abs(filepath.Join(e.luaDir, "zapret-lib.lua"))
	absLuaAntiDpi, _ := filepath.Abs(filepath.Join(e.luaDir, "zapret-antidpi.lua"))
	
	luaLib := filepath.ToSlash(absLuaLib)
	luaAntiDpi := filepath.ToSlash(absLuaAntiDpi)

	args := []string{
		"--intercept=1",
		"--lua-init=@" + luaLib,
		"--lua-init=@" + luaAntiDpi,
	}

	switch profileName {
	case "Unbound Ultimate (God Mode)":
		args = append(args, "--wf-tcp-out=443", "--filter-l7=tls", "--payload=tls_client_hello", "--lua-desync=fake:blob=fake_default_tls", "--lua-desync=multisplit:pos=1", "--new")
		args = append(args, "--wf-udp-out=443", "--filter-l7=quic", "--payload=quic_initial", "--lua-desync=fake:blob=fake_default_quic:repeats=10", "--new")
		args = append(args, "--wf-udp-out=3478,50000-65535", "--lua-desync=fake:blob=fake_default_quic:repeats=6")

	case "Discord Voice Optimized":
		args = append(args, "--wf-tcp-out=443", "--filter-l7=tls", "--payload=tls_client_hello", "--lua-desync=fake:blob=fake_default_tls", "--lua-desync=multisplit:pos=1", "--new")
		args = append(args, "--wf-udp-out=443,3478,50000-65535", "--lua-desync=fake:blob=fake_default_quic:repeats=10")

	case "YouTube QUIC Aggressive":
		args = append(args, "--wf-tcp-out=80,443", "--lua-desync=fake:blob=fake_default_tls", "--lua-desync=multisplit:pos=1,midsld", "--new")
		args = append(args, "--wf-udp-out=443", "--filter-l7=quic", "--payload=quic_initial", "--lua-desync=fake:blob=fake_default_quic:repeats=12")

	case "Telegram API Bypass":
		args = append(args, "--wf-tcp-out=443,5222,5223,5228", "--lua-desync=fake:blob=fake_default_tls", "--lua-desync=multisplit:pos=1", "--new")
		args = append(args, "--wf-udp-out=443", "--lua-desync=fake:blob=fake_default_quic:repeats=8")

	case "Fake TLS & QUIC":
		args = append(args, "--wf-tcp-out=80,443", "--lua-desync=fake:blob=fake_default_tls", "--new")
		args = append(args, "--wf-udp-out=443,50000-65535", "--lua-desync=fake:blob=fake_default_quic:repeats=10")

	case "Multi-Strategy Chaos":
		args = append(args, "--wf-tcp-out=80,443", "--lua-desync=fake:blob=fake_default_tls", "--lua-desync=multidisorder:pos=1,midsld", "--new")
		args = append(args, "--wf-udp-out=443", "--lua-desync=fake:blob=fake_default_quic:repeats=10", "--new")
		args = append(args, "--wf-udp-out=3478,50000-65535", "--lua-desync=fake:blob=fake_default_quic:repeats=8")

	case "Standard Split":
		args = append(args, "--wf-tcp-out=443", "--filter-l7=tls", "--payload=tls_client_hello", "--lua-desync=fake:blob=fake_default_tls", "--lua-desync=multisplit:pos=1")

	case "Fake Packets + BadSeq":
		args = append(args, "--wf-tcp-out=443", "--filter-l7=tls", "--payload=tls_client_hello", "--lua-desync=fake:blob=fake_default_tls", "--lua-desync=multidisorder:pos=1,midsld")

	case "Disorder":
		args = append(args, "--wf-tcp-out=443", "--lua-desync=multisplit:pos=2", "--lua-desync=disorder")

	case "Split Handshake":
		args = append(args, "--wf-tcp-out=443", "--filter-l7=tls", "--payload=tls_client_hello", "--lua-desync=fake:blob=fake_default_tls", "--lua-desync=multisplit:pos=midsld")

	case "Flowseal Legacy":
		args = append(args, "--wf-tcp-out=443", "--filter-l7=tls", "--payload=tls_client_hello", "--lua-desync=fake:blob=fake_default_tls", "--lua-desync=multisplit:pos=1", "--new",
			"--wf-udp-out=443", "--filter-l7=quic", "--payload=quic_initial", "--lua-desync=fake:blob=fake_default_quic:repeats=6", "--new",
			"--wf-udp-out=50000-65535", "--lua-desync=fake:blob=fake_default_quic:repeats=6")
	
	case "Custom Profile":
		customScriptPath, err := getCustomScriptPath()
		if err == nil {
			absCustomScript, _ := filepath.Abs(customScriptPath)
			customScriptSlash := filepath.ToSlash(absCustomScript)
			args = append(args, "--lua-init=@"+customScriptSlash)
			args = append(args, "--wf-tcp-out=443", "--filter-l7=tls", "--payload=tls_client_hello", "--lua-desync=fake:blob=fake_default_tls", "--lua-desync=multisplit:pos=1")
		}
	}

	return args
}

func (e *Zapret2WindowsProvider) Start(ctx context.Context, profileName string) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	WriteLog(fmt.Sprintf("START: Profile=%s, CurrentStatus=%s", profileName, e.status))

	if e.status == StatusRunning && e.currentProfile == profileName {
		WriteLog("START: Already running same profile, skipping")
		return nil
	}

	if e.status == StatusRunning {
		WriteLog("START: Stopping previous profile")
		e.mu.Unlock()
		e.Stop()
		e.mu.Lock()
	}

	e.status = StatusStarting
	winwsPath := filepath.Join(e.binPath, "winws.exe")

	args := e.getProfileArgs(profileName)
	WriteLog(fmt.Sprintf("START: Command=%s Args=%v", winwsPath, args))
	
	e.cmd = exec.Command(winwsPath, args...)
	e.cmd.Dir = e.binPath
	e.cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
	
	stdout, _ := e.cmd.StdoutPipe()
	stderr, _ := e.cmd.StderrPipe()

	e.addLog(fmt.Sprintf("[%s] Starting with profile: %s", e.Name(), profileName))

	if err := e.cmd.Start(); err != nil {
		e.status = StatusError
		errMsg := "Launch Error: " + err.Error()
		e.addLog(errMsg)
		WriteLog("START: " + errMsg)
		return err
	}

	WriteLog(fmt.Sprintf("START: Process started PID=%d", e.cmd.Process.Pid))

	var wg sync.WaitGroup
	var lastStderr string

	wg.Add(1)
	go func() {
		defer wg.Done()
		buf := make([]byte, 1024)
		for {
			n, err := stdout.Read(buf)
			if n > 0 {
				msg := string(buf[:n])
				e.mu.Lock()
				e.addLog(msg)
				e.mu.Unlock()
				WriteLog("STDOUT: " + strings.TrimSpace(msg))
			}
			if err != nil {
				break
			}
		}
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		buf := make([]byte, 1024)
		for {
			n, err := stderr.Read(buf)
			if n > 0 {
				msg := string(buf[:n])
				e.mu.Lock()
				e.addLog(msg)
				lastStderr = msg
				e.mu.Unlock()
				WriteLog("STDERR: " + strings.TrimSpace(msg))
				
				lowerMsg := strings.ToLower(msg)
				if strings.Contains(lowerMsg, "error") || strings.Contains(lowerMsg, "fail") {
					WriteLog("ERROR DETECTED: " + strings.TrimSpace(msg))
				}
			}
			if err != nil {
				break
			}
		}
	}()

	e.status = StatusRunning
	e.currentProfile = profileName
	e.addLog("Engine is ACTIVE.")
	WriteLog("START: Engine status set to RUNNING")

	go func() {
		err := e.cmd.Wait()
		wg.Wait()
		
		e.mu.Lock()
		defer e.mu.Unlock()
		if e.currentProfile == profileName {
			e.status = StatusStopped
			exitMsg := "Engine stopped"
			if err != nil {
				exitMsg = fmt.Sprintf("Engine stopped unexpectedly. Code: %v", err)
				if lastStderr != "" {
					exitMsg += " | Details: " + strings.TrimSpace(lastStderr)
				}
			} else {
				exitMsg = "Engine stopped gracefully"
			}
			e.addLog(exitMsg)
			WriteLog("WAIT: " + exitMsg)
			
			if ctx != context.Background() {
				runtime.EventsEmit(ctx, "status_changed", e.status)
			}
		}
	}()

	return nil
}

func (e *Zapret2WindowsProvider) Stop() error {
	e.mu.Lock()
	defer e.mu.Unlock()

	WriteLog("STOP: Called")

	if e.cmd != nil && e.cmd.Process != nil {
		e.addLog("Terminating winws process tree...")
		WriteLog(fmt.Sprintf("STOP: Killing PID=%d", e.cmd.Process.Pid))
		exec.Command("taskkill", "/F", "/T", "/IM", "winws.exe").Run()
		e.cmd = nil
	}
	e.status = StatusStopped
	e.currentProfile = ""
	WriteLog("STOP: Complete")
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
