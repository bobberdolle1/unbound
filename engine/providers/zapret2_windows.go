//go:build windows
// +build windows

package providers

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"time"

	"golang.org/x/sys/windows"
)

type Zapret2WindowsProvider struct {
	binPath  string
	luaPath  string
	listPath string
	cmd      *exec.Cmd
	logs     []string
	mu       sync.Mutex
	running  bool
	debugMode bool
	gameMode  bool
}

func NewZapret2WindowsProvider(binPath, luaPath, listPath string, debugMode bool, gameMode bool) *Zapret2WindowsProvider {
	return &Zapret2WindowsProvider{
		binPath:   binPath,
		luaPath:   luaPath,
		listPath:  listPath,
		logs:      make([]string, 0),
		debugMode: debugMode,
		gameMode:  gameMode,
	}
}

func (e *Zapret2WindowsProvider) Name() string {
	return "Zapret 2 (winws)"
}

func (e *Zapret2WindowsProvider) buildArgs(profileArgs []string) []string {
	luaAntiDpi := filepath.ToSlash(filepath.Join(e.luaPath, "zapret-antidpi.lua"))
	
	args := []string{
		"--wf-l3=ipv4,ipv6",
		"--wf-tcp=80,443",
		"--wf-udp=443,50000-65535", // Common ports for YouTube (QUIC) and Discord
	}

	// Only ONE main lua-init script as per zapret2 design
	args = append(args, "--lua-init=@"+luaAntiDpi)

	absBinDir, _ := filepath.Abs(e.binPath)
	
	// Core blobs needed for most strategies
	blobFiles := []string{
		"tls_google:tls_clienthello_www_google_com.bin",
		"quic_google:quic_initial_www_google_com.bin",
		"tls1:tls_clienthello_1.bin",
		"tls2:tls_clienthello_2.bin",
		"syn_packet:syn_packet.bin",
	}
	
	for _, blob := range blobFiles {
		parts := strings.Split(blob, ":")
		if len(parts) == 2 {
			blobPath := filepath.ToSlash(filepath.Join(absBinDir, parts[1]))
			if _, err := os.Stat(blobPath); err == nil {
				args = append(args, "--blob="+parts[0]+":@"+blobPath)
			}
		}
	}

	args = append(args, "--ctrack-disable=0")
	args = append(args, "--ipcache-lifetime=8400")
	args = append(args, "--ipcache-hostname=1")

	if e.debugMode {
		args = append(args, "--debug=1")
	}

	// Append profile-specific args
	args = append(args, profileArgs...)

	return args
}

func (e *Zapret2WindowsProvider) Start(ctx context.Context, profileName string) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	if e.running {
		return fmt.Errorf("engine already running")
	}

	hasPriv, err := e.CheckPrivileges()
	if err != nil || !hasPriv {
		return fmt.Errorf("administrator privileges required")
	}

	exePath := filepath.Join(e.binPath, "winws2.exe")
	if _, err := os.Stat(exePath); os.IsNotExist(err) {
		return fmt.Errorf("binary not found: %s", exePath)
	}

	// We need to find the actual profile args by name
	// This is a simplified lookup for this provider
	var profileArgs []string
	if strings.Contains(profileName, "hostfakesplit") {
		profileArgs = []string{"--filter-tcp=443", "--hostfakesplit=2", "--new", "--filter-udp=443", "--hostfakesplit=2"}
	} else if strings.Contains(profileName, "multisplit") {
		profileArgs = []string{"--filter-tcp=443", "--multisplit=2", "--new", "--filter-udp=443", "--multisplit=2"}
	} else {
		// Default fallback if profile not found in hardcoded list
		// In a real app, we'd pull this from a central registry
		profileArgs = []string{"--filter-tcp=443", "--hostfakesplit=2"}
	}

	args := e.buildArgs(profileArgs)
	e.addLog(fmt.Sprintf("🚀 Starting winws2 with args: %s", strings.Join(args, " ")))

	e.cmd = exec.CommandContext(ctx, exePath, args...)
	e.cmd.Dir = e.binPath
	e.cmd.SysProcAttr = &syscall.SysProcAttr{
		HideWindow:    true,
		CreationFlags: windows.CREATE_NEW_PROCESS_GROUP,
	}

	stdout, _ := e.cmd.StdoutPipe()
	stderr, _ := e.cmd.StderrPipe()

	if err := e.cmd.Start(); err != nil {
		return fmt.Errorf("failed to start winws2: %w", err)
	}

	e.running = true
	go e.streamLogs(stdout)
	go e.streamLogs(stderr)
	go e.waitExit()

	return nil
}

func (e *Zapret2WindowsProvider) Stop() error {
	e.mu.Lock()
	defer e.mu.Unlock()

	if e.cmd != nil && e.cmd.Process != nil {
		e.addLog("🛑 Stopping winws2 engine...")
		// Try graceful kill first
		e.cmd.Process.Signal(os.Interrupt)
		
		done := make(chan error, 1)
		go func() { done <- e.cmd.Wait() }()
		
		select {
		case <-done:
		case <-time.After(2 * time.Second):
			e.cmd.Process.Kill()
		}
	}

	e.running = false
	return nil
}

func (e *Zapret2WindowsProvider) WaitReady(timeout time.Duration) bool {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		e.mu.Lock()
		isRunning := e.running
		e.mu.Unlock()
		
		if !isRunning {
			return false
		}

		// Look for WinDivert initialization in logs
		e.mu.Lock()
		logs := e.logs
		e.mu.Unlock()
		
		for _, log := range logs {
			if strings.Contains(log, "windivert initialized") || strings.Contains(log, "filter set") {
				return true
			}
		}
		time.Sleep(200 * time.Millisecond)
	}
	return false
}

func (e *Zapret2WindowsProvider) streamLogs(r io.Reader) {
	scanner := bufio.NewScanner(r)
	for scanner.Scan() {
		e.addLog(scanner.Text())
	}
}

func (e *Zapret2WindowsProvider) waitExit() {
	if e.cmd != nil {
		err := e.cmd.Wait()
		e.mu.Lock()
		e.running = false
		if err != nil {
			e.addLog(fmt.Sprintf("⚠️ Engine exited with error: %v", err))
		} else {
			e.addLog("✅ Engine stopped gracefully")
		}
		e.mu.Unlock()
	}
}

func (e *Zapret2WindowsProvider) IsRunning() bool {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.running
}

func (e *Zapret2WindowsProvider) GetLogs() []string {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.logs
}

func (e *Zapret2WindowsProvider) addLog(msg string) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.logs = append(e.logs, msg)
	if len(e.logs) > 200 {
		e.logs = e.logs[1:]
	}
}

func (e *Zapret2WindowsProvider) CheckPrivileges() (bool, error) {
	var sid *windows.SID
	err := windows.AllocateAndInitializeSid(
		&windows.SECURITY_NT_AUTHORITY,
		2,
		windows.SECURITY_BUILTIN_DOMAIN_RID,
		windows.DOMAIN_ALIAS_RID_ADMINS,
		0, 0, 0, 0, 0, 0,
		&sid,
	)
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

func (e *Zapret2WindowsProvider) RegisterProfile(name string, args []string) {
	// Not needed for this simple implementation but required by interface
}

func (e *Zapret2WindowsProvider) GetProfiles() []string {
	return []string{"hostfakesplit", "multisplit"}
}

func (e *Zapret2WindowsProvider) GetStatus() Status {
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.running {
		return StatusRunning
	}
	return StatusStopped
}

func (e *Zapret2WindowsProvider) SetStatusCallback(cb func(Status)) {
	// Optional status callback
}
