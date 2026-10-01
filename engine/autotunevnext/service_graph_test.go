package autotunevnext

// Hermetic coverage for the bounded validated service graph.
//
// No test here performs DNS, spawns a process, or touches the network. Every
// scope is constructed literally so the suite is deterministic and cannot prove
// anything about the live network.

import (
	"encoding/json"
	"net"
	"strings"
	"testing"

	"unbound/engine/observatory"
	"unbound/engine/strategyir"
)

func gEdge(ip string) ServiceScopeEdge {
	return ServiceScopeEdge{IP: net.ParseIP(ip), Family: observatory.AddressFamilyIPv4}
}

func gTarget(host string) Target {
	return Target{URL: "https://" + host + "/generate_204"}
}

func gNode(id, host string, role ServiceNodeRole, edges []ServiceScopeEdge, state ServiceNodeValidationState) ServiceNode {
	target := gTarget(host)
	return ServiceNode{
		ID: id, Role: role, Required: true, Target: target,
		Strategy: "prod-tls-hostfakesplit-v1", TemplateIdentity: "tmpl-hostfakesplit",
		StrategyFingerprint: "fp-" + id, Validation: state,
		Scope: ServiceScopeSnapshot{Target: target, Edges: edges},
	}
}

// A one-node graph must reduce exactly to the single-host service scope.
func TestServiceGraphSingleNodeMatchesServiceScope(t *testing.T) {
	edges := []ServiceScopeEdge{gEdge("192.0.2.1"), gEdge("192.0.2.2")}
	g, err := NewServiceGraph([]ServiceNode{gNode("ENTRY", "entry.example", ServiceNodeRoleEntry, edges, ServiceNodeActive)})
	if err != nil {
		t.Fatalf("single node graph: %v", err)
	}
	union, err := g.UnionEdges()
	if err != nil {
		t.Fatalf("union: %v", err)
	}
	if len(union) != len(edges) {
		t.Fatalf("union edge count = %d, want %d", len(union), len(edges))
	}
	// The single-host canonicaliser must accept the same set.
	if _, err := canonicalCaptureEdges(union); err != nil {
		t.Fatalf("single-host canonicaliser rejected a one-node graph union: %v", err)
	}
}

// Phase 4's proven case: one template, bound independently to two different hosts.
func TestServiceGraphTwoNodesSameTemplate(t *testing.T) {
	a := gNode("ENTRY", "entry.example", ServiceNodeRoleEntry, []ServiceScopeEdge{gEdge("192.0.2.1")}, ServiceNodeActive)
	m := gNode("MEDIA", "media.example", ServiceNodeRoleMedia, []ServiceScopeEdge{gEdge("198.51.100.7")}, ServiceNodeActive)
	if a.TemplateIdentity != m.TemplateIdentity {
		t.Fatal("fixture precondition: nodes must share one template identity")
	}
	if a.StrategyFingerprint == m.StrategyFingerprint {
		t.Fatal("host-bound fingerprints must differ for the same template on two hosts")
	}
	g, err := NewServiceGraph([]ServiceNode{a, m})
	if err != nil {
		t.Fatalf("same template on two hosts must be representable: %v", err)
	}
	union, err := g.UnionEdges()
	if err != nil || len(union) != 2 {
		t.Fatalf("union = %d edges, err = %v; want 2", len(union), err)
	}
}

// Architecture must permit different bindings per node for generality.
func TestServiceGraphTwoNodesDifferentBindings(t *testing.T) {
	a := gNode("ENTRY", "entry.example", ServiceNodeRoleEntry, []ServiceScopeEdge{gEdge("192.0.2.1")}, ServiceNodeActive)
	m := gNode("MEDIA", "media.example", ServiceNodeRoleMedia, []ServiceScopeEdge{gEdge("198.51.100.7")}, ServiceNodeActive)
	m.TemplateIdentity = "tmpl-multisplit"
	if _, err := NewServiceGraph([]ServiceNode{a, m}); err != nil {
		t.Fatalf("different per-node bindings must be representable: %v", err)
	}
}

func TestServiceGraphPerNodeEdgeBound(t *testing.T) {
	edges := make([]ServiceScopeEdge, 0, MaxServiceScopeEdges+1)
	for i := range MaxServiceScopeEdges + 1 {
		edges = append(edges, gEdge(net.IPv4(192, 0, 2, byte(i+1)).String()))
	}
	_, err := NewServiceGraph([]ServiceNode{gNode("ENTRY", "entry.example", ServiceNodeRoleEntry, edges, ServiceNodeActive)})
	if err == nil || !strings.Contains(err.Error(), "SERVICE_GRAPH_NODE_SCOPE_INVALID") {
		t.Fatalf("per-node edge overflow must fail closed, got %v", err)
	}
}

func TestServiceGraphHostOverflow(t *testing.T) {
	nodes := make([]ServiceNode, 0, MaxServiceGraphHosts+1)
	for i := range MaxServiceGraphHosts + 1 {
		host := "h" + string(rune('a'+i)) + ".example"
		nodes = append(nodes, gNode("N"+string(rune('A'+i)), host, ServiceNodeRoleAuxiliary, []ServiceScopeEdge{gEdge("192.0.2.1")}, ServiceNodeActive))
	}
	_, err := NewServiceGraph(nodes)
	if err == nil || !strings.Contains(err.Error(), "SERVICE_GRAPH_TOO_LARGE") {
		t.Fatalf("graph host overflow must fail closed, got %v", err)
	}
}

// Two nodes may never share one hostname: exact-host authority would blur.
func TestServiceGraphDuplicateTargetRejected(t *testing.T) {
	a := gNode("ENTRY", "same.example", ServiceNodeRoleEntry, []ServiceScopeEdge{gEdge("192.0.2.1")}, ServiceNodeActive)
	b := gNode("MEDIA", "same.example", ServiceNodeRoleMedia, []ServiceScopeEdge{gEdge("198.51.100.7")}, ServiceNodeActive)
	if _, err := NewServiceGraph([]ServiceNode{a, b}); err == nil {
		t.Fatal("two nodes sharing one target must be rejected")
	}
}

// A node must own exactly the scope of its own target; cross-node authority is
// the failure this guards.
func TestServiceGraphNodeScopeTargetMustMatch(t *testing.T) {
	n := gNode("ENTRY", "entry.example", ServiceNodeRoleEntry, []ServiceScopeEdge{gEdge("192.0.2.1")}, ServiceNodeActive)
	n.Scope.Target = gTarget("other.example")
	if _, err := NewServiceGraph([]ServiceNode{n}); err == nil {
		t.Fatal("node scope bound to a different target must be rejected")
	}
}

func TestServiceGraphValidationRequiresBinding(t *testing.T) {
	n := gNode("ENTRY", "entry.example", ServiceNodeRoleEntry, []ServiceScopeEdge{gEdge("192.0.2.1")}, ServiceNodeValidated)
	n.Strategy = ""
	if _, err := NewServiceGraph([]ServiceNode{n}); err == nil {
		t.Fatal("a validated node without a strategy binding must be rejected")
	}
}

func TestServiceGraphDiscoveredNodeCarriesNoAuthority(t *testing.T) {
	n := gNode("ENTRY", "entry.example", ServiceNodeRoleEntry, []ServiceScopeEdge{gEdge("192.0.2.1")}, ServiceNodeDiscovered)
	g, err := NewServiceGraph([]ServiceNode{n})
	if err != nil {
		t.Fatalf("discovered node must be structurally valid: %v", err)
	}
	if _, err := g.ActiveCaptureGraph(); err == nil {
		t.Fatal("a DISCOVERED node must never produce an active capture graph")
	}
}

// --- Revalidation -----------------------------------------------------------

func validatedGraph(t *testing.T, nodes ...ServiceNode) ServiceGraph {
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

func TestRevalidationNewEdgeRequiresExperiment(t *testing.T) {
	prior := validatedGraph(t, gNode("ENTRY", "entry.example", ServiceNodeRoleEntry, []ServiceScopeEdge{gEdge("192.0.2.1")}, ServiceNodeValidated))
	current, err := NewServiceGraph([]ServiceNode{gNode("ENTRY", "entry.example", ServiceNodeRoleEntry, []ServiceScopeEdge{gEdge("192.0.2.1"), gEdge("192.0.2.2")}, ServiceNodeValidated)})
	if err != nil {
		t.Fatalf("current: %v", err)
	}
	if got := prior.RevalidateAgainst(current); got.State != GraphRevalidationNewEdge {
		t.Fatalf("new edge state = %q, want NEW_EDGE", got.State)
	}
}

func TestRevalidationNewNodeRequiresExperiment(t *testing.T) {
	prior := validatedGraph(t, gNode("ENTRY", "entry.example", ServiceNodeRoleEntry, []ServiceScopeEdge{gEdge("192.0.2.1")}, ServiceNodeValidated))
	current, err := NewServiceGraph([]ServiceNode{
		gNode("ENTRY", "entry.example", ServiceNodeRoleEntry, []ServiceScopeEdge{gEdge("192.0.2.1")}, ServiceNodeValidated),
		gNode("MEDIA", "media.example", ServiceNodeRoleMedia, []ServiceScopeEdge{gEdge("198.51.100.7")}, ServiceNodeValidated),
	})
	if err != nil {
		t.Fatalf("current: %v", err)
	}
	got := prior.RevalidateAgainst(current)
	if got.State != GraphRevalidationNewNode || got.NodeID != "MEDIA" {
		t.Fatalf("new node revalidation = %+v, want NEW_NODE on MEDIA", got)
	}
}

func TestRevalidationRequiredNodeDisappeared(t *testing.T) {
	prior := validatedGraph(t,
		gNode("ENTRY", "entry.example", ServiceNodeRoleEntry, []ServiceScopeEdge{gEdge("192.0.2.1")}, ServiceNodeValidated),
		gNode("MEDIA", "media.example", ServiceNodeRoleMedia, []ServiceScopeEdge{gEdge("198.51.100.7")}, ServiceNodeValidated),
	)
	current, err := NewServiceGraph([]ServiceNode{gNode("ENTRY", "entry.example", ServiceNodeRoleEntry, []ServiceScopeEdge{gEdge("192.0.2.1")}, ServiceNodeValidated)})
	if err != nil {
		t.Fatalf("current: %v", err)
	}
	got := prior.RevalidateAgainst(current)
	if got.State != GraphRevalidationNodeDisappeared || got.NodeID != "MEDIA" {
		t.Fatalf("disappeared required node = %+v, want NODE_DISAPPEARED on MEDIA", got)
	}
}

func TestRevalidationOptionalNodeDisappearedIsHarmless(t *testing.T) {
	aux := gNode("AUX", "aux.example", ServiceNodeRoleAuxiliary, []ServiceScopeEdge{gEdge("203.0.113.4")}, ServiceNodeValidated)
	aux.Required = false
	prior := validatedGraph(t, gNode("ENTRY", "entry.example", ServiceNodeRoleEntry, []ServiceScopeEdge{gEdge("192.0.2.1")}, ServiceNodeValidated), aux)
	current, err := NewServiceGraph([]ServiceNode{gNode("ENTRY", "entry.example", ServiceNodeRoleEntry, []ServiceScopeEdge{gEdge("192.0.2.1")}, ServiceNodeValidated)})
	if err != nil {
		t.Fatalf("current: %v", err)
	}
	if got := prior.RevalidateAgainst(current); got.State != GraphRevalidationNone {
		t.Fatalf("optional node disappearance state = %q, want NONE", got.State)
	}
}

// A node that returns after having been dropped must never be silently trusted.
func TestRevalidationReappearingNodeNotAutoTrusted(t *testing.T) {
	// The node was ACTIVE before it dropped; it returns merely VALIDATED.
	prior, err := NewServiceGraph([]ServiceNode{gNode("ENTRY", "entry.example", ServiceNodeRoleEntry, []ServiceScopeEdge{gEdge("192.0.2.1")}, ServiceNodeActive)})
	if err != nil {
		t.Fatalf("prior: %v", err)
	}
	current, err := NewServiceGraph([]ServiceNode{gNode("ENTRY", "entry.example", ServiceNodeRoleEntry, []ServiceScopeEdge{gEdge("192.0.2.1")}, ServiceNodeValidated)})
	if err != nil {
		t.Fatalf("current: %v", err)
	}
	got := prior.RevalidateAgainst(current)
	if got.State != GraphRevalidationNodeReappeared {
		t.Fatalf("reappearing node state = %q, want NODE_REAPPEARED_UNVALIDATED", got.State)
	}
}

// The PR59 lesson: a validated graph wider than what is active must not
// authorize the wider set later.
func TestActiveGraphNarrowerThanExperiment(t *testing.T) {
	entry := gNode("ENTRY", "entry.example", ServiceNodeRoleEntry, []ServiceScopeEdge{gEdge("192.0.2.1")}, ServiceNodeActive)
	media := gNode("MEDIA", "media.example", ServiceNodeRoleMedia, []ServiceScopeEdge{gEdge("198.51.100.7")}, ServiceNodeValidated)
	g, err := NewServiceGraph([]ServiceNode{entry, media})
	if err != nil {
		t.Fatalf("graph: %v", err)
	}
	active, err := g.ActiveCaptureGraph()
	if err != nil {
		t.Fatalf("active graph: %v", err)
	}
	if len(active.Nodes) != 1 || active.Nodes[0].ID != "ENTRY" {
		t.Fatalf("active nodes = %d, want only ENTRY", len(active.Nodes))
	}
	edges, err := active.UnionEdges()
	if err != nil {
		t.Fatalf("active union: %v", err)
	}
	if len(edges) != 1 {
		t.Fatalf("active capture edges = %d, want 1; unactivated node must not be captured", len(edges))
	}
}

// --- Persistence and identity ------------------------------------------------

func TestServiceGraphSerializesNoRawEdges(t *testing.T) {
	g, err := NewServiceGraph([]ServiceNode{gNode("ENTRY", "entry.example", ServiceNodeRoleEntry, []ServiceScopeEdge{gEdge("192.0.2.1"), gEdge("192.0.2.2")}, ServiceNodeActive)})
	if err != nil {
		t.Fatalf("graph: %v", err)
	}
	blob, err := json.Marshal(g)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	for _, forbidden := range []string{"192.0.2.1", "192.0.2.2", "198.51.100.7"} {
		if strings.Contains(string(blob), forbidden) {
			t.Fatalf("serialized graph leaked a resolved address %q", forbidden)
		}
	}
}

// The graph identity is logical: changing current DNS must not change it.
func TestServiceGraphFingerprintExcludesEdges(t *testing.T) {
	a, err := NewServiceGraph([]ServiceNode{gNode("ENTRY", "entry.example", ServiceNodeRoleEntry, []ServiceScopeEdge{gEdge("192.0.2.1")}, ServiceNodeActive)})
	if err != nil {
		t.Fatalf("graph a: %v", err)
	}
	b, err := NewServiceGraph([]ServiceNode{gNode("ENTRY", "entry.example", ServiceNodeRoleEntry, []ServiceScopeEdge{gEdge("192.0.2.9")}, ServiceNodeActive)})
	if err != nil {
		t.Fatalf("graph b: %v", err)
	}
	if a.Fingerprint() != b.Fingerprint() {
		t.Fatal("logical graph identity must not depend on current resolved addresses")
	}
	// Adding a node must change it.
	c, err := NewServiceGraph([]ServiceNode{
		gNode("ENTRY", "entry.example", ServiceNodeRoleEntry, []ServiceScopeEdge{gEdge("192.0.2.1")}, ServiceNodeActive),
		gNode("MEDIA", "media.example", ServiceNodeRoleMedia, []ServiceScopeEdge{gEdge("198.51.100.7")}, ServiceNodeActive),
	})
	if err != nil {
		t.Fatalf("graph c: %v", err)
	}
	if a.Fingerprint() == c.Fingerprint() {
		t.Fatal("adding a node must change graph identity")
	}
}

// Template identity must ignore the host binding that changes Fingerprint.
func TestStrategyTemplateIdentityIgnoresHostBinding(t *testing.T) {
	// discord-tcp is the only fixture bound with explicit hosts, which is exactly
	// the case where Fingerprint and TemplateIdentity must disagree.
	base, ok := strategyir.RepresentativeFixtures()["discord-tcp"]
	if !ok {
		t.Fatal("discord-tcp fixture unavailable")
	}
	canonical, err := strategyir.Canonicalize(base)
	if err != nil {
		t.Fatalf("canonicalize base: %v", err)
	}
	other := canonical
	hostScope := canonical.Selector.Scope.Host
	hostScope.Hosts = []string{"different.example"}
	other.Selector.Scope.Host = hostScope

	fpSame, err := strategyir.Fingerprint(canonical)
	if err != nil {
		t.Fatalf("fingerprint base: %v", err)
	}
	fpOther, err := strategyir.Fingerprint(other)
	if err != nil {
		t.Fatalf("fingerprint other: %v", err)
	}
	if fpSame == fpOther {
		t.Fatal("host binding must change the execution fingerprint")
	}
	tidSame, err := strategyir.TemplateIdentity(canonical)
	if err != nil {
		t.Fatalf("template identity base: %v", err)
	}
	tidOther, err := strategyir.TemplateIdentity(other)
	if err != nil {
		t.Fatalf("template identity other: %v", err)
	}
	if tidSame != tidOther {
		t.Fatal("template identity must ignore the host binding")
	}
	// It must still distinguish genuinely different strategies.
	mutated, err := strategyir.Canonicalize(base)
	if err != nil {
		t.Fatalf("canonicalize mutated: %v", err)
	}
	mutated.Safety.Aggressiveness = "HIGH"
	tidMutated, err := strategyir.TemplateIdentity(mutated)
	if err != nil {
		t.Fatalf("template identity mutated: %v", err)
	}
	if tidMutated == tidSame {
		t.Fatal("template identity must distinguish different strategies")
	}
}

// --- Windows multi-section capture -------------------------------------------

func graphSections() []ServiceGraphSection {
	return []ServiceGraphSection{
		{NodeID: "ENTRY", Host: "entry.example", Argv: []string{"--lua-desync=fake", hostlistDomainsArg("entry.example")}},
		{NodeID: "MEDIA", Host: "media.example", Argv: []string{"--lua-desync=fake", hostlistDomainsArg("media.example")}},
	}
}

func twoNodeGraph(t *testing.T) ServiceGraph {
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

func TestRenderWindowsGraphCaptureTwoHostsOneTemplate(t *testing.T) {
	plan, err := RenderWindowsServiceGraphCapture(twoNodeGraph(t), graphSections())
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	if len(plan.NodeOrder) != 2 || plan.NodeOrder[0] != "ENTRY" || plan.NodeOrder[1] != "MEDIA" {
		t.Fatalf("node order = %v, want deterministic ENTRY,MEDIA", plan.NodeOrder)
	}
	// Exactly one section separator between two sections.
	newCount := 0
	for _, a := range plan.SectionArgv {
		if a == "--new" {
			newCount++
		}
	}
	if newCount != 1 {
		t.Fatalf("--new count = %d, want exactly 1", newCount)
	}
	// Both hosts must be present, each exactly once, anchored.
	if strings.Count(strings.Join(plan.SectionArgv, " "), hostlistDomainsArg("entry.example")) != 1 {
		t.Fatal("entry host selector missing or duplicated")
	}
	if strings.Count(strings.Join(plan.SectionArgv, " "), hostlistDomainsArg("media.example")) != 1 {
		t.Fatal("media host selector missing or duplicated")
	}
	// Union filter must contain both exact addresses and TCP/443 only.
	if !strings.Contains(plan.RawFilter, "192.0.2.1") || !strings.Contains(plan.RawFilter, "198.51.100.7") {
		t.Fatalf("union filter missing an exact edge: %s", plan.RawFilter)
	}
	if !strings.Contains(plan.RawFilter, "tcp.DstPort == 443") || !strings.Contains(plan.RawFilter, "tcp.SrcPort == 443") {
		t.Fatalf("union filter must be TCP/443 only: %s", plan.RawFilter)
	}
	for _, forbidden := range []string{"/24", "/16", "0.0.0.0/0", "googlevideo", "*.", "ipset"} {
		if strings.Contains(plan.RawFilter, forbidden) {
			t.Fatalf("union filter contains wildcard or range authority %q", forbidden)
		}
	}
	if len(plan.CaptureArgv) != 3 {
		t.Fatalf("capture argv = %v, want exactly three process-global arguments", plan.CaptureArgv)
	}
}

// A section bound to the wrong host must be rejected: this is what stops one
// node's authority from being reused for another.
func TestRenderWindowsGraphCaptureRejectsCrossHostSection(t *testing.T) {
	sections := graphSections()
	sections[1].Host = "entry.example"
	if _, err := RenderWindowsServiceGraphCapture(twoNodeGraph(t), sections); err == nil {
		t.Fatal("section bound to another node's host must be rejected")
	}
}

// A wildcard or list-based selector must be rejected outright.
func TestRenderWindowsGraphCaptureRejectsWildcardAuthority(t *testing.T) {
	for _, bad := range [][]string{
		{"--lua-desync=fake", "--hostlist-domains=^*.example"},
		{"--lua-desync=fake", "--hostlist=all"},
		{"--lua-desync=fake", "--ipset=wide"},
		{"--lua-desync=fake", "--wf-tcp-out=443"},
		{"--lua-desync=fake"},
	} {
		sections := graphSections()
		sections[1].Argv = bad
		if _, err := RenderWindowsServiceGraphCapture(twoNodeGraph(t), sections); err == nil {
			t.Fatalf("section argv %v must be rejected", bad)
		}
	}
}

// Only ACTIVE nodes may be rendered; a validated-but-inactive node is omitted.
func TestRenderWindowsGraphCaptureOmitsInactiveNodes(t *testing.T) {
	g, err := NewServiceGraph([]ServiceNode{
		gNode("ENTRY", "entry.example", ServiceNodeRoleEntry, []ServiceScopeEdge{gEdge("192.0.2.1")}, ServiceNodeActive),
		gNode("MEDIA", "media.example", ServiceNodeRoleMedia, []ServiceScopeEdge{gEdge("198.51.100.7")}, ServiceNodeValidated),
	})
	if err != nil {
		t.Fatalf("graph: %v", err)
	}
	plan, err := RenderWindowsServiceGraphCapture(g, graphSections())
	if err == nil {
		t.Fatalf("rendering a section for a non-active node must fail, got plan %+v", plan)
	}
}

// A target that yields no hostname must be rejected: it would render an empty
// host selector and collapse exact-host authority.
func TestServiceGraphRejectsTargetWithoutHostname(t *testing.T) {
	for _, raw := range []string{"entry.example", "https://", "/generate_204", "https://:443/x"} {
		n := gNode("ENTRY", "entry.example", ServiceNodeRoleEntry, []ServiceScopeEdge{gEdge("192.0.2.1")}, ServiceNodeActive)
		n.Target = Target{URL: raw}
		n.Scope.Target = n.Target
		if _, err := NewServiceGraph([]ServiceNode{n}); err == nil {
			t.Fatalf("target %q must be rejected", raw)
		}
	}
}

// The graph canonicaliser duplicates the single-host one to differ only in the
// cap. This guards them from drifting apart for inputs inside the single-host cap.
func TestCanonicalGraphEdgesAgreesWithSingleHost(t *testing.T) {
	inputs := [][]ServiceScopeEdge{
		{gEdge("192.0.2.1")},
		{gEdge("192.0.2.2"), gEdge("192.0.2.1"), gEdge("192.0.2.1")},
		{gEdge("2001:db8::1"), gEdge("192.0.2.1")},
	}
	for i, in := range inputs {
		single, singleErr := canonicalCaptureEdges(in)
		graph, graphErr := canonicalGraphEdges(in)
		if (singleErr == nil) != (graphErr == nil) {
			t.Fatalf("input %d: error mismatch single=%v graph=%v", i, singleErr, graphErr)
		}
		if singleErr != nil {
			continue
		}
		if len(single) != len(graph) {
			t.Fatalf("input %d: length mismatch single=%d graph=%d", i, len(single), len(graph))
		}
		for j := range single {
			if single[j].Family != graph[j].Family || !single[j].IP.Equal(graph[j].IP) {
				t.Fatalf("input %d index %d: edge mismatch", i, j)
			}
		}
	}
}
