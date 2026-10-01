package autotunevnext

// Bounded service-graph Apply.
//
// Apply re-resolves EVERY node. It may only use current scopes that remain valid
// under the validated authority, and it derives its capture from the ACTIVE nodes
// of the freshly re-resolved graph, never from the larger experiment graph.
// Anything that would widen capture fails closed before any mutation.

import (
	"context"
	"fmt"
	"time"

	"unbound/engine/backendcap"
)

// ServiceGraphApplyRequest is a product-owned graph revalidation request.
// It never accepts executable arguments or a pre-resolved edge.
type ServiceGraphApplyRequest struct {
	// Validated is the short-lived graph produced by a VERIFIED_FIXED experiment.
	Validated ServiceGraph
	// Current is the freshly re-resolved graph. Every node scope must have been
	// resolved again immediately before this call.
	Current      ServiceGraph
	Controls     []Target
	Sections     []ServiceGraphSection
	Capture      backendcap.CapturePlan
	Backend      backendcap.Backend
	NetworkLabel string
	Evidence     EvidenceOptions
	Policy       Policy
}

// PrepareServiceGraphApply performs every gate that must pass before any machine
// mutation, and returns the exact activation that would be applied.
//
// It fails closed for a graph-wide revalidation requirement, an empty or invalid
// current graph, a capture the backend cannot express, and any section that does
// not match its node's exact host.
func PrepareServiceGraphApply(request ServiceGraphApplyRequest) (ServiceGraphActivation, error) {
	if err := request.Current.Validate(); err != nil {
		return ServiceGraphActivation{}, fmt.Errorf("NOT_APPLIED: %w", err)
	}
	revalidation := request.Validated.RevalidateAgainst(request.Current)
	if revalidation.State != GraphRevalidationNone {
		return ServiceGraphActivation{}, fmt.Errorf("NOT_APPLIED: SERVICE_GRAPH_CHANGED_REVALIDATION_REQUIRED: %s", revalidation.State)
	}
	active, err := request.Current.ActiveCaptureGraph()
	if err != nil {
		return ServiceGraphActivation{}, fmt.Errorf("NOT_APPLIED: %w", err)
	}
	edges, err := active.UnionEdges()
	if err != nil {
		return ServiceGraphActivation{}, fmt.Errorf("NOT_APPLIED: %w", err)
	}
	if !scopeCaptureSupported(request.Capture, edges) {
		return ServiceGraphActivation{}, fmt.Errorf("NOT_APPLIED: SERVICE_GRAPH_CAPTURE_UNSUPPORTED")
	}
	// Sections are validated against the ACTIVE graph, so a section can never
	// smuggle authority for a node that is not actually active.
	sections := make([]ServiceGraphSection, 0, len(active.Nodes))
	for _, node := range active.Nodes {
		found := false
		for _, section := range request.Sections {
			if section.NodeID != node.ID {
				continue
			}
			if section.Host != node.Hostname() {
				return ServiceGraphActivation{}, fmt.Errorf("NOT_APPLIED: %w: section %q host mismatch", ErrServiceGraphBinding, node.ID)
			}
			if err := validateExactHostSection(section.Argv, node.Hostname()); err != nil {
				return ServiceGraphActivation{}, fmt.Errorf("NOT_APPLIED: %w: node %q: %v", ErrServiceGraphBinding, node.ID, err)
			}
			sections = append(sections, section)
			found = true
			break
		}
		if !found {
			return ServiceGraphActivation{}, fmt.Errorf("NOT_APPLIED: %w: no section for active node %q", ErrServiceGraphBinding, node.ID)
		}
	}
	return ServiceGraphActivation{
		Backend: request.Backend, Capture: request.Capture,
		Graph: active, Sections: sections, UnionEdges: edges,
	}, nil
}

// ServiceGraphManagedActivation owns the product snapshot after a graph mutation
// may have started, so a failed apply can still require a later restore retry.
type ServiceGraphManagedActivation struct {
	graph          ServiceGraph
	executor       ServiceGraphExecutor
	snapshot       StateSnapshot
	restorePending bool
}

// Graph returns the logical, privacy-safe projection of the applied graph.
func (a *ServiceGraphManagedActivation) Graph() ServiceGraph {
	if a == nil {
		return ServiceGraph{}
	}
	return a.graph
}

// Revert restores and proves the original state. It is idempotent.
func (a *ServiceGraphManagedActivation) Revert(ctx context.Context) error {
	if a == nil || !a.restorePending || a.executor == nil {
		return nil
	}
	if err := firstError(a.executor.Deactivate(ctx), a.executor.Restore(ctx, a.snapshot), a.executor.VerifyRestored(ctx, a.snapshot)); err != nil {
		return err
	}
	a.restorePending = false
	return nil
}

// ApplyServiceGraph applies a validated graph after re-resolving every node.
//
// It revalidates each active node directly before activation, activates the whole
// graph once, verifies every node, then always deactivates and restores. Any
// failure or cancellation still enters restore, and a restore failure takes
// precedence over the original error.
func ApplyServiceGraph(ctx context.Context, request ServiceGraphApplyRequest, executor ServiceGraphExecutor, observer Observer) (activation *ServiceGraphManagedActivation, err error) {
	prepared, err := PrepareServiceGraphApply(request)
	if err != nil {
		return nil, err
	}
	policy := request.Policy.normalized()
	operationCtx, cancel := context.WithTimeout(ctx, policy.MaxDuration)
	defer cancel()

	snapshot, err := executor.Snapshot(operationCtx)
	if err != nil {
		return nil, fmt.Errorf("NOT_APPLIED: SNAPSHOT_FAILED: %w", err)
	}
	owned := &ServiceGraphManagedActivation{graph: prepared.Graph, executor: executor, snapshot: snapshot, restorePending: true}
	committed := false
	defer func() {
		if committed {
			return
		}
		rctx, rcancel := context.WithTimeout(context.WithoutCancel(ctx), 30*time.Second)
		defer rcancel()
		if cleanupErr := firstError(executor.Deactivate(rctx), executor.Restore(rctx, snapshot), executor.VerifyRestored(rctx, snapshot)); cleanupErr != nil {
			activation = owned
			err = fmt.Errorf("%w: %v", ErrManagedStateRestoreFailed, cleanupErr)
		}
	}()

	if err := executor.EstablishDirect(operationCtx, snapshot); err != nil {
		return nil, fmt.Errorf("NOT_APPLIED: ESTABLISH_DIRECT_FAILED: %w", err)
	}

	// Every node is revalidated directly before the graph is applied.
	for _, node := range prepared.Graph.Nodes {
		if _, obsErr := observeScope(operationCtx, observer, node.Target, request.NetworkLabel, request.Evidence, policy, node.Scope.Edges, "direct", ""); obsErr != nil {
			return nil, fmt.Errorf("NOT_APPLIED: SERVICE_GRAPH_DIRECT_REVALIDATION_FAILED: %w", obsErr)
		}
	}

	if err := executor.ActivateGraph(operationCtx, prepared); err != nil {
		return nil, fmt.Errorf("NOT_APPLIED: ACTIVATION_FAILED: %w", err)
	}
	if err := executor.VerifyActiveGraph(operationCtx, prepared); err != nil {
		return nil, fmt.Errorf("NOT_APPLIED: ACTIVE_VERIFICATION_FAILED: %w", err)
	}
	for _, node := range prepared.Graph.Nodes {
		observations, obsErr := observeScope(operationCtx, observer, node.Target, request.NetworkLabel, request.Evidence, policy, node.Scope.Edges, "externally_active_profile", node.TemplateIdentity)
		if obsErr != nil {
			return nil, fmt.Errorf("NOT_APPLIED: ACTIVE_VERIFICATION_FAILED: %w", obsErr)
		}
		if !allScopeReachable(observations) {
			return nil, fmt.Errorf("NOT_APPLIED: SERVICE_GRAPH_ACTIVE_NOT_HEALTHY")
		}
	}

	if err := executor.Deactivate(operationCtx); err != nil {
		return nil, fmt.Errorf("NOT_APPLIED: DEACTIVATE_FAILED: %w", err)
	}
	if err := executor.Restore(operationCtx, snapshot); err != nil {
		return nil, fmt.Errorf("NOT_APPLIED: RESTORE_FAILED: %w", err)
	}
	if err := executor.VerifyRestored(operationCtx, snapshot); err != nil {
		return nil, fmt.Errorf("NOT_APPLIED: RESTORE_VERIFICATION_FAILED: %w", err)
	}
	committed = true
	return owned, nil
}
