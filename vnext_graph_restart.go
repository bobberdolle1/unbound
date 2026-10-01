package main

// Bounded service-graph restart semantics.
//
// RESTART MUST NOT immediately replay prior capture. The only path from a saved
// graph intent to an active capture is:
//
//	load logical intent -> validate schema/catalog/backend identity
//	-> fresh resolve EVERY node -> build fresh current graph
//	-> compare against persisted logical authority -> required revalidation
//	-> only then may managed activation be allowed
//
// No persisted DNS edge is ever reused, because the durable intent physically
// cannot carry one. Every address in a restart comes from this run's fresh
// resolution.

import (
	"fmt"
	"net"
	"time"

	"unbound/engine/autotunevnext"
	"unbound/engine/observatory"
)

// GraphRestartState is the product-facing restart state.
type GraphRestartState string

const (
	// GraphRestartNone means there is no saved graph intent.
	GraphRestartNone GraphRestartState = "NONE"
	// GraphRestartRevalidationPending means a saved intent exists and is
	// structurally valid, but this run has not yet completed fresh validation.
	// ACTIVE must never be claimed while this is the state.
	GraphRestartRevalidationPending GraphRestartState = "SAVED_GRAPH_REVALIDATION_PENDING"
	// GraphRestartReady means fresh resolution matched the persisted logical
	// authority and current revalidation was satisfied.
	GraphRestartReady GraphRestartState = "READY"
	// GraphRestartRejected means the saved intent cannot be honoured. This is
	// fail-closed: the caller returns to direct state.
	GraphRestartRejected GraphRestartState = "REJECTED"
)

// GraphRestartPlan is the deterministic outcome of evaluating a saved graph
// intent against this run's fresh resolution.
type GraphRestartPlan struct {
	State GraphRestartState `json:"state"`
	// Validated is the freshly built graph. It is meaningful only when State is
	// Ready. Its edges are this run's resolution, never a saved one.
	Validated autotunevnext.ServiceGraph `json:"-"`
	// Accepted lists node IDs that were freshly resolved and matched intent.
	Accepted []string `json:"accepted_nodes,omitempty"`
	// AbsentOptional lists optional node IDs deliberately allowed to be absent.
	AbsentOptional []string `json:"absent_optional,omitempty"`
	// Trigger is the revalidation trigger that rejected an otherwise valid plan.
	Trigger string `json:"trigger,omitempty"`
	Reason  string `json:"reason,omitempty"`
}

func rejectGraphRestart(reason string, trigger autotunevnext.GraphRevalidationState) GraphRestartPlan {
	return GraphRestartPlan{State: GraphRestartRejected, Trigger: string(trigger), Reason: reason}
}

// freshGraphNode is one freshly resolved node supplied by the resolver. It
// carries this run's addresses and is never sourced from persistence.
type freshGraphNode struct {
	NodeID string
	Role   autotunevnext.ServiceNodeRole
	Target string
	Edges  []autotunevnext.ServiceScopeEdge
}

// PlanGraphRestart evaluates a saved logical graph intent against this run's
// fresh resolution.
//
// It never widens authority:
//   - a fresh node whose ID is absent from the saved intent is NOT adopted;
//   - a required saved node that did not resolve is REJECTED, never substituted;
//   - an optional saved node may stay absent only when the saved intent explicitly
//     allowed it, and it carries no edges while absent, so its later
//     reappearance still requires fresh validation.
func PlanGraphRestart(saved persistedVNextGraphState, fresh []freshGraphNode) (GraphRestartPlan, error) {
	if err := validateVNextGraphState(saved); err != nil {
		return rejectGraphRestart("saved intent failed validation: "+err.Error(), autotunevnext.GraphRevalidationNone), err
	}
	byID := make(map[string]freshGraphNode, len(fresh))
	for _, node := range fresh {
		if _, dup := byID[node.NodeID]; dup {
			return rejectGraphRestart("fresh resolution returned a duplicate node id: "+node.NodeID, autotunevnext.GraphRevalidationNodeDisappeared), nil
		}
		byID[node.NodeID] = node
	}

	plan := GraphRestartPlan{State: GraphRestartRevalidationPending}
	built := make([]autotunevnext.ServiceNode, 0, len(saved.Nodes))
	for _, node := range saved.Nodes {
		host, err := normalizedVNextHost(node.Target)
		if err != nil {
			return rejectGraphRestart("saved node target is unusable: "+err.Error(), autotunevnext.GraphRevalidationNone), nil
		}
		resolved, ok := byID[node.NodeID]
		if !ok {
			if node.Required {
				// Fail closed. A missing required node is never substituted, and
				// the graph must not become smaller just because the network moved.
				return rejectGraphRestart("required node did not resolve: "+node.NodeID, autotunevnext.GraphRevalidationNodeDisappeared), nil
			}
			if !node.OptionalAbsentAllowed {
				return rejectGraphRestart("optional node may not be silently absent: "+node.NodeID, autotunevnext.GraphRevalidationNodeDisappeared), nil
			}
			plan.AbsentOptional = append(plan.AbsentOptional, node.NodeID)
			continue
		}
		resolvedHost, err := normalizedVNextHost(resolved.Target)
		if err != nil {
			return rejectGraphRestart("fresh node target is unusable: "+err.Error(), autotunevnext.GraphRevalidationTargetChanged), nil
		}
		if resolvedHost != host {
			// The saved intent named a different logical target than the network
			// now produced. That is a change of authority, so it needs revalidation
			// rather than a silent swap.
			return rejectGraphRestart("node target changed: "+node.NodeID, autotunevnext.GraphRevalidationTargetChanged), nil
		}
		if len(resolved.Edges) == 0 {
			if node.Required {
				return rejectGraphRestart("required node resolved with no addresses: "+node.NodeID, autotunevnext.GraphRevalidationNewEdge), nil
			}
			if !node.OptionalAbsentAllowed {
				return rejectGraphRestart("optional node may not be empty: "+node.NodeID, autotunevnext.GraphRevalidationNewEdge), nil
			}
			plan.AbsentOptional = append(plan.AbsentOptional, node.NodeID)
			continue
		}
		for _, edge := range resolved.Edges {
			if edge.IP == nil || net.ParseIP(edge.IP.String()) == nil {
				return rejectGraphRestart("node has a malformed address: "+node.NodeID, autotunevnext.GraphRevalidationNone), nil
			}
			// An optional node that reappears is VALIDATED, never ACTIVE: it has
			// just come back and has not proven itself in this run.
			state := autotunevnext.ServiceNodeActive
			if !node.Required {
				state = autotunevnext.ServiceNodeValidated
			}
			nodeTarget := restartNodeTarget(resolved.Target)
			built = append(built, autotunevnext.ServiceNode{
				ID:       node.NodeID,
				Role:     resolved.Role,
				Required: node.Required,
				Target:   nodeTarget,
				Scope: autotunevnext.ServiceScopeSnapshot{
					Target:     nodeTarget,
					ResolvedAt: time.Now(),
					Edges:      resolved.Edges,
				},
				Validation:          state,
				Strategy:            node.StrategyID,
				StrategyFingerprint: node.Fingerprint,
				TemplateIdentity:    node.TemplateIdentity,
			})
		}
		plan.Accepted = append(plan.Accepted, node.NodeID)
	}

	// A node the resolver produced that intent never named is not adopted. It is
	// reported so the product can decide, but it never enters this restart.
	for id := range byID {
		if !containsString(plan.Accepted, id) && !containsString(plan.AbsentOptional, id) {
			return rejectGraphRestart("fresh resolution produced an untrusted node: "+id, autotunevnext.GraphRevalidationNewNode), nil
		}
	}

	graph, err := autotunevnext.NewServiceGraph(built)
	if err != nil {
		return rejectGraphRestart("fresh graph is not valid: "+err.Error(), autotunevnext.GraphRevalidationNone), nil
	}
	plan.Validated = graph
	plan.State = GraphRestartReady
	return plan, nil
}

func containsString(haystack []string, needle string) bool {
	for _, item := range haystack {
		if item == needle {
			return true
		}
	}
	return false
}

// graphRestartStatus maps a plan onto the product-facing managed runtime state
// without leaking addresses, filters, or argv.
func graphRestartStatus(plan GraphRestartPlan) (string, error) {
	switch plan.State {
	case GraphRestartReady:
		return string(ManagedRuntimeGraphManaged), nil
	case GraphRestartRevalidationPending:
		return string(ManagedRuntimeGraphManaged) + ":" + string(GraphRestartRevalidationPending), nil
	case GraphRestartRejected:
		return string(ManagedRuntimeDirect), fmt.Errorf("graph restart rejected: %s", plan.Reason)
	default:
		return string(ManagedRuntimeDirect), nil
	}
}

// restartNodeTarget builds the node target used by both the node and its scope
// snapshot. They must be the same value, because graph validation requires the
// scope to describe the node's own target and nothing else.
func restartNodeTarget(raw string) autotunevnext.Target {
	return autotunevnext.Target{
		URL:           raw,
		Transport:     observatory.TransportTCP,
		AddressFamily: observatory.AddressFamilyAny,
	}
}
