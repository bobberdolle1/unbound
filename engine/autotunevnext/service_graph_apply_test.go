package autotunevnext

// Hermetic coverage for bounded service-graph Apply gating.
//
// Apply must re-resolve every node and refuse anything that would widen capture
// beyond the validated authority, before any machine mutation happens.

import (
	"context"
	"testing"

	"unbound/engine/backendcap"
	"unbound/engine/observatory"
	"unbound/engine/strategyir"
)

func applyCapture() backendcap.CapturePlan {
	return exactCapture(backendcap.CaptureWinDivert, strategyir.IPFamilyV4, strategyir.DirectionOutbound, strategyir.PortRange{Start: 443, End: 443})
}

func validatedGraphFixture(t *testing.T, nodes ...ServiceNode) ServiceGraph {
	t.Helper()
	for i := range nodes {
		nodes[i].Validation = ServiceNodeValidated
	}
	g, err := NewServiceGraph(nodes)
	if err != nil {
		t.Fatalf("validated fixture: %v", err)
	}
	return g
}

func baseApplyRequest(t *testing.T) ServiceGraphApplyRequest {
	t.Helper()
	entry := gNode("ENTRY", "entry.example", ServiceNodeRoleEntry, []ServiceScopeEdge{gEdge("192.0.2.1")}, ServiceNodeValidated)
	media := gNode("MEDIA", "media.example", ServiceNodeRoleMedia, []ServiceScopeEdge{gEdge("198.51.100.7")}, ServiceNodeValidated)
	validated := validatedGraphFixture(t, entry, media)

	current := ServiceGraph{Nodes: []ServiceNode{
		gNode("ENTRY", "entry.example", ServiceNodeRoleEntry, []ServiceScopeEdge{gEdge("192.0.2.1")}, ServiceNodeActive),
		gNode("MEDIA", "media.example", ServiceNodeRoleMedia, []ServiceScopeEdge{gEdge("198.51.100.7")}, ServiceNodeActive),
	}}
	return ServiceGraphApplyRequest{
		Validated: validated,
		Current:   current,
		Sections:  graphSections(),
		Capture:   applyCapture(),
		Backend:   backendcap.Zapret2Windows,
		Policy:    DefaultPolicy(),
	}
}

// An unchanged re-resolution may be applied.
func TestPrepareServiceGraphApplyAllowsUnchangedGraph(t *testing.T) {
	prepared, err := PrepareServiceGraphApply(baseApplyRequest(t))
	if err != nil {
		t.Fatalf("unchanged graph must be applicable: %v", err)
	}
	if len(prepared.Graph.Nodes) != 2 || len(prepared.Sections) != 2 {
		t.Fatalf("prepared nodes=%d sections=%d, want 2/2", len(prepared.Graph.Nodes), len(prepared.Sections))
	}
	if len(prepared.UnionEdges) != 2 {
		t.Fatalf("union edges = %d, want 2", len(prepared.UnionEdges))
	}
}

// A new address on an existing node widens capture and must be refused.
func TestPrepareServiceGraphApplyRejectsNewEdge(t *testing.T) {
	request := baseApplyRequest(t)
	request.Current.Nodes[1].Scope.Edges = append(request.Current.Nodes[1].Scope.Edges, gEdge("198.51.100.8"))
	if _, err := PrepareServiceGraphApply(request); err == nil {
		t.Fatal("a new edge at Apply must fail closed")
	}
}

// A host that appears at Apply was never validated and must be refused.
func TestPrepareServiceGraphApplyRejectsNewNode(t *testing.T) {
	request := baseApplyRequest(t)
	request.Current.Nodes = append(request.Current.Nodes,
		gNode("AUX", "aux.example", ServiceNodeRoleAuxiliary, []ServiceScopeEdge{gEdge("203.0.113.4")}, ServiceNodeActive))
	request.Sections = append(request.Sections, ServiceGraphSection{
		NodeID: "AUX", Host: "aux.example", Argv: []string{hostlistDomainsArg("aux.example")},
	})
	if _, err := PrepareServiceGraphApply(request); err == nil {
		t.Fatal("a new node at Apply must fail closed")
	}
}

// A disappeared required node must be reported, never substituted.
func TestPrepareServiceGraphApplyRejectsMissingRequiredNode(t *testing.T) {
	request := baseApplyRequest(t)
	request.Current.Nodes = request.Current.Nodes[:1]
	request.Sections = request.Sections[:1]
	if _, err := PrepareServiceGraphApply(request); err == nil {
		t.Fatal("a missing required node must fail closed")
	}
}

// Apply derives capture from the ACTIVE nodes only, so a validated node that is
// not active is not authorized by the experiment grant.
func TestPrepareServiceGraphApplyUsesActiveNodesOnly(t *testing.T) {
	request := baseApplyRequest(t)
	// MEDIA was validated by the experiment but is not active now.
	request.Current.Nodes[1].Validation = ServiceNodeValidated
	prepared, err := PrepareServiceGraphApply(request)
	if err != nil {
		t.Fatalf("narrower active graph must still apply: %v", err)
	}
	if len(prepared.Graph.Nodes) != 1 || prepared.Graph.Nodes[0].ID != "ENTRY" {
		t.Fatalf("prepared nodes = %d, want only ENTRY", len(prepared.Graph.Nodes))
	}
	if len(prepared.Sections) != 1 {
		t.Fatalf("prepared sections = %d, want 1", len(prepared.Sections))
	}
	if len(prepared.UnionEdges) != 1 {
		t.Fatalf("union edges = %d, want 1", len(prepared.UnionEdges))
	}
}

// A section bound to another node's host must be refused.
func TestPrepareServiceGraphApplyRejectsSectionHostMismatch(t *testing.T) {
	request := baseApplyRequest(t)
	request.Sections[1].Host = "entry.example"
	if _, err := PrepareServiceGraphApply(request); err == nil {
		t.Fatal("a section bound to another host must fail closed")
	}
}

// A section carrying wildcard or list authority must be refused.
func TestPrepareServiceGraphApplyRejectsWildcardSection(t *testing.T) {
	for _, bad := range [][]string{
		{"--lua-desync=fake", "--hostlist-domains=^*.example"},
		{"--lua-desync=fake", "--hostlist=all"},
		{"--lua-desync=fake", "--wf-tcp-out=443"},
	} {
		request := baseApplyRequest(t)
		request.Sections[0].Argv = bad
		if _, err := PrepareServiceGraphApply(request); err == nil {
			t.Fatalf("section argv %v must fail closed at Apply", bad)
		}
	}
}

// An empty validated graph must not authorize anything.
func TestPrepareServiceGraphApplyRejectsEmptyValidatedGraph(t *testing.T) {
	request := baseApplyRequest(t)
	request.Validated = ServiceGraph{}
	if _, err := PrepareServiceGraphApply(request); err == nil {
		t.Fatal("an empty validated graph must fail closed")
	}
}

// Apply must not run when the gate rejects, and must never reach an executor.
func TestApplyServiceGraphGateStopsBeforeMutation(t *testing.T) {
	request := baseApplyRequest(t)
	request.Current.Nodes[1].Scope.Edges = append(request.Current.Nodes[1].Scope.Edges, gEdge("198.51.100.8"))
	exec := &graphExecLog{}
	if _, err := ApplyServiceGraph(context.Background(), request, exec, &fakeObserver{}); err == nil {
		t.Fatal("expected gate rejection")
	}
	if len(exec.calls) != 0 {
		t.Fatalf("executor must not be touched when the gate rejects; calls = %v", exec.calls)
	}
}

// A successful Apply activates the graph once and restores before returning.
func TestApplyServiceGraphRestoresOnSuccess(t *testing.T) {
	request := baseApplyRequest(t)
	observer := &fakeObserver{results: []observatory.ObservationResult{
		observation("entry-direct", true, "192.0.2.1", "https://entry.example/"),
		observation("media-direct", true, "198.51.100.7", "https://media.example/"),
		observation("entry-active", true, "192.0.2.1", "https://entry.example/"),
		observation("media-active", true, "198.51.100.7", "https://media.example/"),
	}}
	exec := &graphExecLog{}
	activation, err := ApplyServiceGraph(context.Background(), request, exec, observer)
	if err != nil {
		t.Fatalf("apply: %v", err)
	}
	if activation == nil || len(activation.Graph().Nodes) != 2 {
		t.Fatal("apply must return the applied graph")
	}
	if exec.activateCount != 1 {
		t.Fatalf("activations = %d, want 1", exec.activateCount)
	}
	if exec.calls[len(exec.calls)-1] != "verify_restored" {
		t.Fatalf("apply must end with restore verification; calls = %v", exec.calls)
	}
	// A committed apply keeps its revert obligation so the product can still undo it.
	if !activation.restorePending {
		t.Fatal("a committed apply must retain the ability to revert")
	}
	if err := activation.Revert(context.Background()); err != nil {
		t.Fatalf("revert after apply: %v", err)
	}
	if activation.restorePending {
		t.Fatal("revert must clear the pending restore obligation")
	}
}
