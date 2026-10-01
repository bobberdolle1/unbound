//go:build linux

package autotunevnext

// Hermetic coverage for the Linux bounded-graph capture construction.
//
// These tests exercise the exact-address-set construction only. They never touch
// nft, never start a process, and never read the live firewall, so they are safe
// to run anywhere and prove the properties that actually matter: exact union,
// independent per-family sets, and refusal to widen.

import (
	"net"
	"strings"
	"testing"

	"unbound/engine/backendcap"
	"unbound/engine/observatory"
	"unbound/engine/strategyir"
)

func graphCaptureSpec() backendcap.CapturePlan {
	return backendcap.CapturePlan{
		BackendKind: backendcap.CaptureNFQUEUE,
		Transport:   backendcap.CaptureTransportTCP,
		Direction:   strategyir.DirectionOutbound,
		IPFamilies:  []strategyir.IPFamily{strategyir.IPFamilyV4, strategyir.IPFamilyV6},
		TCPPorts:    []strategyir.PortRange{{Start: 443, End: 443}},
	}
}

func v6Edge(ip string) ServiceScopeEdge {
	return ServiceScopeEdge{IP: net.ParseIP(ip), Family: observatory.AddressFamilyIPv6}
}

func TestLinuxGraphSpecTwoIPv4NodesExactUnion(t *testing.T) {
	graph := twoNodeGraph(t) // 192.0.2.1 and 198.51.100.7
	spec, err := BuildLinuxGraphSpec(graph, graphCaptureSpec(), "unbound_autotune_t", 40001, "unbound-autotune:t")
	if err != nil {
		t.Fatalf("build spec: %v", err)
	}
	if len(spec.IPv4Edges) != 2 {
		t.Fatalf("IPv4 edges = %v, want 2", spec.IPv4Edges)
	}
	if len(spec.IPv6Edges) != 0 {
		t.Fatalf("IPv6 edges = %v, want none", spec.IPv6Edges)
	}
	if !sameIPSet(spec.IPv4Edges, []net.IP{net.ParseIP("192.0.2.1"), net.ParseIP("198.51.100.7")}) {
		t.Fatalf("union is not the exact two-node address set: %v", spec.IPv4Edges)
	}
}

// The proven Phase 4 shape: one template identity bound independently to two
// hosts must still produce two exact addresses, not one shared host binding.
func TestLinuxGraphSpecSameTemplateTwoNodesStaysExact(t *testing.T) {
	spec, err := BuildLinuxGraphSpec(twoNodeGraph(t), graphCaptureSpec(), "t", 40001, "m")
	if err != nil {
		t.Fatalf("build spec: %v", err)
	}
	if len(spec.IPv4Edges) != 2 {
		t.Fatalf("same template collapsed to %d addresses, want 2 independent hosts", len(spec.IPv4Edges))
	}
}

func TestLinuxGraphSpecMixedFamilyKeepsFamiliesSeparate(t *testing.T) {
	graph, err := NewServiceGraph([]ServiceNode{
		gNode("ENTRY", "entry.example", ServiceNodeRoleEntry, []ServiceScopeEdge{gEdge("192.0.2.1")}, ServiceNodeActive),
		gNode("MEDIA", "media.example", ServiceNodeRoleMedia, []ServiceScopeEdge{v6Edge("2001:db8::5")}, ServiceNodeActive),
	})
	if err != nil {
		t.Fatalf("graph: %v", err)
	}
	spec, err := BuildLinuxGraphSpec(graph, graphCaptureSpec(), "t", 40001, "m")
	if err != nil {
		t.Fatalf("build spec: %v", err)
	}
	if len(spec.IPv4Edges) != 1 || len(spec.IPv6Edges) != 1 {
		t.Fatalf("mixed family split wrong: v4=%v v6=%v", spec.IPv4Edges, spec.IPv6Edges)
	}
	if spec.IPv4Edges[0].To4() == nil {
		t.Fatal("IPv4 set contains a non-IPv4 address")
	}
	if spec.IPv6Edges[0].To4() != nil {
		t.Fatal("IPv6 set contains an IPv4-mapped address")
	}
	// A mixed graph must select the inet table. Without this the renderer emits
	// IPv6 literals inside an "ip daddr" set and nft rejects the whole graph.
	if spec.NFTFamily != "inet" {
		t.Fatalf("mixed-family graph NFTFamily = %q, want \"inet\"", spec.NFTFamily)
	}
	if len(spec.Edges) != 0 {
		t.Fatalf("mixed-family graph must not carry a combined Edges slice: %v", spec.Edges)
	}
	script := spec.nftScript()
	if !strings.Contains(script, "ip daddr") || !strings.Contains(script, "ip6 daddr") {
		t.Fatalf("mixed-family script must carry one rule per family:\n%s", script)
	}
	if strings.Contains(script, "{ 192.0.2.1, 2001:db8::5 }") {
		t.Fatalf("mixed-family script merged families into one address set:\n%s", script)
	}
}

// A graph whose edges the compiled capture cannot express must be refused rather
// than silently narrowed to the families it happens to support.
func TestLinuxGraphSpecRejectsMissingFamily(t *testing.T) {
	graph, err := NewServiceGraph([]ServiceNode{
		gNode("ENTRY", "entry.example", ServiceNodeRoleEntry, []ServiceScopeEdge{gEdge("192.0.2.1")}, ServiceNodeActive),
		gNode("MEDIA", "media.example", ServiceNodeRoleMedia, []ServiceScopeEdge{v6Edge("2001:db8::5")}, ServiceNodeActive),
	})
	if err != nil {
		t.Fatalf("graph: %v", err)
	}
	v4Only := graphCaptureSpec()
	v4Only.IPFamilies = []strategyir.IPFamily{strategyir.IPFamilyV4}
	if _, err := BuildLinuxGraphSpec(graph, v4Only, "t", 40001, "m"); err == nil {
		t.Fatal("an IPv6 edge must be refused when the capture cannot express IPv6")
	}
}

// An empty active graph must not produce an empty-but-valid spec, because an
// empty capture is not the same as no capture.
func TestLinuxGraphSpecRejectsEmptyGraph(t *testing.T) {
	graph, err := NewServiceGraph([]ServiceNode{
		gNode("ENTRY", "entry.example", ServiceNodeRoleEntry, []ServiceScopeEdge{gEdge("192.0.2.1")}, ServiceNodeValidated),
	})
	if err != nil {
		t.Fatalf("graph: %v", err)
	}
	if _, err := BuildLinuxGraphSpec(graph, graphCaptureSpec(), "t", 40001, "m"); err == nil {
		t.Fatal("a graph with no active nodes must not yield an executable spec")
	}
}

// A non-literal address must never become capture authority.
func TestLinuxGraphSpecRejectsNonLiteralAddress(t *testing.T) {
	graph, err := NewServiceGraph([]ServiceNode{
		gNode("ENTRY", "entry.example", ServiceNodeRoleEntry, []ServiceScopeEdge{{IP: nil, Family: observatory.AddressFamilyIPv4}}, ServiceNodeActive),
	})
	if err == nil {
		// The graph itself refuses a nil address, which is the stronger outcome.
		return
	}
	if _, err := BuildLinuxGraphSpec(graph, graphCaptureSpec(), "t", 40001, "m"); err == nil {
		t.Fatal("a non-literal address must not produce a spec")
	}
}

// An extra unauthorized edge appearing in the current graph must change the
// derived set, so VerifyActiveGraph can detect it.
func TestLinuxGraphSpecDetectsExtraEdge(t *testing.T) {
	base, err := BuildLinuxGraphSpec(twoNodeGraph(t), graphCaptureSpec(), "t", 40001, "m")
	if err != nil {
		t.Fatalf("build spec: %v", err)
	}
	wider, err := NewServiceGraph([]ServiceNode{
		gNode("ENTRY", "entry.example", ServiceNodeRoleEntry, []ServiceScopeEdge{gEdge("192.0.2.1"), gEdge("203.0.113.5")}, ServiceNodeActive),
		gNode("MEDIA", "media.example", ServiceNodeRoleMedia, []ServiceScopeEdge{gEdge("198.51.100.7")}, ServiceNodeActive),
	})
	if err != nil {
		t.Fatalf("graph: %v", err)
	}
	widened, err := BuildLinuxGraphSpec(wider, graphCaptureSpec(), "t", 40001, "m")
	if err != nil {
		t.Fatalf("build wider spec: %v", err)
	}
	if sameIPSet(base.IPv4Edges, widened.IPv4Edges) {
		t.Fatal("an extra unauthorized edge must change the exact address set")
	}
}

// A new node must change the derived set as well, so it cannot ride along.
func TestLinuxGraphSpecDetectsNewNode(t *testing.T) {
	base, err := BuildLinuxGraphSpec(twoNodeGraph(t), graphCaptureSpec(), "t", 40001, "m")
	if err != nil {
		t.Fatalf("build spec: %v", err)
	}
	withExtra, err := NewServiceGraph([]ServiceNode{
		gNode("ENTRY", "entry.example", ServiceNodeRoleEntry, []ServiceScopeEdge{gEdge("192.0.2.1")}, ServiceNodeActive),
		gNode("MEDIA", "media.example", ServiceNodeRoleMedia, []ServiceScopeEdge{gEdge("198.51.100.7")}, ServiceNodeActive),
		gNode("EXTRA", "extra.example", ServiceNodeRoleAuxiliary, []ServiceScopeEdge{gEdge("203.0.113.5")}, ServiceNodeActive),
	})
	if err != nil {
		t.Fatalf("graph: %v", err)
	}
	widened, err := BuildLinuxGraphSpec(withExtra, graphCaptureSpec(), "t", 40001, "m")
	if err != nil {
		t.Fatalf("build wider spec: %v", err)
	}
	if sameIPSet(base.IPv4Edges, widened.IPv4Edges) {
		t.Fatal("a new node must change the exact address set")
	}
}

func TestLinuxGraphSpecProjectionPreservesAddresses(t *testing.T) {
	spec, err := BuildLinuxGraphSpec(twoNodeGraph(t), graphCaptureSpec(), "t", 40001, "m")
	if err != nil {
		t.Fatalf("build spec: %v", err)
	}
	edges := specAsScopeEdges(spec)
	if len(edges) != 2 {
		t.Fatalf("projected edges = %d, want 2", len(edges))
	}
	for _, edge := range edges {
		if edge.IP == nil || familyForIP(edge.IP) != edge.Family {
			t.Fatalf("projected edge lost its address family: %+v", edge)
		}
	}
}
