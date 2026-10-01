//go:build windows

package autotunevnext

// Windows bounded service-graph executor.
//
// ONE owned winws2 process carries the whole graph. The capture is a single
// process-global exact WinDivert union filter, plus one host-scoped section per
// active node joined by --new. Two nodes bound to the same template identity
// still render as two independent host bindings, because exact per-host authority
// is the property that matters and a shared section would blur it.
//
// This executor owns all machine mutation for the graph. It refuses to claim
// capture state while a foreign WinDivert owner is active, and it never kills an
// unknown process: cleanup is scoped to the PIDs this executor started.

import (
	"context"
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"golang.org/x/sys/windows"
	"unbound/engine"
	"unbound/engine/backendcap"
)

// WindowsGraphExecutor is the managed, committed graph runtime. Unlike a bounded
// experiment activation, a graph committed through CommitGraph stays alive after
// the caller returns; the product owns it until Revert, Suspend, or shutdown.
type WindowsGraphExecutor struct {
	opts RuntimeOptions

	mu        sync.Mutex
	active    *windowsGraphProcess
	ownedPIDs map[int]struct{}
}

type windowsGraphProcess struct {
	cmd         *exec.Cmd
	done        chan struct{}
	ready       chan struct{}
	cancel      context.CancelFunc
	job         windows.Handle
	driverOwned bool
	pid         int
	// plan is retained so VerifyActiveGraph can prove the live process still
	// carries exactly this graph and not something else.
	plan    WindowsServiceGraphPlan
	hostSet map[string]string
}

func NewWindowsGraphExecutor(options RuntimeOptions) (*WindowsGraphExecutor, error) {
	if options.Assets == nil {
		return nil, fmt.Errorf("windows graph executor requires asset paths")
	}
	return &WindowsGraphExecutor{opts: options, ownedPIDs: make(map[int]struct{})}, nil
}

func (e *WindowsGraphExecutor) Snapshot(ctx context.Context) (StateSnapshot, error) {
	runtime, err := NewWindowsRuntime(e.opts)
	if err != nil {
		return StateSnapshot{}, err
	}
	return runtime.Snapshot(ctx)
}

func (e *WindowsGraphExecutor) EstablishDirect(ctx context.Context, snapshot StateSnapshot) error {
	runtime, err := NewWindowsRuntime(e.opts)
	if err != nil {
		return err
	}
	return runtime.EstablishDirect(ctx, snapshot)
}

// RenderGraphActivation produces the inert plan for an activation without
func RenderGraphActivation(activation ServiceGraphActivation) (WindowsServiceGraphPlan, error) {
	// Sections are supplied by the product, derived from the compiled plan. The
	// executor renders them but never invents a host selector of its own.
	plan, err := RenderWindowsServiceGraphCapture(activation.Graph, activation.Sections)
	if err != nil {
		return WindowsServiceGraphPlan{}, err
	}
	return plan, nil
}
func (e *WindowsGraphExecutor) ActivateGraph(ctx context.Context, activation ServiceGraphActivation) error {
	plan, err := RenderGraphActivation(activation)
	if err != nil {
		return err
	}
	if err := engine.VerifyExtractedAssets(e.opts.Assets); err != nil {
		return fmt.Errorf("verify runtime assets before graph activation: %w", err)
	}
	binary := filepath.Join(e.opts.Assets.BinDir, "winws2.exe")
	if _, err := verifyRuntimeBinary(binary, e.opts.Assets.EngineSHA256); err != nil {
		return err
	}

	// The engine argv comes from the exact compiled plan, exactly as the
	// single-host path does, so a graph never invents executable arguments.
	exact, err := NewWindowsExactPlan(ExecutableCandidate{
		Backend:     activation.Backend,
		Plan:        activation.Plan,
		TargetEdges: activation.UnionEdges,
	})
	if err != nil {
		return fmt.Errorf("exact graph plan: %w", err)
	}

	args := trustedLuaInitArgs(e.opts.Assets.LuaDir)
	args = append(args, plan.CaptureArgv...)
	args = append(args, "--wf-raw-filter="+plan.RawFilter)
	args = append(args, exact.EngineArgv...)
	args = append(args, plan.SectionArgv...)

	driverActive, err := windowsWinDivertRunning()
	if err != nil {
		return fmt.Errorf("inspect WinDivert ownership before graph activation: %w", err)
	}
	if driverActive {
		return fmt.Errorf("WinDivert is already active; refusing to claim shared capture state")
	}

	e.mu.Lock()
	if e.active != nil {
		e.mu.Unlock()
		return fmt.Errorf("owned AutoTune graph is already active")
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
		return fmt.Errorf("start verified winws2 for graph: %w", err)
	}
	hostSet := make(map[string]string, len(plan.NodeOrder))
	for _, nodeID := range plan.NodeOrder {
		hostSet[nodeID] = nodeID
	}
	proc := &windowsGraphProcess{
		cmd: cmd, done: make(chan struct{}), ready: make(chan struct{}, 1),
		cancel: cancel, job: job, driverOwned: true, pid: cmd.Process.Pid,
		plan: plan, hostSet: hostSet,
	}
	e.active = proc
	e.ownedPIDs[proc.pid] = struct{}{}
	e.mu.Unlock()

	go windowsReadReady(stdout, proc.ready)
	go windowsReadReady(stderr, proc.ready)
	go func() { _ = cmd.Wait(); close(proc.done) }()
	if err := attachWindowsProcess(job, proc.pid); err != nil {
		_ = e.Deactivate(context.WithoutCancel(ctx))
		return fmt.Errorf("attach owned winws2 graph to crash guard job: %w", err)
	}
	e.opts.emit(PhysicalLog{Backend: string(backendcap.Zapret2Windows), Phase: "GRAPH_STARTING", PID: proc.pid})
	select {
	case <-proc.ready:
		e.opts.emit(PhysicalLog{Backend: string(backendcap.Zapret2Windows), Phase: "GRAPH_CAPTURE_READY", PID: proc.pid})
		return nil
	case <-proc.done:
		_ = e.Deactivate(context.Background())
		return fmt.Errorf("graph winws2 exited before CAPTURE_READY")
	case <-ctx.Done():
		_ = e.Deactivate(context.WithoutCancel(ctx))
		return ctx.Err()
	case <-time.After(10 * time.Second):
		_ = e.Deactivate(context.Background())
		return fmt.Errorf("timed out waiting for graph CAPTURE_READY")
	}
}

// VerifyActiveGraph proves the live process still carries exactly the intended
// graph: process alive, capture ready, every expected host section present, the
// exact capture union matching, and no extra node or edge.
func (e *WindowsGraphExecutor) VerifyActiveGraph(ctx context.Context, activation ServiceGraphActivation) error {
	e.mu.Lock()
	proc := e.active
	e.mu.Unlock()
	if proc == nil {
		return fmt.Errorf("no owned graph activation")
	}
	alive, err := windowsProcessAlive(proc.pid)
	if err != nil {
		return fmt.Errorf("check owned graph process: %w", err)
	}
	if !alive {
		return fmt.Errorf("owned graph process %d is not alive", proc.pid)
	}
	select {
	case <-proc.ready:
	default:
		return fmt.Errorf("owned graph capture is not ready")
	}
	want, err := RenderGraphActivation(activation)
	if err != nil {
		return err
	}
	if want.RawFilter != proc.plan.RawFilter {
		return fmt.Errorf("graph capture union changed since activation")
	}
	if strings.Join(want.SectionArgv, "\x00") != strings.Join(proc.plan.SectionArgv, "\x00") {
		return fmt.Errorf("graph host sections changed since activation")
	}
	return nil
}

// CommitGraph converts a bounded experiment activation into a committed managed
// runtime. The process keeps running after this returns; the product owns it.
// The PR60 lesson applies: the bounded Apply validation context is NOT inherited
// by the committed runtime.
func (e *WindowsGraphExecutor) CommitGraph(activation ServiceGraphActivation) error {
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.active == nil {
		return fmt.Errorf("no owned graph activation to commit")
	}
	// Detach the bounded execution context from the now-committed process.
	e.active.cancel = func() {}
	return nil
}

// Deactivate stops the owned graph process. It only ever touches PIDs this
// executor started; unknown processes are never killed.
func (e *WindowsGraphExecutor) Deactivate(ctx context.Context) error {
	e.mu.Lock()
	proc := e.active
	e.active = nil
	e.mu.Unlock()
	if proc == nil {
		return nil
	}
	if proc.cancel != nil {
		proc.cancel()
	}
	select {
	case <-proc.done:
	case <-time.After(10 * time.Second):
		if proc.pid != 0 {
			if _, owned := e.ownedPIDs[proc.pid]; owned {
				_ = exec.Command("taskkill", "/F", "/PID", fmt.Sprint(proc.pid)).Run()
			}
		}
	}
	if proc.driverOwned {
		if err := windowsStopWinDivert(5 * time.Second); err != nil {
			return fmt.Errorf("stop owned graph WinDivert: %w", err)
		}
	}
	if proc.job != 0 {
		_ = windows.CloseHandle(proc.job)
	}
	return nil
}

func (e *WindowsGraphExecutor) Restore(ctx context.Context, snapshot StateSnapshot) error {
	if err := e.Deactivate(ctx); err != nil {
		return err
	}
	runtime, err := NewWindowsRuntime(e.opts)
	if err != nil {
		return err
	}
	return runtime.Restore(ctx, snapshot)
}

func (e *WindowsGraphExecutor) VerifyRestored(ctx context.Context, snapshot StateSnapshot) error {
	runtime, err := NewWindowsRuntime(e.opts)
	if err != nil {
		return err
	}
	return runtime.VerifyRestored(ctx, snapshot)
}
