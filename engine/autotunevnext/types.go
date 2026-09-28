// Package autotunevnext runs bounded, evidence-driven StrategyIR experiments.
package autotunevnext

import (
	"context"
	"errors"
	"net"
	"time"

	"unbound/engine"
	"unbound/engine/attribution"
	"unbound/engine/backendcap"
	"unbound/engine/diagnosis"
	"unbound/engine/observatory"
	"unbound/engine/planner"
	"unbound/engine/strategyir"
)

const (
	SchemaVersion         = 1
	ManagedCleanupTimeout = 30 * time.Second
)

type Status string

const (
	StatusCompletedSelected             Status = "COMPLETED_SELECTED"
	StatusCompletedNoActionNeeded       Status = "COMPLETED_NO_ACTION_NEEDED"
	StatusCompletedNoEligibleCandidates Status = "COMPLETED_NO_ELIGIBLE_CANDIDATES"
	StatusCompletedNoVerifiedCandidate  Status = "COMPLETED_NO_VERIFIED_CANDIDATE"
	StatusInconclusive                  Status = "INCONCLUSIVE"
	StatusCancelled                     Status = "CANCELLED"
	StatusPreflightFailed               Status = "PREFLIGHT_FAILED"
	StatusLifecycleFailed               Status = "LIFECYCLE_FAILED"
	StatusStateRestoreFailed            Status = "STATE_RESTORE_FAILED"
)

type Outcome string

const (
	OutcomeVerifiedFixed         Outcome = "VERIFIED_FIXED"
	OutcomeStillFailing          Outcome = "STILL_FAILING"
	OutcomeRegressionObserved    Outcome = "REGRESSION_OBSERVED"
	OutcomeDirectBecameReachable Outcome = "DIRECT_BECAME_REACHABLE"
	OutcomeInconclusive          Outcome = "INCONCLUSIVE"
	OutcomeBackendUnsupported    Outcome = "BACKEND_UNSUPPORTED"
	OutcomeHostUnsupported       Outcome = "HOST_UNSUPPORTED"
	OutcomeLifecycleFailure      Outcome = "LIFECYCLE_FAILURE"
	OutcomeNotRunBudget          Outcome = "NOT_RUN_BUDGET"
	OutcomeNotRunPolicy          Outcome = "NOT_RUN_POLICY"
)

type PreflightStatus string

const (
	PreflightSupported   PreflightStatus = "SUPPORTED"
	PreflightUnsupported PreflightStatus = "UNSUPPORTED"
	PreflightError       PreflightStatus = "ERROR"
	PreflightNotRun      PreflightStatus = "NOT_RUN"
)

type Reason struct {
	Code   string `json:"code"`
	Detail string `json:"detail,omitempty"`
}

// Target is an explicit, sanitized Observatory endpoint.
type Target struct {
	Name          string                    `json:"name,omitempty"`
	URL           string                    `json:"url"`
	Transport     observatory.Transport     `json:"transport,omitempty"`
	AddressFamily observatory.AddressFamily `json:"address_family,omitempty"`
}

type EvidenceOptions struct {
	Timeouts observatory.Timeouts `json:"timeouts,omitempty"`
}

type Policy struct {
	MaxCandidates         int           `json:"max_candidates"`
	MaxDuration           time.Duration `json:"max_duration"`
	PerObservationTimeout time.Duration `json:"per_observation_timeout"`
}

func DefaultPolicy() Policy {
	return Policy{MaxCandidates: 3, MaxDuration: 2 * time.Minute, PerObservationTimeout: 25 * time.Second}
}

func (p Policy) normalized() Policy {
	d := DefaultPolicy()
	if p.MaxCandidates <= 0 {
		p.MaxCandidates = d.MaxCandidates
	}
	if p.MaxDuration <= 0 {
		p.MaxDuration = d.MaxDuration
	}
	if p.PerObservationTimeout <= 0 {
		p.PerObservationTimeout = d.PerObservationTimeout
	}
	return p
}

type Request struct {
	Target        Target                `json:"target"`
	Controls      []Target              `json:"controls,omitempty"`
	Strategies    []strategyir.Strategy `json:"strategies"`
	Backend       backendcap.Backend    `json:"backend"`
	ScopeSnapshot planner.ScopeSnapshot `json:"scope_snapshot,omitempty"`
	NetworkLabel  string                `json:"network_label,omitempty"`
	Evidence      EvidenceOptions       `json:"evidence,omitempty"`
	Policy        Policy                `json:"experiment_policy,omitempty"`
	// Probe contracts are optional for compatibility. When provided they are
	// fixed before observing and enable the V2 diagnosis/advisor path.
	TargetProbe   *observatory.ProbeSpec  `json:"-"`
	ControlProbes []observatory.ProbeSpec `json:"-"`
	Advisor       CandidateAdvisor        `json:"-"`
}

// CandidateIdentity is the narrow advisor input. It contains only candidates
// Planner already marked ELIGIBLE and cannot convey executable strategy data.
type CandidateIdentity struct {
	StrategyID          string                  `json:"strategy_id"`
	StrategyFingerprint string                  `json:"strategy_fingerprint"`
	Safety              strategyir.SafetyPolicy `json:"safety"`
}

type AdvisorContext struct {
	Diagnosis diagnosis.Report          `json:"diagnosis"`
	Planner   planner.PlannerReport     `json:"planner"`
	Family    observatory.AddressFamily `json:"family"`
	Eligible  []CandidateIdentity       `json:"eligible"`
}

type CandidateAdvice struct {
	StrategyFingerprint string   `json:"strategy_fingerprint"`
	ReasonCodes         []string `json:"reason_codes,omitempty"`
	HistoryUsed         bool     `json:"history_used"`
}

type RecommendationDisposition string

const (
	RecommendationExperimentCandidates RecommendationDisposition = "EXPERIMENT_CANDIDATES"
	RecommendationNoAction             RecommendationDisposition = "NO_ACTION"
	RecommendationNoPacketStrategy     RecommendationDisposition = "NO_PACKET_STRATEGY"
	RecommendationInsufficientEvidence RecommendationDisposition = "INSUFFICIENT_EVIDENCE"
	RecommendationNoEligibleCandidates RecommendationDisposition = "NO_ELIGIBLE_CANDIDATES"
)

type Advice struct {
	SchemaVersion        int                       `json:"schema_version"`
	DiagnosisID          string                    `json:"diagnosis_id"`
	PlannerAttributionID string                    `json:"planner_attribution_id"`
	Disposition          RecommendationDisposition `json:"disposition"`
	Candidates           []CandidateAdvice         `json:"candidates,omitempty"`
	Limitations          []string                  `json:"limitations,omitempty"`
}

// CandidateAdvisor is product-injected. It can order only the supplied
// candidates; AutoTune validates every response and otherwise falls back.
type CandidateAdvisor interface {
	Advise(AdvisorContext) Advice
}

// Observer remains provider-neutral. Supplying ResolvedIP pins an observation
// to one concrete edge while preserving the original hostname for TLS and HTTP.
type Observer interface {
	Observe(context.Context, string, observatory.Options) (observatory.ObservationResult, error)
}

type StateSnapshot struct {
	ID string `json:"id,omitempty"`
}

// AssetKind constrains which compiler placeholder context may consume a
// product-managed runtime value.
type AssetKind string

const (
	AssetKindBlobSymbol       AssetKind = "BLOB_SYMBOL"
	AssetKindHostlistFile     AssetKind = "HOSTLIST_FILE"
	AssetKindIPSetFile        AssetKind = "IPSET_FILE"
	AssetKindAutoHostlistFile AssetKind = "AUTO_HOSTLIST_FILE"
)

// ResolvedAsset is supplied only by trusted product code. EngineValue is one
// argv value fragment, never StrategyIR or remote strategy input.
type ResolvedAsset struct {
	ID          string    `json:"id"`
	Kind        AssetKind `json:"kind"`
	EngineValue string    `json:"engine_value"`
}

type ExecutableCandidate struct {
	Strategy     strategyir.Strategy       `json:"strategy"`
	Fingerprint  string                    `json:"fingerprint"`
	Backend      backendcap.Backend        `json:"backend"`
	Plan         backendcap.Plan           `json:"plan"`
	Assets       []ResolvedAsset           `json:"assets"`
	TargetEdge   net.IP                    `json:"-"`
	TargetFamily observatory.AddressFamily `json:"target_family"`
}

// Executor owns all machine mutation. It must restore and prove original state.
type Executor interface {
	Snapshot(context.Context) (StateSnapshot, error)
	EstablishDirect(context.Context, StateSnapshot) error
	Activate(context.Context, ExecutableCandidate) error
	VerifyActive(context.Context, ExecutableCandidate) error
	Deactivate(context.Context) error
	Restore(context.Context, StateSnapshot) error
	VerifyRestored(context.Context, StateSnapshot) error
}

type HostPreflightRequest struct {
	Backend        backendcap.Backend
	Requirements   engine.StrategyRequirements
	RequiredAssets []string
	Capture        backendcap.CapturePlan
}

type HostPreflightResult struct {
	Status  PreflightStatus `json:"status"`
	Reasons []Reason        `json:"reasons,omitempty"`
}
type HostPreflight interface {
	Check(context.Context, HostPreflightRequest) HostPreflightResult
}

type AssetResolver interface {
	Resolve(context.Context, backendcap.Backend, []string) ([]ResolvedAsset, error)
}

var ErrMissingAsset = errors.New("missing trusted logical asset")

type ControlResult struct {
	Target            Target   `json:"target"`
	DirectRunID       string   `json:"direct_run_id,omitempty"`
	ActiveRunID       string   `json:"active_run_id,omitempty"`
	Outcome           Outcome  `json:"outcome"`
	Reasons           []Reason `json:"reasons,omitempty"`
	activeObservation *observatory.ObservationResult
}

type CandidateExperiment struct {
	StrategyID           string                        `json:"strategy_id"`
	Fingerprint          string                        `json:"fingerprint,omitempty"`
	ExperimentExecuted   bool                          `json:"experiment_executed"`
	PlannerStatus        planner.CandidateStatus       `json:"planner_status"`
	CompileStatus        backendcap.CompileStatus      `json:"compile_status,omitempty"`
	PreflightStatus      PreflightStatus               `json:"preflight_status"`
	DirectBeforeRunIDs   []string                      `json:"direct_before_run_ids,omitempty"`
	ActiveRunIDs         []string                      `json:"active_run_ids,omitempty"`
	DirectAfterRunIDs    []string                      `json:"direct_after_run_ids,omitempty"`
	Attribution          attribution.AttributionReport `json:"attribution,omitempty"`
	ControlResults       []ControlResult               `json:"control_results,omitempty"`
	Outcome              Outcome                       `json:"outcome"`
	Safety               strategyir.SafetyPolicy       `json:"safety"`
	RecommendationReason []string                      `json:"recommendation_reason,omitempty"`
	RejectionReasons     []Reason                      `json:"rejection_reasons,omitempty"`
	Limitations          []string                      `json:"limitations,omitempty"`
	validationEvidence   []observatory.EvidenceRecord
}

// OutcomeEvidence is internal redacted provenance for product-owned ledger
// persistence. It never appears in the product or frontend result contract.
type OutcomeEvidence struct {
	StrategyFingerprint string                       `json:"-"`
	Outcome             Outcome                      `json:"-"`
	Validation          []observatory.EvidenceRecord `json:"-"`
}

type Lifecycle struct {
	SnapshotTaken     bool     `json:"snapshot_taken"`
	DirectEstablished bool     `json:"direct_established"`
	RestoreAttempted  bool     `json:"restore_attempted"`
	RestoreVerified   bool     `json:"restore_verified"`
	Errors            []Reason `json:"errors,omitempty"`
}

type Result struct {
	SchemaVersion       int                           `json:"schema_version"`
	Status              Status                        `json:"status"`
	Target              Target                        `json:"target"`
	Backend             backendcap.Backend            `json:"backend"`
	BaselineAttribution attribution.AttributionReport `json:"baseline_attribution,omitempty"`
	DiagnosisReport     diagnosis.Report              `json:"diagnosis_report,omitempty"`
	PlannerReport       planner.PlannerReport         `json:"planner_report,omitempty"`
	Recommendation      Advice                        `json:"recommendation,omitempty"`
	Experiments         []CandidateExperiment         `json:"experiments,omitempty"`
	SelectedStrategyID  string                        `json:"selected_strategy_id,omitempty"`
	SelectedFingerprint string                        `json:"selected_fingerprint,omitempty"`
	SelectionReason     string                        `json:"selection_reason,omitempty"`
	Limitations         []string                      `json:"limitations,omitempty"`
	Lifecycle           Lifecycle                     `json:"lifecycle"`
	StateRestored       bool                          `json:"state_restored"`
	BaselineEvidence    []observatory.EvidenceRecord  `json:"-"`
	OutcomeEvidence     []OutcomeEvidence             `json:"-"`
}
