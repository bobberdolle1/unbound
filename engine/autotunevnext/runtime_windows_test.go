//go:build windows

package autotunevnext

import (
	"context"
	"errors"
	"io"
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

func TestWindowsCaptureReaderKeepsDrainingAfterReadiness(t *testing.T) {
	reader, writer := io.Pipe()
	ready := make(chan struct{}, 1)
	drained := make(chan struct{})
	go func() {
		windowsReadReady(reader, ready)
		close(drained)
	}()
	go func() {
		_, _ = io.WriteString(writer, "WinDivert initialized. capture is started.\n")
		for range 10000 {
			_, _ = io.WriteString(writer, "diagnostic output that must be drained\n")
		}
		_ = writer.Close()
	}()
	select {
	case <-ready:
	case <-time.After(time.Second):
		t.Fatal("capture readiness was not observed")
	}
	select {
	case <-drained:
	case <-time.After(time.Second):
		t.Fatal("reader returned before draining post-readiness output")
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

func TestWindowsDeactivateRetainsOwnershipUntilFactualExit(t *testing.T) {
	runtime, err := NewWindowsRuntime(RuntimeOptions{Provider: &windowsRuntimeProvider{status: providers.StatusRunning, profile: "original"}, Assets: &engine.AssetPaths{}})
	if err != nil {
		t.Fatal(err)
	}
	snapshot, err := runtime.Snapshot(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	done := make(chan struct{})
	close(done)
	runtime.cleanupWait = time.Millisecond
	runtime.processAlive = func(int) (bool, error) { return true, nil }
	runtime.active = &windowsCandidate{done: done, cancel: func() {}, jobClosed: true, pid: 4242}
	runtime.ownedPIDs[4242] = struct{}{}
	if err := runtime.Deactivate(context.Background()); err == nil {
		t.Fatal("accepted a still-alive owned process")
	}
	if runtime.active == nil {
		t.Fatal("lost ownership after failed teardown")
	}
	if err := runtime.VerifyRestored(context.Background(), snapshot); err == nil {
		t.Fatal("restoration passed while owned PID remained alive")
	}
	runtime.processAlive = func(int) (bool, error) { return false, nil }
	if err := runtime.Restore(context.Background(), snapshot); err != nil {
		t.Fatal(err)
	}
	if runtime.active != nil {
		t.Fatal("ownership remained after factual process exit")
	}
	if err := runtime.VerifyRestored(context.Background(), snapshot); err != nil {
		t.Fatal(err)
	}
}

func TestWindowsJobAttachFailureRetainsPostStartOwnership(t *testing.T) {
	provider := &windowsRuntimeProvider{status: providers.StatusRunning, profile: "original"}
	runtime, err := NewWindowsRuntime(RuntimeOptions{Provider: provider, Assets: &engine.AssetPaths{}})
	if err != nil {
		t.Fatal(err)
	}
	snapshot, err := runtime.Snapshot(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	job, err := newKillOnCloseJob()
	if err != nil {
		t.Fatal(err)
	}
	candidateCtx, cancel := context.WithCancel(context.Background())
	cmd := exec.CommandContext(candidateCtx, "powershell.exe", "-NoProfile", "-Command", "Start-Sleep -Seconds 60")
	if err := cmd.Start(); err != nil {
		_ = windows.CloseHandle(job)
		t.Fatal(err)
	}
	done := make(chan struct{})
	go func() {
		_ = cmd.Wait()
		close(done)
	}()
	active := &windowsCandidate{cmd: cmd, done: done, ready: make(chan struct{}, 1), cancel: cancel, job: job, pid: cmd.Process.Pid}
	runtime.active = active
	runtime.ownedPIDs[active.pid] = struct{}{}
	runtime.attachProcess = func(windows.Handle, int) error { return errors.New("job assignment rejected") }
	runtime.processAlive = func(int) (bool, error) { return true, nil }
	attachErr := runtime.attachProcess(job, active.pid)
	if attachErr == nil {
		t.Fatal("injected job attachment failure did not fire")
	}
	if err := runtime.postStartFailure(context.Background(), active, attachErr); err == nil {
		t.Fatal("uncertain post-start cleanup passed")
	}
	if runtime.active == nil {
		t.Fatal("post-start PID ownership disappeared after uncertain cleanup")
	}
	if err := runtime.VerifyRestored(context.Background(), snapshot); err == nil {
		t.Fatal("restoration passed with retained post-start PID")
	}
	runtime.processAlive = func(int) (bool, error) { return false, nil }
	if err := runtime.Restore(context.Background(), snapshot); err != nil {
		t.Fatal(err)
	}
	if runtime.active != nil {
		t.Fatal("factual post-start cleanup did not clear ownership")
	}
	if err := runtime.VerifyRestored(context.Background(), snapshot); err != nil {
		t.Fatal(err)
	}
}
