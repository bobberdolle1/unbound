package autotunevnext

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"net"
	"net/netip"
	"net/url"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

// Bounded validated service graph.
//
// The single-host service-scope model binds one normalized target to one bounded
// exact DNS scope and one validated strategy. Phase 4 of the research programme
// proved that this is not sufficient for a service: binding a proven strategy to
// an ENTRY host does not affect MEDIA delivery, because strategy authority is
// host-scoped. Each active service host therefore needs its own exact binding,
// each with independent fresh DNS authority.
//
// This file adds that container. It deliberately does NOT implement discovery:
// nodes are supplied explicitly by trusted product code or test fixtures. A role,
// a hostname pattern, a prior result or history can never create packet
// authority here, and no node can ever read another node's edges or validation.

// MaxServiceGraphHosts bounds how many explicit hosts one graph may carry. Four
// covers the shape the research actually observed (an ENTRY host, a possible API
// dependency, and one or two dynamic media hosts) with room to spare, while
// keeping capture, budget and review bounded. Overflow is a failure, never a
// silent truncation.
const MaxServiceGraphHosts = 4

// MaxServiceGraphEdges bounds the union of all per-node exact edges used for one
// capture. It is derived, not chosen: every node is independently bounded by
// MaxServiceScopeEdges.
const MaxServiceGraphEdges = MaxServiceGraphHosts * MaxServiceScopeEdges

// ServiceNodeRole is a logical label. A role creates no packet authority and
// grants no validation: it exists so product code and evidence can reason about
// a node, never so a node can bypass fresh resolution and experiment.
type ServiceNodeRole string

const (
	ServiceNodeRoleEntry     ServiceNodeRole = "ENTRY"
	ServiceNodeRoleAPI       ServiceNodeRole = "API"
	ServiceNodeRoleMedia     ServiceNodeRole = "MEDIA"
	ServiceNodeRoleAuxiliary ServiceNodeRole = "AUXILIARY"
)

// ServiceNodeValidationState tracks what a node has actually proven.
//
// DISCOVERED means a caller supplied the node. It carries no authority at all.
// VALIDATED means this exact target passed a current controlled experiment.
// ACTIVE means a validated node is currently bound in an accepted capture.
// NEEDS_REVALIDATION is a fail-closed state, never a softer success.
type ServiceNodeValidationState string

const (
	ServiceNodeDiscovered        ServiceNodeValidationState = "DISCOVERED"
	ServiceNodeEligible          ServiceNodeValidationState = "ELIGIBLE"
	ServiceNodeValidated         ServiceNodeValidationState = "VALIDATED"
	ServiceNodeActive            ServiceNodeValidationState = "ACTIVE"
	ServiceNodeNeedsRevalidation ServiceNodeValidationState = "NEEDS_REVALIDATION"
)

var (
	ErrServiceGraphEmpty     = errors.New("SERVICE_GRAPH_EMPTY")
	ErrServiceGraphTooLarge  = errors.New("SERVICE_GRAPH_TOO_LARGE")
	ErrServiceGraphNodeCount = errors.New("SERVICE_GRAPH_NODE_COUNT_INVALID")
	ErrServiceGraphNodeID    = errors.New("SERVICE_GRAPH_NODE_ID_INVALID")
	ErrServiceGraphDuplicate = errors.New("SERVICE_GRAPH_NODE_DUPLICATE")
	ErrServiceGraphTarget    = errors.New("SERVICE_GRAPH_NODE_TARGET_INVALID")
	ErrServiceGraphScope     = errors.New("SERVICE_GRAPH_NODE_SCOPE_INVALID")
	ErrServiceGraphBinding   = errors.New("SERVICE_GRAPH_NODE_BINDING_INVALID")
	ErrServiceGraphRole      = errors.New("SERVICE_GRAPH_NODE_ROLE_INVALID")
	ErrServiceGraphState     = errors.New("SERVICE_GRAPH_NODE_STATE_INVALID")
	ErrServiceGraphNotActive = errors.New("SERVICE_GRAPH_NODE_NOT_ACTIVE")
	ErrServiceGraphCrossNode = errors.New("SERVICE_GRAPH_CROSS_NODE_AUTHORITY")
)

// ServiceNode is one explicit host in a bounded graph.
//
// Scope is current execution evidence and is never serialized: the raw remote
// addresses are deliberately outside the durable representation, exactly as for
// a single-host snapshot. StrategyID and StrategyTemplateIdentity are logical
// identities. StrategyFingerprint stays host-bound on purpose and is only ever
// compared against the same node.
type ServiceNode struct {
	ID       string          `json:"id"`
	Role     ServiceNodeRole `json:"role"`
	Required bool            `json:"required"`
	Target   Target          `json:"target"`
	Strategy string          `json:"strategy,omitempty"`
	// TemplateIdentity is the host-independent strategy identity. Two nodes may
	// share one template identity; that is the Phase 4 proven case, and it does
	// not imply the nodes are interchangeable.
	TemplateIdentity    string                     `json:"template_identity,omitempty"`
	StrategyFingerprint string                     `json:"strategy_fingerprint,omitempty"`
	Validation          ServiceNodeValidationState `json:"validation"`
	Scope               ServiceScopeSnapshot       `json:"-"`
}

// Hostname returns the node's normalized logical host. It is used for equality
// and for capture rendering only; it never confers authority by itself.
func (n ServiceNode) Hostname() string {
	parsed, err := url.Parse(n.Target.URL)
	if err != nil {
		return ""
	}
	return strings.ToLower(parsed.Hostname())
}

// ServiceGraph is a small explicit set of host nodes.
type ServiceGraph struct {
	Nodes []ServiceNode `json:"nodes"`
}

var serviceNodeIDPattern = regexp.MustCompile(`^[A-Z][A-Z0-9_]{0,63}$`)

// NewServiceGraph validates and returns a bounded graph. Every structural
// invariant is checked here rather than at use sites, so an invalid graph
// cannot be constructed in the first place.
func NewServiceGraph(nodes []ServiceNode) (ServiceGraph, error) {
	g := ServiceGraph{Nodes: nodes}
	if err := g.Validate(); err != nil {
		return ServiceGraph{}, err
	}
	return g, nil
}

// Validate enforces every structural invariant of a bounded graph.
func (g ServiceGraph) Validate() error {
	if len(g.Nodes) == 0 {
		return ErrServiceGraphEmpty
	}
	if len(g.Nodes) > MaxServiceGraphHosts {
		return fmt.Errorf("%w: %d > %d", ErrServiceGraphTooLarge, len(g.Nodes), MaxServiceGraphHosts)
	}
	seenID := make(map[string]struct{}, len(g.Nodes))
	seenTarget := make(map[string]string, len(g.Nodes))
	for i := range g.Nodes {
		n := g.Nodes[i]
		if !serviceNodeIDPattern.MatchString(n.ID) {
			return fmt.Errorf("%w: %q", ErrServiceGraphNodeID, n.ID)
		}
		if _, dup := seenID[n.ID]; dup {
			return fmt.Errorf("%w: id %q", ErrServiceGraphDuplicate, n.ID)
		}
		seenID[n.ID] = struct{}{}
		switch n.Role {
		case ServiceNodeRoleEntry, ServiceNodeRoleAPI, ServiceNodeRoleMedia, ServiceNodeRoleAuxiliary:
		default:
			return fmt.Errorf("%w: %q", ErrServiceGraphRole, n.Role)
		}
		if strings.TrimSpace(n.Target.URL) == "" {
			return fmt.Errorf("%w: node %q has no target", ErrServiceGraphTarget, n.ID)
		}
		// A target that parses to no hostname would render an empty host selector and
		// silently collapse exact-host authority, so it is rejected here.
		if n.Hostname() == "" {
			return fmt.Errorf("%w: node %q target has no hostname", ErrServiceGraphTarget, n.ID)
		}
		if !sameTargetIdentity(n.Target, n.Scope.Target) {
			return fmt.Errorf("%w: node %q scope target does not match node target", ErrServiceGraphScope, n.ID)
		}
		if len(n.Scope.Edges) == 0 || len(n.Scope.Edges) > MaxServiceScopeEdges {
			return fmt.Errorf("%w: node %q edge count %d", ErrServiceGraphScope, n.ID, len(n.Scope.Edges))
		}
		// A second node claiming the same normalized target would mean two nodes
		// silently sharing one hostname, which would blur exact-host authority.
		key := n.Hostname()
		if other, dup := seenTarget[key]; dup {
			return fmt.Errorf("%w: nodes %q and %q share target %q", ErrServiceGraphTarget, other, n.ID, key)
		}
		seenTarget[key] = n.ID
		switch n.Validation {
		case ServiceNodeDiscovered, ServiceNodeEligible, ServiceNodeValidated, ServiceNodeActive, ServiceNodeNeedsRevalidation:
		default:
			return fmt.Errorf("%w: node %q state %q", ErrServiceGraphState, n.ID, n.Validation)
		}
		if n.Validation == ServiceNodeValidated || n.Validation == ServiceNodeActive {
			if strings.TrimSpace(n.Strategy) == "" || strings.TrimSpace(n.StrategyFingerprint) == "" {
				return fmt.Errorf("%w: node %q claims validation without a strategy binding", ErrServiceGraphBinding, n.ID)
			}
		}
	}
	return nil
}

// Node returns the node with the given ID.
func (g ServiceGraph) Node(id string) (ServiceNode, bool) {
	for _, n := range g.Nodes {
		if n.ID == id {
			return n, true
		}
	}
	return ServiceNode{}, false
}

// ActiveNodes returns only nodes that are currently ACTIVE. Any capture or grant
// derived from a graph must be built from exactly this set, so a validated node
// that was never activated can never be treated as authorized later.
func (g ServiceGraph) ActiveNodes() []ServiceNode {
	out := make([]ServiceNode, 0, len(g.Nodes))
	for _, n := range g.Nodes {
		if n.Validation == ServiceNodeActive {
			out = append(out, n)
		}
	}
	return out
}

// UnionEdges returns the deduplicated exact edges of every node. It is
// ephemeral execution input for capture rendering and is never persisted.
func (g ServiceGraph) UnionEdges() ([]ServiceScopeEdge, error) {
	seen := make(map[string]ServiceScopeEdge)
	for _, n := range g.Nodes {
		for _, e := range n.Scope.Edges {
			key := string(e.Family) + "|" + e.IP.String()
			if _, dup := seen[key]; !dup {
				seen[key] = e
			}
		}
	}
	if len(seen) == 0 {
		return nil, ErrServiceGraphEmpty
	}
	if len(seen) > MaxServiceGraphEdges {
		return nil, fmt.Errorf("%w: %d > %d", ErrServiceGraphTooLarge, len(seen), MaxServiceGraphEdges)
	}
	out := make([]ServiceScopeEdge, 0, len(seen))
	for _, e := range seen {
		out = append(out, e)
	}
	// The single-host canonicaliser deliberately caps the whole set at
	// MaxServiceScopeEdges. A graph union is bounded by MaxServiceGraphEdges
	// instead, so the per-edge validation is shared but the cap is not.
	return canonicalGraphEdges(out)
}

// canonicalGraphEdges applies the same per-edge validation as the single-host
// canonicaliser but bounds the set by the graph cap. It is the only place a
// multi-host edge union may be built.
func canonicalGraphEdges(edges []ServiceScopeEdge) ([]ServiceScopeEdge, error) {
	if len(edges) == 0 {
		return nil, ErrServiceGraphEmpty
	}
	if len(edges) > MaxServiceGraphEdges {
		return nil, fmt.Errorf("%w: %d > %d", ErrServiceGraphTooLarge, len(edges), MaxServiceGraphEdges)
	}
	seen := make(map[string]struct{}, len(edges))
	result := make([]ServiceScopeEdge, 0, len(edges))
	for _, edge := range edges {
		if edge.IP == nil || edge.Family == "" || familyForIP(edge.IP) != edge.Family {
			return nil, ErrServiceGraphScope
		}
		ip := append(net.IP(nil), edge.IP...)
		if edge.Family == "ipv4" {
			ip = ip.To4()
		}
		address, ok := netip.AddrFromSlice(ip)
		if !ok || address.IsUnspecified() || address.Is4In6() {
			return nil, ErrServiceGraphScope
		}
		key := string(edge.Family) + "|" + address.String()
		if _, exists := seen[key]; exists {
			continue
		}
		seen[key] = struct{}{}
		result = append(result, ServiceScopeEdge{IP: ip, Family: edge.Family})
	}
	if len(result) == 0 {
		return nil, ErrServiceGraphScope
	}
	sort.Slice(result, func(i, j int) bool {
		if result[i].Family != result[j].Family {
			return result[i].Family < result[j].Family
		}
		return netip.MustParseAddr(result[i].IP.String()).Less(netip.MustParseAddr(result[j].IP.String()))
	})
	return result, nil
}

// Fingerprint is the durable logical identity of a graph. It covers node
// identities, roles, requiredness, validation states and strategy/template
// identities, and deliberately excludes resolved remote addresses and the
// host-bound strategy fingerprint. Two graphs over the same logical service are
// equal even when their current DNS answers differ; that difference is handled
// by revalidation, not by identity.
func (g ServiceGraph) Fingerprint() string {
	hash := sha256.New()
	nodes := append([]ServiceNode(nil), g.Nodes...)
	sort.Slice(nodes, func(i, j int) bool { return nodes[i].ID < nodes[j].ID })
	for _, n := range nodes {
		_, _ = hash.Write([]byte(n.ID))
		_, _ = hash.Write([]byte{0})
		_, _ = hash.Write([]byte(n.Role))
		_, _ = hash.Write([]byte{0})
		_, _ = hash.Write([]byte(strconv.FormatBool(n.Required)))
		_, _ = hash.Write([]byte{0})
		_, _ = hash.Write([]byte(strings.ToLower(n.Target.URL)))
		_, _ = hash.Write([]byte{0})
		_, _ = hash.Write([]byte(n.Strategy))
		_, _ = hash.Write([]byte{0})
		_, _ = hash.Write([]byte(n.TemplateIdentity))
		_, _ = hash.Write([]byte{0})
		_, _ = hash.Write([]byte{0})
		_, _ = hash.Write([]byte(string(n.Validation)))
		_, _ = hash.Write([]byte{0})
	}
	return hex.EncodeToString(hash.Sum(nil))
}

// LogNodes returns the durable, privacy-safe projection of a graph: logical node
// identities, roles, validation state and strategy identity. It carries no
// resolved address and no signed material, so it is safe for managed status.
func (g ServiceGraph) LogNodes() []map[string]any {
	nodes := append([]ServiceNode(nil), g.Nodes...)
	sort.Slice(nodes, func(i, j int) bool { return nodes[i].ID < nodes[j].ID })
	out := make([]map[string]any, 0, len(nodes))
	for _, n := range nodes {
		out = append(out, map[string]any{
			"node_id":     n.ID,
			"role":        string(n.Role),
			"required":    n.Required,
			"validation":  string(n.Validation),
			"strategy":    n.Strategy,
			"template_id": n.TemplateIdentity,
		})
	}
	return out
}

// GraphRevalidationState names why a current graph can no longer be served by a
// previously validated graph.
type GraphRevalidationState string

const (
	GraphRevalidationNone            GraphRevalidationState = "NONE"
	GraphRevalidationNewEdge         GraphRevalidationState = "NEW_EDGE"
	GraphRevalidationNewNode         GraphRevalidationState = "NEW_NODE"
	GraphRevalidationNodeDisappeared GraphRevalidationState = "NODE_DISAPPEARED"
	GraphRevalidationNodeReappeared  GraphRevalidationState = "NODE_REAPPEARED_UNVALIDATED"
	GraphRevalidationTargetChanged   GraphRevalidationState = "TARGET_CHANGED"
	GraphRevalidationNodeNotActive   GraphRevalidationState = "NODE_NOT_ACTIVE"
	GraphRevalidationStrategyChanged GraphRevalidationState = "STRATEGY_CHANGED"
)

// GraphRevalidation is the single most important fail-closed decision in this
// file: it decides whether a previously validated graph may still serve the
// current network state.
type GraphRevalidation struct {
	State    GraphRevalidationState `json:"state"`
	NodeID   string                 `json:"node_id,omitempty"`
	Required bool                   `json:"required"`
}

// RevalidateAgainst reports whether the previously validated graph can still
// authorize the current graph.
//
// The asymmetry mirrors IsSubsetOf and is deliberate. Disappearing edges are
// harmless. Anything that could widen what is captured, or that appears without
// ever having been validated, requires a fresh experiment. A node that comes
// back after having been dropped is never silently trusted.
func (validated ServiceGraph) RevalidateAgainst(current ServiceGraph) GraphRevalidation {
	if len(validated.Nodes) == 0 {
		return GraphRevalidation{State: GraphRevalidationNewNode, Required: true}
	}
	validatedByID := make(map[string]ServiceNode, len(validated.Nodes))
	for _, n := range validated.Nodes {
		validatedByID[n.ID] = n
	}
	currentIDs := make(map[string]struct{}, len(current.Nodes))

	for _, n := range current.Nodes {
		currentIDs[n.ID] = struct{}{}
		prior, known := validatedByID[n.ID]
		if !known {
			// A host that appears now was never validated. This is the media host
			// case from Phase 4: a new edge on the service must be proven.
			return GraphRevalidation{State: GraphRevalidationNewNode, NodeID: n.ID, Required: n.Required}
		}
		if !sameTargetIdentity(prior.Target, n.Target) {
			return GraphRevalidation{State: GraphRevalidationTargetChanged, NodeID: n.ID, Required: n.Required}
		}
		if prior.Strategy != n.Strategy || prior.TemplateIdentity != n.TemplateIdentity {
			return GraphRevalidation{State: GraphRevalidationStrategyChanged, NodeID: n.ID, Required: n.Required}
		}
		if n.Validation != ServiceNodeActive && n.Validation != ServiceNodeValidated {
			// A node that is present but no longer claiming validation cannot be
			// served from the prior grant.
			return GraphRevalidation{State: GraphRevalidationNodeNotActive, NodeID: n.ID, Required: n.Required}
		}
		if !n.Scope.IsSubsetOf(prior.Scope) {
			// A new answer on an existing host widens capture and must be proven.
			return GraphRevalidation{State: GraphRevalidationNewEdge, NodeID: n.ID, Required: n.Required}
		}
	}
	for _, n := range validated.Nodes {
		if _, present := currentIDs[n.ID]; present {
			continue
		}
		if n.Required {
			// A required node vanished. Report it; never substitute another host.
			return GraphRevalidation{State: GraphRevalidationNodeDisappeared, NodeID: n.ID, Required: true}
		}
	}
	for _, n := range current.Nodes {
		if n.Validation == ServiceNodeActive {
			continue
		}
		prior, known := validatedByID[n.ID]
		if known && prior.Validation == ServiceNodeActive {
			return GraphRevalidation{State: GraphRevalidationNodeReappeared, NodeID: n.ID, Required: n.Required}
		}
	}
	return GraphRevalidation{State: GraphRevalidationNone}
}

// ActiveCaptureGraph returns the exact graph that a capture may be built from:
// only nodes that are currently ACTIVE. If a caller holds a validated graph that
// is wider than what is actually active, the extra nodes are simply absent here
// and therefore are not authorized by anything derived from this graph.
func (g ServiceGraph) ActiveCaptureGraph() (ServiceGraph, error) {
	active := g.ActiveNodes()
	if len(active) == 0 {
		return ServiceGraph{}, ErrServiceGraphNotActive
	}
	out := ServiceGraph{Nodes: active}
	if err := out.Validate(); err != nil {
		return ServiceGraph{}, err
	}
	return out, nil
}

// ServiceGraphSnapshot is the evidence-bearing record of one bounded graph
// evaluation. It is a reduced projection: statuses, counts and logical
// identities only.
type ServiceGraphSnapshot struct {
	Fingerprint  string            `json:"fingerprint"`
	NodeCount    int               `json:"node_count"`
	Nodes        []map[string]any  `json:"nodes"`
	Revalidation GraphRevalidation `json:"revalidation"`
	EdgeCount    int               `json:"edge_count"`
	EdgeCap      int               `json:"edge_cap"`
}

// NewServiceGraphSnapshot builds the reduced, persistable projection.
func NewServiceGraphSnapshot(g ServiceGraph, revalidation GraphRevalidation) (ServiceGraphSnapshot, error) {
	active, err := g.ActiveCaptureGraph()
	if err != nil {
		return ServiceGraphSnapshot{}, err
	}
	edges, err := active.UnionEdges()
	if err != nil {
		return ServiceGraphSnapshot{}, err
	}
	return ServiceGraphSnapshot{
		Fingerprint:  g.Fingerprint(),
		NodeCount:    len(active.Nodes),
		Nodes:        active.LogNodes(),
		Revalidation: revalidation,
		EdgeCount:    len(edges),
		EdgeCap:      MaxServiceGraphEdges,
	}, nil
}
