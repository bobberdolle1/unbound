package autotunevnext

import (
	"context"
	"net"
	"sort"
	"strings"
	"time"

	"unbound/engine/attribution"
	"unbound/engine/backendcap"
	"unbound/engine/observatory"
	"unbound/engine/planner"
	"unbound/engine/strategyir"
)

// Run is the pure orchestration layer apart from its injected boundaries. It
// never starts a provider, changes routes, or selects a legacy profile.
func Run(ctx context.Context, request Request, observer Observer, executor Executor, preflight HostPreflight, assets AssetResolver) (result Result) {
	result = Result{SchemaVersion: SchemaVersion, Target: request.Target, Backend: request.Backend}
	if observer == nil || executor == nil || preflight == nil || assets == nil {
		result.Status = StatusPreflightFailed
		result.Limitations = []string{"AUTOTUNE_VNEXT_DEPENDENCY_MISSING"}
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

	baseline, err := observe(experimentCtx, observer, request.Target, request.NetworkLabel, request.Evidence, policy, nil, "direct", "")
	if err != nil {
		return observationFailure(result, ctx, experimentCtx, "BASELINE_OBSERVATION_FAILED", err)
	}
	controlBaselines := make([]controlBaseline, 0, len(request.Controls))
	for _, control := range request.Controls {
		observation, observeErr := observe(experimentCtx, observer, control, request.NetworkLabel, request.Evidence, policy, nil, "direct", "")
		if observeErr != nil {
			return observationFailure(result, ctx, experimentCtx, "CONTROL_BASELINE_OBSERVATION_FAILED", observeErr)
		}
		controlBaselines = append(controlBaselines, controlBaseline{target: control, observation: observation, edge: selectedEdge(observation)})
	}
	result.BaselineAttribution = attribution.AnalyzeCohort(attribution.Cohort{Target: []observatory.ObservationResult{baseline}, Controls: observationsFromControls(controlBaselines)})
	scope := scopeForBaseline(request.ScopeSnapshot, baseline)
	result.PlannerReport = planner.Plan(planner.Request{Attribution: result.BaselineAttribution, Backend: request.Backend, Strategies: request.Strategies, Scope: scope, Evidence: planner.EvidenceContext{AddressFamily: selectedFamily(baseline)}})
	result.Limitations = append(result.Limitations, result.PlannerReport.Limitations...)

	switch result.PlannerReport.Disposition {
	case planner.DispositionNoActionNeeded:
		result.Status = StatusCompletedNoActionNeeded
		return result
	case planner.DispositionNoPacketStrategyIndicated, planner.DispositionInsufficientEvidence:
		result.Status = StatusInconclusive
		return result
	case planner.DispositionNoCompatibleCandidates:
		for _, input := range candidateInputs(result.PlannerReport, request.Strategies) {
			result.Experiments = append(result.Experiments, notRun(input.assessment, input.strategy, OutcomeNotRunPolicy, "PLANNER_NOT_ELIGIBLE"))
		}
		result.Status = StatusCompletedNoEligibleCandidates
		return result
	case planner.DispositionCandidatesAvailable:
	default:
		result.Status = StatusInconclusive
		return result
	}

	edge := selectedEdge(baseline)
	if edge == nil {
		result.Status = StatusInconclusive
		result.Limitations = append(result.Limitations, "SAME_EDGE_UNAVAILABLE: baseline has no concrete resolved edge.")
		return result
	}
	candidates := candidateInputs(result.PlannerReport, request.Strategies)
	for i := range candidates {
		input := candidates[i]
		if experimentCtx.Err() != nil {
			result.Experiments = append(result.Experiments, notRun(input.assessment, input.strategy, OutcomeNotRunBudget, "DURATION_BUDGET_EXHAUSTED"))
			continue
		}
		if input.assessment.Status != planner.StatusEligible {
			result.Experiments = append(result.Experiments, notRun(input.assessment, input.strategy, OutcomeNotRunPolicy, "PLANNER_NOT_ELIGIBLE"))
			continue
		}
		runCount := executedCount(result.Experiments)
		if runCount >= policy.MaxCandidates {
			result.Experiments = append(result.Experiments, notRun(input.assessment, input.strategy, OutcomeNotRunBudget, "CANDIDATE_BUDGET_EXHAUSTED"))
			continue
		}
		experiment := runCandidate(experimentCtx, request, policy, observer, executor, preflight, assets, snapshot, *edge, controlBaselines, input)
		result.Experiments = append(result.Experiments, experiment)
		if experiment.Outcome == OutcomeLifecycleFailure {
			result.Status = StatusLifecycleFailed
			return result
		}
		if ctx.Err() != nil {
			result.Status = StatusCancelled
			return result
		}
	}
	selectCandidate(&result)
	if result.SelectedStrategyID != "" {
		result.Status = StatusCompletedSelected
	} else if executedCount(result.Experiments) == 0 && anyPreflightFailure(result.Experiments) {
		result.Status = StatusPreflightFailed
	} else {
		result.Status = StatusCompletedNoVerifiedCandidate
	}
	return result
}

type controlBaseline struct {
	target      Target
	observation observatory.ObservationResult
	edge        *net.IP
}

// candidateSession owns exactly one candidate teardown attempt after Activate
// has been called, including partial activation failures.
type candidateSession struct {
	executor      Executor
	parent        context.Context
	activated     bool
	closed        bool
	closeErr      error
	closeReported bool
}

func newCandidateSession(parent context.Context, executor Executor) *candidateSession {
	return &candidateSession{parent: parent, executor: executor}
}

func (s *candidateSession) Activate(ctx context.Context, candidate ExecutableCandidate) error {
	s.activated = true
	return s.executor.Activate(ctx, candidate)
}

func (s *candidateSession) Close() error {
	if !s.activated {
		return nil
	}
	if s.closed {
		return s.closeErr
	}
	s.closed = true
	cleanupCtx, cancel := context.WithTimeout(context.WithoutCancel(s.parent), 30*time.Second)
	defer cancel()
	s.closeErr = s.executor.Deactivate(cleanupCtx)
	return s.closeErr
}

func scopeForBaseline(caller planner.ScopeSnapshot, baseline observatory.ObservationResult) planner.ScopeSnapshot {
	scope := planner.ScopeSnapshot{
		HostListMembers: cloneScopeMembers(caller.HostListMembers),
		IPSetMembers:    cloneScopeMembers(caller.IPSetMembers),
	}
	if edge := selectedEdge(baseline); edge != nil {
		scope.TargetEdgeIPs = []string{edge.String()}
	}
	return scope
}

func cloneScopeMembers(source map[string][]string) map[string][]string {
	if source == nil {
		return nil
	}
	cloned := make(map[string][]string, len(source))
	for id, members := range source {
		cloned[id] = append([]string(nil), members...)
	}
	return cloned
}

type candidateInput struct {
	assessment planner.CandidateAssessment
	strategy   strategyir.Strategy
}

func candidateInputs(report planner.PlannerReport, strategies []strategyir.Strategy) []candidateInput {
	byFingerprint := map[string]strategyir.Strategy{}
	byID := map[string]strategyir.Strategy{}
	for _, strategy := range strategies {
		byID[strategy.ID] = strategy
		fingerprint, err := strategyir.Fingerprint(strategy)
		if err == nil {
			if _, exists := byFingerprint[fingerprint]; !exists {
				byFingerprint[fingerprint] = strategy
			}
		}
	}
	inputs := make([]candidateInput, 0, len(report.Candidates))
	seen := map[string]bool{}
	for _, assessment := range report.Candidates {
		key := assessment.StrategyFingerprint
		strategy, ok := byFingerprint[key]
		if key == "" || !ok {
			key = "invalid:" + assessment.StrategyID
			strategy = byID[assessment.StrategyID]
		}
		if seen[key] {
			continue
		}
		seen[key] = true
		inputs = append(inputs, candidateInput{assessment: assessment, strategy: strategy})
	}
	sort.Slice(inputs, func(i, j int) bool { return safetyLess(inputs[i], inputs[j]) })
	return inputs
}

func safetyLess(left, right candidateInput) bool {
	a, b := left.strategy.Safety, right.strategy.Safety
	if a.TargetOnly != b.TargetOnly {
		return a.TargetOnly
	}
	if a.Experimental != b.Experimental {
		return !a.Experimental
	}
	if a.MayAffectSteam != b.MayAffectSteam {
		return !a.MayAffectSteam
	}
	if a.MayAffectTLS != b.MayAffectTLS {
		return !a.MayAffectTLS
	}
	if aggressivenessRank(a.Aggressiveness) != aggressivenessRank(b.Aggressiveness) {
		return aggressivenessRank(a.Aggressiveness) < aggressivenessRank(b.Aggressiveness)
	}
	if left.assessment.StrategyID != right.assessment.StrategyID {
		return left.assessment.StrategyID < right.assessment.StrategyID
	}
	return left.assessment.StrategyFingerprint < right.assessment.StrategyFingerprint
}
func aggressivenessRank(value string) int {
	switch value {
	case "LOW":
		return 0
	case "MEDIUM":
		return 1
	case "HIGH":
		return 2
	case "EXPERIMENTAL":
		return 3
	default:
		return 4
	}
}

func runCandidate(ctx context.Context, request Request, policy Policy, observer Observer, executor Executor, preflight HostPreflight, assets AssetResolver, snapshot StateSnapshot, edge net.IP, controls []controlBaseline, input candidateInput) (experiment CandidateExperiment) {
	assessment, strategy := input.assessment, input.strategy
	experiment = CandidateExperiment{StrategyID: assessment.StrategyID, Fingerprint: assessment.StrategyFingerprint, PlannerStatus: assessment.Status, CompileStatus: assessment.CompileStatus, PreflightStatus: PreflightNotRun, Safety: assessment.Safety}
	compiled := backendcap.Compile(strategy, request.Backend)
	experiment.CompileStatus = compiled.Status
	if compiled.Status != backendcap.StatusCompiled {
		experiment.Outcome = OutcomeBackendUnsupported
		experiment.PreflightStatus = PreflightUnsupported
		experiment.RejectionReasons = []Reason{{Code: "COMPILE_NOT_COMPILED"}}
		return experiment
	}
	if compiled.Backend != request.Backend || compiled.StrategyFingerprint != assessment.StrategyFingerprint {
		experiment.Outcome = OutcomeHostUnsupported
		experiment.PreflightStatus = PreflightError
		experiment.RejectionReasons = []Reason{{Code: "COMPILER_PLANNER_MISMATCH"}}
		return experiment
	}
	if measurement := MacOSMeasurementPath(request.Backend); measurement.Status != PreflightSupported {
		experiment.Outcome = OutcomeHostUnsupported
		experiment.PreflightStatus = measurement.Status
		experiment.RejectionReasons = measurement.Reasons
		return experiment
	}
	resolved, err := assets.Resolve(ctx, request.Backend, compiled.RequiredAssets)
	if err != nil {
		experiment.Outcome = OutcomeHostUnsupported
		experiment.PreflightStatus = PreflightError
		experiment.RejectionReasons = []Reason{{Code: "MISSING_ASSET", Detail: err.Error()}}
		return experiment
	}
	if !resolvedAssetsCover(compiled.RequiredAssets, resolved) {
		experiment.Outcome = OutcomeHostUnsupported
		experiment.PreflightStatus = PreflightError
		experiment.RejectionReasons = []Reason{{Code: "INVALID_RESOLVED_ASSET", Detail: "resolver did not return every complete trusted logical asset"}}
		return experiment
	}
	materializedArgv, err := MaterializeEngineArgv(compiled.Plan.EngineArgv, resolved)
	if err != nil {
		experiment.Outcome = OutcomeHostUnsupported
		experiment.PreflightStatus = PreflightError
		experiment.RejectionReasons = []Reason{{Code: materializationReason(err), Detail: err.Error()}}
		return experiment
	}
	preflightResult := preflight.Check(ctx, HostPreflightRequest{Backend: request.Backend, Requirements: compiled.DerivedRequirements, RequiredAssets: compiled.RequiredAssets, Capture: compiled.Plan.Capture})
	experiment.PreflightStatus, experiment.RejectionReasons = preflightResult.Status, preflightResult.Reasons
	if preflightResult.Status != PreflightSupported {
		experiment.Outcome = OutcomeHostUnsupported
		return experiment
	}
	plan := compiled.Plan
	plan.EngineArgv = materializedArgv
	candidate := ExecutableCandidate{Strategy: strategy, Fingerprint: compiled.StrategyFingerprint, Backend: request.Backend, Plan: plan, Assets: resolved}
	experiment.ExperimentExecuted = true
	before, err := observe(ctx, observer, request.Target, request.NetworkLabel, request.Evidence, policy, &edge, "direct", "")
	if err != nil {
		return inconclusive(experiment, "DIRECT_BEFORE_OBSERVATION_FAILED", err)
	}
	experiment.DirectBeforeRunIDs = []string{before.RunID}
	if !sameEdge(before, edge) {
		experiment.Outcome = OutcomeInconclusive
		experiment.Limitations = append(experiment.Limitations, "SAME_EDGE_REQUIRED")
		return experiment
	}
	session := newCandidateSession(ctx, executor)
	defer func() {
		if err := session.Close(); err != nil && !session.closeReported {
			session.closeReported = true
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
	activeObservation, err := observe(ctx, observer, request.Target, request.NetworkLabel, request.Evidence, policy, &edge, "externally_active_profile", strategy.ID)
	if err != nil {
		return inconclusive(experiment, "ACTIVE_OBSERVATION_FAILED", err)
	}
	experiment.ActiveRunIDs = []string{activeObservation.RunID}
	if !sameEdge(activeObservation, edge) || !sameFamily(before, activeObservation) {
		experiment.Outcome = OutcomeInconclusive
		experiment.Limitations = append(experiment.Limitations, "SAME_EDGE_OR_ADDRESS_FAMILY_REQUIRED")
		return experiment
	}
	for _, control := range controls {
		experiment.ControlResults = append(experiment.ControlResults, observeControl(ctx, observer, request, policy, edge, control, strategy.ID))
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
	after, err := observe(ctx, observer, request.Target, request.NetworkLabel, request.Evidence, policy, &edge, "direct", "")
	if err != nil {
		return inconclusive(experiment, "DIRECT_AFTER_OBSERVATION_FAILED", err)
	}
	experiment.DirectAfterRunIDs = []string{after.RunID}
	experiment.Attribution = attribution.Analyze([]observatory.ObservationResult{before, activeObservation, after})
	if !sameEdge(after, edge) || !sameFamily(before, after) {
		experiment.Outcome = OutcomeInconclusive
		experiment.Limitations = append(experiment.Limitations, "SAME_EDGE_OR_ADDRESS_FAMILY_REQUIRED")
		return experiment
	}
	if !profileApplicable(before, activeObservation) || !profileApplicable(after, activeObservation) {
		experiment.Outcome = OutcomeInconclusive
		experiment.Limitations = append(experiment.Limitations, "PROFILE_COMPARISON_INAPPLICABLE")
		return experiment
	}
	if controlRegression(experiment.ControlResults) {
		experiment.Outcome = OutcomeRegressionObserved
		return experiment
	}
	beforeOK, activeOK, afterOK := observationSuccess(before), observationSuccess(activeObservation), observationSuccess(after)
	switch {
	case beforeOK && !activeOK:
		experiment.Outcome = OutcomeRegressionObserved
	case !beforeOK && activeOK && !afterOK && hasFinding(experiment.Attribution, attribution.FindingFixedByProfile):
		experiment.Outcome = OutcomeVerifiedFixed
	case !beforeOK && !activeOK && !afterOK:
		experiment.Outcome = OutcomeStillFailing
	case !beforeOK && activeOK && afterOK:
		experiment.Outcome = OutcomeDirectBecameReachable
	default:
		experiment.Outcome = OutcomeInconclusive
	}
	return experiment
}

func observeControl(ctx context.Context, observer Observer, request Request, policy Policy, targetEdge net.IP, baseline controlBaseline, strategyID string) ControlResult {
	result := ControlResult{Target: baseline.target, DirectRunID: baseline.observation.RunID}
	if baseline.edge == nil {
		result.Outcome = OutcomeInconclusive
		result.Reasons = []Reason{{Code: "CONTROL_EDGE_UNAVAILABLE"}}
		return result
	}
	active, err := observe(ctx, observer, baseline.target, request.NetworkLabel, request.Evidence, policy, baseline.edge, "externally_active_profile", strategyID)
	if err != nil {
		result.Outcome = OutcomeInconclusive
		result.Reasons = []Reason{{Code: "CONTROL_ACTIVE_OBSERVATION_FAILED", Detail: err.Error()}}
		return result
	}
	result.ActiveRunID = active.RunID
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

func observe(ctx context.Context, observer Observer, target Target, label string, evidence EvidenceOptions, policy Policy, ip *net.IP, mode, strategyID string) (observatory.ObservationResult, error) {
	observationCtx, cancel := context.WithTimeout(ctx, policy.PerObservationTimeout)
	defer cancel()
	options := observatory.Options{Timeouts: evidence.Timeouts, AddressFamily: target.AddressFamily, Transport: target.Transport, NetworkLabel: label}
	if ip != nil {
		options.ResolvedIP = append(net.IP(nil), (*ip)...)
	}
	result, err := observer.Observe(observationCtx, target.URL, options)
	result.ExecutionContext = observatory.ExecutionContext{Mode: mode, StrategyID: strategyID}
	return result, err
}

func selectedEdge(observation observatory.ObservationResult) *net.IP {
	if observation.PrimaryAttemptIndex != nil && *observation.PrimaryAttemptIndex >= 0 && *observation.PrimaryAttemptIndex < len(observation.Attempts) {
		ip := net.ParseIP(observation.Attempts[*observation.PrimaryAttemptIndex].ResolvedIP)
		if ip != nil {
			return &ip
		}
	}
	for _, attempt := range observation.Attempts {
		ip := net.ParseIP(attempt.ResolvedIP)
		if ip != nil {
			return &ip
		}
	}
	return nil
}
func selectedFamily(observation observatory.ObservationResult) observatory.AddressFamily {
	if observation.PrimaryAttemptIndex != nil && *observation.PrimaryAttemptIndex >= 0 && *observation.PrimaryAttemptIndex < len(observation.Attempts) {
		return observation.Attempts[*observation.PrimaryAttemptIndex].AddressFamily
	}
	return ""
}
func sameEdge(observation observatory.ObservationResult, expected net.IP) bool {
	if observation.PrimaryAttemptIndex == nil {
		return false
	}
	index := *observation.PrimaryAttemptIndex
	if index < 0 || index >= len(observation.Attempts) {
		return false
	}
	attempt := observation.Attempts[index]
	actual := net.ParseIP(attempt.ResolvedIP)
	return actual != nil && actual.Equal(expected) && attempt.AddressFamily == familyForIP(expected)
}

func familyForIP(ip net.IP) observatory.AddressFamily {
	if ip.To4() != nil {
		return observatory.AddressFamilyIPv4
	}
	if ip.To16() != nil {
		return observatory.AddressFamilyIPv6
	}
	return ""
}
func sameFamily(left, right observatory.ObservationResult) bool {
	return selectedFamily(left) != "" && selectedFamily(left) == selectedFamily(right)
}
func profileApplicable(direct, active observatory.ObservationResult) bool {
	ok, _ := attribution.ProfileComparisonApplicable(direct, active)
	return ok
}
func observationSuccess(observation observatory.ObservationResult) bool {
	report := attribution.Analyze([]observatory.ObservationResult{observation})
	return hasFinding(report, attribution.FindingNoAnomaly) || hasFinding(report, attribution.FindingReachableDirectly)
}
func hasFinding(report attribution.AttributionReport, code attribution.FindingCode) bool {
	for _, finding := range report.Findings {
		if finding.Code == code {
			return true
		}
	}
	return false
}
func observationsFromControls(controls []controlBaseline) []observatory.ObservationResult {
	out := make([]observatory.ObservationResult, 0, len(controls))
	for _, control := range controls {
		out = append(out, control.observation)
	}
	return out
}
func controlRegression(controls []ControlResult) bool {
	for _, control := range controls {
		if control.Outcome == OutcomeRegressionObserved {
			return true
		}
	}
	return false
}
func executedCount(experiments []CandidateExperiment) int {
	count := 0
	for _, experiment := range experiments {
		if experiment.ExperimentExecuted {
			count++
		}
	}
	return count
}
func anyPreflightFailure(experiments []CandidateExperiment) bool {
	for _, experiment := range experiments {
		if experiment.PreflightStatus == PreflightUnsupported || experiment.PreflightStatus == PreflightError {
			return true
		}
	}
	return false
}
func notRun(assessment planner.CandidateAssessment, strategy strategyir.Strategy, outcome Outcome, reason string) CandidateExperiment {
	reasons := make([]Reason, 0, len(assessment.Reasons)+1)
	for _, plannerReason := range assessment.Reasons {
		reasons = append(reasons, Reason{Code: plannerReason.Code, Detail: plannerReason.Detail})
	}
	reasons = append(reasons, Reason{Code: reason})
	safety := assessment.Safety
	if safety.Aggressiveness == "" {
		safety = strategy.Safety
	}
	return CandidateExperiment{StrategyID: assessment.StrategyID, Fingerprint: assessment.StrategyFingerprint, PlannerStatus: assessment.Status, CompileStatus: assessment.CompileStatus, PreflightStatus: PreflightNotRun, Outcome: outcome, Safety: safety, RejectionReasons: reasons}
}
func resolvedAssetsCover(required []string, resolved []ResolvedAsset) bool {
	found := make(map[string]struct{}, len(resolved))
	for _, asset := range resolved {
		if asset.ID == "" || asset.Kind == "" || asset.EngineValue == "" {
			return false
		}
		found[asset.ID] = struct{}{}
	}
	for _, id := range required {
		if _, ok := found[id]; !ok {
			return false
		}
	}
	return true
}

func materializationReason(err error) string {
	if strings.HasPrefix(err.Error(), "MISSING_ASSET:") {
		return "MISSING_ASSET"
	}
	return "INVALID_RESOLVED_ASSET"
}
func inconclusive(experiment CandidateExperiment, code string, err error) CandidateExperiment {
	experiment.Outcome = OutcomeInconclusive
	experiment.RejectionReasons = append(experiment.RejectionReasons, Reason{Code: code, Detail: err.Error()})
	return experiment
}
func lifecycleFailure(result Result, code string, err error) Result {
	result.Status = StatusLifecycleFailed
	result.Lifecycle.Errors = append(result.Lifecycle.Errors, Reason{Code: code, Detail: err.Error()})
	return result
}
func observationFailure(result Result, parent, experiment context.Context, code string, err error) Result {
	result.Lifecycle.Errors = append(result.Lifecycle.Errors, Reason{Code: code, Detail: err.Error()})
	if parent.Err() != nil {
		result.Status = StatusCancelled
	} else if experiment.Err() != nil {
		result.Status = StatusInconclusive
	} else {
		result.Status = StatusInconclusive
	}
	return result
}
func finalizeRestore(ctx context.Context, executor Executor, snapshot StateSnapshot, result *Result) {
	result.Lifecycle.RestoreAttempted = true
	restoreCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 30*time.Second)
	defer cancel()
	if err := executor.Restore(restoreCtx, snapshot); err != nil {
		result.Status = StatusStateRestoreFailed
		result.SelectedStrategyID = ""
		result.SelectedFingerprint = ""
		result.SelectionReason = ""
		result.Lifecycle.Errors = append(result.Lifecycle.Errors, Reason{Code: "RESTORE_FAILED", Detail: err.Error()})
		return
	}
	if err := executor.VerifyRestored(restoreCtx, snapshot); err != nil {
		result.Status = StatusStateRestoreFailed
		result.SelectedStrategyID = ""
		result.SelectedFingerprint = ""
		result.SelectionReason = ""
		result.Lifecycle.Errors = append(result.Lifecycle.Errors, Reason{Code: "RESTORE_VERIFY_FAILED", Detail: err.Error()})
		return
	}
	result.Lifecycle.RestoreVerified = true
	result.StateRestored = true
}

func selectCandidate(result *Result) {
	verified := make([]CandidateExperiment, 0)
	for _, experiment := range result.Experiments {
		if experiment.Outcome == OutcomeVerifiedFixed {
			verified = append(verified, experiment)
		}
	}
	if len(verified) == 0 {
		return
	}
	sort.Slice(verified, func(i, j int) bool { return safetyExperimentLess(verified[i], verified[j]) })
	result.SelectedStrategyID = verified[0].StrategyID
	result.SelectedFingerprint = verified[0].Fingerprint
	result.SelectionReason = "Selected by deterministic safety-first policy after same-edge direct failure, active success, direct re-failure, and no protected-control regression."
}
func safetyExperimentLess(left, right CandidateExperiment) bool {
	return safetyLess(candidateInput{assessment: planner.CandidateAssessment{StrategyID: left.StrategyID, StrategyFingerprint: left.Fingerprint}, strategy: strategyir.Strategy{Safety: left.Safety}}, candidateInput{assessment: planner.CandidateAssessment{StrategyID: right.StrategyID, StrategyFingerprint: right.Fingerprint}, strategy: strategyir.Strategy{Safety: right.Safety}})
}
