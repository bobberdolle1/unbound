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

const windowsStillActive = 259

type WindowsRuntime struct {
	opts          RuntimeOptions
	mu            sync.Mutex
	snapshots     map[string]providerSnapshot
	nextID        atomic.Uint64
	active        *windowsCandidate
	ownedPIDs     map[int]struct{}
	attachProcess func(windows.Handle, int) error
	processAlive  func(int) (bool, error)
	cleanupWait   time.Duration
}

type windowsCandidate struct {
	cmd         *exec.Cmd
	done        chan struct{}
	ready       chan struct{}
	cancel      context.CancelFunc
	job         windows.Handle
	jobClosed   bool
	cleaning    bool
	driverOwned bool
	pid         int
}

func NewWindowsRuntime(options RuntimeOptions) (*WindowsRuntime, error) {
	if options.Provider == nil || options.Assets == nil {
		return nil, fmt.Errorf("Windows vNext runtime needs product provider and assets")
	}
	return &WindowsRuntime{
		opts:          options,
		snapshots:     make(map[string]providerSnapshot),
		ownedPIDs:     make(map[int]struct{}),
		attachProcess: attachWindowsProcess,
		processAlive:  windowsProcessAlive,
		cleanupWait:   10 * time.Second,
	}, nil
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
	if request.Capture.Transport != backendcap.CaptureTransportTCP {
		return unsupported("UNSUPPORTED_CAPTURE_TRANSPORT")
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
	driverActive, err := windowsWinDivertRunning()
	if err != nil {
		return errored("WINDIVERT_STATE_UNAVAILABLE", err)
	}
	if driverActive {
		return unsupported("WINDIVERT_OWNERSHIP_COLLISION")
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

	driverActive, err := windowsWinDivertRunning()
	if err != nil {
		return fmt.Errorf("inspect WinDivert ownership before activation: %w", err)
	}
	if driverActive {
		return fmt.Errorf("WinDivert is already active; refusing to claim shared capture state")
	}
	e.mu.Lock()
	if e.active != nil {
		e.mu.Unlock()
		return fmt.Errorf("owned AutoTune candidate is already active")
	}
	candidateCtx, cancel := candidateExecutionContext(ctx)
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
	active := &windowsCandidate{cmd: cmd, done: make(chan struct{}), ready: make(chan struct{}, 1), cancel: cancel, job: job, driverOwned: true, pid: cmd.Process.Pid}
	e.active = active
	e.ownedPIDs[active.pid] = struct{}{}
	e.mu.Unlock()
	go windowsReadReady(stdout, active.ready)
	go windowsReadReady(stderr, active.ready)
	go func() { _ = cmd.Wait(); close(active.done) }()
	if err := e.attachProcess(job, active.pid); err != nil {
		return e.postStartFailure(ctx, active, fmt.Errorf("attach owned winws2 to crash guard job: %w", err))
	}
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

func (e *WindowsRuntime) postStartFailure(ctx context.Context, _ *windowsCandidate, cause error) error {
	cleanupCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), e.cleanupTimeout())
	defer cancel()
	if err := e.Deactivate(cleanupCtx); err != nil {
		return fmt.Errorf("%w; retain owned startup PID after rollback failure: %v", cause, err)
	}
	return cause
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
	active.cleaning = true
	closeJob := !active.jobClosed
	e.mu.Unlock()

	active.cancel()
	if closeJob {
		if err := windows.CloseHandle(active.job); err != nil {
			return fmt.Errorf("close owned winws2 crash-guard job: %w", err)
		}
		e.mu.Lock()
		if e.active == active {
			active.jobClosed = true
		}
		e.mu.Unlock()
	}
	select {
	case <-active.done:
	case <-time.After(e.cleanupTimeout()):
		return fmt.Errorf("owned winws2 PID %d did not exit after crash-guard teardown", active.pid)
	}
	alive, err := e.processAlive(active.pid)
	if err != nil {
		return fmt.Errorf("audit owned winws2 PID %d after teardown: %w", active.pid, err)
	}
	if alive {
		return fmt.Errorf("owned winws2 PID %d remains alive after teardown", active.pid)
	}
	if active.driverOwned {
		if err := windowsStopWinDivert(e.cleanupTimeout()); err != nil {
			return fmt.Errorf("stop owned WinDivert driver after owned PID %d exit: %w", active.pid, err)
		}
	}
	e.mu.Lock()
	if e.active == active {
		e.active = nil
	}
	e.mu.Unlock()
	e.opts.emit(PhysicalLog{Backend: string(backendcap.Zapret2Windows), Phase: "EXITED", PID: active.pid})
	return nil
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
	active := e.active
	pids := make([]int, 0, len(e.ownedPIDs))
	for pid := range e.ownedPIDs {
		pids = append(pids, pid)
	}
	e.mu.Unlock()
	if active != nil {
		alive, auditErr := e.processAlive(active.pid)
		if auditErr != nil {
			return fmt.Errorf("audit retained owned winws2 PID %d: %w", active.pid, auditErr)
		}
		if alive {
			return fmt.Errorf("owned winws2 PID %d remains active", active.pid)
		}
		return fmt.Errorf("owned winws2 cleanup remains incomplete")
	}
	for _, pid := range pids {
		alive, auditErr := e.processAlive(pid)
		if auditErr != nil {
			return fmt.Errorf("audit retired owned winws2 PID %d: %w", pid, auditErr)
		}
		if alive {
			return fmt.Errorf("retired owned winws2 PID %d remains active", pid)
		}
	}
	if e.opts.Provider.GetStatus() != state.status || e.opts.Provider.CurrentProfile() != state.profile {
		return fmt.Errorf("provider does not match original snapshot")
	}
	e.mu.Lock()
	clear(e.ownedPIDs)
	e.mu.Unlock()
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

func (e *WindowsRuntime) cleanupTimeout() time.Duration {
	if e.cleanupWait > 0 {
		return e.cleanupWait
	}
	return 10 * time.Second
}

func windowsReadReady(reader interface{ Read([]byte) (int, error) }, ready chan<- struct{}) {
	scanner := bufio.NewScanner(reader)
	signaled := false
	for scanner.Scan() {
		if !signaled && strings.Contains(strings.ToLower(scanner.Text()), windowsCaptureReadyMarker) {
			select {
			case ready <- struct{}{}:
			default:
			}
			signaled = true
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

func attachWindowsProcess(job windows.Handle, pid int) error {
	process, err := windows.OpenProcess(windows.PROCESS_SET_QUOTA|windows.PROCESS_TERMINATE, false, uint32(pid))
	if err != nil {
		return err
	}
	defer windows.CloseHandle(process)
	return windows.AssignProcessToJobObject(job, process)
}

func windowsProcessAlive(pid int) (bool, error) {
	process, err := windows.OpenProcess(windows.PROCESS_QUERY_LIMITED_INFORMATION, false, uint32(pid))
	if err != nil {
		if err == windows.ERROR_INVALID_PARAMETER {
			return false, nil
		}
		return false, err
	}
	defer windows.CloseHandle(process)
	var exitCode uint32
	if err := windows.GetExitCodeProcess(process, &exitCode); err != nil {
		return false, err
	}
	return exitCode == windowsStillActive, nil
}

func windowsWinDivertRunning() (bool, error) {
	output, err := exec.Command("sc.exe", "query", "WinDivert").CombinedOutput()
	text := strings.ToUpper(string(output))
	if strings.Contains(text, "FAILED 1060") || strings.Contains(text, "DOES NOT EXIST") {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("query WinDivert service: %w: %s", err, strings.TrimSpace(string(output)))
	}
	return strings.Contains(text, "RUNNING"), nil
}

func windowsStopWinDivert(timeout time.Duration) error {
	output, err := exec.Command("sc.exe", "stop", "WinDivert").CombinedOutput()
	if err != nil {
		running, stateErr := windowsWinDivertRunning()
		if stateErr != nil {
			return stateErr
		}
		if running {
			return fmt.Errorf("stop WinDivert service: %w: %s", err, strings.TrimSpace(string(output)))
		}
		return nil
	}
	deadline := time.Now().Add(timeout)
	for {
		running, stateErr := windowsWinDivertRunning()
		if stateErr != nil {
			return stateErr
		}
		if !running {
			return nil
		}
		if time.Now().After(deadline) {
			return fmt.Errorf("WinDivert service remains active after stop request")
		}
		time.Sleep(100 * time.Millisecond)
	}
}
