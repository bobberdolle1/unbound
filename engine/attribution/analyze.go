package attribution

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/url"
	"sort"
	"strings"
	"time"

	"unbound/engine/observatory"
)

const comparableWindow = 30 * time.Minute

// Analyze derives a report from observations for one target and no controls.
func Analyze(observations []observatory.ObservationResult) AttributionReport {
	return AnalyzeCohort(Cohort{Target: observations})
}

// AnalyzeCohort is deterministic and read-only. It never mutates the supplied
// evidence, accesses the network, or controls an externally active profile.
func AnalyzeCohort(cohort Cohort) AttributionReport {
	all := append(append([]observatory.ObservationResult(nil), cohort.Target...), cohort.Controls...)
	report := newReport(all, cohort.Target)
	if len(cohort.Target) == 0 {
		finding := insufficientFinding("No target observations were supplied.", observatory.StageResolve, nil, []string{"target_observation"})
		return completeReport(report, finding, []Finding{finding}, []string{"Attribution requires at least one target ObservationResult."})
	}

	target := cohort.Target[0]
	report.Target = targetRef(target.Target)
	if key, ok := networkContextKey(target); ok {
		report.NetworkContextKey = key
	} else {
		report.Limitations = append(report.Limitations, "Network context identity is incomplete; cross-run confidence cannot be elevated.")
	}

	targets := make([]observatory.ObservationResult, 0, len(cohort.Target))
	for _, observation := range cohort.Target {
		if !sameTarget(target, observation) {
			report.Limitations = append(report.Limitations, "Observation with a different endpoint identity was excluded from target-level comparison.")
			continue
		}
		targets = append(targets, observation)
	}
	if len(targets) == 0 {
		finding := insufficientFinding("No observations match the selected target.", observatory.StageResolve, nil, []string{"matching_target"})
		return completeReport(report, finding, []Finding{finding}, report.Limitations)
	}

	comparableTargets, incompatibleTargets := compatibleTargetObservations(target, targets)
	if incompatibleTargets > 0 {
		report.Limitations = append(report.Limitations, "Target observations outside the compatible network-context and time-window cohort did not elevate confidence.")
	}
	targetFacts := collectFacts(comparableTargets)
	compatibleControls, incompatibleControls := compatibleControlObservations(target, cohort.Controls)
	if len(cohort.Controls) > 0 && len(compatibleControls) == 0 {
		report.Limitations = append(report.Limitations, "No control observation has a compatible network context, address family, platform, and time window.")
	}
	if incompatibleControls > 0 {
		report.Limitations = append(report.Limitations, "Incompatible control observations were not used for target-specific confidence.")
	}
	controlFacts := collectFacts(compatibleControls)
	base := targetFinding(targetFacts, controlFacts)
	report.Limitations = append(report.Limitations, findingLimitations(base)...)

	findings := make([]Finding, 0, 6)
	if control := controlFinding(controlFacts, base.Stage); control != nil {
		findings = append(findings, *control)
	}
	findings = append(findings, base)
	if variability := outcomeVariabilityFinding(targetFacts); variability != nil {
		findings = append(findings, *variability)
	}
	if edge := edgeFinding(targetFacts); edge != nil {
		findings = append(findings, *edge)
	}

	if ab, applicable, limitations := profileFinding(comparableTargets); applicable {
		report.Applicability.ProfileComparison = true
		report.Limitations = append(report.Limitations, limitations...)
		if ab != nil {
			findings = append([]Finding{*ab}, findings...)
		}
	} else if len(limitations) > 0 {
		report.Limitations = append(report.Limitations, limitations...)
	}

	primary := base
	if len(findings) > 0 && isProfileFinding(findings[0].Code) {
		primary = findings[0]
	} else if edge := firstFinding(findings, FindingEdgeDependentFailure); edge != nil {
		primary = *edge
	}
	return completeReport(report, primary, findings, report.Limitations)
}

func newReport(all, targets []observatory.ObservationResult) AttributionReport {
	ids := make([]string, 0, len(all))
	var createdAt time.Time
	for index, observation := range all {
		id := observation.RunID
		if id == "" {
			id = fmt.Sprintf("unknown-%d", index)
		}
		ids = append(ids, id)
		if observation.FinishedAt.After(createdAt) {
			createdAt = observation.FinishedAt.UTC()
		}
	}
	sort.Strings(ids)
	idMaterial := strings.Join(ids, "\n")
	if len(targets) > 0 {
		idMaterial += "\n" + targetIdentity(targets[0].Target)
	}
	digest := sha256.Sum256([]byte(idMaterial))
	return AttributionReport{
		SchemaVersion: SchemaVersion,
		AttributionID: "attribution-v1-" + hex.EncodeToString(digest[:8]),
		CreatedAt:     createdAt,
		InputRunIDs:   ids,
		Applicability: Applicability{ObservationAnalysis: len(targets) > 0},
	}
}

func completeReport(report AttributionReport, primary Finding, findings []Finding, limitations []string) AttributionReport {
	report.PrimaryFinding = normalizeFinding(primary)
	report.Findings = make([]Finding, len(findings))
	for index, finding := range findings {
		report.Findings[index] = normalizeFinding(finding)
	}
	report.Limitations = uniqueStrings(limitations)
	for _, finding := range report.Findings {
		report.EvidenceRefs = append(report.EvidenceRefs, finding.SupportingEvidence...)
		report.Counterevidence = append(report.Counterevidence, finding.Counterevidence...)
	}
	report.EvidenceRefs = uniqueRefs(report.EvidenceRefs)
	report.Counterevidence = uniqueRefs(report.Counterevidence)
	report.Confidence = report.PrimaryFinding.Confidence
	report.Applicability.StrategyAssessment = false
	return report
}

type attemptFact struct {
	ref           EvidenceRef
	boundary      observatory.Stage
	class         observatory.Classification
	success       bool
	validHTTP     bool
	passed        map[observatory.Stage]bool
	resolvedIP    string
	addressFamily observatory.AddressFamily
}

func collectFacts(observations []observatory.ObservationResult) []attemptFact {
	facts := make([]attemptFact, 0)
	for _, observation := range observations {
		if observation.Classification == observatory.ClassQUICUnsupported {
			facts = append(facts, attemptFact{
				ref:        EvidenceRef{RunID: observation.RunID, AttemptIndex: 0, Stage: observatory.StageConnect},
				boundary:   observatory.StageConnect,
				class:      observatory.ClassQUICUnsupported,
				resolvedIP: firstAttemptIP(observation),
			})
			continue
		}
		if len(observation.Attempts) == 0 {
			facts = append(facts, attemptFact{
				ref:      EvidenceRef{RunID: observation.RunID, AttemptIndex: 0, Stage: observation.FinalBoundary},
				boundary: observation.FinalBoundary,
				class:    observation.Classification,
			})
			continue
		}
		for index, attempt := range observation.Attempts {
			facts = append(facts, summarizeAttempt(observation.RunID, index, attempt, observation))
		}
	}
	sort.Slice(facts, func(left, right int) bool {
		if facts[left].ref.RunID != facts[right].ref.RunID {
			return facts[left].ref.RunID < facts[right].ref.RunID
		}
		return facts[left].ref.AttemptIndex < facts[right].ref.AttemptIndex
	})
	return facts
}

func summarizeAttempt(runID string, index int, attempt observatory.ConnectionAttempt, observation observatory.ObservationResult) attemptFact {
	fact := attemptFact{
		ref:           EvidenceRef{RunID: runID, AttemptIndex: index, Stage: observation.FinalBoundary},
		boundary:      observatory.StageResolve,
		class:         observation.Classification,
		passed:        make(map[observatory.Stage]bool),
		resolvedIP:    attempt.ResolvedIP,
		addressFamily: attempt.AddressFamily,
	}
	for _, evidence := range attempt.Stages {
		if evidence.Status == observatory.StatusPass {
			fact.passed[evidence.Stage] = true
		}
		if evidence.Stage == observatory.StageHTTP && evidence.Status == observatory.StatusPass && evidence.PathComplete {
			fact.validHTTP = true
			fact.boundary = observatory.StageHTTP
			fact.ref.Stage = observatory.StageHTTP
			if evidence.HTTPStatus >= 200 && evidence.HTTPStatus < 400 {
				fact.success = true
				fact.class = observatory.ClassSuccess
			} else {
				fact.class = observatory.ClassHTTPStatus
			}
			continue
		}
		if isTerminalStatus(evidence.Status) || evidence.Status == observatory.StatusSkippedUnsupported {
			if stageDepth(evidence.Stage) >= stageDepth(fact.boundary) {
				fact.boundary = evidence.Stage
				fact.ref.Stage = evidence.Stage
				if evidence.Class != "" {
					fact.class = evidence.Class
				}
			}
		}
	}
	if fact.success {
		return fact
	}
	if fact.class == "" {
		fact.class = observation.Classification
	}
	if fact.boundary == "" {
		fact.boundary = observation.FinalBoundary
		fact.ref.Stage = observation.FinalBoundary
	}
	return fact
}

func targetFinding(facts, controlFacts []attemptFact) Finding {
	if len(facts) == 0 {
		return insufficientFinding("No attempt-level evidence is available.", observatory.StageResolve, nil, []string{"attempt_evidence"})
	}
	if quic := factsWithClass(facts, observatory.ClassQUICUnsupported); len(quic) > 0 {
		return Finding{
			Code:                 FindingInsufficientEvidence,
			Kind:                 FindingKindInference,
			Confidence:           ConfidenceLow,
			Summary:              "QUIC observation is unsupported, so no QUIC-path inference is available.",
			Stage:                observatory.StageConnect,
			SupportingEvidence:   refs(quic),
			PrerequisitesMissing: []string{"real_quic_handshake"},
		}
	}
	if allSuccessful(facts) {
		return Finding{
			Code:               FindingNoAnomaly,
			Kind:               FindingKindFact,
			Confidence:         ConfidenceHigh,
			Summary:            "Target observations completed a valid HTTP path successfully.",
			Stage:              observatory.StageHTTP,
			SupportingEvidence: refs(facts),
		}
	}

	candidate := deepestFailure(facts)
	if candidate.boundary == observatory.StageHTTP && candidate.validHTTP {
		return Finding{
			Code:               FindingHTTPApplicationFailure,
			Kind:               FindingKindFact,
			Confidence:         ConfidenceHigh,
			Summary:            "A valid HTTP response completed the network path with a non-success status.",
			Stage:              observatory.StageHTTP,
			SupportingEvidence: refs(sameFailure(facts, candidate)),
			Counterevidence:    refs(otherFacts(facts, candidate)),
			PrerequisitesMet:   []string{"valid_http_response", "path_complete"},
		}
	}
	if candidate.class == observatory.ClassTCPConnectionReset {
		return resetFinding(FindingTCPResetObserved, candidate, facts, "A TCP reset was observed; the evidence does not identify its sender.")
	}
	if candidate.class == observatory.ClassTLSHandshakeReset {
		return resetFinding(FindingTLSResetObserved, candidate, facts, "A TLS handshake reset was observed; the evidence does not identify its sender.")
	}

	code, summary := suspectedFailure(candidate)
	if code == FindingInsufficientEvidence {
		return insufficientFinding("The observed failure does not satisfy an attribution rule.", candidate.boundary, refs(sameFailure(facts, candidate)), []string{"applicable_failure_rule"})
	}
	matching := sameFailure(facts, candidate)
	counter := contradictoryFacts(facts, candidate)
	met, missing := stagePrerequisites(candidate, matching)
	if len(missing) > 0 {
		return insufficientFinding("The failed stage lacks required preceding-stage evidence.", candidate.boundary, refs(matching), missing)
	}
	matchingControls := failedAtStage(controlFacts, candidate.boundary)
	if healthyControlRunCount(controlFacts) == 0 && uniqueRunCount(matchingControls) > 0 {
		return Finding{
			Code:               FindingNetworkContextFailure,
			Kind:               FindingKindInference,
			Confidence:         ConfidenceLow,
			Summary:            "The target and compatible control both failed at the same observed boundary.",
			Stage:              candidate.boundary,
			SupportingEvidence: append(refs(matching), refs(matchingControls)...),
			Counterevidence:    refs(counter),
			PrerequisitesMet:   append(met, "compatible_control_failure"),
		}
	}
	confidence := ConfidenceLow
	if len(counter) == 0 &&
		uniqueRunCount(matching) >= 2 &&
		healthyControlRunCount(controlFacts) >= 2 &&
		uniqueRunCount(matchingControls) == 0 {
		confidence = ConfidenceMedium
		summary += " The boundary repeated across independent target runs while independent compatible controls completed HTTP."
	}
	if len(counter) > 0 {
		summary += " Materially different target outcomes are recorded as counterevidence."
	}
	return Finding{
		Code:                 code,
		Kind:                 FindingKindInference,
		Confidence:           confidence,
		Summary:              summary,
		Stage:                candidate.boundary,
		SupportingEvidence:   refs(matching),
		Counterevidence:      refs(counter),
		PrerequisitesMet:     met,
		PrerequisitesMissing: missing,
	}
}

func resetFinding(code FindingCode, candidate attemptFact, facts []attemptFact, summary string) Finding {
	return Finding{
		Code:               code,
		Kind:               FindingKindFact,
		Confidence:         ConfidenceHigh,
		Summary:            summary,
		Stage:              candidate.boundary,
		SupportingEvidence: refs(sameFailure(facts, candidate)),
		Counterevidence:    refs(otherFacts(facts, candidate)),
	}
}

func insufficientFinding(summary string, stage observatory.Stage, support []EvidenceRef, missing []string) Finding {
	return Finding{
		Code:                 FindingInsufficientEvidence,
		Kind:                 FindingKindInference,
		Confidence:           ConfidenceLow,
		Summary:              summary,
		Stage:                stage,
		SupportingEvidence:   support,
		PrerequisitesMissing: missing,
	}
}

func suspectedFailure(fact attemptFact) (FindingCode, string) {
	switch fact.boundary {
	case observatory.StageResolve:
		return FindingDNSPathFailureSuspected, "DNS resolution did not produce usable evidence for the target."
	case observatory.StageConnect:
		return FindingTCPPathFailureSuspected, "TCP connection did not complete for the target."
	case observatory.StageHandshake:
		return FindingTLSPathFailureSuspected, "TLS handshake did not complete after the connection and ClientHello stages."
	default:
		return FindingInsufficientEvidence, ""
	}
}

func stagePrerequisites(candidate attemptFact, matching []attemptFact) (met, missing []string) {
	switch candidate.boundary {
	case observatory.StageResolve:
		return nil, nil
	case observatory.StageConnect:
		allDNS := true
		for _, fact := range matching {
			allDNS = allDNS && fact.passed[observatory.StageResolve]
		}
		if allDNS {
			return []string{"dns_pass"}, nil
		}
		return nil, []string{"dns_pass"}
	case observatory.StageHandshake:
		allConnect, allHello := true, true
		for _, fact := range matching {
			allConnect = allConnect && fact.passed[observatory.StageConnect]
			allHello = allHello && fact.passed[observatory.StageHello]
		}
		if allConnect {
			met = append(met, "tcp_connect_pass")
		} else {
			missing = append(missing, "tcp_connect_pass")
		}
		if allHello {
			met = append(met, "client_hello_pass")
		} else {
			missing = append(missing, "client_hello_pass")
		}
	}
	return met, missing
}

func findingLimitations(finding Finding) []string {
	switch finding.Code {
	case FindingTLSPathFailureSuspected:
		return []string{"TLS path evidence does not distinguish DPI, a remote edge, routing, a middlebox, local security software, or provider policy."}
	case FindingTCPResetObserved, FindingTLSResetObserved:
		return []string{"A reset observation does not identify the sender or prove packet injection."}
	case FindingInsufficientEvidence:
		return []string{"The available evidence cannot support a more specific mechanism inference."}
	default:
		return nil
	}
}

func failedAtStage(facts []attemptFact, stage observatory.Stage) []attemptFact {
	var result []attemptFact
	for _, fact := range facts {
		if !fact.success && fact.boundary == stage {
			result = append(result, fact)
		}
	}
	return result
}

func controlFinding(facts []attemptFact, targetStage observatory.Stage) *Finding {
	healthy := healthyControlFacts(facts)
	relevantFailures := failedAtStage(facts, targetStage)
	if uniqueRunCount(relevantFailures) > 0 {
		return &Finding{
			Code:               FindingControlPathDegraded,
			Kind:               FindingKindFact,
			Confidence:         ConfidenceLow,
			Summary:            "Compatible controls include a failure at the target-relevant boundary and do not establish a cleanly healthy cohort.",
			Stage:              targetStage,
			SupportingEvidence: append(refs(healthy), refs(relevantFailures)...),
		}
	}
	switch healthyRuns := healthyControlRunCount(facts); {
	case healthyRuns >= 2:
		return &Finding{
			Code:               FindingControlPathHealthy,
			Kind:               FindingKindFact,
			Confidence:         ConfidenceHigh,
			Summary:            "At least two independent compatible control runs completed valid HTTP paths.",
			Stage:              observatory.StageHTTP,
			SupportingEvidence: refs(healthy),
		}
	case healthyRuns == 1:
		return &Finding{
			Code:               FindingControlPathHealthy,
			Kind:               FindingKindFact,
			Confidence:         ConfidenceMedium,
			Summary:            "One compatible control run completed a valid HTTP path.",
			Stage:              observatory.StageHTTP,
			SupportingEvidence: refs(healthy),
		}
	case len(facts) > 0:
		return &Finding{
			Code:               FindingControlPathDegraded,
			Kind:               FindingKindFact,
			Confidence:         ConfidenceLow,
			Summary:            "Compatible control runs did not establish a healthy HTTP path.",
			Stage:              deepestFailure(facts).boundary,
			SupportingEvidence: refs(facts),
		}
	default:
		return nil
	}
}

func edgeFinding(facts []attemptFact) *Finding {
	analysis := analyzeEdgeOutcomes(facts)
	if !analysis.edgeDependent {
		return nil
	}
	return &Finding{
		Code:               FindingEdgeDependentFailure,
		Kind:               FindingKindInference,
		Confidence:         ConfidenceLow,
		Summary:            "Distinct resolved edges consistently produced materially different observed stage outcomes.",
		Stage:              deepestFailure(facts).boundary,
		SupportingEvidence: refs(analysis.stableFacts),
		Counterevidence:    refs(successfulFacts(facts)),
		PrerequisitesMet:   []string{"multiple_resolved_edges", "consistent_edge_outcomes", "materially_different_outcomes"},
	}
}

func outcomeVariabilityFinding(facts []attemptFact) *Finding {
	analysis := analyzeEdgeOutcomes(facts)
	if len(analysis.variableFacts) == 0 {
		return nil
	}
	return &Finding{
		Code:               FindingOutcomeVariability,
		Kind:               FindingKindFact,
		Confidence:         ConfidenceHigh,
		Summary:            "The same resolved edge produced materially different outcomes across independent runs.",
		Stage:              deepestFailure(analysis.variableFacts).boundary,
		SupportingEvidence: refs(analysis.variableFacts),
		PrerequisitesMet:   []string{"same_edge", "independent_runs", "materially_different_outcomes"},
	}
}

func profileFinding(observations []observatory.ObservationResult) (*Finding, bool, []string) {
	direct := make([]observatory.ObservationResult, 0)
	profiles := make([]observatory.ObservationResult, 0)
	for _, observation := range observations {
		switch strings.ToLower(strings.TrimSpace(observation.ExecutionContext.Mode)) {
		case "direct":
			direct = append(direct, observation)
		case "externally_active_profile":
			profiles = append(profiles, observation)
		}
	}
	if len(direct) == 0 || len(profiles) == 0 {
		return nil, false, nil
	}
	sortObservations(direct)
	sortObservations(profiles)
	var limitations []string
	for _, directRun := range direct {
		for _, profileRun := range profiles {
			applicable, missing := ProfileComparisonApplicable(directRun, profileRun)
			if !applicable {
				limitations = append(limitations, "Direct/profile observations were not compared: "+strings.Join(missing, ", ")+".")
				continue
			}
			pairs := pairedEdgeFacts(directRun, profileRun)
			if len(pairs) == 0 {
				limitations = append(limitations, "Direct/profile observations were not compared: same_resolved_edge.")
				continue
			}
			pair := pairs[0]
			code, summary := profileOutcome(pair.direct.success, pair.profile.success)
			return &Finding{
				Code:               code,
				Kind:               FindingKindFact,
				Confidence:         ConfidenceHigh,
				Summary:            summary,
				Stage:              profilePairStage(pair),
				SupportingEvidence: []EvidenceRef{pair.direct.ref, pair.profile.ref},
				PrerequisitesMet:   []string{"same_target", "compatible_network_context", "same_protocol", "close_time_window", "same_resolved_edge"},
			}, true, uniqueStrings(limitations)
		}
	}
	return nil, false, uniqueStrings(limitations)
}

func profileOutcome(directPass, profilePass bool) (FindingCode, string) {
	switch {
	case !directPass && profilePass:
		return FindingFixedByProfile, "Same-edge direct evidence failed while the externally active profile evidence succeeded."
	case directPass && !profilePass:
		return FindingBrokenByProfile, "Same-edge direct evidence succeeded while the externally active profile evidence failed."
	case !directPass && !profilePass:
		return FindingStillFailing, "Same-edge direct and externally active profile evidence both failed."
	default:
		return FindingReachableDirectly, "Same-edge direct and externally active profile evidence both succeeded."
	}
}

// ProfileComparisonApplicable prevents A/B labels across different endpoints,
// network contexts, address families, protocols, time windows, or edges.
func ProfileComparisonApplicable(direct, profile observatory.ObservationResult) (bool, []string) {
	var missing []string
	if !sameTarget(direct, profile) {
		missing = append(missing, "same_target")
	}
	if direct.Target.RequestedProtocol != profile.Target.RequestedProtocol {
		missing = append(missing, "same_protocol")
	}
	if !networkContextCompatible(direct, profile) {
		missing = append(missing, "compatible_network_context")
	}
	if !withinComparableWindow(direct, profile) {
		missing = append(missing, "close_time_window")
	}
	if len(pairedEdgeFacts(direct, profile)) == 0 {
		missing = append(missing, "same_resolved_edge")
	}
	return len(missing) == 0, missing
}

// CouldStrategyAffectFailure only answers structural applicability. It does not
// rank, select, run, credit, or blame a strategy.
func CouldStrategyAffectFailure(capabilities StrategyCapabilities, boundary observatory.Stage, transport observatory.Transport) bool {
	if !supportsTransport(capabilities.Transports, transport) {
		return false
	}
	for _, affected := range capabilities.AffectedStages {
		if affected == affectedStageForBoundary(boundary) {
			return true
		}
		if boundary == observatory.StageHandshake && affected == AffectedStageHello {
			return true
		}
	}
	return false
}

func affectedStageForBoundary(boundary observatory.Stage) AffectedStage {
	switch boundary {
	case observatory.StageResolve:
		return AffectedStageDNS
	case observatory.StageConnect:
		return AffectedStageConnect
	case observatory.StageHello:
		return AffectedStageHello
	case observatory.StageHandshake:
		return AffectedStageHandshake
	case observatory.StageHTTP:
		return AffectedStageHTTP
	default:
		return ""
	}
}

func supportsTransport(transports []observatory.Transport, target observatory.Transport) bool {
	for _, transport := range transports {
		if transport == target {
			return true
		}
	}
	return false
}

func compatibleTargetObservations(reference observatory.ObservationResult, targets []observatory.ObservationResult) ([]observatory.ObservationResult, int) {
	comparable := make([]observatory.ObservationResult, 0, len(targets))
	incompatible := 0
	for _, target := range targets {
		if target.RunID == reference.RunID && target.StartedAt.Equal(reference.StartedAt) {
			comparable = append(comparable, target)
			continue
		}
		if networkContextCompatible(reference, target) && withinComparableWindow(reference, target) {
			comparable = append(comparable, target)
		} else {
			incompatible++
		}
	}
	return comparable, incompatible
}

func compatibleControlObservations(target observatory.ObservationResult, controls []observatory.ObservationResult) ([]observatory.ObservationResult, int) {
	compatible := make([]observatory.ObservationResult, 0, len(controls))
	incompatible := 0
	for _, control := range controls {
		if networkContextCompatible(target, control) && withinComparableWindow(target, control) {
			compatible = append(compatible, control)
		} else {
			incompatible++
		}
	}
	return compatible, incompatible
}

func networkContextCompatible(left, right observatory.ObservationResult) bool {
	leftKey, leftOK := networkContextKey(left)
	rightKey, rightOK := networkContextKey(right)
	if !leftOK || !rightOK || leftKey != rightKey {
		return false
	}
	if left.NetworkContext.Interface != "" || right.NetworkContext.Interface != "" {
		if left.NetworkContext.Interface != right.NetworkContext.Interface {
			return false
		}
	}
	if left.NetworkContext.DefaultGateway != "" || right.NetworkContext.DefaultGateway != "" {
		if left.NetworkContext.DefaultGateway != right.NetworkContext.DefaultGateway {
			return false
		}
	}
	return true
}

func networkContextKey(observation observatory.ObservationResult) (string, bool) {
	label := strings.TrimSpace(observation.NetworkContext.NetworkLabel)
	platform := strings.TrimSpace(observation.Platform)
	family := observation.NetworkContext.AddressFamily
	if family == "" || family == observatory.AddressFamilyAny {
		family = primaryAddressFamily(observation)
	}
	if label == "" || platform == "" || family == "" || family == observatory.AddressFamilyAny {
		return "", false
	}
	return strings.ToLower(platform) + ":" + label + ":" + string(family), true
}

func primaryAddressFamily(observation observatory.ObservationResult) observatory.AddressFamily {
	if observation.PrimaryAttemptIndex != nil && *observation.PrimaryAttemptIndex >= 0 && *observation.PrimaryAttemptIndex < len(observation.Attempts) {
		return observation.Attempts[*observation.PrimaryAttemptIndex].AddressFamily
	}
	if len(observation.Attempts) > 0 {
		return observation.Attempts[0].AddressFamily
	}
	return ""
}

func withinComparableWindow(left, right observatory.ObservationResult) bool {
	if left.StartedAt.IsZero() || right.StartedAt.IsZero() {
		return false
	}
	delta := left.StartedAt.Sub(right.StartedAt)
	if delta < 0 {
		delta = -delta
	}
	return delta <= comparableWindow
}

func sameTarget(left, right observatory.ObservationResult) bool {
	return targetIdentity(left.Target) == targetIdentity(right.Target)
}

func targetIdentity(target observatory.Target) string {
	scheme, hostname, port, path := endpointIdentity(target)
	return strings.Join([]string{scheme, hostname, port, path, string(target.RequestedProtocol)}, "\x00")
}

func targetRef(target observatory.Target) TargetRef {
	scheme, hostname, port, path := endpointIdentity(target)
	return TargetRef{
		Name:              target.Name,
		Scheme:            scheme,
		Hostname:          hostname,
		Port:              port,
		Path:              path,
		RequestedProtocol: target.RequestedProtocol,
	}
}

func endpointIdentity(target observatory.Target) (scheme, hostname, port, path string) {
	hostname = strings.ToLower(strings.TrimSpace(target.Hostname))
	port = strings.TrimSpace(target.Port)
	path = "/"
	parsed, err := url.Parse(strings.TrimSpace(target.URL))
	if err == nil && parsed != nil {
		scheme = strings.ToLower(parsed.Scheme)
		if hostname == "" {
			hostname = strings.ToLower(parsed.Hostname())
		}
		if port == "" {
			port = parsed.Port()
		}
		if parsed.EscapedPath() != "" {
			path = parsed.EscapedPath()
		}
	}
	if port == "" {
		switch scheme {
		case "http":
			port = "80"
		case "https":
			port = "443"
		}
	}
	return scheme, hostname, port, path
}

type edgePair struct {
	key     string
	direct  attemptFact
	profile attemptFact
}

func pairedEdgeFacts(direct, profile observatory.ObservationResult) []edgePair {
	directFacts := factsByEdge(collectFacts([]observatory.ObservationResult{direct}))
	profileFacts := factsByEdge(collectFacts([]observatory.ObservationResult{profile}))
	keys := make([]string, 0)
	for key := range directFacts {
		if _, ok := profileFacts[key]; ok {
			keys = append(keys, key)
		}
	}
	sort.Strings(keys)
	pairs := make([]edgePair, 0, len(keys))
	for _, key := range keys {
		pairs = append(pairs, edgePair{key: key, direct: directFacts[key], profile: profileFacts[key]})
	}
	return pairs
}

func factsByEdge(facts []attemptFact) map[string]attemptFact {
	result := make(map[string]attemptFact)
	for _, fact := range facts {
		key := edgeKey(fact)
		if key == "" {
			continue
		}
		if _, exists := result[key]; !exists {
			result[key] = fact
		}
	}
	return result
}

func edgeKey(fact attemptFact) string {
	if fact.resolvedIP == "" || fact.addressFamily == "" || fact.addressFamily == observatory.AddressFamilyAny {
		return ""
	}
	return fact.resolvedIP + "\x00" + string(fact.addressFamily)
}

func profilePairStage(pair edgePair) observatory.Stage {
	if !pair.profile.success {
		return pair.profile.boundary
	}
	return pair.direct.boundary
}

type edgeOutcomeAnalysis struct {
	edgeDependent bool
	stableFacts   []attemptFact
	variableFacts []attemptFact
}

func analyzeEdgeOutcomes(facts []attemptFact) edgeOutcomeAnalysis {
	type record struct {
		facts      []attemptFact
		runIDs     map[string]struct{}
		signatures map[string]struct{}
	}
	records := make(map[string]*record)
	for _, fact := range facts {
		key := edgeKey(fact)
		if key == "" {
			continue
		}
		if records[key] == nil {
			records[key] = &record{runIDs: make(map[string]struct{}), signatures: make(map[string]struct{})}
		}
		records[key].facts = append(records[key].facts, fact)
		records[key].runIDs[fact.ref.RunID] = struct{}{}
		records[key].signatures[factSignature(fact)] = struct{}{}
	}

	keys := make([]string, 0, len(records))
	for key := range records {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	analysis := edgeOutcomeAnalysis{}
	stableSignatures := make(map[string]struct{})
	for _, key := range keys {
		record := records[key]
		switch len(record.signatures) {
		case 1:
			analysis.stableFacts = append(analysis.stableFacts, record.facts...)
			for signature := range record.signatures {
				stableSignatures[signature] = struct{}{}
			}
		default:
			if len(record.runIDs) >= 2 {
				analysis.variableFacts = append(analysis.variableFacts, record.facts...)
			}
			// Multiple outcomes on one edge, even inside one run, make the
			// edge identity insufficient to explain the cohort difference.
			return analysis
		}
	}
	analysis.edgeDependent = len(analysis.stableFacts) > 0 && len(stableSignatures) > 1 && len(keys) >= 2
	return analysis
}

func factSignature(fact attemptFact) string {
	if fact.success {
		return "success"
	}
	return string(fact.boundary) + ":" + string(fact.class)
}

func deepestFailure(facts []attemptFact) attemptFact {
	best := facts[0]
	for _, fact := range facts[1:] {
		if fact.success {
			continue
		}
		if best.success || stageDepth(fact.boundary) > stageDepth(best.boundary) || (stageDepth(fact.boundary) == stageDepth(best.boundary) && factSignature(fact) < factSignature(best)) {
			best = fact
		}
	}
	return best
}

func stageDepth(stage observatory.Stage) int {
	switch stage {
	case observatory.StageResolve:
		return 1
	case observatory.StageConnect:
		return 2
	case observatory.StageHello:
		return 3
	case observatory.StageHandshake:
		return 4
	case observatory.StageHTTP:
		return 5
	case observatory.StageCarry:
		return 6
	default:
		return 0
	}
}

func isTerminalStatus(status observatory.Status) bool {
	switch status {
	case observatory.StatusFail, observatory.StatusTimeout, observatory.StatusReset, observatory.StatusCancelled:
		return true
	default:
		return false
	}
}

func allSuccessful(facts []attemptFact) bool {
	return len(facts) > 0 && len(successfulFacts(facts)) == len(facts)
}

func successfulFacts(facts []attemptFact) []attemptFact {
	result := make([]attemptFact, 0, len(facts))
	for _, fact := range facts {
		if fact.success {
			result = append(result, fact)
		}
	}
	return result
}

func healthyControlFacts(facts []attemptFact) []attemptFact { return successfulFacts(facts) }

func healthyControlRunCount(facts []attemptFact) int {
	return uniqueRunCount(healthyControlFacts(facts))
}

func uniqueRunCount(facts []attemptFact) int {
	runs := make(map[string]struct{})
	for _, fact := range facts {
		runID := fact.ref.RunID
		if runID == "" {
			// Missing run identity cannot establish independence.
			runID = "unknown"
		}
		runs[runID] = struct{}{}
	}
	return len(runs)
}

func factsWithClass(facts []attemptFact, class observatory.Classification) []attemptFact {
	var result []attemptFact
	for _, fact := range facts {
		if fact.class == class {
			result = append(result, fact)
		}
	}
	return result
}

func sameFailure(facts []attemptFact, candidate attemptFact) []attemptFact {
	var result []attemptFact
	for _, fact := range facts {
		if !fact.success && fact.boundary == candidate.boundary && fact.class == candidate.class {
			result = append(result, fact)
		}
	}
	return result
}

func otherFacts(facts []attemptFact, candidate attemptFact) []attemptFact {
	var result []attemptFact
	for _, fact := range facts {
		if fact.success || fact.boundary != candidate.boundary || fact.class != candidate.class {
			result = append(result, fact)
		}
	}
	return result
}

func contradictoryFacts(facts []attemptFact, candidate attemptFact) []attemptFact {
	var result []attemptFact
	candidateSignature := factSignature(candidate)
	for _, fact := range facts {
		if fact.success || fact.validHTTP || factSignature(fact) != candidateSignature {
			result = append(result, fact)
		}
	}
	return result
}

func refs(facts []attemptFact) []EvidenceRef {
	result := make([]EvidenceRef, 0, len(facts))
	for _, fact := range facts {
		result = append(result, fact.ref)
	}
	return uniqueRefs(result)
}

func runReference(observation observatory.ObservationResult) EvidenceRef {
	stage := observation.FinalBoundary
	if stage == "" {
		stage = observatory.StageResolve
	}
	return EvidenceRef{RunID: observation.RunID, AttemptIndex: 0, Stage: stage}
}

func firstAttemptIP(observation observatory.ObservationResult) string {
	if len(observation.Attempts) > 0 {
		return observation.Attempts[0].ResolvedIP
	}
	return ""
}

func firstFinding(findings []Finding, code FindingCode) *Finding {
	for index := range findings {
		if findings[index].Code == code {
			return &findings[index]
		}
	}
	return nil
}

func isProfileFinding(code FindingCode) bool {
	switch code {
	case FindingFixedByProfile, FindingBrokenByProfile, FindingStillFailing, FindingReachableDirectly:
		return true
	default:
		return false
	}
}

func sortObservations(observations []observatory.ObservationResult) {
	sort.Slice(observations, func(left, right int) bool {
		if observations[left].RunID != observations[right].RunID {
			return observations[left].RunID < observations[right].RunID
		}
		return observations[left].StartedAt.Before(observations[right].StartedAt)
	})
}

func normalizeFinding(finding Finding) Finding {
	finding.SupportingEvidence = uniqueRefs(finding.SupportingEvidence)
	finding.Counterevidence = uniqueRefs(finding.Counterevidence)
	finding.PrerequisitesMet = uniqueStrings(finding.PrerequisitesMet)
	finding.PrerequisitesMissing = uniqueStrings(finding.PrerequisitesMissing)
	return finding
}

func uniqueRefs(values []EvidenceRef) []EvidenceRef {
	if len(values) == 0 {
		return nil
	}
	sort.Slice(values, func(left, right int) bool {
		if values[left].RunID != values[right].RunID {
			return values[left].RunID < values[right].RunID
		}
		if values[left].AttemptIndex != values[right].AttemptIndex {
			return values[left].AttemptIndex < values[right].AttemptIndex
		}
		return values[left].Stage < values[right].Stage
	})
	result := values[:0]
	for _, value := range values {
		if len(result) == 0 || result[len(result)-1] != value {
			result = append(result, value)
		}
	}
	return result
}

func uniqueStrings(values []string) []string {
	if len(values) == 0 {
		return nil
	}
	sort.Strings(values)
	result := values[:0]
	for _, value := range values {
		if value != "" && (len(result) == 0 || result[len(result)-1] != value) {
			result = append(result, value)
		}
	}
	return result
}
