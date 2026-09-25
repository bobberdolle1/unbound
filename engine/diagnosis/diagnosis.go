// Package diagnosis normalizes validated observatory evidence and attribution
// findings into conservative product-level failure classes. It is pure.
package diagnosis

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/url"
	"sort"
	"strings"
	"time"

	"unbound/engine/attribution"
	"unbound/engine/observatory"
)

const SchemaVersion = 1

type Kind string

const (
	KindNoAnomaly               Kind = "NO_ANOMALY"
	KindDNSPathAnomaly          Kind = "DNS_PATH_ANOMALY"
	KindTCPPathFailure          Kind = "TCP_PATH_FAILURE"
	KindTLSHandshakePathFailure Kind = "TLS_HANDSHAKE_PATH_FAILURE"
	KindHTTPApplicationFailure  Kind = "HTTP_APPLICATION_FAILURE"
	KindPartialTransferAnomaly  Kind = "PARTIAL_TRANSFER_ANOMALY"
	KindEdgeDependentFailure    Kind = "EDGE_DEPENDENT_FAILURE"
	KindNetworkContextFailure   Kind = "NETWORK_CONTEXT_FAILURE"
	KindInsufficientEvidence    Kind = "INSUFFICIENT_EVIDENCE"
	KindUnknown                 Kind = "UNKNOWN"
)

// EvidenceRef adds the V2.1 evidence event identity to Attribution's redacted
// run/stage reference.
type EvidenceRef struct {
	EvidenceFingerprint string            `json:"evidence_fingerprint"`
	RunID               string            `json:"run_id"`
	AttemptIndex        int               `json:"attempt_index"`
	Stage               observatory.Stage `json:"stage,omitempty"`
}

type Input struct {
	Target      []observatory.EvidenceRecord
	Controls    []observatory.EvidenceRecord
	Attribution attribution.AttributionReport
}

type Report struct {
	SchemaVersion        int                    `json:"schema_version"`
	DiagnosisID          string                 `json:"diagnosis_id"`
	ProbeIdentity        string                 `json:"probe_identity"`
	Kind                 Kind                   `json:"kind"`
	Confidence           attribution.Confidence `json:"confidence"`
	AffectedStage        observatory.Stage      `json:"affected_stage,omitempty"`
	SupportingEvidence   []EvidenceRef          `json:"supporting_evidence,omitempty"`
	Counterevidence      []EvidenceRef          `json:"counterevidence,omitempty"`
	PrerequisitesMet     []string               `json:"prerequisites_met,omitempty"`
	PrerequisitesMissing []string               `json:"prerequisites_missing,omitempty"`
	Limitations          []string               `json:"limitations,omitempty"`
	EvidenceFingerprints []string               `json:"evidence_fingerprints"`
	AttributionID        string                 `json:"attribution_id"`
	EffectiveAt          time.Time              `json:"effective_at"`
}

// Diagnose validates correlation first, then applies explicit precedence:
// unsupported -> network context -> transfer -> HTTP -> edge -> stage finding
// -> no anomaly -> insufficient -> unknown. It never calls the network.
func Diagnose(input Input) (Report, error) {
	all, targetRuns, err := validateInput(input)
	if err != nil {
		return Report{}, err
	}
	report := baseReport(input, all)
	findings := findingsByCode(input.Attribution.Findings)
	primary := input.Attribution.PrimaryFinding
	if primary.Code != "" {
		findings[primary.Code] = primary
	}
	if unsupportedTarget(targetRuns) {
		report.Kind, report.Confidence = KindInsufficientEvidence, attribution.ConfidenceLow
		report.PrerequisitesMissing = []string{unsupportedPrerequisite(targetRuns)}
		return finish(report)
	}
	if finding, ok := findings[attribution.FindingNetworkContextFailure]; ok {
		applyFinding(&report, KindNetworkContextFailure, finding, all)
		return finish(report)
	}
	if kind, ok := partialTransfer(input.Target, input.Controls); ok {
		report.Kind, report.Confidence, report.AffectedStage = kind, attribution.ConfidenceLow, observatory.StageCarry
		report.PrerequisitesMet = []string{"audited_transfer_contract", "repeated_partial_target_transfer", "healthy_completed_control"}
		return finish(report)
	}
	if finding, ok := findings[attribution.FindingHTTPApplicationFailure]; ok {
		applyFinding(&report, KindHTTPApplicationFailure, finding, all)
		return finish(report)
	}
	if finding, ok := findings[attribution.FindingEdgeDependentFailure]; ok {
		applyFinding(&report, KindEdgeDependentFailure, finding, all)
		return finish(report)
	}
	for _, mapping := range []struct {
		code attribution.FindingCode
		kind Kind
	}{
		{attribution.FindingDNSPathFailureSuspected, KindDNSPathAnomaly},
		{attribution.FindingTCPPathFailureSuspected, KindTCPPathFailure},
		{attribution.FindingTLSPathFailureSuspected, KindTLSHandshakePathFailure},
	} {
		if finding, ok := findings[mapping.code]; ok {
			applyFinding(&report, mapping.kind, finding, all)
			return finish(report)
		}
	}
	if _, ok := findings[attribution.FindingNoAnomaly]; ok && allCompleted(input.Target) {
		report.Kind, report.Confidence = KindNoAnomaly, attribution.ConfidenceHigh
		return finish(report)
	}
	if finding, ok := findings[attribution.FindingInsufficientEvidence]; ok {
		applyFinding(&report, KindInsufficientEvidence, finding, all)
		return finish(report)
	}
	report.Kind, report.Confidence = KindUnknown, attribution.ConfidenceLow
	report.Limitations = []string{"No validated deterministic diagnosis rule matched the evidence and attribution cohort."}
	return finish(report)
}

func validateInput(input Input) ([]observatory.EvidenceRecord, map[string]observatory.EvidenceRun, error) {
	if len(input.Target) == 0 {
		return nil, nil, fmt.Errorf("diagnosis requires target evidence")
	}
	if input.Attribution.SchemaVersion != attribution.SchemaVersion {
		return nil, nil, fmt.Errorf("unsupported attribution schema version %d", input.Attribution.SchemaVersion)
	}
	all := append(append([]observatory.EvidenceRecord(nil), input.Target...), input.Controls...)
	probe := input.Target[0].ProbeIdentity
	runs := map[string]observatory.EvidenceRun{}
	for i, record := range all {
		if err := record.Validate(); err != nil {
			return nil, nil, fmt.Errorf("invalid evidence record: %w", err)
		}
		if i < len(input.Target) && record.ProbeIdentity != probe {
			return nil, nil, fmt.Errorf("target evidence probe identities differ")
		}
		for _, run := range record.Runs {
			if _, exists := runs[run.Observation.RunID]; exists {
				return nil, nil, fmt.Errorf("duplicate evidence run ID %q", run.Observation.RunID)
			}
			runs[run.Observation.RunID] = run
		}
	}
	if !targetMatches(input.Attribution.Target, input.Target[0].Probe.Target) {
		return nil, nil, fmt.Errorf("attribution target does not match target evidence")
	}
	for _, id := range input.Attribution.InputRunIDs {
		if _, ok := runs[id]; !ok {
			return nil, nil, fmt.Errorf("attribution references unknown run ID %q", id)
		}
	}
	for _, ref := range allAttributionRefs(input.Attribution) {
		run, ok := runs[ref.RunID]
		if !ok {
			return nil, nil, fmt.Errorf("attribution evidence ref has unknown run ID %q", ref.RunID)
		}
		if ref.AttemptIndex < 0 || ref.AttemptIndex >= len(run.Observation.Attempts) {
			return nil, nil, fmt.Errorf("attribution evidence ref has invalid attempt index")
		}
		if ref.Stage != "" && !hasStage(run.Observation.Attempts[ref.AttemptIndex], ref.Stage) {
			return nil, nil, fmt.Errorf("attribution evidence ref has unknown stage")
		}
	}
	return all, runs, nil
}

func targetMatches(target attribution.TargetRef, evidence observatory.Target) bool {
	if !strings.EqualFold(target.Hostname, evidence.Hostname) || target.Port != evidence.Port || target.RequestedProtocol != evidence.RequestedProtocol {
		return false
	}
	parsed, err := url.Parse(evidence.URL)
	return err == nil && target.Path == parsed.Path
}
func hasStage(attempt observatory.ConnectionAttempt, wanted observatory.Stage) bool {
	for _, stage := range attempt.Stages {
		if stage.Stage == wanted {
			return true
		}
	}
	return false
}
func allAttributionRefs(report attribution.AttributionReport) []attribution.EvidenceRef {
	refs := append(append([]attribution.EvidenceRef{}, report.EvidenceRefs...), report.Counterevidence...)
	for _, finding := range append(append([]attribution.Finding{}, report.Findings...), report.PrimaryFinding) {
		refs = append(refs, finding.SupportingEvidence...)
		refs = append(refs, finding.Counterevidence...)
	}
	return refs
}
func findingsByCode(findings []attribution.Finding) map[attribution.FindingCode]attribution.Finding {
	out := map[attribution.FindingCode]attribution.Finding{}
	for _, f := range findings {
		out[f.Code] = f
	}
	return out
}
func baseReport(input Input, records []observatory.EvidenceRecord) Report {
	fingerprints := make([]string, 0, len(records))
	var effective time.Time
	for _, r := range records {
		fingerprints = append(fingerprints, r.Fingerprint)
		if r.CollectedAt.After(effective) {
			effective = r.CollectedAt.UTC()
		}
	}
	sort.Strings(fingerprints)
	return Report{SchemaVersion: SchemaVersion, ProbeIdentity: input.Target[0].ProbeIdentity, EvidenceFingerprints: fingerprints, AttributionID: input.Attribution.AttributionID, EffectiveAt: effective}
}
func unsupportedTarget(runs map[string]observatory.EvidenceRun) bool {
	for _, r := range runs {
		if r.Observation.Target.RequestedProtocol == observatory.TransportQUIC || r.Observation.Target.RequestedProtocol == observatory.TransportUDP {
			return r.Measurement == observatory.MeasurementUnsupported
		}
	}
	return false
}
func unsupportedPrerequisite(runs map[string]observatory.EvidenceRun) string {
	for _, r := range runs {
		if r.Observation.Target.RequestedProtocol == observatory.TransportQUIC {
			return "real_quic_handshake_evidence"
		}
	}
	return "real_udp_probe_evidence"
}
func allCompleted(records []observatory.EvidenceRecord) bool {
	for _, r := range records {
		for _, run := range r.Runs {
			if run.Measurement != observatory.MeasurementCompleted || run.Response.ExpectedSemantics != observatory.ResponseSemanticsMatch {
				return false
			}
		}
	}
	return len(records) > 0
}
func partialTransfer(target, controls []observatory.EvidenceRecord) (Kind, bool) {
	if len(target) < 2 || len(controls) == 0 {
		return "", false
	}
	for _, r := range target {
		if r.Probe.Transfer == nil || r.Transfer.Status != observatory.MeasurementPartial {
			return "", false
		}
	}
	for _, r := range controls {
		if r.Transfer.Status != observatory.MeasurementCompleted {
			return "", false
		}
	}
	return KindPartialTransferAnomaly, true
}
func applyFinding(report *Report, kind Kind, finding attribution.Finding, records []observatory.EvidenceRecord) {
	report.Kind, report.Confidence, report.AffectedStage = kind, finding.Confidence, finding.Stage
	report.PrerequisitesMet = sorted(finding.PrerequisitesMet)
	report.PrerequisitesMissing = sorted(finding.PrerequisitesMissing)
	report.Limitations = sorted(findingLimitations(finding))
	report.SupportingEvidence = mapRefs(finding.SupportingEvidence, records)
	report.Counterevidence = mapRefs(finding.Counterevidence, records)
}
func findingLimitations(f attribution.Finding) []string {
	if f.Code == attribution.FindingTLSPathFailureSuspected {
		return []string{"TLS handshake path failure does not identify a TLS fingerprint mechanism."}
	}
	if f.Code == attribution.FindingDNSPathFailureSuspected {
		return []string{"DNS path anomaly does not establish DNS interception."}
	}
	return nil
}
func mapRefs(refs []attribution.EvidenceRef, records []observatory.EvidenceRecord) []EvidenceRef {
	out := make([]EvidenceRef, 0, len(refs))
	for _, ref := range refs {
		for _, r := range records {
			for _, run := range r.Runs {
				if run.Observation.RunID == ref.RunID {
					out = append(out, EvidenceRef{r.Fingerprint, ref.RunID, ref.AttemptIndex, ref.Stage})
				}
			}
		}
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].EvidenceFingerprint+out[i].RunID+string(out[i].Stage) < out[j].EvidenceFingerprint+out[j].RunID+string(out[j].Stage)
	})
	return out
}
func sorted(values []string) []string {
	out := append([]string(nil), values...)
	sort.Strings(out)
	return out
}
func finish(report Report) (Report, error) {
	report.SupportingEvidence = dedupeRefs(report.SupportingEvidence)
	report.Counterevidence = dedupeRefs(report.Counterevidence)
	report.PrerequisitesMet = sorted(report.PrerequisitesMet)
	report.PrerequisitesMissing = sorted(report.PrerequisitesMissing)
	report.Limitations = sorted(report.Limitations)
	copy := report
	copy.DiagnosisID = ""
	data, err := json.Marshal(copy)
	if err != nil {
		return Report{}, err
	}
	sum := sha256.Sum256(data)
	report.DiagnosisID = "diagnosis-v1-" + hex.EncodeToString(sum[:])
	return report, nil
}
func dedupeRefs(in []EvidenceRef) []EvidenceRef {
	out := make([]EvidenceRef, 0, len(in))
	seen := map[string]bool{}
	for _, r := range in {
		key := r.EvidenceFingerprint + "\n" + r.RunID + "\n" + fmt.Sprint(r.AttemptIndex) + "\n" + string(r.Stage)
		if !seen[key] {
			seen[key] = true
			out = append(out, r)
		}
	}
	return out
}
