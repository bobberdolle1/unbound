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
	"regexp"
	"strconv"
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
	opts           RuntimeOptions
	mu             sync.Mutex
	snapshots      map[string]providerSnapshot
	nextID         atomic.Uint64
	active         *linuxCandidate
	completed      []linuxOwnedState
	runner         linuxRunner
	processAlive   func(int) (bool, error)
	killGroup      func(int, syscall.Signal) error
	readQueueState func() ([]byte, error)
	randomBytes    func([]byte) (int, error)
	cleanupWait    time.Duration
	forceWait      time.Duration
}

type linuxCandidate struct {
	cmd      *exec.Cmd
	done     chan struct{}
	cancel   context.CancelFunc
	spec     LinuxNFQueueSpec
	mode     string
	pid      int
	cleaning bool
}

type linuxOwnedState struct {
	pid  int
	spec LinuxNFQueueSpec
	mode string
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
	return &LinuxRuntime{
		opts:           options,
		snapshots:      make(map[string]providerSnapshot),
		runner:         execLinuxRunner{},
		processAlive:   linuxProcessAlive,
		killGroup:      linuxKillGroup,
		readQueueState: readNFNetlinkQueues,
		randomBytes:    rand.Read,
		cleanupWait:    10 * time.Second,
		forceWait:      2 * time.Second,
	}, nil
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
	candidateCtx, cancel := candidateExecutionContext(ctx)
	cmd := exec.CommandContext(candidateCtx, filepath.Join(e.opts.Assets.BinDir, "nfqws2"), args...)
	cmd.Dir = e.opts.Assets.BinDir
	cmd.SysProcAttr = linuxCandidateSysProcAttr()
	if err := cmd.Start(); err != nil {
		cancel()
		return fmt.Errorf("start verified nfqws2 after exact rule preparation: %w", err)
	}
	active := &linuxCandidate{cmd: cmd, done: make(chan struct{}), cancel: cancel, spec: spec, mode: mode, pid: cmd.Process.Pid}
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
	if active == nil {
		e.mu.Unlock()
		return nil
	}
	active.cleaning = true
	e.mu.Unlock()

	active.cancel()
	if err := e.killGroup(active.pid, syscall.SIGTERM); err != nil && err != syscall.ESRCH {
		return fmt.Errorf("terminate owned nfqws2: %w", err)
	}
	select {
	case <-active.done:
	case <-time.After(e.cleanupTimeout()):
		if err := e.killGroup(active.pid, syscall.SIGKILL); err != nil && err != syscall.ESRCH {
			return fmt.Errorf("force terminate owned nfqws2: %w", err)
		}
		select {
		case <-active.done:
		case <-time.After(e.forceCleanupTimeout()):
			return fmt.Errorf("owned nfqws2 PID %d did not exit", active.pid)
		}
	}
	alive, err := e.processAlive(active.pid)
	if err != nil {
		return fmt.Errorf("audit owned nfqws2 PID %d after teardown: %w", active.pid, err)
	}
	if alive {
		return fmt.Errorf("owned nfqws2 PID %d remains alive after teardown", active.pid)
	}
	if err := e.deleteRule(context.WithoutCancel(ctx), active.mode, active.spec); err != nil {
		return fmt.Errorf("delete exact owned NFQUEUE rule: %w", err)
	}
	e.mu.Lock()
	if e.active == active {
		e.completed = append(e.completed, linuxOwnedState{pid: active.pid, spec: active.spec, mode: active.mode})
		e.active = nil
	}
	e.mu.Unlock()
	e.opts.emit(PhysicalLog{Backend: string(backendcap.Zapret2Linux), Phase: "EXITED", PID: active.pid})
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

func (e *LinuxRuntime) VerifyRestored(ctx context.Context, snapshot StateSnapshot) error {
	state, err := e.snapshot(snapshot)
	if err != nil {
		return err
	}
	e.mu.Lock()
	active := e.active
	completed := append([]linuxOwnedState(nil), e.completed...)
	e.mu.Unlock()
	if active != nil {
		alive, auditErr := e.processAlive(active.pid)
		if auditErr != nil {
			return fmt.Errorf("audit retained owned nfqws2 PID %d: %w", active.pid, auditErr)
		}
		if alive {
			return fmt.Errorf("owned nfqws2 PID %d remains active", active.pid)
		}
		if err := e.ownedRuleAbsent(ctx, active.mode, active.spec); err != nil {
			return err
		}
		return fmt.Errorf("owned nfqws2 cleanup remains incomplete")
	}
	for _, owned := range completed {
		alive, auditErr := e.processAlive(owned.pid)
		if auditErr != nil {
			return fmt.Errorf("audit retired owned nfqws2 PID %d: %w", owned.pid, auditErr)
		}
		if alive {
			return fmt.Errorf("retired owned nfqws2 PID %d remains active", owned.pid)
		}
		if err := e.ownedRuleAbsent(ctx, owned.mode, owned.spec); err != nil {
			return err
		}
	}
	if e.opts.Provider.GetStatus() != state.status || e.opts.Provider.CurrentProfile() != state.profile {
		return fmt.Errorf("provider does not match original snapshot")
	}
	e.mu.Lock()
	e.completed = nil
	e.mu.Unlock()
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
	ipv4, err := e.runner.run(ctx, "iptables", "-t", "mangle", "-S")
	if err != nil {
		return "", err
	}
	if _, err := e.runner.lookPath("ip6tables"); err != nil {
		return ipv4, nil
	}
	ipv6, err := e.runner.run(ctx, "ip6tables", "-t", "mangle", "-S")
	if err != nil {
		return "", fmt.Errorf("list ip6tables mangle rules: %w", err)
	}
	return ipv4 + "\n" + ipv6, nil
}

var queueNumberPattern = regexp.MustCompile(`(?:queue\s+num|--queue-num)\s+([0-9]+)`)

func queuesInRuleListing(listing string) map[uint16]struct{} {
	queues := make(map[uint16]struct{})
	for _, match := range queueNumberPattern.FindAllStringSubmatch(listing, -1) {
		value, err := strconv.ParseUint(match[1], 10, 16)
		if err == nil {
			queues[uint16(value)] = struct{}{}
		}
	}
	return queues
}

func (e *LinuxRuntime) allocateOwnership(ctx context.Context, mode string) (uint16, string, string, error) {
	listing, err := e.firewallListing(ctx)
	if err != nil {
		return 0, "", "", err
	}
	if strings.Contains(listing, linuxOwnershipPrefix) {
		return 0, "", "", fmt.Errorf("existing AutoTune NFQUEUE ownership collision")
	}
	queues := queuesInRuleListing(listing)
	bound, err := e.readQueueState()
	if err != nil {
		return 0, "", "", fmt.Errorf("read nfnetlink queue ownership: %w", err)
	}
	for range 16 {
		var random [3]byte
		if _, err := e.randomBytes(random[:]); err != nil {
			return 0, "", "", err
		}
		queue := uint16(40000 + (uint16(random[0])<<8|uint16(random[1]))%20000)
		if _, used := queues[queue]; used || nfNetlinkQueueBound(bound, queue) {
			continue
		}
		token := hex.EncodeToString(random[:])
		return queue, "unbound_autotune_" + token, linuxOwnershipPrefix + ":" + token, nil
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
			return fmt.Errorf("nft delete owned table: %s: %w", strings.TrimSpace(out), err)
		}
		listing, err := e.runner.run(ctx, "nft", "list", "ruleset")
		if err != nil {
			return fmt.Errorf("audit nft ruleset after owned delete: %w", err)
		}
		if strings.Contains(listing, spec.Table) || strings.Contains(listing, spec.Marker) || strings.Contains(listing, "queue num "+fmt.Sprint(spec.Queue)) {
			return fmt.Errorf("owned nft state remains after delete")
		}
		return nil
	}
	parts := spec.iptablesArgs("-D")
	out, err := e.runner.run(ctx, parts[0], parts[1:]...)
	if err != nil {
		return fmt.Errorf("iptables delete owned rule: %s", strings.TrimSpace(out))
	}
	parts = spec.iptablesArgs("-C")
	if out, err := e.runner.run(ctx, parts[0], parts[1:]...); err == nil {
		return fmt.Errorf("owned iptables rule remains after delete: %s", strings.TrimSpace(out))
	}
	return nil
}

func (e *LinuxRuntime) ownedRuleAbsent(ctx context.Context, mode string, spec LinuxNFQueueSpec) error {
	if mode == "nft" {
		listing, err := e.runner.run(ctx, "nft", "list", "ruleset")
		if err != nil {
			return fmt.Errorf("audit nft ruleset for owned absence: %w", err)
		}
		if strings.Contains(listing, spec.Table) || strings.Contains(listing, spec.Marker) || strings.Contains(listing, "queue num "+fmt.Sprint(spec.Queue)) {
			return fmt.Errorf("owned nft state remains active")
		}
		return nil
	}
	parts := spec.iptablesArgs("-C")
	if out, err := e.runner.run(ctx, parts[0], parts[1:]...); err == nil {
		return fmt.Errorf("owned iptables rule remains active: %s", strings.TrimSpace(out))
	}
	return nil
}

func (e *LinuxRuntime) cleanupTimeout() time.Duration {
	if e.cleanupWait > 0 {
		return e.cleanupWait
	}
	return 10 * time.Second
}

func (e *LinuxRuntime) forceCleanupTimeout() time.Duration {
	if e.forceWait > 0 {
		return e.forceWait
	}
	return 2 * time.Second
}

func linuxKillGroup(pid int, signal syscall.Signal) error {
	return syscall.Kill(-pid, signal)
}

func linuxProcessAlive(pid int) (bool, error) {
	err := syscall.Kill(pid, 0)
	if err == nil || err == syscall.EPERM {
		return true, nil
	}
	if err == syscall.ESRCH {
		return false, nil
	}
	return false, err
}

func linuxCandidateSysProcAttr() *syscall.SysProcAttr {
	return &syscall.SysProcAttr{Setpgid: true, Pdeathsig: syscall.SIGTERM}
}

func readNFNetlinkQueues() ([]byte, error) {
	data, err := os.ReadFile("/proc/net/netfilter/nfnetlink_queue")
	if os.IsNotExist(err) {
		return nil, nil
	}
	return data, err
}

func nfNetlinkQueueBound(state []byte, queue uint16) bool {
	for _, field := range strings.Fields(string(state)) {
		field = strings.Trim(field, "[](),:")
		value, err := strconv.ParseUint(field, 0, 16)
		if err == nil && uint16(value) == queue {
			return true
		}
	}
	return false
}
