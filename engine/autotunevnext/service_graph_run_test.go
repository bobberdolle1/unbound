package autotunevnext

// Hermetic coverage for bounded service-graph experiment execution.
//
// The budget rule under test is the one that makes graphs affordable: ONE
// activation covers the whole graph, never one activation per edge. These tests
// assert the exact executor call sequence and that every failure and cancellation
// path still restores original state.

import (
	"context"
	"errors"
	"testing"

	"unbound/engine/backendcap"
	"unbound/engine/observatory"
	"unbound/engine/strategyir"
)

type graphExecLog struct {
	calls         []string
	activateCount int
	failActivate  bool
	failVerifyAct bool
	failRestored  bool
}

func (g *graphExecLog) Snapshot(context.Context) (StateSnapshot, error) {
	g.calls = append(g.calls, "snapshot")
	return StateSnapshot{ID: "snap"}, nil
}

func (g *graphExecLog) EstablishDirect(context.Context, StateSnapshot) error {
	g.calls = append(g.calls, "establish")
	return nil
}

func (g *graphExecLog) ActivateGraph(context.Context, ServiceGraphActivation) error {
	g.calls = append(g.calls, "activate")
	g.activateCount++
	if g.failActivate {
		return errors.New("boom")
	}
	return nil
}

func (g *graphExecLog) VerifyActiveGraph(context.Context, ServiceGraphActivation) error {
	g.calls = append(g.calls, "verify_active")
	if g.failVerifyAct {
		return errors.New("boom")
	}
	return nil
}

func (g *graphExecLog) Deactivate(context.Context) error {
	g.calls = append(g.calls, "deactivate")
	return nil
}

// CommitGraph satisfies the executor contract. The bounded experiment path never
// commits a graph, so it only records the call.
func (g *graphExecLog) CommitGraph(ServiceGraphActivation) error {
	g.calls = append(g.calls, "commit")
	return nil
}

func (g *graphExecLog) Restore(context.Context, StateSnapshot) error {
	g.calls = append(g.calls, "restore")
	return nil
}

func (g *graphExecLog) VerifyRestored(context.Context, StateSnapshot) error {
	g.calls = append(g.calls, "verify_restored")
	if g.failRestored {
		return errors.New("boom")
	}
	return nil
}

func graphFailure(id, ip, target string) observatory.ObservationResult {
	result := observation(id, true, ip, target)
	result.Attempts[0].Stages[len(result.Attempts[0].Stages)-1].HTTPStatus = 451
	result.Classification = observatory.ClassHTTPStatus
	return result
}

func graphExecCapture() backendcap.CapturePlan {
	return exactCapture(backendcap.CaptureWinDivert, strategyir.IPFamilyV4, strategyir.DirectionOutbound, strategyir.PortRange{Start: 443, End: 443})
}

func twoActiveNodeGraph(t *testing.T) ServiceGraph {
	t.Helper()
	g, err := NewServiceGraph([]ServiceNode{
		gNode("ENTRY", "entry.example", ServiceNodeRoleEntry, []ServiceScopeEdge{gEdge("192.0.2.1")}, ServiceNodeActive),
		gNode("MEDIA", "media.example", ServiceNodeRoleMedia, []ServiceScopeEdge{gEdge("198.51.100.7")}, ServiceNodeActive),
	})
	if err != nil {
		t.Fatalf("graph: %v", err)
	}
	return g
}

func graphExperimentRequest(t *testing.T) ServiceGraphExperimentRequest {
	t.Helper()
	return ServiceGraphExperimentRequest{
		Graph:    twoActiveNodeGraph(t),
		Controls: []Target{gTarget("control.example")},
		Sections: graphSections(),
		Capture:  graphExecCapture(),
		Backend:  backendcap.Zapret2Windows,
		Policy:   DefaultPolicy(),
	}
}

// One activation for the whole graph, in the documented order.
func TestServiceGraphExperimentActivatesGraphExactlyOnce(t *testing.T) {
	request := graphExperimentRequest(t)
	observer := &fakeObserver{results: []observatory.ObservationResult{
		observation("c-direct", true, "203.0.113.9", "https://control.example/"),
		graphFailure("entry-before", "192.0.2.1", "https://entry.example/"),
		graphFailure("media-before", "198.51.100.7", "https://media.example/"),
		observation("entry-active", true, "192.0.2.1", "https://entry.example/"),
		observation("media-active", true, "198.51.100.7", "https://media.example/"),
		observation("c-active", true, "203.0.113.9", "https://control.example/"),
		graphFailure("entry-after", "192.0.2.1", "https://entry.example/"),
		graphFailure("media-after", "198.51.100.7", "https://media.example/"),
	}}
	exec := &graphExecLog{}
	result, err := RunServiceGraphExperiment(context.Background(), request, exec, observer)
	if err != nil {
		t.Fatalf("experiment: %v", err)
	}
	if exec.activateCount != 1 {
		t.Fatalf("activations = %d, want exactly 1 for a two-node graph", exec.activateCount)
	}
	want := []string{"snapshot", "establish", "activate", "verify_active", "deactivate", "restore", "verify_restored"}
	if len(exec.calls) != len(want) {
		t.Fatalf("call sequence = %v, want %v", exec.calls, want)
	}
	for i := range want {
		if exec.calls[i] != want[i] {
			t.Fatalf("call %d = %q, want %q (full %v)", i, exec.calls[i], want[i], exec.calls)
		}
	}
	if result.Status != ServiceGraphVerifiedFixed {
		t.Fatalf("status = %q, want %q (nodes %+v)", result.Status, ServiceGraphVerifiedFixed, result.Nodes)
	}
	if !result.StateRestored {
		t.Fatal("result must report restored state")
	}
	if len(result.Nodes) != 2 {
		t.Fatalf("node results = %d, want 2", len(result.Nodes))
	}
}

// A failure during activation must still restore and verify restoration.
func TestServiceGraphExperimentRestoresOnActivationFailure(t *testing.T) {
	request := graphExperimentRequest(t)
	observer := &fakeObserver{results: []observatory.ObservationResult{
		observation("c-direct", true, "203.0.113.9", "https://control.example/"),
		graphFailure("entry-before", "192.0.2.1", "https://entry.example/"),
		graphFailure("media-before", "198.51.100.7", "https://media.example/"),
	}}
	exec := &graphExecLog{failActivate: true}
	if _, err := RunServiceGraphExperiment(context.Background(), request, exec, observer); err == nil {
		t.Fatal("activation failure must return an error")
	}
	if exec.activateCount != 1 {
		t.Fatalf("activations = %d, want 1", exec.activateCount)
	}
	sawRestore, sawVerify := false, false
	for _, c := range exec.calls {
		if c == "restore" {
			sawRestore = true
		}
		if c == "verify_restored" {
			sawVerify = true
		}
	}
	if !sawRestore || !sawVerify {
		t.Fatalf("failed activation must restore and verify; calls = %v", exec.calls)
	}
}

// Restoration failure must surface as the restore-failed precedence.
func TestServiceGraphExperimentRestoreFailureTakesPrecedence(t *testing.T) {
	request := graphExperimentRequest(t)
	observer := &fakeObserver{results: []observatory.ObservationResult{
		observation("c-direct", true, "203.0.113.9", "https://control.example/"),
		graphFailure("entry-before", "192.0.2.1", "https://entry.example/"),
		graphFailure("media-before", "198.51.100.7", "https://media.example/"),
	}}
	exec := &graphExecLog{failActivate: true, failRestored: true}
	_, err := RunServiceGraphExperiment(context.Background(), request, exec, observer)
	if err == nil {
		t.Fatal("expected an error")
	}
	if !errors.Is(err, ErrManagedStateRestoreFailed) {
		t.Fatalf("err = %v, want ErrManagedStateRestoreFailed precedence", err)
	}
}

// Cancellation during an observation must still enter restore.
func TestServiceGraphExperimentCancellationRestores(t *testing.T) {
	request := graphExperimentRequest(t)
	ctx, cancel := context.WithCancel(context.Background())
	observer := &fakeObserver{results: []observatory.ObservationResult{
		observation("c-direct", true, "203.0.113.9", "https://control.example/"),
	}}
	observer.cancel = cancel
	observer.errAt = 1
	exec := &graphExecLog{}
	if _, err := RunServiceGraphExperiment(ctx, request, exec, observer); err == nil {
		t.Fatal("cancellation must return an error")
	}
	sawRestore := false
	for _, c := range exec.calls {
		if c == "restore" {
			sawRestore = true
		}
	}
	if !sawRestore {
		t.Fatalf("cancellation must enter restore; calls = %v", exec.calls)
	}
}

// A control that was healthy and breaks under the graph is a hard stop.
func TestServiceGraphExperimentControlRegressionStops(t *testing.T) {
	request := graphExperimentRequest(t)
	observer := &fakeObserver{results: []observatory.ObservationResult{
		observation("c-direct", true, "203.0.113.9", "https://control.example/"),
		graphFailure("entry-before", "192.0.2.1", "https://entry.example/"),
		graphFailure("media-before", "198.51.100.7", "https://media.example/"),
		observation("entry-active", true, "192.0.2.1", "https://entry.example/"),
		observation("media-active", true, "198.51.100.7", "https://media.example/"),
		graphFailure("c-active", "203.0.113.9", "https://control.example/"),
	}}
	exec := &graphExecLog{}
	_, err := RunServiceGraphExperiment(context.Background(), request, exec, observer)
	if err == nil {
		t.Fatal("control regression must stop the experiment")
	}
	if exec.activateCount != 1 {
		t.Fatalf("activations = %d, want 1", exec.activateCount)
	}
	if len(exec.calls) == 0 || exec.calls[len(exec.calls)-1] != "verify_restored" {
		t.Fatalf("control regression must still restore; calls = %v", exec.calls)
	}
}

// If nothing was actually fixed, the graph must not claim success.
func TestServiceGraphExperimentStillFailingWhenNoFix(t *testing.T) {
	request := graphExperimentRequest(t)
	observer := &fakeObserver{results: []observatory.ObservationResult{
		observation("c-direct", true, "203.0.113.9", "https://control.example/"),
		graphFailure("entry-before", "192.0.2.1", "https://entry.example/"),
		graphFailure("media-before", "198.51.100.7", "https://media.example/"),
		graphFailure("entry-active", "192.0.2.1", "https://entry.example/"),
		graphFailure("media-active", "198.51.100.7", "https://media.example/"),
		observation("c-active", true, "203.0.113.9", "https://control.example/"),
		graphFailure("entry-after", "192.0.2.1", "https://entry.example/"),
		graphFailure("media-after", "198.51.100.7", "https://media.example/"),
	}}
	exec := &graphExecLog{}
	result, err := RunServiceGraphExperiment(context.Background(), request, exec, observer)
	if err != nil {
		t.Fatalf("experiment: %v", err)
	}
	if result.Status == ServiceGraphVerifiedFixed {
		t.Fatal("a graph that fixed nothing must not report VERIFIED_FIXED")
	}
}
