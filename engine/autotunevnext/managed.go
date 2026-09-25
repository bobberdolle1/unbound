package autotunevnext

import (
	"context"
	"errors"
	"fmt"
	"net"
	"time"

	"unbound/engine/attribution"
	"unbound/engine/backendcap"
	"unbound/engine/observatory"
	"unbound/engine/planner"
	"unbound/engine/strategyir"
)

// ErrManagedStateRestoreFailed means a failed Apply could not prove that the
// original runtime state was restored. Callers must retain factual ownership.
var ErrManagedStateRestoreFailed = errors.New("STATE_RESTORE_FAILED")

// ManagedRequest is a product-owned revalidation request. The caller provides
// the strategy identity selected by an earlier experiment; it never accepts
// executable arguments or a pre-resolved edge.
type ManagedRequest struct {
	Target       Target
	Controls     []Target
	Strategy     strategyir.Strategy
	Fingerprint  string
	Backend      backendcap.Backend
	NetworkLabel string
	Evidence     EvidenceOptions
	Policy       Policy
}

// ManagedActivation owns one live runtime candidate and its original product
// snapshot. Revert is the only path that may clear this ownership.
type ManagedActivation struct {
	executor  Executor
	snapshot  StateSnapshot
	candidate ExecutableCandidate
	parent    context.Context
	active    bool
}

// Candidate returns only logical candidate identity. It never exposes argv.
func (a *ManagedActivation) Candidate() ExecutableCandidate {
	return ExecutableCandidate{
		Strategy: a.candidate.Strategy, Fingerprint: a.candidate.Fingerprint,
		Backend: a.candidate.Backend, TargetEdge: append(net.IP(nil), a.candidate.TargetEdge...),
		TargetFamily: a.candidate.TargetFamily,
	}
}

// Revert deactivates the owned candidate and restores the exact snapshot. A
// restoration failure leaves ownership intact so callers cannot falsely report
// direct state.
func (a *ManagedActivation) Revert(ctx context.Context) error {
	if a == nil || !a.active {
		return nil
	}
	cleanupCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 30*time.Second)
	defer cancel()
	if err := a.executor.Deactivate(cleanupCtx); err != nil {
		return fmt.Errorf("DEACTIVATION_FAILED: %w", err)
	}
	if err := a.executor.Restore(cleanupCtx, a.snapshot); err != nil {
		return fmt.Errorf("RESTORE_FAILED: %w", err)
	}
	if err := a.executor.VerifyRestored(cleanupCtx, a.snapshot); err != nil {
		return fmt.Errorf("RESTORE_VERIFY_FAILED: %w", err)
	}
	a.active = false
	return nil
}

// ApplyVerified performs a fresh exact-edge revalidation before retaining a
// managed candidate. It does not reuse the experiment session or its edge.
func ApplyVerified(ctx context.Context, request ManagedRequest, observer Observer, executor Executor, preflight HostPreflight, assets AssetResolver) (activation *ManagedActivation, err error) {
	if observer == nil || executor == nil || preflight == nil || assets == nil {
		return nil, fmt.Errorf("NOT_APPLIED: dependency missing")
	}
	policy := request.Policy.normalized()
	if request.Strategy.ID == "" || request.Fingerprint == "" || request.Backend == "" {
		return nil, fmt.Errorf("NOT_APPLIED: invalid managed selection")
	}
	canonical, err := strategyir.Canonicalize(request.Strategy)
	if err != nil {
		return nil, fmt.Errorf("NOT_APPLIED: canonical strategy: %w", err)
	}
	fingerprint, err := strategyir.Fingerprint(canonical)
	if err != nil || fingerprint != request.Fingerprint {
		return nil, fmt.Errorf("NOT_APPLIED: FINGERPRINT_MISMATCH")
	}
	operationCtx, cancel := context.WithTimeout(ctx, policy.MaxDuration)
	defer cancel()
	snapshot, err := executor.Snapshot(operationCtx)
	if err != nil {
		return nil, fmt.Errorf("NOT_APPLIED: SNAPSHOT_FAILED: %w", err)
	}
	activation = &ManagedActivation{executor: executor, snapshot: snapshot, parent: ctx}
	owned := activation
	committed := false
	defer func() {
		if committed {
			return
		}
		cleanupCtx, cleanupCancel := context.WithTimeout(context.WithoutCancel(ctx), 30*time.Second)
		defer cleanupCancel()
		var cleanupErr error
		if owned.active {
			if deactivateErr := executor.Deactivate(cleanupCtx); deactivateErr != nil {
				cleanupErr = deactivateErr
			}
		}
		if restoreErr := executor.Restore(cleanupCtx, snapshot); restoreErr != nil && cleanupErr == nil {
			cleanupErr = restoreErr
		}
		if verifyErr := executor.VerifyRestored(cleanupCtx, snapshot); verifyErr != nil && cleanupErr == nil {
			cleanupErr = verifyErr
		}
		if cleanupErr != nil {
			activation = nil
			err = fmt.Errorf("%w: %v", ErrManagedStateRestoreFailed, cleanupErr)
		}
	}()
	if err := executor.EstablishDirect(operationCtx, snapshot); err != nil {
		return nil, fmt.Errorf("NOT_APPLIED: ESTABLISH_DIRECT_FAILED: %w", err)
	}
	baseline, err := observe(operationCtx, observer, request.Target, request.NetworkLabel, request.Evidence, policy, nil, "direct", "")
	if err != nil {
		return nil, fmt.Errorf("NOT_APPLIED: DIRECT_OBSERVATION_FAILED: %w", err)
	}
	edge := selectedEdge(baseline)
	if edge == nil || selectedFamily(baseline) == "" {
		return nil, fmt.Errorf("NOT_APPLIED: CONCRETE_EDGE_REQUIRED")
	}
	report := planner.Plan(planner.Request{
		Attribution: attribution.AnalyzeCohort(attribution.Cohort{Target: []observatory.ObservationResult{baseline}}),
		Backend:     request.Backend, Strategies: []strategyir.Strategy{canonical},
		Scope:    scopeForBaseline(planner.ScopeSnapshot{}, baseline),
		Evidence: planner.EvidenceContext{AddressFamily: selectedFamily(baseline)},
	})
	if report.Disposition != planner.DispositionCandidatesAvailable || len(report.Candidates) != 1 || report.Candidates[0].Status != planner.StatusEligible || report.Candidates[0].StrategyFingerprint != request.Fingerprint {
		return nil, fmt.Errorf("NOT_APPLIED: STRATEGY_NOT_ELIGIBLE")
	}
	compiled := backendcap.Compile(canonical, request.Backend)
	if compiled.Status != backendcap.StatusCompiled || compiled.StrategyFingerprint != request.Fingerprint {
		return nil, fmt.Errorf("NOT_APPLIED: COMPILE_FAILED")
	}
	if measurement := MacOSMeasurementPath(request.Backend); measurement.Status != PreflightSupported {
		return nil, fmt.Errorf("NOT_APPLIED: MEASUREMENT_PATH_UNSUPPORTED")
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
	before, err := observe(operationCtx, observer, request.Target, request.NetworkLabel, request.Evidence, policy, edge, "direct", "")
	if err != nil || !sameEdge(before, *edge) || !sameFamily(baseline, before) || observationSuccess(before) {
		return nil, fmt.Errorf("NOT_APPLIED: DIRECT_REVALIDATION_FAILED")
	}
	controls := make([]controlBaseline, 0, len(request.Controls))
	for _, control := range request.Controls {
		controlDirect, observeErr := observe(operationCtx, observer, control, request.NetworkLabel, request.Evidence, policy, nil, "direct", "")
		if observeErr != nil {
			return nil, fmt.Errorf("NOT_APPLIED: CONTROL_DIRECT_OBSERVATION_FAILED")
		}
		controlEdge := selectedEdge(controlDirect)
		if controlEdge == nil {
			return nil, fmt.Errorf("NOT_APPLIED: CONTROL_EDGE_REQUIRED")
		}
		controls = append(controls, controlBaseline{target: control, observation: controlDirect, edge: controlEdge})
	}
	plan := compiled.Plan
	plan.EngineArgv = argv
	activation.candidate = ExecutableCandidate{Strategy: canonical, Fingerprint: request.Fingerprint, Backend: request.Backend, Plan: plan, Assets: resolved, TargetEdge: append(net.IP(nil), (*edge)...), TargetFamily: selectedFamily(before)}
	activation.active = true
	if err := executor.Activate(operationCtx, activation.candidate); err != nil {
		return nil, fmt.Errorf("NOT_APPLIED: ACTIVATION_FAILED: %w", err)
	}
	if err := executor.VerifyActive(operationCtx, activation.candidate); err != nil {
		return nil, fmt.Errorf("NOT_APPLIED: ACTIVE_VERIFY_FAILED: %w", err)
	}
	active, err := observe(operationCtx, observer, request.Target, request.NetworkLabel, request.Evidence, policy, edge, "externally_active_profile", canonical.ID)
	if err != nil || !sameEdge(active, *edge) || !sameFamily(before, active) || !observationSuccess(active) {
		return nil, fmt.Errorf("NOT_APPLIED: ACTIVE_TARGET_VERIFICATION_FAILED")
	}
	for _, control := range controls {
		controlActive, observeErr := observe(operationCtx, observer, control.target, request.NetworkLabel, request.Evidence, policy, control.edge, "externally_active_profile", canonical.ID)
		if observeErr != nil || !sameEdge(controlActive, *control.edge) || !sameFamily(control.observation, controlActive) || (observationSuccess(control.observation) && !observationSuccess(controlActive)) {
			return nil, fmt.Errorf("NOT_APPLIED: CONTROL_REGRESSION")
		}
	}
	committed = true
	return activation, nil
}
