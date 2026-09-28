package autotunevnext

import (
	"context"
	"fmt"
	"net"
	"sync"

	"unbound/engine"
	"unbound/engine/attribution"
	"unbound/engine/backendcap"
	"unbound/engine/observatory"
	"unbound/engine/planner"
	"unbound/engine/strategyir"
)

// RunServiceScopeCoordinated is the production service-scope entrypoint.
func RunServiceScopeCoordinated(ctx context.Context, request Request, scope ServiceScopeSnapshot, observer Observer, executor Executor, preflight HostPreflight, assets AssetResolver) (Result, error) {
	release, err := engine.GetCoordinator().Acquire(engine.OpAutoTune, "autotune-vnext", false)
	if err != nil {
		return Result{}, err
	}
	defer release()
	return RunServiceScope(ctx, request, scope, observer, executor, preflight, assets), nil
}

// RunServiceScope keeps the existing candidate lifecycle, but evidence and
// capture authority are a complete bounded DNS snapshot rather than one edge.
func RunServiceScope(ctx context.Context, request Request, scope ServiceScopeSnapshot, observer Observer, executor Executor, preflight HostPreflight, assets AssetResolver) (result Result) {
	result = Result{SchemaVersion: SchemaVersion, Target: request.Target, Backend: request.Backend, ValidatedScope: scope, ServiceScope: ServiceScopeValidation{EdgeCount: len(scope.Edges), Status: ServiceScopePending}}
	if observer == nil || executor == nil || preflight == nil || assets == nil || !sameTargetIdentity(request.Target, scope.Target) || len(scope.Edges) == 0 || len(scope.Edges) > MaxServiceScopeEdges {
		result.Status = StatusPreflightFailed
		result.Limitations = []string{"SERVICE_SCOPE_INVALID"}
		return result
	}
	policy := request.Policy.normalized()
	experimentCtx, cancel := context.WithTimeout(ctx, policy.MaxDuration)
	defer cancel()
	snapshot, err := executor.Snapshot(experimentCtx)
	if err != nil {
		return lifecycleFailure(result, "SNAPSHOT_FAILED", err)
	}
	result.Lifecycle.SnapshotTaken = true
	defer func() { finalizeRestore(ctx, executor, snapshot, &result) }()
	if err := executor.EstablishDirect(experimentCtx, snapshot); err != nil {
		return lifecycleFailure(result, "ESTABLISH_DIRECT_FAILED", err)
	}
	result.Lifecycle.DirectEstablished = true

	baseline, err := observeScope(experimentCtx, observer, request.Target, request.NetworkLabel, request.Evidence, policy, scope.Edges, "direct", "")
	if err != nil {
		return observationFailure(result, ctx, experimentCtx, "SERVICE_SCOPE_BASELINE_FAILED", err)
	}
	baselineTarget := selectScopePlannerObservation(baseline)
	if baselineTarget == nil {
		result.Status = StatusInconclusive
		result.Limitations = append(result.Limitations, "SERVICE_SCOPE_BASELINE_UNAVAILABLE")
		return result
	}
	controls := make([]controlBaseline, 0, len(request.Controls))
	for _, control := range request.Controls {
		observation, observeErr := observe(experimentCtx, observer, control, request.NetworkLabel, request.Evidence, policy, nil, "direct", "")
		if observeErr != nil {
			return observationFailure(result, ctx, experimentCtx, "CONTROL_BASELINE_OBSERVATION_FAILED", observeErr)
		}
		controls = append(controls, controlBaseline{target: control, observation: observation, edge: selectedEdge(observation)})
	}
	result.BaselineAttribution = attribution.AnalyzeCohort(attribution.Cohort{Target: []observatory.ObservationResult{*baselineTarget}, Controls: observationsFromControls(controls)})
	plannerScope := scopeForServiceScope(request.ScopeSnapshot, scope)
	result.PlannerReport = planner.Plan(planner.Request{Attribution: result.BaselineAttribution, Backend: request.Backend, Strategies: request.Strategies, Scope: plannerScope, Evidence: planner.EvidenceContext{AddressFamily: selectedFamily(*baselineTarget)}})
	result.Limitations = append(result.Limitations, result.PlannerReport.Limitations...)
	if allScopeReachable(baseline) {
		result.Status = StatusCompletedNoActionNeeded
		result.ServiceScope = ServiceScopeValidation{EdgeCount: len(scope.Edges), Status: ServiceScopeNoActionNeeded, Reachable: len(scope.Edges)}
		return result
	}
	if result.PlannerReport.Disposition != planner.DispositionCandidatesAvailable {
		for _, input := range candidateInputs(result.PlannerReport, request.Strategies) {
			result.Experiments = append(result.Experiments, notRun(input.assessment, input.strategy, OutcomeNotRunPolicy, "PLANNER_NOT_ELIGIBLE"))
		}
		result.Status = StatusCompletedNoEligibleCandidates
		return result
	}
	for _, input := range candidateInputs(result.PlannerReport, request.Strategies) {
		if experimentCtx.Err() != nil || executedCount(result.Experiments) >= policy.MaxCandidates {
			result.Experiments = append(result.Experiments, notRun(input.assessment, input.strategy, OutcomeNotRunBudget, "DURATION_BUDGET_EXHAUSTED"))
			continue
		}
		if input.assessment.Status != planner.StatusEligible {
			result.Experiments = append(result.Experiments, notRun(input.assessment, input.strategy, OutcomeNotRunPolicy, "PLANNER_NOT_ELIGIBLE"))
			continue
		}
		experiment := runServiceScopeCandidate(experimentCtx, request, scope, policy, observer, executor, preflight, assets, snapshot, controls, input)
		result.Experiments = append(result.Experiments, experiment)
		if experiment.ScopeValidation.Status == ServiceScopeVerifiedFixed {
			result.ServiceScope = experiment.ScopeValidation
		}
		if experiment.Outcome == OutcomeLifecycleFailure {
			result.Status = StatusLifecycleFailed
			return result
		}
	}
	selectServiceScopeCandidate(&result)
	if result.SelectedStrategyID != "" {
		result.Status = StatusCompletedSelected
	} else {
		result.Status = StatusCompletedNoVerifiedCandidate
	}
	return result
}

func scopeForServiceScope(caller planner.ScopeSnapshot, scope ServiceScopeSnapshot) planner.ScopeSnapshot {
	result := planner.ScopeSnapshot{HostListMembers: cloneScopeMembers(caller.HostListMembers), IPSetMembers: cloneScopeMembers(caller.IPSetMembers)}
	for _, edge := range scope.Edges {
		result.TargetEdgeIPs = append(result.TargetEdgeIPs, edge.IP.String())
	}
	return result
}

func selectScopePlannerObservation(observations []observatory.ObservationResult) *observatory.ObservationResult {
	for index := range observations {
		if !observationSuccess(observations[index]) {
			return &observations[index]
		}
	}
	if len(observations) == 0 {
		return nil
	}
	return &observations[0]
}

func allScopeReachable(observations []observatory.ObservationResult) bool {
	return len(observations) > 0 && len(observations) <= MaxServiceScopeEdges && func() bool {
		for _, observation := range observations {
			if !observationSuccess(observation) {
				return false
			}
		}
		return true
	}()
}

func observeScope(ctx context.Context, observer Observer, target Target, label string, evidence EvidenceOptions, policy Policy, edges []ServiceScopeEdge, mode, strategyID string) ([]observatory.ObservationResult, error) {
	if len(edges) == 0 || len(edges) > MaxServiceScopeEdges {
		return nil, fmt.Errorf("invalid service scope edge count")
	}
	results := make([]observatory.ObservationResult, len(edges))
	parallel := MaxParallelEdgeProbes
	if parallel > len(edges) {
		parallel = len(edges)
	}
	sem := make(chan struct{}, parallel)
	var group sync.WaitGroup
	var firstErr error
	var mu sync.Mutex
	for index := range edges {
		index := index
		group.Add(1)
		go func() {
			defer group.Done()
			select {
			case sem <- struct{}{}:
			case <-ctx.Done():
				mu.Lock()
				if firstErr == nil {
					firstErr = ctx.Err()
				}
				mu.Unlock()
				return
			}
			defer func() { <-sem }()
			ip := append(net.IP(nil), edges[index].IP...)
			observation, err := observe(ctx, observer, target, label, evidence, policy, &ip, mode, strategyID)
			if err != nil {
				mu.Lock()
				if firstErr == nil {
					firstErr = err
				}
				mu.Unlock()
				return
			}
			if !sameEdge(observation, ip) || selectedFamily(observation) != edges[index].Family {
				mu.Lock()
				if firstErr == nil {
					firstErr = fmt.Errorf("service scope observer escaped pinned edge")
				}
				mu.Unlock()
				return
			}

			results[index] = observation
		}()
	}
	group.Wait()
	if firstErr != nil {
		return nil, firstErr
	}
	return results, nil
}

// ObserveServiceScope performs bounded exact-edge observations for a fresh
// current scope. It is used by product managed health after structural scope
// comparison; raw edges stay inside the engine boundary.
func ObserveServiceScope(ctx context.Context, observer Observer, target Target, scope ServiceScopeSnapshot) ([]observatory.ObservationResult, error) {
	return observeScope(ctx, observer, target, "product-autotune-vnext", EvidenceOptions{}, DefaultPolicy(), scope.Edges, "externally_active_profile", "")
}

func ServiceScopeObservationsHealthy(observations []observatory.ObservationResult) bool {
	return allScopeReachable(observations)
}

func runServiceScopeCandidate(ctx context.Context, request Request, scope ServiceScopeSnapshot, policy Policy, observer Observer, executor Executor, preflight HostPreflight, assets AssetResolver, snapshot StateSnapshot, controls []controlBaseline, input candidateInput) (experiment CandidateExperiment) {
	experiment = CandidateExperiment{StrategyID: input.assessment.StrategyID, Fingerprint: input.assessment.StrategyFingerprint, PlannerStatus: input.assessment.Status, CompileStatus: input.assessment.CompileStatus, PreflightStatus: PreflightNotRun, Safety: input.assessment.Safety, ScopeValidation: ServiceScopeValidation{EdgeCount: len(scope.Edges), Status: ServiceScopePending}}
	compiled := backendcap.Compile(input.strategy, request.Backend)
	if compiled.Status != backendcap.StatusCompiled || !scopeCaptureSupported(compiled.Plan.Capture, scope.Edges) {
		experiment.Outcome, experiment.PreflightStatus = OutcomeHostUnsupported, PreflightUnsupported
		experiment.RejectionReasons = []Reason{{Code: "SERVICE_SCOPE_CAPTURE_UNSUPPORTED"}}
		return experiment
	}
	resolved, err := assets.Resolve(ctx, request.Backend, compiled.RequiredAssets)
	if err != nil || !resolvedAssetsCover(compiled.RequiredAssets, resolved) {
		experiment.Outcome, experiment.PreflightStatus = OutcomeHostUnsupported, PreflightError
		experiment.RejectionReasons = []Reason{{Code: "MISSING_ASSET"}}
		return experiment
	}
	argv, err := MaterializeEngineArgv(compiled.Plan.EngineArgv, resolved)
	if err != nil {
		experiment.Outcome, experiment.PreflightStatus = OutcomeHostUnsupported, PreflightError
		experiment.RejectionReasons = []Reason{{Code: materializationReason(err)}}
		return experiment
	}
	preflightResult := preflight.Check(ctx, HostPreflightRequest{Backend: request.Backend, Requirements: compiled.DerivedRequirements, RequiredAssets: compiled.RequiredAssets, Capture: compiled.Plan.Capture})
	experiment.PreflightStatus, experiment.RejectionReasons = preflightResult.Status, preflightResult.Reasons
	if preflightResult.Status != PreflightSupported {
		experiment.Outcome = OutcomeHostUnsupported
		return experiment
	}
	candidate := ExecutableCandidate{Strategy: input.strategy, Fingerprint: compiled.StrategyFingerprint, Backend: request.Backend, Plan: compiled.Plan, Assets: resolved, TargetEdges: cloneScopeEdges(scope.Edges)}
	candidate.Plan.EngineArgv = argv
	experiment.ExperimentExecuted = true
	before, err := observeScope(ctx, observer, request.Target, request.NetworkLabel, request.Evidence, policy, scope.Edges, "direct", "")
	if err != nil {
		return inconclusive(experiment, "SERVICE_SCOPE_DIRECT_BEFORE_FAILED", err)
	}
	experiment.DirectBeforeRunIDs = scopeRunIDs(before)
	session := newCandidateSession(ctx, executor)
	defer func() {
		if err := session.Close(); err != nil && !session.closeReported {
			experiment.Outcome = OutcomeLifecycleFailure
			experiment.RejectionReasons = append(experiment.RejectionReasons, Reason{Code: "DEACTIVATION_FAILED", Detail: err.Error()})
		}
	}()
	if err := session.Activate(ctx, candidate); err != nil {
		experiment.Outcome = OutcomeLifecycleFailure
		experiment.RejectionReasons = append(experiment.RejectionReasons, Reason{Code: "ACTIVATION_FAILED", Detail: err.Error()})
		return experiment
	}
	if err := executor.VerifyActive(ctx, candidate); err != nil {
		experiment.Outcome = OutcomeLifecycleFailure
		experiment.RejectionReasons = append(experiment.RejectionReasons, Reason{Code: "ACTIVE_VERIFY_FAILED", Detail: err.Error()})
		return experiment
	}
	active, err := observeScope(ctx, observer, request.Target, request.NetworkLabel, request.Evidence, policy, scope.Edges, "externally_active_profile", input.strategy.ID)
	if err != nil {
		return inconclusive(experiment, "SERVICE_SCOPE_ACTIVE_FAILED", err)
	}
	experiment.ActiveRunIDs = scopeRunIDs(active)
	for _, control := range controls {
		experiment.ControlResults = append(experiment.ControlResults, observeControl(ctx, observer, request, policy, scope.Edges[0].IP, control, input.strategy.ID))
	}
	if controlRegression(experiment.ControlResults) {
		experiment.Outcome = OutcomeRegressionObserved
		experiment.ScopeValidation = ServiceScopeValidation{EdgeCount: len(scope.Edges), Status: ServiceScopeTargetRegression}
		return experiment
	}
	if err := session.Close(); err != nil {
		session.closeReported = true
		experiment.Outcome = OutcomeLifecycleFailure
		experiment.RejectionReasons = append(experiment.RejectionReasons, Reason{Code: "DEACTIVATION_FAILED", Detail: err.Error()})
		return experiment
	}
	if err := executor.EstablishDirect(ctx, snapshot); err != nil {
		experiment.Outcome = OutcomeLifecycleFailure
		experiment.RejectionReasons = append(experiment.RejectionReasons, Reason{Code: "ESTABLISH_DIRECT_AFTER_FAILED", Detail: err.Error()})
		return experiment
	}
	after, err := observeScope(ctx, observer, request.Target, request.NetworkLabel, request.Evidence, policy, scope.Edges, "direct", "")
	if err != nil {
		return inconclusive(experiment, "SERVICE_SCOPE_DIRECT_AFTER_FAILED", err)
	}
	experiment.DirectAfterRunIDs = scopeRunIDs(after)
	evidence := make([]EdgeEvidence, len(scope.Edges))
	for index := range scope.Edges {
		evidence[index] = EdgeEvidence{Edge: scope.Edges[index], DirectBefore: before[index], Active: active[index], DirectAfter: after[index], State: classifyScopeEdge(before[index], active[index], after[index])}
	}
	experiment.ScopeValidation = aggregateServiceScope(evidence)
	switch experiment.ScopeValidation.Status {
	case ServiceScopeVerifiedFixed:
		experiment.Outcome = OutcomeVerifiedFixed
	case ServiceScopeStillFailing:
		experiment.Outcome = OutcomeStillFailing
	case ServiceScopeTargetRegression:
		experiment.Outcome = OutcomeRegressionObserved
	default:
		experiment.Outcome = OutcomeInconclusive
	}
	return experiment
}

func scopeCaptureSupported(capture backendcap.CapturePlan, edges []ServiceScopeEdge) bool {
	for _, edge := range edges {
		if edge.IP == nil || edge.Family == "" || familyForIP(edge.IP) != edge.Family || !captureIncludesFamily(capture.IPFamilies, edge.Family) {
			return false
		}
	}
	return true
}
func cloneScopeEdges(edges []ServiceScopeEdge) []ServiceScopeEdge {
	result := make([]ServiceScopeEdge, len(edges))
	for i, edge := range edges {
		result[i] = ServiceScopeEdge{IP: append(net.IP(nil), edge.IP...), Family: edge.Family}
	}
	return result
}
func scopeRunIDs(values []observatory.ObservationResult) []string {
	result := make([]string, 0, len(values))
	for _, value := range values {
		result = append(result, value.RunID)
	}
	return result
}
func classifyScopeEdge(before, active, after observatory.ObservationResult) EdgeEvidenceState {
	beforeOK, activeOK, afterOK := observationSuccess(before), observationSuccess(active), observationSuccess(after)
	switch {
	case !beforeOK && activeOK && !afterOK:
		return EdgeFixedByProfile
	case beforeOK && activeOK && afterOK:
		return EdgeUnaffectedReachable
	case beforeOK && !activeOK:
		return EdgeTargetRegression
	case !beforeOK && !activeOK && !afterOK:
		return EdgeStillFailing
	default:
		return EdgeInconclusive
	}
}
func selectServiceScopeCandidate(result *Result) {
	for _, experiment := range result.Experiments {
		if experiment.ScopeValidation.Status == ServiceScopeVerifiedFixed {
			result.SelectedStrategyID, result.SelectedFingerprint, result.SelectionReason, result.ServiceScope = experiment.StrategyID, experiment.Fingerprint, "SERVICE_SCOPE_VERIFIED_FIXED", experiment.ScopeValidation
			return
		}
	}
}

var _ = strategyir.Strategy{}
