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

const CREATE_NO_WINDOW = 0x08000000

type Zapret2WindowsProvider struct {
	binPath   string
	luaPath   string
	listPath  string
	cmd       *exec.Cmd
	logs      []string
	mu        sync.Mutex
	running   bool
	debugMode bool
	profiles  map[string][]string
}

func NewZapret2WindowsProvider(binPath, luaPath, listPath string, debugMode bool, gameMode bool) *Zapret2WindowsProvider {
	return &Zapret2WindowsProvider{
		binPath:   binPath,
		luaPath:   luaPath,
		listPath:  listPath,
		logs:      make([]string, 0),
		debugMode: debugMode,
		profiles:  make(map[string][]string),
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
		"--wf-udp=443,50000-65535",
		"--lua-init=@" + luaAntiDpi,
	}

	absBinDir, _ := filepath.Abs(e.binPath)
	blobFiles := []string{
		"tls_google:tls_clienthello_www_google_com.bin",
		"quic_google:quic_initial_www_google_com.bin",
		"syn_packet:syn_packet.bin",
	}
	
	for _, blob := range blobFiles {
		parts := strings.Split(blob, ":")
		blobPath := filepath.ToSlash(filepath.Join(absBinDir, parts[1]))
		if _, err := os.Stat(blobPath); err == nil {
			args = append(args, "--blob="+parts[0]+":@"+blobPath)
		}
	}

	args = append(args, "--ctrack-disable=0", "--ipcache-lifetime=8400")
	if e.debugMode {
		args = append(args, "--debug=1")
	}

	args = append(args, profileArgs...)
	return args
}

func (e *Zapret2WindowsProvider) Start(ctx context.Context, profileName string) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	if e.running {
		return fmt.Errorf("already running")
	}

	exePath := filepath.Join(e.binPath, "winws2.exe")
	args := e.buildArgs(e.profiles[profileName])
	
	e.addLog(fmt.Sprintf("🚀 Launching with profile: %s", profileName))

	e.cmd = exec.CommandContext(ctx, exePath, args...)
	e.cmd.Dir = e.binPath
	e.cmd.SysProcAttr = &syscall.SysProcAttr{
		HideWindow:    true,
		CreationFlags: windows.CREATE_NEW_PROCESS_GROUP | CREATE_NO_WINDOW,
	}

	stdout, _ := e.cmd.StdoutPipe()
	stderr, _ := e.cmd.StderrPipe()

	if err := e.cmd.Start(); err != nil {
		return fmt.Errorf("failed: %w", err)
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
		e.cmd.Process.Kill()
	}
	e.running = false
	return nil
}

func (e *Zapret2WindowsProvider) WaitReady(timeout time.Duration) bool {
	time.Sleep(2 * time.Second) // Simple wait for WinDivert
	return true
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
			e.addLog(fmt.Sprintf("❌ Exited: %v", err))
		}
		e.mu.Unlock()
	}
}

func (e *Zapret2WindowsProvider) IsRunning() bool {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.running
}

func (e *Zapret2WindowsProvider) GetStatus() Status {
	if e.IsRunning() { return StatusRunning }
	return StatusStopped
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
}

func (e *Zapret2WindowsProvider) CheckPrivileges() (bool, error) {
	return true, nil // Checked in app.go
}

func (e *Zapret2WindowsProvider) RegisterProfile(name string, args []string) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.profiles[name] = args
}

func (e *Zapret2WindowsProvider) GetProfiles() []string {
	e.mu.Lock()
	defer e.mu.Unlock()
	keys := make([]string, 0, len(e.profiles))
	for k := range e.profiles {
		keys = append(keys, k)
	}
	return keys
}

func (e *Zapret2WindowsProvider) SetStatusCallback(cb func(Status)) {}
