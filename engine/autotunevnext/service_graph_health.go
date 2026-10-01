package autotunevnext

// Bounded service-graph managed health.
//
// Health compares fresh graph state against the ACTUAL ACTIVE CAPTURE GRAPH, not
// against the larger experiment or validation grant. If an experiment validated
// H1, H2 and H3 but Apply activated only H1 and H2, then H3 is not silently
// authorized later; a fresh H3 edge is a revalidation trigger.
//
// A dead owned process is a lifecycle FAULT, which is a different condition from
// "the network changed". Resolver failure fails closed into revalidation rather
// than reporting health.

import "fmt"

// ServiceGraphHealth mirrors the single-host managed health vocabulary.
type ServiceGraphHealth string

const (
	ServiceGraphHealthy           ServiceGraphHealth = "HEALTHY"
	ServiceGraphNeedsRevalidation ServiceGraphHealth = "NEEDS_REVALIDATION"
	ServiceGraphFault             ServiceGraphHealth = "FAULT"
)

// ServiceGraphHealthInput is the factual input to a health decision. It carries no
// resolved addresses; callers resolve before calling.
type ServiceGraphHealthInput struct {
	// ActiveCapture is the exact graph that is currently bound in the capture.
	ActiveCapture ServiceGraph
	// Current is the freshly re-resolved graph.
	Current ServiceGraph
	// OwnedProcessAlive reports the owned engine process lifecycle.
	OwnedProcessAlive bool
	// ResolverFailed reports that at least one node could not be re-resolved.
	ResolverFailed bool
}

// EvaluateServiceGraphHealth decides managed health for a bounded graph.
//
// Precedence is deliberate: a lifecycle fault outranks a revalidation trigger,
// because a dead process is not a network condition and must not be silently
// re-absorbed by a fresh experiment.
func EvaluateServiceGraphHealth(input ServiceGraphHealthInput) (ServiceGraphHealth, error) {
	if !input.OwnedProcessAlive {
		return ServiceGraphFault, fmt.Errorf("SERVICE_GRAPH_OWNED_PROCESS_DEAD")
	}
	if input.ResolverFailed {
		return ServiceGraphNeedsRevalidation, fmt.Errorf("SERVICE_GRAPH_RESOLVER_FAILED")
	}
	active, err := input.ActiveCapture.ActiveCaptureGraph()
	if err != nil {
		// No active graph means nothing is actually bound, which is not health.
		return ServiceGraphNeedsRevalidation, fmt.Errorf("SERVICE_GRAPH_NO_ACTIVE_CAPTURE")
	}
	// The comparison baseline is the active capture graph, never the larger grant.
	trigger := active.RevalidateAgainst(input.Current)
	if trigger.State != GraphRevalidationNone {
		return ServiceGraphNeedsRevalidation, fmt.Errorf("SERVICE_GRAPH_%s", trigger.State)
	}
	return ServiceGraphHealthy, nil
}

// ServiceGraphHealthSnapshot is the reduced, persistable projection of graph
// health. It carries logical node identities only.
type ServiceGraphHealthSnapshot struct {
	Fingerprint string             `json:"fingerprint"`
	Health      ServiceGraphHealth `json:"health"`
	NodeCount   int                `json:"node_count"`
	EdgeCount   int                `json:"edge_count"`
	Nodes       []map[string]any   `json:"nodes"`
}

// NewServiceGraphHealthSnapshot builds the durable health projection from the
// active capture graph.
func NewServiceGraphHealthSnapshot(graph ServiceGraph, health ServiceGraphHealth) (ServiceGraphHealthSnapshot, error) {
	active, err := graph.ActiveCaptureGraph()
	if err != nil {
		return ServiceGraphHealthSnapshot{}, err
	}
	edges, err := active.UnionEdges()
	if err != nil {
		return ServiceGraphHealthSnapshot{}, err
	}
	return ServiceGraphHealthSnapshot{
		Fingerprint: graph.Fingerprint(),
		Health:      health,
		NodeCount:   len(active.Nodes),
		EdgeCount:   len(edges),
		Nodes:       active.LogNodes(),
	}, nil
}
