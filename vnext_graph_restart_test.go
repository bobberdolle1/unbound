package main

// Hermetic coverage for bounded service-graph restart semantics.
//
// The invariant under test throughout: a saved graph intent never replays prior
// capture. Every address in a restart plan comes from this call's fresh
// resolution, and no saved artifact can substitute for one.

import (
	"net"
	"strings"
	"testing"

	"unbound/engine/autotunevnext"
	"unbound/engine/observatory"
)

func restartFresh(id, target, ip string, role autotunevnext.ServiceNodeRole) freshGraphNode {
	return freshGraphNode{
		NodeID: id,
		Role:   role,
		Target: target,
		Edges: []autotunevnext.ServiceScopeEdge{{
			IP:     net.ParseIP(ip),
			Family: observatory.AddressFamilyIPv4,
		}},
	}
}

func baseRestartFresh() []freshGraphNode {
	return []freshGraphNode{
		restartFresh("ENTRY", "https://entry.example/generate_204", "192.0.2.1", autotunevnext.ServiceNodeRoleEntry),
		restartFresh("MEDIA", "https://media.example/", "198.51.100.7", autotunevnext.ServiceNodeRoleMedia),
	}
}

func TestGraphRestartReadyWhenFreshResolutionMatchesIntent(t *testing.T) {
	plan, err := PlanGraphRestart(validGraphState(), baseRestartFresh())
	if err != nil {
		t.Fatalf("restart: %v", err)
	}
	if plan.State != GraphRestartReady {
		t.Fatalf("state = %q (%s), want READY", plan.State, plan.Reason)
	}
	if len(plan.Accepted) != 2 {
		t.Fatalf("accepted = %v, want 2 nodes", plan.Accepted)
	}
}

// The saved intent has no address to replay, so every address in the plan must be
// the one this call supplied.
func TestGraphRestartUsesOnlyFreshEdges(t *testing.T) {
	plan, err := PlanGraphRestart(validGraphState(), baseRestartFresh())
	if err != nil || plan.State != GraphRestartReady {
		t.Fatalf("state=%q err=%v", plan.State, err)
	}
	edges, err := plan.Validated.UnionEdges()
	if err != nil {
		t.Fatalf("union: %v", err)
	}
	seen := map[string]bool{}
	for _, edge := range edges {
		seen[edge.IP.String()] = true
	}
	for _, want := range []string{"192.0.2.1", "198.51.100.7"} {
		if !seen[want] {
			t.Fatalf("fresh edge %s missing from restart graph", want)
		}
	}
}

// A saved node whose hostname now resolves elsewhere still restarts from the NEW
// address; the stale one is never carried over because it was never stored.
func TestGraphRestartNewEdgeUsesFreshAddressOnly(t *testing.T) {
	fresh := baseRestartFresh()
	fresh[0].Edges[0].IP = net.ParseIP("203.0.113.9")
	plan, err := PlanGraphRestart(validGraphState(), fresh)
	if err != nil || plan.State != GraphRestartReady {
		t.Fatalf("state=%q err=%v reason=%s", plan.State, err, plan.Reason)
	}
	edges, _ := plan.Validated.UnionEdges()
	found := map[string]bool{}
	for _, edge := range edges {
		found[edge.IP.String()] = true
	}
	if !found["203.0.113.9"] {
		t.Fatal("restart must adopt the freshly resolved address")
	}
	if found["192.0.2.1"] {
		t.Fatal("restart must not carry a stale address")
	}
}

func TestGraphRestartMissingRequiredNodeFailsClosed(t *testing.T) {
	fresh := baseRestartFresh()[:1] // MEDIA does not resolve
	plan, err := PlanGraphRestart(validGraphState(), fresh)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if plan.State != GraphRestartRejected {
		t.Fatalf("state = %q, want REJECTED", plan.State)
	}
	if plan.Trigger != string(autotunevnext.GraphRevalidationNodeDisappeared) {
		t.Fatalf("trigger = %q, want NODE_DISAPPEARED", plan.Trigger)
	}
}

func TestGraphRestartUntrustedFreshNodeIsRejected(t *testing.T) {
	fresh := append(baseRestartFresh(), restartFresh("EXTRA", "https://extra.example/", "203.0.113.5", autotunevnext.ServiceNodeRoleMedia))
	plan, err := PlanGraphRestart(validGraphState(), fresh)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if plan.State != GraphRestartRejected {
		t.Fatalf("state = %q, want REJECTED for a node intent never named", plan.State)
	}
	if plan.Trigger != string(autotunevnext.GraphRevalidationNewNode) {
		t.Fatalf("trigger = %q, want NEW_NODE", plan.Trigger)
	}
}

func TestGraphRestartTargetChangeIsRejected(t *testing.T) {
	fresh := baseRestartFresh()
	fresh[1].Target = "https://other.example/"
	plan, err := PlanGraphRestart(validGraphState(), fresh)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if plan.State != GraphRestartRejected {
		t.Fatalf("state = %q, want REJECTED", plan.State)
	}
	if plan.Trigger != string(autotunevnext.GraphRevalidationTargetChanged) {
		t.Fatalf("trigger = %q, want TARGET_CHANGED", plan.Trigger)
	}
}

// An optional node may be absent only when the validated graph explicitly allowed
// it. Absent without that flag is rejected, never quietly dropped.
func TestGraphRestartOptionalNodeAbsentRequiresExplicitAllowance(t *testing.T) {
	state := validGraphState()
	state.Nodes[1].Required = false
	state.Nodes[1].OptionalAbsentAllowed = false
	fresh := baseRestartFresh()[:1]

	plan, err := PlanGraphRestart(state, fresh)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if plan.State != GraphRestartRejected {
		t.Fatalf("state = %q, want REJECTED without explicit allowance", plan.State)
	}

	state.Nodes[1].OptionalAbsentAllowed = true
	plan, err = PlanGraphRestart(state, fresh)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if plan.State != GraphRestartReady {
		t.Fatalf("state = %q (%s), want READY", plan.State, plan.Reason)
	}
	if len(plan.AbsentOptional) != 1 || plan.AbsentOptional[0] != "MEDIA" {
		t.Fatalf("absent optional = %v, want [MEDIA]", plan.AbsentOptional)
	}
	// Absence must not expand or shrink the active graph beyond what's allowed.
	edges, err := plan.Validated.UnionEdges()
	if err != nil {
		t.Fatalf("union: %v", err)
	}
	if len(edges) != 1 {
		t.Fatalf("edges = %d, want 1 with the optional node absent", len(edges))
	}
}

// A reappearing optional node comes back VALIDATED, never ACTIVE. It has not
// proven itself in this run.
func TestGraphRestartReappearingOptionalNodeIsNotAutoTrusted(t *testing.T) {
	state := validGraphState()
	state.Nodes[1].Required = false
	state.Nodes[1].OptionalAbsentAllowed = true
	plan, err := PlanGraphRestart(state, baseRestartFresh())
	if err != nil || plan.State != GraphRestartReady {
		t.Fatalf("state=%q err=%v", plan.State, err)
	}
	for _, node := range plan.Validated.Nodes {
		if node.ID == "MEDIA" {
			if node.Validation != autotunevnext.ServiceNodeValidated {
				t.Fatalf("reappearing optional node validation = %q, want VALIDATED", node.Validation)
			}
			return
		}
	}
	t.Fatal("reappearing optional node was dropped instead of carried as VALIDATED")
}

func TestGraphRestartRequiredNodeWithNoAddressesIsRejected(t *testing.T) {
	fresh := baseRestartFresh()
	fresh[1].Edges = nil
	plan, err := PlanGraphRestart(validGraphState(), fresh)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if plan.State != GraphRestartRejected {
		t.Fatalf("state = %q, want REJECTED", plan.State)
	}
}

func TestGraphRestartCorruptSavedIntentFailsClosed(t *testing.T) {
	state := validGraphState()
	state.SchemaVersion = 99
	if _, err := PlanGraphRestart(state, baseRestartFresh()); err == nil {
		t.Fatal("a schema-mismatched intent must not plan a restart")
	}
}

func TestGraphRestartMalformedFreshAddressIsRejected(t *testing.T) {
	fresh := baseRestartFresh()
	// net.ParseIP returns nil for this literal, which is exactly the malformed
	// input the restart gate must refuse.
	fresh[0].Edges[0].IP = net.ParseIP("not-an-ip")
	plan, err := PlanGraphRestart(validGraphState(), fresh)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if plan.State != GraphRestartRejected {
		t.Fatalf("state = %q, want REJECTED for a malformed address", plan.State)
	}
}

// ACTIVE must never be claimable while revalidation is still pending.
func TestGraphRestartStatusNeverClaimsActiveBeforeReady(t *testing.T) {
	pending := GraphRestartPlan{State: GraphRestartRevalidationPending}
	status, err := graphRestartStatus(pending)
	if err != nil {
		t.Fatalf("pending status errored: %v", err)
	}
	if status == string(ManagedRuntimeGraphManaged) {
		t.Fatalf("pending restart claimed plain graph-managed status: %q", status)
	}
	if want := string(GraphRestartRevalidationPending); !strings.Contains(status, want) {
		t.Fatalf("status %q does not expose %s", status, want)
	}
}
