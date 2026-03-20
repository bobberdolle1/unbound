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

type GoodbyeDPIProvider struct {
	status         Status
	logs           []string
	cmd            *exec.Cmd
	mu             sync.Mutex
	binPath        string
	currentProfile string
}

func NewGoodbyeDPIProvider(binPath string) BypassProvider {
	return &GoodbyeDPIProvider{
		status:  StatusStopped,
		binPath: binPath,
		logs:    []string{"GoodbyeDPI Engine initialized."},
	}
}

func (e *GoodbyeDPIProvider) Name() string {
	return "GoodbyeDPI"
}

func (e *GoodbyeDPIProvider) CheckPrivileges() (bool, error) {
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

func (e *GoodbyeDPIProvider) GetProfiles() []string {
	return []string{
		"Mode -9 (Max Bypass / Reverse Frag)", 
		"Mode -7 (Standard Bypass)", 
		"Mode -5 (Compatibility)", 
		"Mode -1 (Legacy DPI)",
	}
}

func (e *GoodbyeDPIProvider) getProfileArgs(profileName string) []string {
	// Base argument for DNS protection
	args := []string{"--dns-addr", "1.1.1.1", "--dns-port", "53", "--dnsv6-addr", "2606:4700:4700::1111", "--dnsv6-port", "53"}

	switch profileName {
	case "Mode -9 (Max Bypass / Reverse Frag)":
		args = append(args, "-9") // Equivalent to -p -r -s -f 2 -k 2 -n -e 2 -g 2 -a -q --wrong-seq --wrong-chksum --reverse-frag
	case "Mode -7 (Standard Bypass)":
		args = append(args, "-7") // Equivalent to -p -r -s -f 2 -k 2 -n -e 2 -g 2 -a -q --wrong-chksum --reverse-frag
	case "Mode -5 (Compatibility)":
		args = append(args, "-5") // Equivalent to -p -r -s -f 2 -k 2 -n -e 2 -g 2 -a -q
	case "Mode -1 (Legacy DPI)":
		args = append(args, "-1") // Equivalent to -p -r -s -f 2 -k 2 -n -e 2 -g 2 -a
	default:
		args = append(args, "-7")
	}

	return args
}

func (e *GoodbyeDPIProvider) Start(ctx context.Context, profileName string) error {
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
	gdpiPath := filepath.Join(e.binPath, "goodbyedpi.exe")

	args := e.getProfileArgs(profileName)
	
	e.cmd = exec.Command(gdpiPath, args...)
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

func (e *GoodbyeDPIProvider) Stop() error {
	e.mu.Lock()
	defer e.mu.Unlock()

	if e.cmd != nil && e.cmd.Process != nil {
		e.addLog("Terminating goodbyedpi process...")
		exec.Command("taskkill", "/F", "/T", "/IM", "goodbyedpi.exe").Run()
		e.cmd.Process.Kill()
		e.cmd = nil
	}
	e.status = StatusStopped
	e.currentProfile = ""
	return nil
}

func (e *GoodbyeDPIProvider) GetStatus() Status {
	return e.status
}

func (e *GoodbyeDPIProvider) GetLogs() []string {
	return e.logs
}

func (e *GoodbyeDPIProvider) addLog(msg string) {
	e.logs = append(e.logs, msg)
	if len(e.logs) > 100 {
		e.logs = e.logs[1:]
	}
}
