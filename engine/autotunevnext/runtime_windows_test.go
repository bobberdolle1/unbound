//go:build windows

package autotunevnext

import (
	"context"
	"errors"
	"os/exec"
	"strings"
	"testing"
	"time"

	"unbound/engine"
	"unbound/engine/providers"

	"golang.org/x/sys/windows"
)

func TestWindowsCaptureReadinessRequiresCanonicalMarker(t *testing.T) {
	ready := make(chan struct{}, 1)
	go windowsReadReady(strings.NewReader("booting\nWinDivert initialized. capture is started.\n"), ready)
	select {
	case <-ready:
	case <-time.After(time.Second):
		t.Fatal("canonical capture-ready marker was not observed")
	}

	absent := make(chan struct{}, 1)
	windowsReadReady(strings.NewReader("capture configured but not started\n"), absent)
	select {
	case <-absent:
		t.Fatal("noncanonical output passed readiness gate")
	default:
	}
}

type windowsRuntimeProvider struct {
	status  providers.Status
	profile string
	stopErr error
}

func (p *windowsRuntimeProvider) CheckPrivileges() (bool, error) { return true, nil }
func (p *windowsRuntimeProvider) Start(_ context.Context, profile string) error {
	p.status, p.profile = providers.StatusRunning, profile
	return nil
}
func (p *windowsRuntimeProvider) Stop() error {
	if p.stopErr != nil {
		return p.stopErr
	}
	p.status, p.profile = providers.StatusStopped, ""
	return nil
}
func (p *windowsRuntimeProvider) GetStatus() providers.Status { return p.status }
func (p *windowsRuntimeProvider) CurrentProfile() string      { return p.profile }
func (p *windowsRuntimeProvider) Name() string                { return "fake-windows" }

func TestWindowsRuntimeDirectLifecycleRestoresOriginalProvider(t *testing.T) {
	provider := &windowsRuntimeProvider{status: providers.StatusRunning, profile: "original"}
	runtime, err := NewWindowsRuntime(RuntimeOptions{Provider: provider, Assets: &engine.AssetPaths{}})
	if err != nil {
		t.Fatal(err)
	}
	snapshot, err := runtime.Snapshot(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if err := runtime.EstablishDirect(context.Background(), snapshot); err != nil {
		t.Fatal(err)
	}
	if provider.status != providers.StatusStopped || provider.profile != "" {
		t.Fatalf("direct lifecycle did not stop original provider: %#v", provider)
	}
	if err := runtime.Restore(context.Background(), snapshot); err != nil {
		t.Fatal(err)
	}
	if err := runtime.VerifyRestored(context.Background(), snapshot); err != nil {
		t.Fatal(err)
	}
}

func TestWindowsRuntimeDirectLifecycleRefusesUnstoppedProvider(t *testing.T) {
	provider := &windowsRuntimeProvider{status: providers.StatusRunning, profile: "original", stopErr: errors.New("stop failed")}
	runtime, err := NewWindowsRuntime(RuntimeOptions{Provider: provider, Assets: &engine.AssetPaths{}})
	if err != nil {
		t.Fatal(err)
	}
	snapshot, err := runtime.Snapshot(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if err := runtime.EstablishDirect(context.Background(), snapshot); err == nil {
		t.Fatal("accepted uncertain direct-observation boundary")
	}
}

func TestWindowsCrashGuardKillsOwnedStuckProcess(t *testing.T) {
	runtime, err := NewWindowsRuntime(RuntimeOptions{Provider: &windowsRuntimeProvider{}, Assets: &engine.AssetPaths{}})
	if err != nil {
		t.Fatal(err)
	}
	job, err := newKillOnCloseJob()
	if err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command("powershell.exe", "-NoProfile", "-Command", "Start-Sleep -Seconds 60")
	if err := cmd.Start(); err != nil {
		_ = windows.CloseHandle(job)
		t.Fatal(err)
	}
	process, err := windows.OpenProcess(windows.PROCESS_SET_QUOTA|windows.PROCESS_TERMINATE, false, uint32(cmd.Process.Pid))
	if err != nil {
		_ = cmd.Process.Kill()
		_ = windows.CloseHandle(job)
		t.Fatal(err)
	}
	err = windows.AssignProcessToJobObject(job, process)
	_ = windows.CloseHandle(process)
	if err != nil {
		_ = cmd.Process.Kill()
		_ = windows.CloseHandle(job)
		t.Fatal(err)
	}
	done := make(chan struct{})
	go func() {
		_ = cmd.Wait()
		close(done)
	}()
	runtime.active = &windowsCandidate{cmd: cmd, done: done, ready: make(chan struct{}, 1), cancel: func() {}, job: job, pid: cmd.Process.Pid}
	if err := runtime.Deactivate(context.Background()); err != nil {
		t.Fatal(err)
	}
	select {
	case <-done:
	default:
		t.Fatal("owned process remained after job close")
	}
}
