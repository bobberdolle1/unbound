// Package attribution derives conservative, deterministic hypotheses from
// Observatory evidence. It never performs network or bypass-engine actions.
package attribution

import (
	"time"

	"unbound/engine/observatory"
)

const SchemaVersion = 1

type Confidence string

const (
	ConfidenceLow    Confidence = "LOW"
	ConfidenceMedium Confidence = "MEDIUM"
	ConfidenceHigh   Confidence = "HIGH"
)

type FindingKind string

const (
	FindingKindFact      FindingKind = "FACT"
	FindingKindInference FindingKind = "INFERENCE"
)

type FindingCode string

const (
	FindingNoAnomaly               FindingCode = "NO_ANOMALY"
	FindingInsufficientEvidence    FindingCode = "INSUFFICIENT_EVIDENCE"
	FindingDNSPathFailureSuspected FindingCode = "DNS_PATH_FAILURE_SUSPECTED"
	FindingTCPPathFailureSuspected FindingCode = "TCP_PATH_FAILURE_SUSPECTED"
	FindingTLSPathFailureSuspected FindingCode = "TLS_PATH_FAILURE_SUSPECTED"
	FindingHTTPApplicationFailure  FindingCode = "HTTP_APPLICATION_FAILURE_OBSERVED"
	FindingEdgeDependentFailure    FindingCode = "EDGE_DEPENDENT_FAILURE_SUSPECTED"
	FindingOutcomeVariability      FindingCode = "OUTCOME_VARIABILITY_OBSERVED"
	FindingNetworkContextFailure   FindingCode = "NETWORK_CONTEXT_FAILURE_SUSPECTED"
	FindingControlPathHealthy      FindingCode = "CONTROL_PATH_HEALTHY"
	FindingControlPathDegraded     FindingCode = "CONTROL_PATH_DEGRADED"
	FindingFixedByProfile          FindingCode = "FIXED_BY_PROFILE"
	FindingBrokenByProfile         FindingCode = "BROKEN_BY_PROFILE"
	FindingStillFailing            FindingCode = "STILL_FAILING"
	FindingReachableDirectly       FindingCode = "REACHABLE_DIRECTLY"
	FindingTCPResetObserved        FindingCode = "TCP_RESET_OBSERVED"
	FindingTLSResetObserved        FindingCode = "TLS_RESET_OBSERVED"
)

// EvidenceRef deliberately contains only stable Observatory identifiers and
// stage positions. It does not copy native errors, headers, URLs, or bodies.
type EvidenceRef struct {
	RunID        string            `json:"run_id"`
	AttemptIndex int               `json:"attempt_index"`
	Stage        observatory.Stage `json:"stage"`
}

// TargetRef deliberately excludes URL userinfo, query values, and fragments.
// Scheme and path distinguish endpoint contracts that share a host.
type TargetRef struct {
	Name              string                `json:"name,omitempty"`
	Scheme            string                `json:"scheme,omitempty"`
	Hostname          string                `json:"hostname"`
	Port              string                `json:"port"`
	Path              string                `json:"path"`
	RequestedProtocol observatory.Transport `json:"requested_protocol"`
}

type Finding struct {
	Code                 FindingCode       `json:"code"`
	Kind                 FindingKind       `json:"kind"`
	Confidence           Confidence        `json:"confidence"`
	Summary              string            `json:"summary"`
	Stage                observatory.Stage `json:"stage,omitempty"`
	SupportingEvidence   []EvidenceRef     `json:"supporting_evidence,omitempty"`
	Counterevidence      []EvidenceRef     `json:"counterevidence,omitempty"`
	PrerequisitesMet     []string          `json:"prerequisites_met,omitempty"`
	PrerequisitesMissing []string          `json:"prerequisites_missing,omitempty"`
}

type Applicability struct {
	ObservationAnalysis bool     `json:"observation_analysis"`
	ProfileComparison   bool     `json:"profile_comparison"`
	StrategyAssessment  bool     `json:"strategy_assessment"`
	Limitations         []string `json:"limitations,omitempty"`
}

type AttributionReport struct {
	SchemaVersion     int           `json:"schema_version"`
	AttributionID     string        `json:"attribution_id"`
	CreatedAt         time.Time     `json:"created_at"`
	InputRunIDs       []string      `json:"input_run_ids"`
	Target            TargetRef     `json:"target"`
	NetworkContextKey string        `json:"network_context_key,omitempty"`
	PrimaryFinding    Finding       `json:"primary_finding"`
	Findings          []Finding     `json:"findings"`
	EvidenceRefs      []EvidenceRef `json:"evidence_refs,omitempty"`
	Counterevidence   []EvidenceRef `json:"counterevidence,omitempty"`
	Limitations       []string      `json:"limitations,omitempty"`
	Confidence        Confidence    `json:"confidence"`
	Applicability     Applicability `json:"applicability"`
}

// Cohort separates the target observations from independently chosen controls.
// Calling Analyze is equivalent to AnalyzeCohort with no controls.
type Cohort struct {
	Target   []observatory.ObservationResult
	Controls []observatory.ObservationResult
}

// AffectedStage describes a protocol boundary. Transport is modeled separately.
type AffectedStage string

const (
	AffectedStageDNS       AffectedStage = "DNS"
	AffectedStageConnect   AffectedStage = "CONNECT"
	AffectedStageHello     AffectedStage = "HELLO"
	AffectedStageHandshake AffectedStage = "HANDSHAKE"
	AffectedStageHTTP      AffectedStage = "HTTP"
)

type StrategyCapabilities struct {
	AffectedStages []AffectedStage         `json:"affected_stages"`
	Transports     []observatory.Transport `json:"transports"`
}
