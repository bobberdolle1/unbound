//go:build linux

package autotunevnext

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"time"

	"unbound/engine"
	"unbound/engine/backendcap"
	"unbound/engine/providers"
)

type LinuxRuntime struct {
	opts      RuntimeOptions
	mu        sync.Mutex
	snapshots map[string]providerSnapshot
	nextID    atomic.Uint64
	active    *linuxCandidate
	runner    linuxRunner
}

type linuxCandidate struct {
	cmd    *exec.Cmd
	done   chan struct{}
	cancel context.CancelFunc
	spec   LinuxNFQueueSpec
	mode   string
}

type linuxRunner interface {
	run(context.Context, string, ...string) (string, error)
	lookPath(string) (string, error)
}

type execLinuxRunner struct{}

func (execLinuxRunner) run(ctx context.Context, name string, args ...string) (string, error) {
	out, err := exec.CommandContext(ctx, name, args...).CombinedOutput()
	return string(out), err
}
func (execLinuxRunner) lookPath(name string) (string, error) { return exec.LookPath(name) }

func NewLinuxRuntime(options RuntimeOptions) (*LinuxRuntime, error) {
	if options.Provider == nil || options.Assets == nil {
		return nil, fmt.Errorf("Linux vNext runtime needs product provider and assets")
	}
	return &LinuxRuntime{opts: options, snapshots: make(map[string]providerSnapshot), runner: execLinuxRunner{}}, nil
}

func (e *LinuxRuntime) Resolve(ctx context.Context, backend backendcap.Backend, ids []string) ([]ResolvedAsset, error) {
	resolver, err := NewProductAssetResolver(e.opts.Assets)
	if err != nil {
		return nil, err
	}
	return resolver.Resolve(ctx, backend, ids)
}

func (e *LinuxRuntime) Check(ctx context.Context, request HostPreflightRequest) HostPreflightResult {
	if request.Backend != backendcap.Zapret2Linux {
		return unsupported("BACKEND_OS_MISMATCH")
	}
	if request.Capture.BackendKind != backendcap.CaptureNFQUEUE {
		return unsupported("UNSUPPORTED_CAPTURE")
	}
	if os.Geteuid() != 0 {
		return unsupported("PRIVILEGES_REQUIRED")
	}
	if err := engine.VerifyExtractedAssets(e.opts.Assets); err != nil {
		return errored("ASSET_IDENTITY_INVALID", err)
	}
	if _, err := verifyRuntimeBinary(filepath.Join(e.opts.Assets.BinDir, "nfqws2"), e.opts.Assets.EngineSHA256); err != nil {
		return errored("ENGINE_IDENTITY_INVALID", err)
	}
	if request.Requirements.TCPTimestamps == engine.TimestampsRequired && !runtimeTimestampsActive() {
		return unsupported("TCP_TIMESTAMPS_REQUIRED")
	}
	if _, err := e.firewallMode(); err != nil {
		return unsupported("NFQUEUE_TOOLING_UNAVAILABLE")
	}
	if listing, err := e.firewallListing(ctx); err != nil || strings.Contains(listing, linuxOwnershipPrefix) {
		return unsupported("NFQUEUE_OWNERSHIP_COLLISION")
	}
	e.mu.Lock()
	active := e.active != nil
	e.mu.Unlock()
	if active {
		return unsupported("OWNED_AUTOTUNE_PROCESS_ACTIVE")
	}
	return HostPreflightResult{Status: PreflightSupported}
}

func (e *LinuxRuntime) Snapshot(_ context.Context) (StateSnapshot, error) {
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.active != nil {
		return StateSnapshot{}, fmt.Errorf("owned AutoTune candidate remains active")
	}
	id := fmt.Sprintf("linux-%d", e.nextID.Add(1))
	e.snapshots[id] = providerSnapshot{status: e.opts.Provider.GetStatus(), profile: e.opts.Provider.CurrentProfile()}
	e.opts.emit(PhysicalLog{ExperimentID: id, Backend: string(backendcap.Zapret2Linux), Phase: "SNAPSHOT"})
	return StateSnapshot{ID: id}, nil
}

func (e *LinuxRuntime) EstablishDirect(_ context.Context, snapshot StateSnapshot) error {
	if _, err := e.snapshot(snapshot); err != nil {
		return err
	}
	if err := e.opts.Provider.Stop(); err != nil {
		return fmt.Errorf("stop original provider for direct observation: %w", err)
	}
	if e.opts.Provider.GetStatus() != providers.StatusStopped || e.opts.Provider.CurrentProfile() != "" {
		return fmt.Errorf("provider is not factually stopped for direct observation")
	}
	return nil
}

func (e *LinuxRuntime) Activate(ctx context.Context, candidate ExecutableCandidate) error {
	plan, err := NewLinuxExactPlan(candidate)
	if err != nil {
		return err
	}
	mode, err := e.firewallMode()
	if err != nil {
		return err
	}
	queue, table, marker, err := e.allocateOwnership(ctx, mode)
	if err != nil {
		return err
	}
	spec, err := NewLinuxNFQueueSpec(plan.Capture, candidate.TargetEdge, candidate.TargetFamily, table, queue, marker)
	if err != nil {
		return err
	}
	if err := e.applyRule(ctx, mode, spec); err != nil {
		return err
	}
	cleanupRule := true
	defer func() {
		if cleanupRule {
			_ = e.deleteRule(context.WithoutCancel(ctx), mode, spec)
		}
	}()
	args := append([]string{"--qnum=" + fmt.Sprint(queue)}, trustedLuaInitArgs(e.opts.Assets.LuaDir)...)
	args = append(args, plan.EngineArgv...)
	candidateCtx, cancel := context.WithTimeout(ctx, physicalCandidateTimeout)
	cmd := exec.CommandContext(candidateCtx, filepath.Join(e.opts.Assets.BinDir, "nfqws2"), args...)
	cmd.Dir = e.opts.Assets.BinDir
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	if err := cmd.Start(); err != nil {
		cancel()
		return fmt.Errorf("start verified nfqws2 after exact rule preparation: %w", err)
	}
	active := &linuxCandidate{cmd: cmd, done: make(chan struct{}), cancel: cancel, spec: spec, mode: mode}
	e.mu.Lock()
	if e.active != nil {
		e.mu.Unlock()
		_ = syscall.Kill(-cmd.Process.Pid, syscall.SIGTERM)
		cancel()
		return fmt.Errorf("owned AutoTune candidate is already active")
	}
	e.active = active
	e.mu.Unlock()
	go func() { _ = cmd.Wait(); close(active.done) }()
	// A live process plus an exact, kernel-listed owned rule is the factual
	// pre-observation capture proof. The active Observatory run drives packets.
	select {
	case <-active.done:
		return fmt.Errorf("nfqws2 exited before capture became usable")
	case <-time.After(150 * time.Millisecond):
	}
	if err := e.verifyRule(ctx, mode, spec); err != nil {
		_ = e.Deactivate(context.WithoutCancel(ctx))
		return err
	}
	cleanupRule = false
	e.opts.emit(PhysicalLog{Backend: string(backendcap.Zapret2Linux), StrategyID: candidate.Strategy.ID, Fingerprint: shortFingerprint(candidate.Fingerprint), Edge: candidate.TargetEdge.String(), Phase: "CAPTURE_READY", PID: cmd.Process.Pid})
	return nil
}

func (e *LinuxRuntime) VerifyActive(_ context.Context, _ ExecutableCandidate) error {
	e.mu.Lock()
	active := e.active
	e.mu.Unlock()
	if active == nil {
		return fmt.Errorf("no owned candidate to verify")
	}
	select {
	case <-active.done:
		return fmt.Errorf("owned nfqws2 exited while active")
	default:
		return nil
	}
}

func (e *LinuxRuntime) Deactivate(ctx context.Context) error {
	e.mu.Lock()
	active := e.active
	if active != nil {
		e.active = nil
	}
	e.mu.Unlock()
	if active == nil {
		return nil
	}
	active.cancel()
	if err := syscall.Kill(-active.cmd.Process.Pid, syscall.SIGTERM); err != nil && err != syscall.ESRCH {
		return fmt.Errorf("terminate owned nfqws2: %w", err)
	}
	select {
	case <-active.done:
	case <-time.After(10 * time.Second):
		if err := syscall.Kill(-active.cmd.Process.Pid, syscall.SIGKILL); err != nil && err != syscall.ESRCH {
			return fmt.Errorf("owned nfqws2 stop timeout: %w", err)
		}
		select {
		case <-active.done:
		case <-time.After(2 * time.Second):
			return fmt.Errorf("owned nfqws2 did not exit")
		}
	}
	if err := e.deleteRule(context.WithoutCancel(ctx), active.mode, active.spec); err != nil {
		return fmt.Errorf("delete exact owned NFQUEUE rule: %w", err)
	}
	e.opts.emit(PhysicalLog{Backend: string(backendcap.Zapret2Linux), Phase: "EXITED", PID: active.cmd.Process.Pid})
	return nil
}

func (e *LinuxRuntime) Restore(ctx context.Context, snapshot StateSnapshot) error {
	state, err := e.snapshot(snapshot)
	if err != nil {
		return err
	}
	if err := e.Deactivate(context.WithoutCancel(ctx)); err != nil {
		return err
	}
	if state.status != providers.StatusRunning {
		if err := e.opts.Provider.Stop(); err != nil {
			return err
		}
		return nil
	}
	if state.profile == "" {
		return fmt.Errorf("running provider snapshot lacks profile identity")
	}
	return e.opts.Provider.Start(context.WithoutCancel(ctx), state.profile)
}

func (e *LinuxRuntime) VerifyRestored(_ context.Context, snapshot StateSnapshot) error {
	state, err := e.snapshot(snapshot)
	if err != nil {
		return err
	}
	e.mu.Lock()
	active := e.active != nil
	e.mu.Unlock()
	if active || e.opts.Provider.GetStatus() != state.status || e.opts.Provider.CurrentProfile() != state.profile {
		return fmt.Errorf("provider or owned NFQUEUE candidate does not match original snapshot")
	}
	return nil
}

func (e *LinuxRuntime) snapshot(snapshot StateSnapshot) (providerSnapshot, error) {
	e.mu.Lock()
	defer e.mu.Unlock()
	state, ok := e.snapshots[snapshot.ID]
	if !ok {
		return providerSnapshot{}, fmt.Errorf("unknown state snapshot")
	}
	return state, nil
}
func (e *LinuxRuntime) firewallMode() (string, error) {
	if _, err := e.runner.lookPath("nft"); err == nil {
		return "nft", nil
	}
	if _, err := e.runner.lookPath("iptables"); err == nil {
		return "iptables", nil
	}
	return "", fmt.Errorf("neither nft nor iptables is available")
}
func (e *LinuxRuntime) firewallListing(ctx context.Context) (string, error) {
	mode, err := e.firewallMode()
	if err != nil {
		return "", err
	}
	if mode == "nft" {
		return e.runner.run(ctx, "nft", "list", "ruleset")
	}
	return e.runner.run(ctx, "iptables", "-t", "mangle", "-S")
}

func (e *LinuxRuntime) allocateOwnership(ctx context.Context, mode string) (uint16, string, string, error) {
	listing, err := e.firewallListing(ctx)
	if err != nil {
		return 0, "", "", err
	}
	if strings.Contains(listing, linuxOwnershipPrefix) {
		return 0, "", "", fmt.Errorf("existing AutoTune NFQUEUE ownership collision")
	}
	for range 16 {
		var random [3]byte
		if _, err := rand.Read(random[:]); err != nil {
			return 0, "", "", err
		}
		queue := uint16(40000 + (uint16(random[0])<<8|uint16(random[1]))%20000)
		token := hex.EncodeToString(random[:])
		if !strings.Contains(listing, fmt.Sprint(queue)) {
			return queue, "unbound_autotune_" + token, linuxOwnershipPrefix + ":" + token, nil
		}
	}
	return 0, "", "", fmt.Errorf("could not establish an unused owned NFQUEUE number")
}

func (e *LinuxRuntime) applyRule(ctx context.Context, mode string, spec LinuxNFQueueSpec) error {
	if mode == "nft" {
		script, err := os.CreateTemp("", "unbound-autotune-nft-*.nft")
		if err != nil {
			return err
		}
		path := script.Name()
		defer os.Remove(path)
		if err := script.Chmod(0600); err != nil {
			script.Close()
			return err
		}
		if _, err := script.WriteString(spec.nftScript()); err != nil {
			script.Close()
			return err
		}
		if err := script.Close(); err != nil {
			return err
		}
		out, err := e.runner.run(ctx, "nft", "-f", path)
		if err != nil {
			return fmt.Errorf("nft exact rule: %s: %w", strings.TrimSpace(out), err)
		}
		return nil
	}
	parts := spec.iptablesArgs("-I")
	if len(parts) == 0 {
		return fmt.Errorf("empty iptables rule")
	}
	out, err := e.runner.run(ctx, parts[0], parts[1:]...)
	if err != nil {
		return fmt.Errorf("iptables exact rule: %s: %w", strings.TrimSpace(out), err)
	}
	return nil
}

func (e *LinuxRuntime) verifyRule(ctx context.Context, mode string, spec LinuxNFQueueSpec) error {
	if mode == "nft" {
		out, err := e.runner.run(ctx, "nft", "list", "table", spec.nftFamily(), spec.Table)
		if err != nil || !strings.Contains(out, spec.Marker) || !strings.Contains(out, "queue num "+fmt.Sprint(spec.Queue)) {
			return fmt.Errorf("owned nft rule is not factually present")
		}
		return nil
	}
	parts := spec.iptablesArgs("-C")
	out, err := e.runner.run(ctx, parts[0], parts[1:]...)
	if err != nil {
		return fmt.Errorf("owned iptables rule is not factually present: %s", strings.TrimSpace(out))
	}
	return nil
}

func (e *LinuxRuntime) deleteRule(ctx context.Context, mode string, spec LinuxNFQueueSpec) error {
	if mode == "nft" {
		out, err := e.runner.run(ctx, "nft", "delete", "table", spec.nftFamily(), spec.Table)
		if err != nil {
			return fmt.Errorf("nft delete owned table: %s", strings.TrimSpace(out))
		}
		if _, err := e.runner.run(ctx, "nft", "list", "table", spec.nftFamily(), spec.Table); err == nil {
			return fmt.Errorf("owned nft table remains after delete")
		}
		return nil
	}
	parts := spec.iptablesArgs("-D")
	out, err := e.runner.run(ctx, parts[0], parts[1:]...)
	if err != nil {
		return fmt.Errorf("iptables delete owned rule: %s", strings.TrimSpace(out))
	}
	return nil
}
