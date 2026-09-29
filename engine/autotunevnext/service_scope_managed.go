package autotunevnext

import (
	"context"
	"fmt"

	"unbound/engine/attribution"
	"unbound/engine/backendcap"
	"unbound/engine/observatory"
	"unbound/engine/planner"
	"unbound/engine/strategyir"
)

// ApplyVerifiedServiceScope revalidates every currently retained scope edge
// before activating one exact bounded capture. The caller has already proven
// currentScope is a subset of the short-lived validated grant scope.
func ApplyVerifiedServiceScope(ctx context.Context, request ManagedRequest, currentScope ServiceScopeSnapshot, observer Observer, executor Executor, preflight HostPreflight, assets AssetResolver) (activation *ManagedActivation, err error) {
	if len(currentScope.Edges) == 0 || !sameTargetIdentity(request.Target, currentScope.Target) || !currentScope.IsSubsetOf(request.ValidatedScope) {
		return nil, fmt.Errorf("NOT_APPLIED: SERVICE_SCOPE_CHANGED_REVALIDATION_REQUIRED")
	}
	policy := request.Policy.normalized()
	operationCtx, cancel := context.WithTimeout(ctx, policy.MaxDuration)
	defer cancel()
	canonical, err := strategyir.Canonicalize(request.Strategy)
	if err != nil {
		return nil, fmt.Errorf("NOT_APPLIED: canonical strategy: %w", err)
	}
	fingerprint, err := strategyir.Fingerprint(canonical)
	if err != nil || fingerprint != request.Fingerprint {
		return nil, fmt.Errorf("NOT_APPLIED: FINGERPRINT_MISMATCH")
	}
	snapshot, err := executor.Snapshot(operationCtx)
	if err != nil {
		return nil, fmt.Errorf("NOT_APPLIED: SNAPSHOT_FAILED: %w", err)
	}
	activation = &ManagedActivation{executor: executor, snapshot: snapshot, restorePending: true}
	owned := activation
	committed := false
	defer func() {
		if committed {
			return
		}
		if cleanupErr := owned.Revert(ctx); cleanupErr != nil {
			activation = owned
			err = fmt.Errorf("%w: %v", ErrManagedStateRestoreFailed, cleanupErr)
		}
	}()
	if err := executor.EstablishDirect(operationCtx, snapshot); err != nil {
		return nil, fmt.Errorf("NOT_APPLIED: ESTABLISH_DIRECT_FAILED: %w", err)
	}
	before, err := observeScope(operationCtx, observer, request.Target, request.NetworkLabel, request.Evidence, policy, currentScope.Edges, "direct", "")
	if err != nil {
		return nil, fmt.Errorf("NOT_APPLIED: SERVICE_SCOPE_DIRECT_REVALIDATION_FAILED: %w", err)
	}
	baseline := selectScopePlannerObservation(before)
	if baseline == nil {
		return nil, fmt.Errorf("NOT_APPLIED: SERVICE_SCOPE_BASELINE_UNAVAILABLE")
	}
	report := planner.Plan(planner.Request{Attribution: attribution.AnalyzeCohort(attribution.Cohort{Target: []observatory.ObservationResult{*baseline}}), Backend: request.Backend, Strategies: []strategyir.Strategy{canonical}, Scope: scopeForServiceScope(planner.ScopeSnapshot{}, currentScope), Evidence: planner.EvidenceContext{AddressFamily: selectedFamily(*baseline)}})
	if report.Disposition != planner.DispositionCandidatesAvailable || len(report.Candidates) != 1 || report.Candidates[0].Status != planner.StatusEligible || report.Candidates[0].StrategyFingerprint != request.Fingerprint {
		return nil, fmt.Errorf("NOT_APPLIED: STRATEGY_NOT_ELIGIBLE")
	}
	compiled := backendcap.Compile(canonical, request.Backend)
	if compiled.Status != backendcap.StatusCompiled || !scopeCaptureSupported(compiled.Plan.Capture, currentScope.Edges) {
		return nil, fmt.Errorf("NOT_APPLIED: SERVICE_SCOPE_CAPTURE_UNSUPPORTED")
	}
	resolved, err := assets.Resolve(operationCtx, request.Backend, compiled.RequiredAssets)
	if err != nil || !resolvedAssetsCover(compiled.RequiredAssets, resolved) {
		return nil, fmt.Errorf("NOT_APPLIED: ASSET_RESOLUTION_FAILED")
	}
	argv, err := MaterializeEngineArgv(compiled.Plan.EngineArgv, resolved)
	if err != nil {
		return nil, fmt.Errorf("NOT_APPLIED: ASSET_MATERIALIZATION_FAILED")
	}
	preflightResult := preflight.Check(operationCtx, HostPreflightRequest{Backend: request.Backend, Requirements: compiled.DerivedRequirements, RequiredAssets: compiled.RequiredAssets, Capture: compiled.Plan.Capture})
	if preflightResult.Status != PreflightSupported {
		return nil, fmt.Errorf("NOT_APPLIED: PREFLIGHT_FAILED")
	}
	controls := make([]controlBaseline, 0, len(request.Controls))
	for _, control := range request.Controls {
		direct, observeErr := observe(operationCtx, observer, control, request.NetworkLabel, request.Evidence, policy, nil, "direct", "")
		if observeErr != nil || selectedEdge(direct) == nil {
			return nil, fmt.Errorf("NOT_APPLIED: CONTROL_DIRECT_OBSERVATION_FAILED")
		}
		controls = append(controls, controlBaseline{target: control, observation: direct, edge: selectedEdge(direct)})
	}
	plan := compiled.Plan
	plan.EngineArgv = argv
	activation.candidate = ExecutableCandidate{Strategy: canonical, Fingerprint: request.Fingerprint, Backend: request.Backend, Plan: plan, Assets: resolved, TargetEdges: cloneScopeEdges(currentScope.Edges)}
	activation.candidateActive = true
	// Successful Apply transfers process ownership to ManagedActivation; the
	// bounded validation context must not terminate that committed process.
	managedProcessCtx := context.WithoutCancel(operationCtx)
	if err := executor.Activate(managedProcessCtx, activation.candidate); err != nil {
		return nil, fmt.Errorf("NOT_APPLIED: ACTIVATION_FAILED: %w", err)
	}
	if err := executor.VerifyActive(operationCtx, activation.candidate); err != nil {
		return nil, fmt.Errorf("NOT_APPLIED: ACTIVE_VERIFY_FAILED: %w", err)
	}
	active, err := observeScope(operationCtx, observer, request.Target, request.NetworkLabel, request.Evidence, policy, currentScope.Edges, "externally_active_profile", canonical.ID)
	if err != nil {
		return nil, fmt.Errorf("NOT_APPLIED: ACTIVE_TARGET_VERIFICATION_FAILED: %w", err)
	}
	for index := range currentScope.Edges {
		if !observationSuccess(active[index]) {
			return nil, fmt.Errorf("NOT_APPLIED: ACTIVE_TARGET_VERIFICATION_FAILED")
		}
	}
	for _, control := range controls {
		activeControl, observeErr := observe(operationCtx, observer, control.target, request.NetworkLabel, request.Evidence, policy, control.edge, "externally_active_profile", canonical.ID)
		if observeErr != nil || !sameEdge(activeControl, *control.edge) || (observationSuccess(control.observation) && !observationSuccess(activeControl)) {
			return nil, fmt.Errorf("NOT_APPLIED: CONTROL_REGRESSION")
		}
	}
	committed = true
	return activation, nil
}
