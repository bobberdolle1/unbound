package main

// Hermetic coverage for the bounded-graph managed lifecycle.
//
// The properties under test are the ones that keep a working machine working:
// a committed Apply really is committed, Suspend keeps the user's intent, Revert
// removes it, a manual action always wins, and no failure path can leave a
// half-applied runtime or a stale persisted intent behind.

import (
	"context"
	"net"
	"testing"

	"unbound/engine"
	"unbound/engine/autotunevnext"
	"unbound/engine/observatory"
)

type fakeGraphExecutor struct {
	calls []string
	// activation failure injection
	activateErr     error
	verifyActiveErr error
	alive           bool
}

func (f *fakeGraphExecutor) record(call string) { f.calls = append(f.calls, call) }

func (f *fakeGraphExecutor) Snapshot(context.Context) (autotunevnext.StateSnapshot, error) {
	f.record("snapshot")
	return autotunevnext.StateSnapshot{ID: "snap"}, nil
}

func (f *fakeGraphExecutor) EstablishDirect(context.Context, autotunevnext.StateSnapshot) error {
	f.record("establish_direct")
	return nil
}

func (f *fakeGraphExecutor) ActivateGraph(context.Context, autotunevnext.ServiceGraphActivation) error {
	f.record("activate")
	if f.activateErr != nil {
		return f.activateErr
	}
	f.alive = true
	return nil
}

func (f *fakeGraphExecutor) VerifyActiveGraph(context.Context, autotunevnext.ServiceGraphActivation) error {
	f.record("verify_active")
	return f.verifyActiveErr
}

func (f *fakeGraphExecutor) Deactivate(context.Context) error {
	f.record("deactivate")
	f.alive = false
	return nil
}

func (f *fakeGraphExecutor) Restore(context.Context, autotunevnext.StateSnapshot) error {
	f.record("restore")
	return nil
}

func (f *fakeGraphExecutor) VerifyRestored(context.Context, autotunevnext.StateSnapshot) error {
	f.record("verify_restored")
	return nil
}

func (f *fakeGraphExecutor) saw(call string) bool {
	for _, c := range f.calls {
		if c == call {
			return true
		}
	}
	return false
}

func graphLifecycleFixture(t *testing.T) (*graphManagedService, *fakeGraphExecutor) {
	t.Helper()
	restore := engine.SetConfigDirForTest(t.TempDir())
	t.Cleanup(restore)
	exec := &fakeGraphExecutor{}
	return newGraphManagedService(exec), exec
}

// A committed Apply must leave the runtime alive after it returns.
func TestGraphManagedApplyCommitsAndKeepsRuntimeAlive(t *testing.T) {
	service, exec := graphLifecycleFixture(t)
	if err := service.ApplyGraph(context.Background(), autotunevnext.ServiceGraphActivation{}, validGraphState()); err != nil {
		t.Fatalf("apply: %v", err)
	}
	if !exec.alive {
		t.Fatal("a committed apply must leave the graph runtime alive")
	}
	if service.state != GraphManagedActive {
		t.Fatalf("state = %q, want GRAPH_MANAGED", service.state)
	}
	if _, ok, err := loadVNextGraphState(); err != nil || !ok {
		t.Fatalf("committed apply must persist intent, ok=%v err=%v", ok, err)
	}
}

// Suspend restores the direct path but keeps the intent, so a later restart can
// rebuild it from fresh authority instead of replaying it.
func TestGraphManagedSuspendRestoresAndRetainsIntent(t *testing.T) {
	service, exec := graphLifecycleFixture(t)
	if err := service.ApplyGraph(context.Background(), autotunevnext.ServiceGraphActivation{}, validGraphState()); err != nil {
		t.Fatalf("apply: %v", err)
	}
	if err := service.Suspend(context.Background()); err != nil {
		t.Fatalf("suspend: %v", err)
	}
	if exec.alive {
		t.Fatal("suspend must stop the owned graph process")
	}
	if !exec.saw("verify_restored") {
		t.Fatal("suspend must prove the direct state was restored")
	}
	if _, ok, err := loadVNextGraphState(); err != nil || !ok {
		t.Fatalf("suspend must retain the logical intent, ok=%v err=%v", ok, err)
	}
	if service.state != GraphManagedRevalidation {
		t.Fatalf("state = %q, want revalidation pending after suspend", service.state)
	}
}

// Revert clears the intent entirely.
func TestGraphManagedRevertRestoresAndClearsIntent(t *testing.T) {
	service, exec := graphLifecycleFixture(t)
	if err := service.ApplyGraph(context.Background(), autotunevnext.ServiceGraphActivation{}, validGraphState()); err != nil {
		t.Fatalf("apply: %v", err)
	}
	if err := service.Revert(context.Background()); err != nil {
		t.Fatalf("revert: %v", err)
	}
	if exec.alive {
		t.Fatal("revert must stop the owned graph process")
	}
	if !exec.saw("verify_restored") {
		t.Fatal("revert must prove the direct state was restored")
	}
	if _, ok, err := loadVNextGraphState(); ok || err != nil {
		t.Fatalf("revert must clear the intent, ok=%v err=%v", ok, err)
	}
	if service.state != GraphManagedDirect {
		t.Fatalf("state = %q, want DIRECT after revert", service.state)
	}
}

// Manual user action wins: the managed graph yields completely.
func TestGraphManagedManualTakeoverYieldsRuntime(t *testing.T) {
	service, exec := graphLifecycleFixture(t)
	if err := service.ApplyGraph(context.Background(), autotunevnext.ServiceGraphActivation{}, validGraphState()); err != nil {
		t.Fatalf("apply: %v", err)
	}
	if err := service.YieldToManualAction(context.Background()); err != nil {
		t.Fatalf("yield: %v", err)
	}
	if exec.alive {
		t.Fatal("a manual action must stop the managed graph runtime")
	}
	if service.state != GraphManagedDirect {
		t.Fatalf("state = %q, want DIRECT after manual takeover", service.state)
	}
	if _, ok, _ := loadVNextGraphState(); ok {
		t.Fatal("manual takeover must clear the managed graph intent")
	}
}

// An activation failure must not leave intent behind and must restore the
// machine's original state.
func TestGraphManagedApplyFailureLeavesNoIntent(t *testing.T) {
	service, exec := graphLifecycleFixture(t)
	exec.activateErr = errFake
	if err := service.ApplyGraph(context.Background(), autotunevnext.ServiceGraphActivation{}, validGraphState()); err == nil {
		t.Fatal("an activation failure must fail the apply")
	}
	if _, ok, _ := loadVNextGraphState(); ok {
		t.Fatal("a failed apply must not leave a persisted intent")
	}
	if !exec.saw("restore") {
		t.Fatal("a failed apply must restore the original state")
	}
	if exec.alive {
		t.Fatal("a failed apply must not leave a live runtime")
	}
}

// A VerifyActive failure is a failed apply, not a committed one.
func TestGraphManagedVerifyActiveFailureDoesNotCommit(t *testing.T) {
	service, exec := graphLifecycleFixture(t)
	exec.verifyActiveErr = errFake
	if err := service.ApplyGraph(context.Background(), autotunevnext.ServiceGraphActivation{}, validGraphState()); err == nil {
		t.Fatal("a VerifyActive failure must fail the apply")
	}
	if exec.alive {
		t.Fatal("a VerifyActive failure must not leave a live runtime")
	}
	if _, ok, _ := loadVNextGraphState(); ok {
		t.Fatal("a VerifyActive failure must not persist intent")
	}
	if service.state != GraphManagedDirect {
		t.Fatalf("state = %q, want DIRECT", service.state)
	}
}

// Cancellation must still enter cleanup rather than abandoning the machine.
func TestGraphManagedApplyCancellationRestores(t *testing.T) {
	service, exec := graphLifecycleFixture(t)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	exec.activateErr = ctx.Err()
	if err := service.ApplyGraph(ctx, autotunevnext.ServiceGraphActivation{}, validGraphState()); err == nil {
		t.Fatal("a cancelled apply must fail")
	}
	if !exec.saw("restore") {
		t.Fatal("a cancelled apply must still restore the original state")
	}
	if _, ok, _ := loadVNextGraphState(); ok {
		t.Fatal("a cancelled apply must not persist intent")
	}
}

// A second Apply while one is active must be refused rather than stacking
// runtimes.
func TestGraphManagedRejectsSecondApplyWhileActive(t *testing.T) {
	service, _ := graphLifecycleFixture(t)
	if err := service.ApplyGraph(context.Background(), autotunevnext.ServiceGraphActivation{}, validGraphState()); err != nil {
		t.Fatalf("first apply: %v", err)
	}
	if err := service.ApplyGraph(context.Background(), autotunevnext.ServiceGraphActivation{}, validGraphState()); err == nil {
		t.Fatal("a second apply must be refused while a graph is active")
	}
}

// The product status must never leak an address, filter, or argv.
func TestGraphManagedStatusCarriesOnlyLogicalFields(t *testing.T) {
	service, _ := graphLifecycleFixture(t)
	if err := service.ApplyGraph(context.Background(), autotunevnext.ServiceGraphActivation{}, validGraphState()); err != nil {
		t.Fatalf("apply: %v", err)
	}
	status := service.status
	if status.ServiceID != "svc-1" || status.Backend != "zapret2/windows" || status.NodeCount != 2 {
		t.Fatalf("status missing logical fields: %+v", status)
	}
	if status.EdgeCount != 0 {
		t.Fatalf("status must not report edge counts from a logical intent, got %d", status.EdgeCount)
	}
}

// healthGraphFor builds a real one-node graph carrying one exact address.
func healthGraphFor(t *testing.T, ip string) autotunevnext.ServiceGraph {
	t.Helper()
	target := autotunevnext.Target{URL: "https://entry.example/"}
	graph, err := autotunevnext.NewServiceGraph([]autotunevnext.ServiceNode{{
		ID: "ENTRY", Role: autotunevnext.ServiceNodeRoleEntry, Required: true,
		Target: target, Validation: autotunevnext.ServiceNodeActive,
		Strategy: "s1", StrategyFingerprint: "fp1",
		Scope: autotunevnext.ServiceScopeSnapshot{
			Target: target,
			Edges:  []autotunevnext.ServiceScopeEdge{{IP: net.ParseIP(ip), Family: observatory.AddressFamilyIPv4}},
		},
	}})
	if err != nil {
		t.Fatalf("graph: %v", err)
	}
	return graph
}

// A dead owned process is a lifecycle FAULT, not a network condition.
func TestGraphManagedHealthFaultWhenProcessDead(t *testing.T) {
	service, _ := graphLifecycleFixture(t)
	if err := service.ApplyGraph(context.Background(), autotunevnext.ServiceGraphActivation{}, validGraphState()); err != nil {
		t.Fatalf("apply: %v", err)
	}
	graph := healthGraphFor(t, "192.0.2.1")
	dead := newGraphManagedService(&deadProcessExecutor{})
	dead.state = GraphManagedActive
	if got := dead.EvaluateHealth(graph, graph, false); got != GraphManagedFault {
		t.Fatalf("health = %q, want GRAPH_FAULT when the owned process is dead", got)
	}
	if service.state != GraphManagedActive {
		t.Fatalf("the healthy service must be untouched, state=%q", service.state)
	}
}

// A fresh resolver answer with a new address must move the graph to revalidation,
// not stay healthy.
func TestGraphManagedHealthRevalidatesOnNewEdge(t *testing.T) {
	service, _ := graphLifecycleFixture(t)
	if err := service.ApplyGraph(context.Background(), autotunevnext.ServiceGraphActivation{}, validGraphState()); err != nil {
		t.Fatalf("apply: %v", err)
	}
	active := healthGraphFor(t, "192.0.2.1")
	moved := healthGraphFor(t, "203.0.113.9")
	if got := service.EvaluateHealth(active, moved, true); got != GraphManagedRevalidation {
		t.Fatalf("health = %q, want revalidation on a new edge", got)
	}
}

// Unchanged state stays healthy.
func TestGraphManagedHealthStaysHealthyWhenUnchanged(t *testing.T) {
	service, _ := graphLifecycleFixture(t)
	if err := service.ApplyGraph(context.Background(), autotunevnext.ServiceGraphActivation{}, validGraphState()); err != nil {
		t.Fatalf("apply: %v", err)
	}
	graph := healthGraphFor(t, "192.0.2.1")
	if got := service.EvaluateHealth(graph, graph, true); got != GraphManagedActive {
		t.Fatalf("health = %q, want GRAPH_MANAGED when unchanged", got)
	}
}

// deadProcessExecutor reports that the owned process is gone.
type deadProcessExecutor struct{ fakeGraphExecutor }

var errFake = errTestFailure{}

type errTestFailure struct{}

func (errTestFailure) Error() string { return "injected lifecycle failure" }
