//go:build linux

package autotunevnext

// Linux bounded service-graph executor.
//
// The graph is expressed as ONE executor-owned ephemeral nft table holding the
// exact union of every active node's edges, plus one owned nfqws2 process. It
// deliberately reuses the existing LinuxNFQueueSpec, applyRule/verifyRule/
// deleteRule lifecycle rather than introducing a second firewall system.
//
// Capture authority is an exact union only: no wildcard, no broad subnet, no
// ipset imported from an external list. Every node edge is bounded by the graph
// bound (MaxServiceGraphHosts nodes, MaxServiceScopeEdges per node).
//
// Mixed-family graphs build one complete rule set per required family. Each
// family is verified independently, so a fragmented or partial rule set can
// never be mistaken for a correct one.

import (
	"context"
	"fmt"
	"net"
	"path/filepath"
	"syscall"
	"time"

	"unbound/engine"
	"unbound/engine/backendcap"
	"unbound/engine/observatory"
)

// LinuxGraphExecutor owns all machine mutation for one bounded Linux graph.
type LinuxGraphExecutor struct {
	runtime *LinuxRuntime

	mode  string
	spec  LinuxNFQueueSpec
	proc  *linuxCandidate
	table string
}

// BuildLinuxGraphSpec folds every active node's edges into one exact spec.
// Family separation is preserved: IPv4 and IPv6 never share a rule set.
func BuildLinuxGraphSpec(graph ServiceGraph, capture backendcap.CapturePlan, table string, queue uint16, marker string) (LinuxNFQueueSpec, error) {
	active, err := graph.ActiveCaptureGraph()
	if err != nil {
		return LinuxNFQueueSpec{}, err
	}
	edges, err := active.UnionEdges()
	if err != nil {
		return LinuxNFQueueSpec{}, err
	}
	if len(edges) == 0 {
		return LinuxNFQueueSpec{}, fmt.Errorf("%w: graph has no active edges", ErrServiceGraphEmpty)
	}
	v4, v6, err := partitionGraphEdges(edges)
	if err != nil {
		return LinuxNFQueueSpec{}, err
	}
	// Anchor on the first edge only to obtain the validated shape (ports, family
	// checks); the address sets themselves come from the whole graph.
	anchor := edges[0]
	spec, err := NewLinuxNFQueueSpec(capture, anchor.IP, familyForIP(anchor.IP), table, queue, marker)
	if err != nil {
		return LinuxNFQueueSpec{}, err
	}
	// Every family's address set must be independently representable, and every
	// edge must be a literal address. A missing family is an error rather than a
	// silently narrower capture.
	if len(v4) > 0 && !captureIncludesFamily(capture.IPFamilies, observatory.AddressFamilyIPv4) {
		return LinuxNFQueueSpec{}, fmt.Errorf("graph contains IPv4 edges the compiled capture cannot express")
	}
	if len(v6) > 0 && !captureIncludesFamily(capture.IPFamilies, observatory.AddressFamilyIPv6) {
		return LinuxNFQueueSpec{}, fmt.Errorf("graph contains IPv6 edges the compiled capture cannot express")
	}
	spec.IPv4Edges = v4
	spec.IPv6Edges = v6
	spec.Edges = allGraphEdges(v4, v6)
	spec.Edge = v4[0]
	if len(v4) == 0 {
		spec.Edge = v6[0]
		spec.Family = observatory.AddressFamilyIPv6
		spec.NFTFamily = "ip6"
	}
	return spec, nil
}

func partitionGraphEdges(edges []ServiceScopeEdge) ([]net.IP, []net.IP, error) {
	var v4, v6 []net.IP
	for _, edge := range edges {
		if edge.IP == nil || net.ParseIP(edge.IP.String()) == nil {
			return nil, nil, fmt.Errorf("graph edge is not a literal address")
		}
		if familyForIP(edge.IP) == observatory.AddressFamilyIPv6 {
			v6 = append(v6, edge.IP)
			continue
		}
		v4 = append(v4, edge.IP)
	}
	return v4, v6, nil
}

func allGraphEdges(v4, v6 []net.IP) []net.IP {
	out := make([]net.IP, 0, len(v4)+len(v6))
	out = append(out, v4...)
	out = append(out, v6...)
	return out
}

func NewLinuxGraphExecutor(options RuntimeOptions) (*LinuxGraphExecutor, error) {
	runtime, err := NewLinuxRuntime(options)
	if err != nil {
		return nil, err
	}
	return &LinuxGraphExecutor{runtime: runtime}, nil
}

func (e *LinuxGraphExecutor) Snapshot(ctx context.Context) (StateSnapshot, error) {
	return e.runtime.Snapshot(ctx)
}

func (e *LinuxGraphExecutor) EstablishDirect(ctx context.Context, snapshot StateSnapshot) error {
	return e.runtime.EstablishDirect(ctx, snapshot)
}

// ActivateGraph installs exactly one owned table and starts exactly one owned
// process. The whole graph costs one activation, never one per node or per edge.
func (e *LinuxGraphExecutor) ActivateGraph(ctx context.Context, activation ServiceGraphActivation) error {
	if err := engine.VerifyExtractedAssets(e.runtime.opts.Assets); err != nil {
		return fmt.Errorf("verify runtime assets before graph activation: %w", err)
	}
	mode, err := e.runtime.firewallMode()
	if err != nil {
		return err
	}
	queue, table, marker, err := e.runtime.allocateOwnership(ctx, mode)
	if err != nil {
		return err
	}
	spec, err := BuildLinuxGraphSpec(activation.Graph, activation.Capture, table, queue, marker)
	if err != nil {
		return err
	}
	if mode != "nft" && len(spec.Edges) > 1 {
		return fmt.Errorf("Linux graph capture requires nft for multi-edge exact capture")
	}
	active := &linuxCandidate{done: make(chan struct{}), spec: spec, mode: mode}
	e.runtime.mu.Lock()
	if e.runtime.active != nil {
		e.runtime.mu.Unlock()
		return fmt.Errorf("owned AutoTune candidate is already active")
	}
	e.runtime.active = active
	e.runtime.mu.Unlock()

	e.mode, e.spec, e.table, e.proc = mode, spec, table, active
	if err := e.runtime.applyRule(ctx, mode, spec); err != nil {
		e.runtime.mu.Lock()
		if e.runtime.active == active {
			e.runtime.active = nil
		}
		e.runtime.mu.Unlock()
		e.proc, e.table = nil, ""
		return err
	}
	active.ruleInstalled = true

	exact, err := NewLinuxExactPlan(ExecutableCandidate{
		Backend:     activation.Backend,
		Plan:        activation.Plan,
		TargetEdges: specAsScopeEdges(spec),
	})
	if err != nil {
		return e.runtime.activationFailure(ctx, active, fmt.Errorf("exact graph plan: %w", err))
	}
	args := append([]string{"--qnum=" + fmt.Sprint(queue)}, trustedLuaInitArgs(e.runtime.opts.Assets.LuaDir)...)
	args = append(args, exact.EngineArgv...)
	candidateCtx, cancel := candidateExecutionContext(ctx)
	active.cancel = cancel
	cmd, err := e.runtime.startProcess(candidateCtx, filepath.Join(e.runtime.opts.Assets.BinDir, "nfqws2"), args, e.runtime.opts.Assets.BinDir, linuxCandidateSysProcAttr())
	if err != nil {
		cancel()
		return e.runtime.activationFailure(ctx, active, fmt.Errorf("start verified nfqws2 for graph: %w", err))
	}
	active.cmd, active.pid, active.processStarted = cmd, cmd.Process.Pid, true
	go func() { _ = cmd.Wait(); close(active.done) }()
	select {
	case <-active.done:
		active.processStopped = true
		return e.runtime.activationFailure(ctx, active, fmt.Errorf("nfqws2 exited before graph capture became usable"))
	case <-ctx.Done():
		return e.runtime.activationFailure(ctx, active, ctx.Err())
	case <-time.After(150 * time.Millisecond):
	}
	if err := e.runtime.verifyRule(ctx, mode, spec); err != nil {
		return e.runtime.activationFailure(ctx, active, err)
	}
	e.runtime.opts.emit(PhysicalLog{Backend: string(backendcap.Zapret2Linux), Phase: "GRAPH_CAPTURE_READY", PID: active.pid})
	return nil
}

// VerifyActiveGraph proves the owned table still carries exactly the intended
// union: process alive and each family's exact set intact.
func (e *LinuxGraphExecutor) VerifyActiveGraph(ctx context.Context, activation ServiceGraphActivation) error {
	active := e.proc
	if active == nil || active.cmd == nil || active.cmd.Process == nil {
		return fmt.Errorf("no owned graph activation")
	}
	if err := active.cmd.Process.Signal(syscall.Signal(0)); err != nil {
		return fmt.Errorf("owned graph process is not alive: %w", err)
	}
	if err := e.runtime.verifyRule(ctx, e.mode, e.spec); err != nil {
		return err
	}
	want, err := BuildLinuxGraphSpec(activation.Graph, activation.Capture, e.spec.Table, e.spec.Queue, e.spec.Marker)
	if err != nil {
		return err
	}
	if !sameIPSet(want.IPv4Edges, e.spec.IPv4Edges) || !sameIPSet(want.IPv6Edges, e.spec.IPv6Edges) {
		return fmt.Errorf("active graph address set changed since activation")
	}
	return nil
}

// CommitGraph detaches the bounded experiment cancellation context so a committed
// runtime survives Apply, applying the PR60 lesson to graphs.
func (e *LinuxGraphExecutor) CommitGraph(activation ServiceGraphActivation) error {
	if e.proc == nil {
		return fmt.Errorf("no owned graph activation to commit")
	}
	e.proc.cancel = func() {}
	return nil
}

// Deactivate removes the owned table and stops the owned process. It never
// touches foreign firewall state.
func (e *LinuxGraphExecutor) Deactivate(ctx context.Context) error {
	active := e.proc
	if active == nil {
		return nil
	}
	cleanupCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), e.runtime.cleanupTimeout())
	defer cancel()
	e.proc, e.table = nil, ""
	e.runtime.mu.Lock()
	if e.runtime.active == active {
		e.runtime.active = nil
	}
	e.runtime.mu.Unlock()
	return e.runtime.Deactivate(cleanupCtx)
}

// specAsScopeEdges projects a compiled spec's exact address set back into the
// scope-edge shape the exact-plan compiler expects. No address is invented.
func specAsScopeEdges(spec LinuxNFQueueSpec) []ServiceScopeEdge {
	out := make([]ServiceScopeEdge, 0, len(spec.Edges))
	for _, ip := range spec.Edges {
		family := observatory.AddressFamilyIPv4
		if familyForIP(ip) == observatory.AddressFamilyIPv6 {
			family = observatory.AddressFamilyIPv6
		}
		out = append(out, ServiceScopeEdge{IP: ip, Family: family})
	}
	return out
}

func (e *LinuxGraphExecutor) Restore(ctx context.Context, snapshot StateSnapshot) error {
	if err := e.Deactivate(ctx); err != nil {
		return err
	}
	return e.runtime.Restore(ctx, snapshot)
}

func (e *LinuxGraphExecutor) VerifyRestored(ctx context.Context, snapshot StateSnapshot) error {
	return e.runtime.VerifyRestored(ctx, snapshot)
}

func sameIPSet(left, right []net.IP) bool {
	if len(left) != len(right) {
		return false
	}
	seen := make(map[string]bool, len(left))
	for _, ip := range left {
		seen[ip.String()] = true
	}
	for _, ip := range right {
		if !seen[ip.String()] {
			return false
		}
	}
	return true
}

// ensure the exec import stays meaningful if the runner is swapped.
var _ = time.Second
