package attribution

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
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
			report.Limitations = append(report.Limitations, "Observation with a different target was excluded from target-level comparison.")
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

	findings := make([]Finding, 0, 5)
	if control := controlFinding(controlFacts); control != nil {
		findings = append(findings, *control)
	}

	base := targetFinding(targetFacts, controlFacts)
	report.Limitations = append(report.Limitations, findingLimitations(base)...)
	findings = append(findings, base)
	if edge := edgeFinding(comparableTargets, targetFacts); edge != nil {
		findings = append(findings, *edge)
	}

	if ab, applicable, limitations := profileFinding(targets); applicable {
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
	counter := contradictoryFacts(facts)
	met, missing := stagePrerequisites(candidate, matching)
	if len(missing) > 0 {
		return insufficientFinding("The failed stage lacks required preceding-stage evidence.", candidate.boundary, refs(matching), missing)
	}
	if healthyControlCount(controlFacts) == 0 {
		if matchingControls := failedAtStage(controlFacts, candidate.boundary); len(matchingControls) > 0 {
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
	}
	confidence := ConfidenceLow
	if len(counter) == 0 && len(matching) >= 2 && healthyControlCount(controlFacts) >= 2 {
		confidence = ConfidenceMedium
		summary += " The boundary repeated across target attempts while compatible controls completed HTTP."
	}
	if len(counter) > 0 {
		summary += " Different target outcomes are recorded as counterevidence."
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

func controlFinding(facts []attemptFact) *Finding {
	healthy := healthyControlFacts(facts)
	if len(healthy) >= 2 {
		return &Finding{
			Code:               FindingControlPathHealthy,
			Kind:               FindingKindFact,
			Confidence:         ConfidenceHigh,
			Summary:            "Compatible control attempts completed valid HTTP paths.",
			Stage:              observatory.StageHTTP,
			SupportingEvidence: refs(healthy),
		}
	}
	if len(healthy) == 1 {
		return &Finding{
			Code:               FindingControlPathHealthy,
			Kind:               FindingKindFact,
			Confidence:         ConfidenceMedium,
			Summary:            "One compatible control attempt completed a valid HTTP path.",
			Stage:              observatory.StageHTTP,
			SupportingEvidence: refs(healthy),
		}
	}
	if len(facts) > 0 {
		return &Finding{
			Code:               FindingControlPathDegraded,
			Kind:               FindingKindFact,
			Confidence:         ConfidenceLow,
			Summary:            "Compatible control attempts did not establish a healthy HTTP path.",
			Stage:              deepestFailure(facts).boundary,
			SupportingEvidence: refs(facts),
		}
	}
	return nil
}

func edgeFinding(observations []observatory.ObservationResult, facts []attemptFact) *Finding {
	if !hasEdgeDependentOutcomes(observations) {
		return nil
	}
	return &Finding{
		Code:               FindingEdgeDependentFailure,
		Kind:               FindingKindInference,
		Confidence:         ConfidenceLow,
		Summary:            "Different resolved edges produced materially different observed stage outcomes.",
		Stage:              deepestFailure(facts).boundary,
		SupportingEvidence: refs(facts),
		Counterevidence:    refs(successfulFacts(facts)),
		PrerequisitesMet:   []string{"multiple_resolved_edges", "materially_different_outcomes"},
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
	var incompatibilities []string
	for _, directRun := range direct {
		for _, profileRun := range profiles {
			applicable, missing := ProfileComparisonApplicable(directRun, profileRun)
			if !applicable {
				incompatibilities = append(incompatibilities, "Direct/profile observations were not compared: "+strings.Join(missing, ", ")+".")
				continue
			}
			directPass := observationSucceeded(directRun)
			profilePass := observationSucceeded(profileRun)
			code, summary := profileOutcome(directPass, profilePass)
			refs := []EvidenceRef{runReference(directRun), runReference(profileRun)}
			finding := &Finding{
				Code:               code,
				Kind:               FindingKindFact,
				Confidence:         ConfidenceHigh,
				Summary:            summary,
				Stage:              profileStage(directRun, profileRun),
				SupportingEvidence: refs,
				PrerequisitesMet:   []string{"same_target", "compatible_network_context", "same_protocol", "close_time_window"},
			}
			if !sameResolvedSet(directRun, profileRun) {
				incompatibilities = append(incompatibilities, "Comparable direct/profile runs used different resolved edge sets.")
			}
			return finding, true, uniqueStrings(incompatibilities)
		}
	}
	return nil, false, uniqueStrings(incompatibilities)
}

func profileOutcome(directPass, profilePass bool) (FindingCode, string) {
	switch {
	case !directPass && profilePass:
		return FindingFixedByProfile, "Comparable direct evidence failed while the externally active profile evidence succeeded."
	case directPass && !profilePass:
		return FindingBrokenByProfile, "Comparable direct evidence succeeded while the externally active profile evidence failed."
	case !directPass && !profilePass:
		return FindingStillFailing, "Comparable direct and externally active profile evidence both failed."
	default:
		return FindingReachableDirectly, "Comparable direct and externally active profile evidence both succeeded."
	}
}

// ProfileComparisonApplicable prevents A/B labels across different targets,
// network contexts, address families, protocols, or distant runs.
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
	return strings.ToLower(target.Hostname) + ":" + target.Port + ":" + string(target.RequestedProtocol)
}

func targetRef(target observatory.Target) TargetRef {
	return TargetRef{Name: target.Name, Hostname: target.Hostname, Port: target.Port, RequestedProtocol: target.RequestedProtocol}
}

func observationSucceeded(observation observatory.ObservationResult) bool {
	if observation.Classification == observatory.ClassSuccess {
		return true
	}
	for _, fact := range collectFacts([]observatory.ObservationResult{observation}) {
		if fact.success {
			return true
		}
	}
	return false
}

func profileStage(direct, profile observatory.ObservationResult) observatory.Stage {
	if !observationSucceeded(profile) {
		return profile.FinalBoundary
	}
	return direct.FinalBoundary
}

func sameResolvedSet(left, right observatory.ObservationResult) bool {
	leftIPs := resolvedIPs(left)
	rightIPs := resolvedIPs(right)
	if len(leftIPs) != len(rightIPs) {
		return false
	}
	for index := range leftIPs {
		if leftIPs[index] != rightIPs[index] {
			return false
		}
	}
	return true
}

func resolvedIPs(observation observatory.ObservationResult) []string {
	ips := make([]string, 0, len(observation.ResolvedAddresses))
	for _, address := range observation.ResolvedAddresses {
		ips = append(ips, address.IP)
	}
	sort.Strings(ips)
	return ips
}

func hasEdgeDependentOutcomes(observations []observatory.ObservationResult) bool {
	for _, observation := range observations {
		facts := collectFacts([]observatory.ObservationResult{observation})
		outcomes := make(map[string]map[string]struct{})
		for _, fact := range facts {
			if fact.resolvedIP == "" {
				continue
			}
			if outcomes[fact.resolvedIP] == nil {
				outcomes[fact.resolvedIP] = make(map[string]struct{})
			}
			outcomes[fact.resolvedIP][factSignature(fact)] = struct{}{}
		}
		if len(outcomes) < 2 {
			continue
		}
		all := make(map[string]struct{})
		for _, edgeOutcomes := range outcomes {
			for outcome := range edgeOutcomes {
				all[outcome] = struct{}{}
			}
		}
		if len(all) > 1 {
			return true
		}
	}
	return false
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
func healthyControlCount(facts []attemptFact) int           { return len(healthyControlFacts(facts)) }

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

func contradictoryFacts(facts []attemptFact) []attemptFact {
	var result []attemptFact
	for _, fact := range facts {
		if fact.success || fact.validHTTP {
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
