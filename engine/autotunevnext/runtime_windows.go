//go:build windows

package autotunevnext

import (
	"bufio"
	"context"
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"time"
	"unsafe"

	"unbound/engine"
	"unbound/engine/backendcap"
	"unbound/engine/providers"

	"golang.org/x/sys/windows"
)

const windowsCaptureReadyMarker = "windivert initialized. capture is started."

type WindowsRuntime struct {
	opts      RuntimeOptions
	mu        sync.Mutex
	snapshots map[string]providerSnapshot
	nextID    atomic.Uint64
	active    *windowsCandidate
}

type windowsCandidate struct {
	cmd    *exec.Cmd
	done   chan struct{}
	ready  chan struct{}
	cancel context.CancelFunc
	job    windows.Handle
	pid    int
}

func NewWindowsRuntime(options RuntimeOptions) (*WindowsRuntime, error) {
	if options.Provider == nil || options.Assets == nil {
		return nil, fmt.Errorf("Windows vNext runtime needs product provider and assets")
	}
	return &WindowsRuntime{opts: options, snapshots: make(map[string]providerSnapshot)}, nil
}

func (e *WindowsRuntime) Resolve(ctx context.Context, backend backendcap.Backend, ids []string) ([]ResolvedAsset, error) {
	resolver, err := NewProductAssetResolver(e.opts.Assets)
	if err != nil {
		return nil, err
	}
	return resolver.Resolve(ctx, backend, ids)
}

func (e *WindowsRuntime) Check(_ context.Context, request HostPreflightRequest) HostPreflightResult {
	if request.Backend != backendcap.Zapret2Windows {
		return unsupported("BACKEND_OS_MISMATCH")
	}
	if request.Capture.BackendKind != backendcap.CaptureWinDivert {
		return unsupported("UNSUPPORTED_CAPTURE")
	}
	if e.opts.Provider.GetStatus() == providers.StatusStarting || e.opts.Provider.GetStatus() == providers.StatusStopping {
		return unsupported("PROVIDER_STATE_UNCERTAIN")
	}
	privileged, err := e.opts.Provider.CheckPrivileges()
	if err != nil || !privileged {
		return unsupported("PRIVILEGES_REQUIRED")
	}
	if err := engine.VerifyExtractedAssets(e.opts.Assets); err != nil {
		return errored("ASSET_IDENTITY_INVALID", err)
	}
	if _, err := verifyRuntimeBinary(filepath.Join(e.opts.Assets.BinDir, "winws2.exe"), e.opts.Assets.EngineSHA256); err != nil {
		return errored("ENGINE_IDENTITY_INVALID", err)
	}
	if request.Requirements.TCPTimestamps == engine.TimestampsRequired && !runtimeTimestampsActive() {
		return unsupported("TCP_TIMESTAMPS_REQUIRED")
	}
	e.mu.Lock()
	active := e.active != nil
	e.mu.Unlock()
	if active {
		return unsupported("OWNED_AUTOTUNE_PROCESS_ACTIVE")
	}
	return HostPreflightResult{Status: PreflightSupported}
}

func (e *WindowsRuntime) Snapshot(_ context.Context) (StateSnapshot, error) {
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.active != nil {
		return StateSnapshot{}, fmt.Errorf("owned AutoTune candidate remains active")
	}
	id := fmt.Sprintf("windows-%d", e.nextID.Add(1))
	e.snapshots[id] = providerSnapshot{status: e.opts.Provider.GetStatus(), profile: e.opts.Provider.CurrentProfile()}
	e.opts.emit(PhysicalLog{ExperimentID: id, Backend: string(backendcap.Zapret2Windows), Phase: "SNAPSHOT"})
	return StateSnapshot{ID: id}, nil
}

func (e *WindowsRuntime) EstablishDirect(_ context.Context, snapshot StateSnapshot) error {
	if _, err := e.snapshot(snapshot); err != nil {
		return err
	}
	if err := e.opts.Provider.Stop(); err != nil {
		return fmt.Errorf("stop original provider for direct observation: %w", err)
	}
	if e.opts.Provider.GetStatus() != providers.StatusStopped || e.opts.Provider.CurrentProfile() != "" {
		return fmt.Errorf("provider is not factually stopped for direct observation")
	}
	e.opts.emit(PhysicalLog{ExperimentID: snapshot.ID, Backend: string(backendcap.Zapret2Windows), Phase: "DIRECT_ESTABLISHED"})
	return nil
}

func (e *WindowsRuntime) Activate(ctx context.Context, candidate ExecutableCandidate) error {
	plan, err := NewWindowsExactPlan(candidate)
	if err != nil {
		return err
	}
	filter, err := RenderWindowsTargetCapture(candidate.Plan.Capture, candidate.TargetEdge, candidate.TargetFamily)
	if err != nil {
		return fmt.Errorf("exact target-edge capture: %w", err)
	}
	if err := engine.VerifyExtractedAssets(e.opts.Assets); err != nil {
		return fmt.Errorf("verify runtime assets before activation: %w", err)
	}
	binary := filepath.Join(e.opts.Assets.BinDir, "winws2.exe")
	if _, err := verifyRuntimeBinary(binary, e.opts.Assets.EngineSHA256); err != nil {
		return err
	}
	args := trustedLuaInitArgs(e.opts.Assets.LuaDir)
	args = append(args, plan.CaptureArgv...)
	args = append(args, "--wf-raw-filter="+filter)
	args = append(args, plan.EngineArgv...)

	e.mu.Lock()
	if e.active != nil {
		e.mu.Unlock()
		return fmt.Errorf("owned AutoTune candidate is already active")
	}
	candidateCtx, cancel := context.WithTimeout(ctx, physicalCandidateTimeout)
	cmd := exec.CommandContext(candidateCtx, binary, args...)
	cmd.Dir = e.opts.Assets.BinDir
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		cancel()
		e.mu.Unlock()
		return err
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		cancel()
		e.mu.Unlock()
		return err
	}
	job, err := newKillOnCloseJob()
	if err != nil {
		cancel()
		e.mu.Unlock()
		return err
	}
	if err := cmd.Start(); err != nil {
		_ = windows.CloseHandle(job)
		cancel()
		e.mu.Unlock()
		return fmt.Errorf("start verified winws2: %w", err)
	}
	process, err := windows.OpenProcess(windows.PROCESS_SET_QUOTA|windows.PROCESS_TERMINATE, false, uint32(cmd.Process.Pid))
	if err == nil {
		err = windows.AssignProcessToJobObject(job, process)
		_ = windows.CloseHandle(process)
	}
	if err != nil {
		_ = cmd.Process.Kill()
		_ = windows.CloseHandle(job)
		cancel()
		e.mu.Unlock()
		return fmt.Errorf("attach owned winws2 to crash guard job: %w", err)
	}
	active := &windowsCandidate{cmd: cmd, done: make(chan struct{}), ready: make(chan struct{}, 1), cancel: cancel, job: job, pid: cmd.Process.Pid}
	e.active = active
	e.mu.Unlock()
	go windowsReadReady(stdout, active.ready)
	go windowsReadReady(stderr, active.ready)
	go func() { _ = cmd.Wait(); close(active.done) }()
	e.opts.emit(PhysicalLog{Backend: string(backendcap.Zapret2Windows), StrategyID: candidate.Strategy.ID, Fingerprint: shortFingerprint(candidate.Fingerprint), Edge: candidate.TargetEdge.String(), Phase: "STARTING", PID: active.pid})
	select {
	case <-active.ready:
		e.opts.emit(PhysicalLog{Backend: string(backendcap.Zapret2Windows), StrategyID: candidate.Strategy.ID, Edge: candidate.TargetEdge.String(), Phase: "CAPTURE_READY", PID: active.pid})
		return nil
	case <-active.done:
		_ = e.Deactivate(context.Background())
		return fmt.Errorf("winws2 exited before CAPTURE_READY")
	case <-ctx.Done():
		_ = e.Deactivate(context.WithoutCancel(ctx))
		return ctx.Err()
	case <-time.After(10 * time.Second):
		_ = e.Deactivate(context.Background())
		return fmt.Errorf("timed out waiting for CAPTURE_READY")
	}
}

func (e *WindowsRuntime) VerifyActive(_ context.Context, _ ExecutableCandidate) error {
	e.mu.Lock()
	active := e.active
	e.mu.Unlock()
	if active == nil {
		return fmt.Errorf("no owned candidate to verify")
	}
	select {
	case <-active.done:
		return fmt.Errorf("owned winws2 exited after CAPTURE_READY")
	default:
		return nil
	}
}

func (e *WindowsRuntime) Deactivate(_ context.Context) error {
	e.mu.Lock()
	active := e.active
	if active == nil {
		e.mu.Unlock()
		return nil
	}
	e.active = nil
	e.mu.Unlock()
	active.cancel()
	_ = windows.CloseHandle(active.job) // KILL_ON_JOB_CLOSE is the parent-crash guard.
	select {
	case <-active.done:
		e.opts.emit(PhysicalLog{Backend: string(backendcap.Zapret2Windows), Phase: "EXITED", PID: active.pid})
		return nil
	case <-time.After(10 * time.Second):
		return fmt.Errorf("owned winws2 PID %d did not exit after crash-guard teardown", active.pid)
	}
}

func (e *WindowsRuntime) Restore(ctx context.Context, snapshot StateSnapshot) error {
	state, err := e.snapshot(snapshot)
	if err != nil {
		return err
	}
	if err := e.Deactivate(context.WithoutCancel(ctx)); err != nil {
		return err
	}
	if state.status != providers.StatusRunning {
		if err := e.opts.Provider.Stop(); err != nil {
			return fmt.Errorf("restore stopped provider state: %w", err)
		}
		return nil
	}
	if state.profile == "" {
		return fmt.Errorf("running provider snapshot lacks profile identity")
	}
	if err := e.opts.Provider.Start(context.WithoutCancel(ctx), state.profile); err != nil {
		return fmt.Errorf("restore provider profile %q: %w", state.profile, err)
	}
	return nil
}

func (e *WindowsRuntime) VerifyRestored(_ context.Context, snapshot StateSnapshot) error {
	state, err := e.snapshot(snapshot)
	if err != nil {
		return err
	}
	e.mu.Lock()
	active := e.active != nil
	e.mu.Unlock()
	if active || e.opts.Provider.GetStatus() != state.status || e.opts.Provider.CurrentProfile() != state.profile {
		return fmt.Errorf("provider or owned candidate does not match original snapshot")
	}
	e.opts.emit(PhysicalLog{ExperimentID: snapshot.ID, Backend: string(backendcap.Zapret2Windows), Phase: "RESTORE_VERIFIED"})
	return nil
}

func (e *WindowsRuntime) snapshot(snapshot StateSnapshot) (providerSnapshot, error) {
	e.mu.Lock()
	defer e.mu.Unlock()
	state, ok := e.snapshots[snapshot.ID]
	if !ok {
		return providerSnapshot{}, fmt.Errorf("unknown state snapshot")
	}
	return state, nil
}

func windowsReadReady(reader interface{ Read([]byte) (int, error) }, ready chan<- struct{}) {
	scanner := bufio.NewScanner(reader)
	for scanner.Scan() {
		if strings.Contains(strings.ToLower(scanner.Text()), windowsCaptureReadyMarker) {
			select {
			case ready <- struct{}{}:
			default:
			}
			return
		}
	}
}

func newKillOnCloseJob() (windows.Handle, error) {
	job, err := windows.CreateJobObject(nil, nil)
	if err != nil {
		return 0, err
	}
	limits := windows.JOBOBJECT_EXTENDED_LIMIT_INFORMATION{}
	limits.BasicLimitInformation.LimitFlags = windows.JOB_OBJECT_LIMIT_KILL_ON_JOB_CLOSE
	if _, err := windows.SetInformationJobObject(job, windows.JobObjectExtendedLimitInformation, uintptr(unsafe.Pointer(&limits)), uint32(unsafe.Sizeof(limits))); err != nil {
		_ = windows.CloseHandle(job)
		return 0, err
	}
	return job, nil
}
