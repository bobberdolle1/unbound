package autotunevnext

// Bounded service-graph experiment execution.
//
// The budget rule is deliberate: ONE activation covers the whole graph. The
// sequence is direct-before across every node's exact edges, one complete graph
// activation, active observations across all nodes, protected controls, then
// deactivation and direct-after. Running one activation per edge would multiply
// machine mutation without adding evidence.
//
// Evidence stays attributable per node. A graph-level positive outcome requires
// no node regression, no inconclusive required node, at least one factual fixed
// failure, all required nodes valid, controls healthy, and state restored.

import (
	"context"
	"fmt"
	"time"

	"unbound/engine/backendcap"
	"unbound/engine/observatory"
)

// ServiceGraphStatus is the graph-level outcome.
type ServiceGraphStatus string

const (
	ServiceGraphVerifiedFixed   ServiceGraphStatus = "SERVICE_GRAPH_VERIFIED_FIXED"
	ServiceGraphStillFailing    ServiceGraphStatus = "SERVICE_GRAPH_STILL_FAILING"
	ServiceGraphRegression      ServiceGraphStatus = "SERVICE_GRAPH_REGRESSION"
	ServiceGraphInconclusive    ServiceGraphStatus = "SERVICE_GRAPH_INCONCLUSIVE"
	ServiceGraphNeedsRevalidate ServiceGraphStatus = "SERVICE_GRAPH_CHANGED_REVALIDATION_REQUIRED"
)

// ServiceGraphActivation is the complete exact candidate handed to an executor.
// UnionEdges is current evidence and is never serialized.
type ServiceGraphActivation struct {
	Backend backendcap.Backend
	Capture backendcap.CapturePlan
	// Plan is the deterministically compiled exact plan. The executor derives
	// EngineArgv from it and never invents executable arguments itself.
	Plan       backendcap.Plan
	Graph      ServiceGraph
	Sections   []ServiceGraphSection
	UnionEdges []ServiceScopeEdge `json:"-"`
}

// ServiceGraphExecutor owns all machine mutation for one bounded graph.
// ActivateGraph must bind every node in a single owned activation.
type ServiceGraphExecutor interface {
	Snapshot(context.Context) (StateSnapshot, error)
	EstablishDirect(context.Context, StateSnapshot) error
	ActivateGraph(context.Context, ServiceGraphActivation) error
	VerifyActiveGraph(context.Context, ServiceGraphActivation) error
	Deactivate(context.Context) error
	Restore(context.Context, StateSnapshot) error
	VerifyRestored(context.Context, StateSnapshot) error
}

// ServiceGraphExperimentRequest is a product-owned graph experiment request.
// It never accepts executable arguments or a pre-resolved edge.
type ServiceGraphExperimentRequest struct {
	Graph        ServiceGraph
	Controls     []Target
	Sections     []ServiceGraphSection
	Capture      backendcap.CapturePlan
	Backend      backendcap.Backend
	NetworkLabel string
	Evidence     EvidenceOptions
	Policy       Policy
}

// ServiceGraphNodeResult is the reduced per-node evidence projection.
type ServiceGraphNodeResult struct {
	NodeID     string                 `json:"node_id"`
	Role       ServiceNodeRole        `json:"role"`
	Required   bool                   `json:"required"`
	Validation ServiceScopeValidation `json:"validation"`
}

// ServiceGraphExperiment is the reduced graph outcome. It carries no addresses.
type ServiceGraphExperiment struct {
	Fingerprint   string                   `json:"fingerprint"`
	Status        ServiceGraphStatus       `json:"status"`
	Nodes         []ServiceGraphNodeResult `json:"nodes"`
	Controls      []ControlResult          `json:"controls"`
	NodeCount     int                      `json:"node_count"`
	EdgeCount     int                      `json:"edge_count"`
	StateRestored bool                     `json:"state_restored"`
}

// RunServiceGraphExperiment executes one bounded controlled experiment across a
// graph. It is transactional: any failure or cancellation still restores and
// proves the original state.
func RunServiceGraphExperiment(ctx context.Context, request ServiceGraphExperimentRequest, executor ServiceGraphExecutor, observer Observer) (result ServiceGraphExperiment, err error) {
	if err := request.Graph.Validate(); err != nil {
		return ServiceGraphExperiment{}, fmt.Errorf("NOT_RUN: %w", err)
	}
	active, err := request.Graph.ActiveCaptureGraph()
	if err != nil {
		return ServiceGraphExperiment{}, fmt.Errorf("NOT_RUN: %w", err)
	}
	edges, err := active.UnionEdges()
	if err != nil {
		return ServiceGraphExperiment{}, fmt.Errorf("NOT_RUN: %w", err)
	}
	if !scopeCaptureSupported(request.Capture, edges) {
		return ServiceGraphExperiment{}, fmt.Errorf("NOT_RUN: SERVICE_GRAPH_CAPTURE_UNSUPPORTED")
	}

	policy := request.Policy.normalized()
	operationCtx, cancel := context.WithTimeout(ctx, policy.MaxDuration)
	defer cancel()
	snapshot, err := executor.Snapshot(operationCtx)
	if err != nil {
		return ServiceGraphExperiment{}, fmt.Errorf("NOT_RUN: SNAPSHOT_FAILED: %w", err)
	}
	restored := false
	committed := false
	defer func() {
		if committed {
			return
		}
		// Cancellation and failure both enter restore. State restoration is verified
		// before the caller's error is reported.
		rctx, rcancel := context.WithTimeout(context.WithoutCancel(ctx), 30*time.Second)
		defer rcancel()
		cleanupErr := firstError(executor.Deactivate(rctx), executor.Restore(rctx, snapshot), executor.VerifyRestored(rctx, snapshot))
		restored = cleanupErr == nil
		if cleanupErr != nil {
			err = fmt.Errorf("%w: %v", ErrManagedStateRestoreFailed, cleanupErr)
		}
	}()

	if err := executor.EstablishDirect(operationCtx, snapshot); err != nil {
		return ServiceGraphExperiment{}, fmt.Errorf("NOT_RUN: ESTABLISH_DIRECT_FAILED: %w", err)
	}

	// Protected controls: direct baseline before any mutation, pinned to the edge the
	// observer actually used so the active observation stays comparable.
	controlBaselines := make([]controlBaseline, 0, len(request.Controls))
	for _, control := range request.Controls {
		observation, obsErr := observe(operationCtx, observer, control, request.NetworkLabel, request.Evidence, policy, nil, "direct", "")
		if obsErr != nil {
			return ServiceGraphExperiment{}, fmt.Errorf("NOT_RUN: CONTROL_DIRECT_OBSERVATION_FAILED: %w", obsErr)
		}
		controlBaselines = append(controlBaselines, controlBaseline{target: control, observation: observation, edge: selectedEdge(observation)})
	}

	// direct-before across every node's exact edges.
	before := make(map[string][]observatory.ObservationResult, len(active.Nodes))
	for _, node := range active.Nodes {
		observations, obsErr := observeScope(operationCtx, observer, node.Target, request.NetworkLabel, request.Evidence, policy, node.Scope.Edges, "direct", node.TemplateIdentity)
		if obsErr != nil {
			return ServiceGraphExperiment{}, fmt.Errorf("NOT_RUN: DIRECT_BEFORE_FAILED: %w", obsErr)
		}
		before[node.ID] = observations
	}

	activation := ServiceGraphActivation{Backend: request.Backend, Capture: request.Capture, Graph: active, Sections: request.Sections, UnionEdges: edges}
	if err := executor.ActivateGraph(operationCtx, activation); err != nil {
		return ServiceGraphExperiment{}, fmt.Errorf("NOT_RUN: ACTIVATION_FAILED: %w", err)
	}
	if err := executor.VerifyActiveGraph(operationCtx, activation); err != nil {
		return ServiceGraphExperiment{}, fmt.Errorf("NOT_RUN: ACTIVE_VERIFICATION_FAILED: %w", err)
	}

	// active observations across all nodes.
	activeObservations := make(map[string][]observatory.ObservationResult, len(active.Nodes))
	for _, node := range active.Nodes {
		observations, obsErr := observeScope(operationCtx, observer, node.Target, request.NetworkLabel, request.Evidence, policy, node.Scope.Edges, "externally_active_profile", node.TemplateIdentity)
		if obsErr != nil {
			return ServiceGraphExperiment{}, fmt.Errorf("NOT_RUN: ACTIVE_OBSERVATION_FAILED: %w", obsErr)
		}
		activeObservations[node.ID] = observations
	}

	// controls while the graph is active, pinned to the same baseline edge.
	controls := make([]ControlResult, 0, len(request.Controls))
	for _, baseline := range controlBaselines {
		var observation observatory.ObservationResult
		var obsErr error
		if baseline.edge != nil {
			observation, obsErr = observe(operationCtx, observer, baseline.target, request.NetworkLabel, request.Evidence, policy, baseline.edge, "externally_active_profile", "")
		} else {
			controls = append(controls, ControlResult{
				Target: baseline.target, DirectRunID: baseline.observation.RunID,
				Outcome: OutcomeInconclusive, Reasons: []Reason{{Code: "CONTROL_EDGE_UNAVAILABLE"}},
			})
			continue
		}
		if obsErr != nil {
			return ServiceGraphExperiment{}, fmt.Errorf("NOT_RUN: CONTROL_ACTIVE_OBSERVATION_FAILED: %w", obsErr)
		}
		controls = append(controls, classifyGraphControl(baseline, observation))
	}
	if controlRegression(controls) {
		return ServiceGraphExperiment{}, fmt.Errorf("NOT_RUN: PROTECTED_CONTROL_REGRESSION")
	}

	if err := executor.Deactivate(operationCtx); err != nil {
		return ServiceGraphExperiment{}, fmt.Errorf("NOT_RUN: DEACTIVATE_FAILED: %w", err)
	}

	// direct-after across every node.
	after := make(map[string][]observatory.ObservationResult, len(active.Nodes))
	for _, node := range active.Nodes {
		observations, obsErr := observeScope(operationCtx, observer, node.Target, request.NetworkLabel, request.Evidence, policy, node.Scope.Edges, "direct", "")
		if obsErr != nil {
			return ServiceGraphExperiment{}, fmt.Errorf("NOT_RUN: DIRECT_AFTER_FAILED: %w", obsErr)
		}
		after[node.ID] = observations
	}

	if err := executor.Restore(operationCtx, snapshot); err != nil {
		return ServiceGraphExperiment{}, fmt.Errorf("NOT_RUN: RESTORE_FAILED: %w", err)
	}
	if err := executor.VerifyRestored(operationCtx, snapshot); err != nil {
		return ServiceGraphExperiment{}, fmt.Errorf("NOT_RUN: RESTORE_VERIFICATION_FAILED: %w", err)
	}
	restored = true

	result = aggregateServiceGraph(active, before, activeObservations, after, controls, len(edges))
	result.StateRestored = restored
	committed = true
	return result, nil
}

// aggregateServiceGraph classifies per node and then decides the graph outcome.
func aggregateServiceGraph(graph ServiceGraph, before, active, after map[string][]observatory.ObservationResult, controls []ControlResult, edgeCount int) ServiceGraphExperiment {
	out := ServiceGraphExperiment{
		Fingerprint: graph.Fingerprint(),
		Controls:    controls,
		NodeCount:   len(graph.Nodes),
		EdgeCount:   edgeCount,
		Status:      ServiceGraphInconclusive,
	}
	regressed := false
	fixedSomething := false
	allRequiredValid := true
	for _, node := range graph.Nodes {
		b, a, f := before[node.ID], active[node.ID], after[node.ID]
		edges := make([]EdgeEvidence, 0, len(b))
		for i := range b {
			evidence := EdgeEvidence{Edge: node.Scope.Edges[i]}
			if i < len(a) {
				evidence.Active = a[i]
			}
			if i < len(f) {
				evidence.DirectAfter = f[i]
			}
			evidence.DirectBefore = b[i]
			evidence.State = classifyScopeEdge(evidence.DirectBefore, evidence.Active, evidence.DirectAfter)
			edges = append(edges, evidence)
		}
		validation := aggregateServiceScope(edges)
		out.Nodes = append(out.Nodes, ServiceGraphNodeResult{
			NodeID: node.ID, Role: node.Role, Required: node.Required, Validation: validation,
		})
		if validation.Failed > 0 {
			regressed = true
		}
		if validation.Fixed > 0 {
			fixedSomething = true
		}
		if node.Required && validation.Status != ServiceScopeVerifiedFixed {
			allRequiredValid = false
		}
	}
	switch {
	case regressed:
		out.Status = ServiceGraphRegression
	case fixedSomething && allRequiredValid && !controlRegression(controls):
		out.Status = ServiceGraphVerifiedFixed
	case fixedSomething:
		out.Status = ServiceGraphInconclusive
	default:
		out.Status = ServiceGraphStillFailing
	}
	return out
}

// classifyGraphControl mirrors the single-host control classification: a control that
// was directly healthy and stops being healthy while the graph is active is a
// regression, and an incomparable edge is inconclusive rather than a pass.
func classifyGraphControl(baseline controlBaseline, active observatory.ObservationResult) ControlResult {
	result := ControlResult{
		Target: baseline.target, DirectRunID: baseline.observation.RunID,
		ActiveRunID: active.RunID, activeObservation: &active,
	}
	if baseline.edge == nil {
		result.Outcome = OutcomeInconclusive
		result.Reasons = []Reason{{Code: "CONTROL_EDGE_UNAVAILABLE"}}
		return result
	}
	if !sameEdge(active, *baseline.edge) || !sameFamily(baseline.observation, active) || !profileApplicable(baseline.observation, active) {
		result.Outcome = OutcomeInconclusive
		result.Reasons = []Reason{{Code: "CONTROL_COMPARISON_INAPPLICABLE"}}
		return result
	}
	if observationSuccess(baseline.observation) && !observationSuccess(active) {
		result.Outcome = OutcomeRegressionObserved
		return result
	}
	result.Outcome = OutcomeStillFailing
	return result
}

func firstError(errs ...error) error {
	for _, e := range errs {
		if e != nil {
			return e
		}
	}
	return nil
}
