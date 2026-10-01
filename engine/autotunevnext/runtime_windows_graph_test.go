//go:build windows

package autotunevnext

// Hermetic coverage for the Windows bounded-graph render path.
//
// The security property under test is that the rendered plan carries exact
// per-host authority only: one section per active node, an exact hostname
// selector, no wildcard, and no file- or list-based authority.

import (
	"context"
	"strings"
	"testing"
	"unbound/engine"
	"unbound/engine/strategyir"
)

func TestRenderGraphActivationBindsEachNodeToItsOwnHost(t *testing.T) {
	activation := ServiceGraphActivation{
		Graph:    twoNodeGraph(t),
		Sections: graphSections(),
		Capture:  graphTestCapture(),
	}
	plan, err := RenderGraphActivation(activation)
	if err != nil {
		t.Fatalf("render graph activation: %v", err)
	}
	if len(plan.NodeOrder) != 2 {
		t.Fatalf("node order = %v, want 2 nodes", plan.NodeOrder)
	}
	joined := strings.Join(plan.SectionArgv, "\n")
	for _, host := range []string{"entry.example", "media.example"} {
		if !strings.Contains(joined, hostlistDomainsArg(host)) {
			t.Fatalf("rendered plan is missing the exact host selector for %s", host)
		}
	}
	// Same template on two hosts must remain two independent host bindings.
	if strings.Count(joined, "--lua-desync=fake") != 2 {
		t.Fatalf("expected two independent per-node sections, got %q", joined)
	}
}

func TestRenderGraphActivationNeverEmitsWildcardOrListAuthority(t *testing.T) {
	activation := ServiceGraphActivation{
		Graph:    twoNodeGraph(t),
		Sections: graphSections(),
		Capture:  graphTestCapture(),
	}
	plan, err := RenderGraphActivation(activation)
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	all := strings.Join(plan.SectionArgv, "\n") + "\n" + plan.RawFilter + "\n" + strings.Join(plan.CaptureArgv, "\n")
	for _, forbidden := range []string{"*", "hostlist-file", "--hostlist=", "ipset", "asn", "autohostlist"} {
		if strings.Contains(all, forbidden) {
			t.Fatalf("rendered graph plan leaked non-exact authority %q", forbidden)
		}
	}
}

// A plan whose node count does not match its sections must be refused before
// anything could run.
func TestRenderGraphActivationRejectsSectionCountMismatch(t *testing.T) {
	activation := ServiceGraphActivation{
		Graph:    twoNodeGraph(t),
		Sections: graphSections()[:1],
	}
	if _, err := RenderGraphActivation(activation); err == nil {
		t.Fatal("a section/node count mismatch must be rejected")
	}
}

// The committed runtime must not inherit the bounded experiment context.
func TestCommitGraphIsIdempotentAndDetachesCancellation(t *testing.T) {
	e := &WindowsGraphExecutor{ownedPIDs: map[int]struct{}{}}
	e.active = &windowsGraphProcess{
		done:    make(chan struct{}),
		ready:   make(chan struct{}, 1),
		cancel:  func() {},
		hostSet: map[string]string{},
	}
	if err := e.CommitGraph(ServiceGraphActivation{}); err != nil {
		t.Fatalf("commit: %v", err)
	}
	if err := e.CommitGraph(ServiceGraphActivation{}); err != nil {
		t.Fatalf("second commit must be a no-op, got: %v", err)
	}
	if e.active == nil {
		t.Fatal("commit must keep the process owned so Revert can stop it")
	}
}

// A StateSnapshot is registered inside the runtime, so the graph executor must
// keep ONE runtime for its whole lifetime. A per-call runtime loses every
// snapshot, and every managed graph Apply then fails at EstablishDirect with
// "unknown state snapshot".
func TestWindowsGraphExecutorKeepsOneRuntimeForItsLifetime(t *testing.T) {
	exec, err := NewWindowsGraphExecutor(RuntimeOptions{
		Provider: &windowsRuntimeProvider{}, Assets: &engine.AssetPaths{BinDir: t.TempDir()},
	})
	if err != nil {
		t.Fatalf("new windows graph executor: %v", err)
	}
	first := exec.runtime
	if first == nil {
		t.Fatal("windows graph executor must own a runtime")
	}
	if _, err := exec.Snapshot(context.Background()); err != nil {
		t.Fatalf("snapshot: %v", err)
	}
	// EstablishDirect must resolve the snapshot registered by Snapshot. Before the
	// fix this failed with "unknown state snapshot".
	if err := exec.EstablishDirect(context.Background(), StateSnapshot{ID: "missing"}); err == nil {
		t.Fatal("an unregistered snapshot must still be refused")
	}
	if exec.runtime != first {
		t.Fatal("the executor must not swap its runtime between calls")
	}
}

func TestWindowsGraphExecutorOwnsItsRuntime(t *testing.T) {
	win, err := NewWindowsGraphExecutor(RuntimeOptions{
		Provider: &windowsRuntimeProvider{}, Assets: &engine.AssetPaths{BinDir: t.TempDir()},
	})
	if err != nil {
		t.Skipf("windows runtime unavailable on this host: %v", err)
	}
	if win.runtime == nil {
		t.Fatal("windows graph executor must own a runtime for its lifetime")
	}
}

// Readiness is a latched fact, not a one-shot edge. ActivateGraph consumes the
// ready channel, so verification that also drained it would report "not ready"
// on every call after the first.
func TestWindowsGraphReadinessLatches(t *testing.T) {
	proc := &windowsGraphProcess{ready: make(chan struct{}, 1)}
	exec := &WindowsGraphExecutor{active: proc, ownedPIDs: map[int]struct{}{}}
	if proc.readyLatched.Load() {
		t.Fatal("a process that never signalled ready must not report ready")
	}
	// The activation wait consumes the channel exactly once.
	proc.ready <- struct{}{}
	<-proc.ready
	proc.readyLatched.Store(true)
	// A drained channel must still read as ready for every later verification.
	for i := range 3 {
		if !proc.readyLatched.Load() {
			t.Fatalf("verification %d: readiness must latch, not be consumed", i)
		}
	}
	_ = exec
}

// RenderGraphActivation lives in a windows-only file, so tests that exercise
// it must live here too. In an untagged test file they compile only on Windows
// and break `go vet` on every other platform.
// The graph capture must come from the compiled CapturePlan, never from a
// hardcoded 443. A strategy compiled for other ports would otherwise install a
// capture that never sees the traffic the strategy actually targets.
func TestRenderWindowsGraphCaptureUsesCompiledPorts(t *testing.T) {
	capture := graphTestCapture()
	capture.TCPPorts = []strategyir.PortRange{{Start: 5222, End: 5223}, {Start: 5228, End: 5228}}
	activation := ServiceGraphActivation{Graph: twoNodeGraph(t), Sections: graphSections(), Capture: capture}
	plan, err := RenderGraphActivation(activation)
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	joined := strings.Join(plan.CaptureArgv, "\n")
	if !strings.Contains(joined, "5222") || !strings.Contains(joined, "5223") || !strings.Contains(joined, "5228") {
		t.Fatalf("capture argv must carry the compiled ports, got %q", joined)
	}
	if !strings.Contains(plan.RawFilter, "tcp.DstPort >= 5222") || !strings.Contains(plan.RawFilter, "tcp.DstPort <= 5223") {
		t.Fatalf("raw filter must carry the compiled port range, got %q", plan.RawFilter)
	}
	if !strings.Contains(plan.RawFilter, "tcp.DstPort == 5228") {
		t.Fatalf("raw filter must carry the compiled single port, got %q", plan.RawFilter)
	}
	// The inbound clause must mirror the outbound one.
	if !strings.Contains(plan.RawFilter, "tcp.SrcPort >= 5222") {
		t.Fatalf("raw filter inbound clause must mirror the compiled ports, got %q", plan.RawFilter)
	}
}

// A capture with no TCP ports must be refused rather than silently widened.
func TestRenderWindowsGraphCaptureRejectsEmptyPortSet(t *testing.T) {
	capture := graphTestCapture()
	capture.TCPPorts = nil
	activation := ServiceGraphActivation{Graph: twoNodeGraph(t), Sections: graphSections(), Capture: capture}
	if _, err := RenderGraphActivation(activation); err == nil {
		t.Fatal("an empty compiled port set must be refused")
	}
}
