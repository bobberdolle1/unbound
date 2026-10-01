package autotunevnext

// Hermetic coverage for bounded service-graph managed health.
//
// The PR59 lesson under test: health is judged against the ACTUAL ACTIVE CAPTURE
// GRAPH, so a node the experiment once validated but Apply never activated is not
// silently authorized later.

import (
	"encoding/json"
	"strings"
	"testing"
)

func healthGraph(t *testing.T, states ...ServiceNodeValidationState) ServiceGraph {
	t.Helper()
	nodes := []ServiceNode{
		gNode("ENTRY", "entry.example", ServiceNodeRoleEntry, []ServiceScopeEdge{gEdge("192.0.2.1")}, states[0]),
	}
	if len(states) > 1 {
		nodes = append(nodes, gNode("MEDIA", "media.example", ServiceNodeRoleMedia, []ServiceScopeEdge{gEdge("198.51.100.7")}, states[1]))
	}
	g, err := NewServiceGraph(nodes)
	if err != nil {
		t.Fatalf("graph: %v", err)
	}
	return g
}

func TestServiceGraphHealthHealthyWhenUnchanged(t *testing.T) {
	health, err := EvaluateServiceGraphHealth(ServiceGraphHealthInput{
		ActiveCapture:     healthGraph(t, ServiceNodeActive),
		Current:           healthGraph(t, ServiceNodeActive),
		OwnedProcessAlive: true,
	})
	if err != nil {
		t.Fatalf("health: %v", err)
	}
	if health != ServiceGraphHealthy {
		t.Fatalf("health = %q, want HEALTHY", health)
	}
}

func TestServiceGraphHealthDeadProcessIsFault(t *testing.T) {
	health, err := EvaluateServiceGraphHealth(ServiceGraphHealthInput{
		ActiveCapture:     healthGraph(t, ServiceNodeActive),
		Current:           healthGraph(t, ServiceNodeActive),
		OwnedProcessAlive: false,
	})
	if health != ServiceGraphFault {
		t.Fatalf("health = %q, want FAULT", health)
	}
	if err == nil {
		t.Fatal("a dead owned process must report a reason")
	}
}

// Resolver failure fails closed rather than reporting health.
func TestServiceGraphHealthResolverFailureFailsClosed(t *testing.T) {
	health, err := EvaluateServiceGraphHealth(ServiceGraphHealthInput{
		ActiveCapture:     healthGraph(t, ServiceNodeActive),
		Current:           healthGraph(t, ServiceNodeActive),
		OwnedProcessAlive: true,
		ResolverFailed:    true,
	})
	if health != ServiceGraphNeedsRevalidation {
		t.Fatalf("health = %q, want NEEDS_REVALIDATION", health)
	}
	if err == nil {
		t.Fatal("resolver failure must report a reason")
	}
}

// A new address on an active node is a revalidation trigger.
func TestServiceGraphHealthNewEdgeNeedsRevalidation(t *testing.T) {
	current := healthGraph(t, ServiceNodeActive)
	current.Nodes[0].Scope.Edges = append(current.Nodes[0].Scope.Edges, gEdge("192.0.2.2"))
	if err := current.Validate(); err != nil {
		t.Fatalf("current graph: %v", err)
	}
	health, err := EvaluateServiceGraphHealth(ServiceGraphHealthInput{
		ActiveCapture:     healthGraph(t, ServiceNodeActive),
		Current:           current,
		OwnedProcessAlive: true,
	})
	if health != ServiceGraphNeedsRevalidation {
		t.Fatalf("health = %q, want NEEDS_REVALIDATION", health)
	}
	if err == nil {
		t.Fatal("a new edge must report a reason")
	}
}

// A new required node is a revalidation trigger.
func TestServiceGraphHealthNewNodeNeedsRevalidation(t *testing.T) {
	health, err := EvaluateServiceGraphHealth(ServiceGraphHealthInput{
		ActiveCapture:     healthGraph(t, ServiceNodeActive),
		Current:           healthGraph(t, ServiceNodeActive, ServiceNodeActive),
		OwnedProcessAlive: true,
	})
	if health != ServiceGraphNeedsRevalidation {
		t.Fatalf("health = %q, want NEEDS_REVALIDATION", health)
	}
	if err == nil {
		t.Fatal("a new node must report a reason")
	}
}

// The PR59 lesson: a node the experiment validated but that is not in the active
// capture must not be silently authorized by a later fresh answer.
func TestServiceGraphHealthDoesNotTrustInactiveNodeFromGrant(t *testing.T) {
	// The capture graph holds only ENTRY.
	active := healthGraph(t, ServiceNodeActive)
	// A fresh resolution brings MEDIA back with an edge.
	current := healthGraph(t, ServiceNodeActive, ServiceNodeValidated)
	health, err := EvaluateServiceGraphHealth(ServiceGraphHealthInput{
		ActiveCapture:     active,
		Current:           current,
		OwnedProcessAlive: true,
	})
	if health == ServiceGraphHealthy {
		t.Fatal("a node absent from the active capture must not report healthy")
	}
	if health != ServiceGraphNeedsRevalidation {
		t.Fatalf("health = %q, want NEEDS_REVALIDATION", health)
	}
	_ = err
}

// With no active capture there is nothing bound, so it cannot be healthy.
func TestServiceGraphHealthNoActiveCaptureNeedsRevalidation(t *testing.T) {
	health, err := EvaluateServiceGraphHealth(ServiceGraphHealthInput{
		ActiveCapture:     healthGraph(t, ServiceNodeValidated),
		Current:           healthGraph(t, ServiceNodeActive),
		OwnedProcessAlive: true,
	})
	if health != ServiceGraphNeedsRevalidation {
		t.Fatalf("health = %q, want NEEDS_REVALIDATION", health)
	}
	if err == nil {
		t.Fatal("missing active capture must report a reason")
	}
}

func TestServiceGraphHealthSnapshotCarriesNoAddresses(t *testing.T) {
	snapshot, err := NewServiceGraphHealthSnapshot(healthGraph(t, ServiceNodeActive), ServiceGraphHealthy)
	if err != nil {
		t.Fatalf("snapshot: %v", err)
	}
	if snapshot.NodeCount != 1 || snapshot.EdgeCount != 1 {
		t.Fatalf("snapshot counts = %d/%d, want 1/1", snapshot.NodeCount, snapshot.EdgeCount)
	}
	blob, err := json.Marshal(snapshot)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	for _, forbidden := range []string{"192.0.2.1", "198.51.100.7"} {
		if strings.Contains(string(blob), forbidden) {
			t.Fatalf("health snapshot leaked a resolved address %q", forbidden)
		}
	}
}
