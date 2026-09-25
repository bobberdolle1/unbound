package observatory

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/url"
	"reflect"
	"sort"
	"strings"
	"time"
)

const (
	ProbeSpecSchemaVersion        = 1
	EvidenceRecordSchemaVersion   = 1
	maxProbeIdentifierLength      = 128
	maxExperimentIdentifierLength = 128
)

// ProbeMode names the bounded request contract. V2.1 implements HTTPS GET
// through the existing direct TCP/TLS/HTTP observer only.
type ProbeMode string

const (
	ProbeModeHTTPSGet ProbeMode = "HTTPS_GET"
)

// ControlRole records a probe's product role without drawing diagnosis
// conclusions from it.
type ControlRole string

const (
	ControlRolePrimary ControlRole = "PRIMARY_TARGET"
	ControlRoleHealthy ControlRole = "HEALTHY_CONTROL"
	ControlRoleNeutral ControlRole = "NEUTRAL_CONTROL"
)

// PrivacyMode declares the serialization policy for a probe. Every V2.1
// EvidenceRecord is redacted before it can be serialized.
type PrivacyMode string

const (
	PrivacyModeRedacted PrivacyMode = "REDACTED"
)

// MeasurementStatus never uses an omitted boolean as an implied success.
type MeasurementStatus string

const (
	MeasurementCompleted    MeasurementStatus = "COMPLETED"
	MeasurementPartial      MeasurementStatus = "PARTIAL"
	MeasurementFailed       MeasurementStatus = "FAILED"
	MeasurementUnsupported  MeasurementStatus = "UNSUPPORTED"
	MeasurementNotRequested MeasurementStatus = "NOT_REQUESTED"
)

// ResponseSemanticsStatus records whether an observed completed HTTP response
// satisfies the ProbeSpec expectation. It is not a diagnosis.
type ResponseSemanticsStatus string

const (
	ResponseSemanticsMatch        ResponseSemanticsStatus = "MATCH"
	ResponseSemanticsMismatch     ResponseSemanticsStatus = "MISMATCH"
	ResponseSemanticsNotEvaluated ResponseSemanticsStatus = "NOT_EVALUATED"
)

// ExpectedResponse is deliberately limited to facts the direct observer
// already records. V2.1 never retains a response body for contract matching.
type ExpectedResponse struct {
	AllowedStatusCodes  []int `json:"allowed_status_codes"`
	RequirePathComplete bool  `json:"require_path_complete"`
}

// TransferContract is permitted only for an externally audited endpoint with
// a known response-length contract. Declaring a contract does not cause V2.1
// to issue arbitrary padded requests or read arbitrary response bodies.
type TransferContract struct {
	AuditID                  string `json:"audit_id"`
	ResponseContractRevision string `json:"response_contract_revision"`
	ExpectedBytes            int64  `json:"expected_bytes"`
}

// ProbeSpec identifies exactly what is measured. Contract revisions are part
// of its canonical identity so changed semantics cannot reinterpret evidence
// emitted for an older contract.
type ProbeSpec struct {
	SchemaVersion          int               `json:"schema_version"`
	ID                     string            `json:"id"`
	ServiceID              string            `json:"service_id"`
	TargetContractRevision string            `json:"target_contract_revision"`
	Target                 Target            `json:"target"`
	Transport              Transport         `json:"transport"`
	AddressFamilyPolicy    AddressFamily     `json:"address_family_policy"`
	Mode                   ProbeMode         `json:"mode"`
	ExpectedResponse       ExpectedResponse  `json:"expected_response"`
	Transfer               *TransferContract `json:"transfer,omitempty"`
	ControlRole            ControlRole       `json:"control_role"`
	Privacy                PrivacyMode       `json:"privacy"`
}

// Identity returns a stable, semantic fingerprint for this exact ProbeSpec.
func (spec ProbeSpec) Identity() (string, error) {
	normalized, err := normalizeProbeSpec(spec)
	if err != nil {
		return "", err
	}
	data, err := json.Marshal(normalized)
	if err != nil {
		return "", fmt.Errorf("marshal canonical probe spec: %w", err)
	}
	digest := sha256.Sum256(data)
	return "probe-v1-" + hex.EncodeToString(digest[:]), nil
}

// Validate rejects unknown schema versions and unsupported contract shapes.
// QUIC and UDP are structurally valid specs, but V2.1 reports their attempted
// measurements as explicitly unsupported rather than pretending to observe them.
func (spec ProbeSpec) Validate() error {
	_, err := normalizeProbeSpec(spec)
	return err
}

func normalizeProbeSpec(spec ProbeSpec) (ProbeSpec, error) {
	if spec.SchemaVersion != ProbeSpecSchemaVersion {
		return ProbeSpec{}, fmt.Errorf("unsupported probe spec schema version %d", spec.SchemaVersion)
	}
	if !validIdentifier(spec.ID, maxProbeIdentifierLength) {
		return ProbeSpec{}, fmt.Errorf("probe ID must be a bounded non-empty identifier")
	}
	if !validIdentifier(spec.ServiceID, maxProbeIdentifierLength) {
		return ProbeSpec{}, fmt.Errorf("service ID must be a bounded non-empty identifier")
	}
	if !validIdentifier(spec.TargetContractRevision, maxProbeIdentifierLength) {
		return ProbeSpec{}, fmt.Errorf("target contract revision must be a bounded non-empty identifier")
	}
	if spec.Transport != TransportTCP && spec.Transport != TransportQUIC && spec.Transport != TransportUDP {
		return ProbeSpec{}, fmt.Errorf("unsupported probe transport %q", spec.Transport)
	}
	if spec.AddressFamilyPolicy != AddressFamilyAny && spec.AddressFamilyPolicy != AddressFamilyIPv4 && spec.AddressFamilyPolicy != AddressFamilyIPv6 {
		return ProbeSpec{}, fmt.Errorf("unsupported address-family policy %q", spec.AddressFamilyPolicy)
	}
	if spec.Mode != ProbeModeHTTPSGet {
		return ProbeSpec{}, fmt.Errorf("unsupported probe mode %q", spec.Mode)
	}
	if spec.ControlRole != ControlRolePrimary && spec.ControlRole != ControlRoleHealthy && spec.ControlRole != ControlRoleNeutral {
		return ProbeSpec{}, fmt.Errorf("unsupported probe control role %q", spec.ControlRole)
	}
	if spec.Privacy != PrivacyModeRedacted {
		return ProbeSpec{}, fmt.Errorf("probe privacy mode must be %q", PrivacyModeRedacted)
	}
	parsed, err := url.Parse(spec.Target.URL)
	if err != nil || parsed.Scheme != "https" || parsed.Hostname() == "" || parsed.User != nil || parsed.RawQuery != "" || parsed.Fragment != "" {
		return ProbeSpec{}, fmt.Errorf("probe target must be a redacted absolute https URL")
	}
	port := parsed.Port()
	if port == "" {
		port = "443"
	}
	if !strings.EqualFold(spec.Target.Hostname, parsed.Hostname()) || spec.Target.Port != port || spec.Target.RequestedProtocol != spec.Transport {
		return ProbeSpec{}, fmt.Errorf("probe target fields do not match target URL and transport")
	}
	if len(spec.ExpectedResponse.AllowedStatusCodes) == 0 {
		return ProbeSpec{}, fmt.Errorf("probe expected response requires at least one HTTP status code")
	}
	codes := append([]int(nil), spec.ExpectedResponse.AllowedStatusCodes...)
	sort.Ints(codes)
	for index, code := range codes {
		if code < 100 || code > 599 {
			return ProbeSpec{}, fmt.Errorf("invalid expected HTTP status %d", code)
		}
		if index > 0 && code == codes[index-1] {
			return ProbeSpec{}, fmt.Errorf("duplicate expected HTTP status %d", code)
		}
	}
	spec.Target.Name = ""
	spec.Target.URL = sanitizeURL(parsed)
	spec.Target.Hostname = strings.ToLower(parsed.Hostname())
	spec.Target.Port = port
	spec.Target.RequestedProtocol = spec.Transport
	spec.ExpectedResponse.AllowedStatusCodes = codes
	if spec.Transfer != nil {
		if spec.Transport != TransportTCP || spec.Mode != ProbeModeHTTPSGet {
			return ProbeSpec{}, fmt.Errorf("transfer contracts require TCP HTTPS GET probes")
		}
		if !validIdentifier(spec.Transfer.AuditID, maxProbeIdentifierLength) || !validIdentifier(spec.Transfer.ResponseContractRevision, maxProbeIdentifierLength) {
			return ProbeSpec{}, fmt.Errorf("transfer contract requires bounded audit and response-contract identifiers")
		}
		if spec.Transfer.ExpectedBytes <= 0 {
			return ProbeSpec{}, fmt.Errorf("transfer expected bytes must be greater than zero")
		}
	}
	return spec, nil
}

// TimingFacts stores only bounded, externally supplied protocol timing facts.
type TimingFacts struct {
	StartedAt  time.Time `json:"started_at"`
	FinishedAt time.Time `json:"finished_at"`
	ElapsedMS  int64     `json:"elapsed_ms"`
}

// ResponseMeasurement contains completed-path facts and contract matching.
type ResponseMeasurement struct {
	Status            MeasurementStatus       `json:"status"`
	HTTPStatus        int                     `json:"http_status,omitempty"`
	PathComplete      bool                    `json:"path_complete"`
	ExpectedSemantics ResponseSemanticsStatus `json:"expected_semantics"`
}

// TransferMeasurement supports an audited, bounded transfer result. It is a
// data contract only in V2.1; the direct observer does not implement a body
// transfer measurement and reports it as unsupported when one is requested.
type TransferMeasurement struct {
	Status        MeasurementStatus `json:"status"`
	ExpectedBytes int64             `json:"expected_bytes,omitempty"`
	ActualBytes   int64             `json:"actual_bytes,omitempty"`
	Completed     bool              `json:"completed"`
	FirstByteMS   *int64            `json:"first_byte_ms,omitempty"`
	TotalMS       *int64            `json:"total_ms,omitempty"`
}

// EvidenceRun is a redacted, ObservationResult-compatible run plus the
// factual selected edge, family, execution context, and derived measurement
// state for one observation.
type EvidenceRun struct {
	Observation   ObservationResult   `json:"observation"`
	SelectedEdge  string              `json:"selected_edge,omitempty"`
	AddressFamily AddressFamily       `json:"address_family,omitempty"`
	Execution     ExecutionContext    `json:"execution,omitempty"`
	Timing        TimingFacts         `json:"timing"`
	Measurement   MeasurementStatus   `json:"measurement"`
	Response      ResponseMeasurement `json:"response"`
}

// EvidenceInput supplies factual experiment metadata that ObservationResult
// intentionally does not own. It creates no persistent outcome history.
type EvidenceInput struct {
	Observations        []ObservationResult
	ExperimentID        string
	StrategyFingerprint string
	BackendFingerprint  string
	Transfer            *TransferMeasurement
}

// EvidenceRecord is a versioned, redacted transport structure. It collects
// observations; it never invokes a strategy, classifies censorship, or learns
// across sessions.
type EvidenceRecord struct {
	SchemaVersion       int                 `json:"schema_version"`
	Probe               ProbeSpec           `json:"probe"`
	ProbeIdentity       string              `json:"probe_identity"`
	ExperimentID        string              `json:"experiment_id,omitempty"`
	StrategyFingerprint string              `json:"strategy_fingerprint,omitempty"`
	BackendFingerprint  string              `json:"backend_fingerprint,omitempty"`
	Runs                []EvidenceRun       `json:"runs"`
	Transfer            TransferMeasurement `json:"transfer"`
	CollectedAt         time.Time           `json:"collected_at"`
	Fingerprint         string              `json:"fingerprint"`
}

// BuildEvidenceRecord wraps one or more compatible ObservationResults without
// modifying their V1 consumers. The result is ordered, redacted, and carries a
// deterministic fingerprint.
func BuildEvidenceRecord(spec ProbeSpec, input EvidenceInput) (EvidenceRecord, error) {
	normalizedSpec, err := normalizeProbeSpec(spec)
	if err != nil {
		return EvidenceRecord{}, err
	}
	if len(input.Observations) == 0 {
		return EvidenceRecord{}, fmt.Errorf("evidence record requires at least one observation")
	}
	if input.ExperimentID != "" && !validIdentifier(input.ExperimentID, maxExperimentIdentifierLength) {
		return EvidenceRecord{}, fmt.Errorf("experiment ID must be a bounded identifier")
	}
	if err := validateOptionalFingerprint("strategy", input.StrategyFingerprint); err != nil {
		return EvidenceRecord{}, err
	}
	if err := validateOptionalFingerprint("backend", input.BackendFingerprint); err != nil {
		return EvidenceRecord{}, err
	}
	identity, err := normalizedSpec.Identity()
	if err != nil {
		return EvidenceRecord{}, err
	}
	runs := make([]EvidenceRun, 0, len(input.Observations))
	var collectedAt time.Time
	for _, observation := range input.Observations {
		run, err := evidenceRunFromObservation(normalizedSpec, observation)
		if err != nil {
			return EvidenceRecord{}, err
		}
		runs = append(runs, run)
		if run.Timing.FinishedAt.After(collectedAt) {
			collectedAt = run.Timing.FinishedAt.UTC()
		}
	}
	sort.SliceStable(runs, func(i, j int) bool { return evidenceRunSortKey(runs[i]) < evidenceRunSortKey(runs[j]) })
	transfer, err := normalizeTransferMeasurement(normalizedSpec.Transfer, input.Transfer)
	if err != nil {
		return EvidenceRecord{}, err
	}
	record := EvidenceRecord{
		SchemaVersion:       EvidenceRecordSchemaVersion,
		Probe:               normalizedSpec,
		ProbeIdentity:       identity,
		ExperimentID:        input.ExperimentID,
		StrategyFingerprint: input.StrategyFingerprint,
		BackendFingerprint:  input.BackendFingerprint,
		Runs:                runs,
		Transfer:            transfer,
		CollectedAt:         collectedAt,
	}
	fingerprint, err := record.canonicalFingerprint()
	if err != nil {
		return EvidenceRecord{}, err
	}
	record.Fingerprint = fingerprint
	return record, nil
}

// ObserveProbe uses the existing direct observer and wraps its result in the
// V2.1 contract. QUIC and UDP therefore remain explicit unsupported results.
func (o *DirectTCPHTTPSObserver) ObserveProbe(ctx context.Context, spec ProbeSpec, options Options, input EvidenceInput) (EvidenceRecord, error) {
	normalized, err := normalizeProbeSpec(spec)
	if err != nil {
		return EvidenceRecord{}, err
	}
	if options.Transport != "" && options.Transport != normalized.Transport {
		return EvidenceRecord{}, fmt.Errorf("observer transport does not match probe spec")
	}
	if options.AddressFamily != "" && options.AddressFamily != normalized.AddressFamilyPolicy {
		return EvidenceRecord{}, fmt.Errorf("observer address family does not match probe spec")
	}
	options.Transport = normalized.Transport
	options.AddressFamily = normalized.AddressFamilyPolicy
	observation, err := o.Observe(ctx, normalized.Target.URL, options)
	if err != nil {
		return EvidenceRecord{}, err
	}
	input.Observations = append(input.Observations, observation)
	if normalized.Transfer != nil && input.Transfer == nil {
		input.Transfer = &TransferMeasurement{Status: MeasurementUnsupported, ExpectedBytes: normalized.Transfer.ExpectedBytes}
	}
	return BuildEvidenceRecord(normalized, input)
}

// CanonicalJSON returns a stable JSON representation for fingerprinting. The
// fingerprint field is cleared to avoid self-reference.
func (record EvidenceRecord) CanonicalJSON() ([]byte, error) {
	if err := record.Validate(); err != nil {
		return nil, err
	}
	copy := record
	copy.Fingerprint = ""
	return json.Marshal(copy)
}

func (record EvidenceRecord) canonicalFingerprint() (string, error) {
	copy := record
	copy.Fingerprint = ""
	data, err := json.Marshal(copy)
	if err != nil {
		return "", fmt.Errorf("marshal canonical evidence record: %w", err)
	}
	digest := sha256.Sum256(data)
	return "evidence-v1-" + hex.EncodeToString(digest[:]), nil
}

// Validate rejects future schemas, mismatched contract identities, unsafe
// serialized observations, and mismatched canonical fingerprints.
func (record EvidenceRecord) Validate() error {
	if record.SchemaVersion != EvidenceRecordSchemaVersion {
		return fmt.Errorf("unsupported evidence record schema version %d", record.SchemaVersion)
	}
	normalizedSpec, err := normalizeProbeSpec(record.Probe)
	if err != nil {
		return err
	}
	if !reflect.DeepEqual(record.Probe, normalizedSpec) {
		return fmt.Errorf("evidence probe spec is not canonical")
	}
	identity, err := normalizedSpec.Identity()
	if err != nil {
		return err
	}
	if record.ProbeIdentity != identity {
		return fmt.Errorf("evidence probe identity does not match probe spec")
	}
	if len(record.Runs) == 0 {
		return fmt.Errorf("evidence record requires at least one run")
	}
	if record.ExperimentID != "" && !validIdentifier(record.ExperimentID, maxExperimentIdentifierLength) {
		return fmt.Errorf("experiment ID must be a bounded identifier")
	}
	if err := validateOptionalFingerprint("strategy", record.StrategyFingerprint); err != nil {
		return err
	}
	if err := validateOptionalFingerprint("backend", record.BackendFingerprint); err != nil {
		return err
	}
	for index, run := range record.Runs {
		if index > 0 && evidenceRunSortKey(record.Runs[index-1]) > evidenceRunSortKey(run) {
			return fmt.Errorf("evidence runs are not canonically ordered")
		}
		if _, err := validateEvidenceRun(normalizedSpec, run); err != nil {
			return err
		}
	}
	normalizedTransfer, err := normalizeTransferMeasurement(normalizedSpec.Transfer, &record.Transfer)
	if err != nil {
		return err
	}
	if !reflect.DeepEqual(record.Transfer, normalizedTransfer) {
		return fmt.Errorf("evidence transfer measurement is not canonical")
	}
	var collectedAt time.Time
	for _, run := range record.Runs {
		if run.Timing.FinishedAt.After(collectedAt) {
			collectedAt = run.Timing.FinishedAt.UTC()
		}
	}
	if !record.CollectedAt.Equal(collectedAt) {
		return fmt.Errorf("evidence collected timestamp does not match runs")
	}
	if record.Fingerprint == "" {
		return fmt.Errorf("evidence fingerprint is required")
	}
	fingerprint, err := record.canonicalFingerprint()
	if err != nil {
		return err
	}
	if record.Fingerprint != fingerprint {
		return fmt.Errorf("evidence fingerprint does not match canonical record")
	}
	return nil
}

// MarshalEvidenceRecord serializes only records that meet the V2.1 redaction
// and canonical fingerprint invariants.
func MarshalEvidenceRecord(record EvidenceRecord) ([]byte, error) {
	if err := record.Validate(); err != nil {
		return nil, err
	}
	return json.Marshal(record)
}

// ParseEvidenceRecord rejects unknown schema versions and tampered or unsafe
// records rather than silently reinterpreting them.
func ParseEvidenceRecord(data []byte) (EvidenceRecord, error) {
	var record EvidenceRecord
	if err := json.Unmarshal(data, &record); err != nil {
		return EvidenceRecord{}, fmt.Errorf("decode evidence record: %w", err)
	}
	if err := record.Validate(); err != nil {
		return EvidenceRecord{}, err
	}
	return record, nil
}

func evidenceRunFromObservation(spec ProbeSpec, observation ObservationResult) (EvidenceRun, error) {
	if observation.Target.RequestedProtocol != spec.Transport || !strings.EqualFold(observation.Target.Hostname, spec.Target.Hostname) || observation.Target.Port != spec.Target.Port || sanitizeEvidenceURL(observation.Target.URL) != spec.Target.URL {
		return EvidenceRun{}, fmt.Errorf("observation target does not match probe spec")
	}
	observation = sanitizeEvidenceObservation(observation)
	attempt, ok := selectedAttempt(observation)
	selectedEdge := ""
	family := AddressFamily("")
	if ok {
		selectedEdge = attempt.ResolvedIP
		family = attempt.AddressFamily
	}
	status := observationMeasurementStatus(observation, attempt, ok)
	response := responseMeasurement(spec, attempt, ok, status)
	elapsed := observation.FinishedAt.Sub(observation.StartedAt).Milliseconds()
	if elapsed < 0 {
		elapsed = 0
	}
	return EvidenceRun{
		Observation:   observation,
		SelectedEdge:  selectedEdge,
		AddressFamily: family,
		Execution:     observation.ExecutionContext,
		Timing:        TimingFacts{StartedAt: observation.StartedAt.UTC(), FinishedAt: observation.FinishedAt.UTC(), ElapsedMS: elapsed},
		Measurement:   status,
		Response:      response,
	}, nil
}

func validateEvidenceRun(spec ProbeSpec, run EvidenceRun) (EvidenceRun, error) {
	if !safeEvidenceObservation(run.Observation) {
		return EvidenceRun{}, fmt.Errorf("evidence observation contains data excluded by redaction policy")
	}
	expected, err := evidenceRunFromObservation(spec, run.Observation)
	if err != nil {
		return EvidenceRun{}, err
	}
	if !reflect.DeepEqual(run, expected) {
		return EvidenceRun{}, fmt.Errorf("evidence run facts do not match observation")
	}
	return expected, nil
}

func selectedAttempt(observation ObservationResult) (ConnectionAttempt, bool) {
	if observation.PrimaryAttemptIndex == nil || *observation.PrimaryAttemptIndex < 0 || *observation.PrimaryAttemptIndex >= len(observation.Attempts) {
		return ConnectionAttempt{}, false
	}
	return observation.Attempts[*observation.PrimaryAttemptIndex], true
}

func observationMeasurementStatus(observation ObservationResult, attempt ConnectionAttempt, hasAttempt bool) MeasurementStatus {
	if observation.Classification == ClassQUICUnsupported || observation.Classification == ClassUDPUnsupported {
		return MeasurementUnsupported
	}
	if !hasAttempt {
		return MeasurementFailed
	}
	if stage, ok := evidenceStage(attempt, StageHTTP); ok && stage.PathComplete {
		return MeasurementCompleted
	}
	if stage, ok := evidenceStage(attempt, observation.FinalBoundary); ok {
		switch stage.Status {
		case StatusTimeout, StatusCancelled, StatusNotReached, StatusSkipped:
			return MeasurementPartial
		case StatusSkippedUnsupported:
			return MeasurementUnsupported
		}
	}
	return MeasurementFailed
}

func responseMeasurement(spec ProbeSpec, attempt ConnectionAttempt, hasAttempt bool, status MeasurementStatus) ResponseMeasurement {
	if !hasAttempt {
		return ResponseMeasurement{Status: status, ExpectedSemantics: ResponseSemanticsNotEvaluated}
	}
	http, ok := evidenceStage(attempt, StageHTTP)
	if !ok {
		return ResponseMeasurement{Status: status, ExpectedSemantics: ResponseSemanticsNotEvaluated}
	}
	response := ResponseMeasurement{Status: status, HTTPStatus: http.HTTPStatus, PathComplete: http.PathComplete, ExpectedSemantics: ResponseSemanticsNotEvaluated}
	if status != MeasurementCompleted {
		return response
	}
	if responseMatches(spec.ExpectedResponse, http) {
		response.ExpectedSemantics = ResponseSemanticsMatch
	} else {
		response.ExpectedSemantics = ResponseSemanticsMismatch
	}
	return response
}

func responseMatches(expected ExpectedResponse, http StageEvidence) bool {
	if expected.RequirePathComplete && !http.PathComplete {
		return false
	}
	for _, status := range expected.AllowedStatusCodes {
		if status == http.HTTPStatus {
			return true
		}
	}
	return false
}

func evidenceStage(attempt ConnectionAttempt, wanted Stage) (StageEvidence, bool) {
	for _, stage := range attempt.Stages {
		if stage.Stage == wanted {
			return stage, true
		}
	}
	return StageEvidence{}, false
}

func normalizeTransferMeasurement(contract *TransferContract, input *TransferMeasurement) (TransferMeasurement, error) {
	if contract == nil {
		if input == nil {
			return TransferMeasurement{Status: MeasurementNotRequested}, nil
		}
		if input.Status != MeasurementNotRequested && input.Status != MeasurementUnsupported {
			return TransferMeasurement{}, fmt.Errorf("transfer measurement requires an audited transfer contract")
		}
		if input.ExpectedBytes != 0 || input.ActualBytes != 0 || input.Completed || input.FirstByteMS != nil || input.TotalMS != nil {
			return TransferMeasurement{}, fmt.Errorf("transfer facts require an audited transfer contract")
		}
		return *input, nil
	}
	if input == nil {
		return TransferMeasurement{Status: MeasurementNotRequested, ExpectedBytes: contract.ExpectedBytes}, nil
	}
	measurement := *input
	if measurement.Status != MeasurementCompleted && measurement.Status != MeasurementPartial && measurement.Status != MeasurementFailed && measurement.Status != MeasurementUnsupported && measurement.Status != MeasurementNotRequested {
		return TransferMeasurement{}, fmt.Errorf("unsupported transfer measurement status %q", measurement.Status)
	}
	if measurement.ExpectedBytes == 0 {
		measurement.ExpectedBytes = contract.ExpectedBytes
	}
	if measurement.ExpectedBytes != contract.ExpectedBytes || measurement.ActualBytes < 0 {
		return TransferMeasurement{}, fmt.Errorf("transfer bytes do not match audited transfer contract")
	}
	if measurement.FirstByteMS != nil && *measurement.FirstByteMS < 0 || measurement.TotalMS != nil && *measurement.TotalMS < 0 {
		return TransferMeasurement{}, fmt.Errorf("transfer timing cannot be negative")
	}
	if measurement.FirstByteMS != nil && measurement.TotalMS != nil && *measurement.FirstByteMS > *measurement.TotalMS {
		return TransferMeasurement{}, fmt.Errorf("transfer first-byte timing exceeds total timing")
	}
	switch measurement.Status {
	case MeasurementCompleted:
		if !measurement.Completed || measurement.ActualBytes != contract.ExpectedBytes {
			return TransferMeasurement{}, fmt.Errorf("completed transfer must exactly satisfy audited byte contract")
		}
	case MeasurementPartial:
		if measurement.Completed || measurement.ActualBytes >= contract.ExpectedBytes {
			return TransferMeasurement{}, fmt.Errorf("partial transfer must be incomplete and shorter than audited byte contract")
		}
	case MeasurementFailed, MeasurementUnsupported, MeasurementNotRequested:
		if measurement.Completed || measurement.ActualBytes != 0 {
			return TransferMeasurement{}, fmt.Errorf("non-completed transfer cannot claim completed bytes")
		}
	}
	return measurement, nil
}

func sanitizeEvidenceObservation(observation ObservationResult) ObservationResult {
	observation = sanitizeForPersistence(observation)
	observation.Target.Name = ""
	observation.Target.URL = sanitizeEvidenceURL(observation.Target.URL)
	observation.NetworkContext = NetworkContext{}
	for attemptIndex := range observation.Attempts {
		attempt := &observation.Attempts[attemptIndex]
		attempt.LocalAddress = ""
		for stageIndex := range attempt.Stages {
			stage := &attempt.Stages[stageIndex]
			stage.Detail = ""
			stage.Redirect = ""
			stage.ResponseHeaders = allowedResponseHeaderMap(stage.ResponseHeaders)
		}
	}
	return observation
}

func safeEvidenceObservation(observation ObservationResult) bool {
	if observation.Target.Name != "" || observation.Target.URL != sanitizeEvidenceURL(observation.Target.URL) || observation.NetworkContext.Interface != "" || observation.NetworkContext.LocalAddress != "" || observation.NetworkContext.AddressFamily != "" || observation.NetworkContext.DefaultGateway != "" || observation.NetworkContext.NetworkLabel != "" || len(observation.NetworkContext.ConfiguredResolvers) != 0 {
		return false
	}
	for _, attempt := range observation.Attempts {
		if attempt.LocalAddress != "" {
			return false
		}
		for _, stage := range attempt.Stages {
			if stage.Detail != "" || stage.Redirect != "" || !sameHeaderMap(stage.ResponseHeaders, allowedResponseHeaderMap(stage.ResponseHeaders)) {
				return false
			}
		}
	}
	return true
}

func sameHeaderMap(left, right map[string]string) bool {
	if len(left) != len(right) {
		return false
	}
	for key, value := range left {
		if right[key] != value {
			return false
		}
	}
	return true
}

func evidenceRunSortKey(run EvidenceRun) string {
	data, _ := json.Marshal(run.Observation)
	return run.Observation.RunID + "\n" + string(data)
}

func validIdentifier(value string, limit int) bool {
	value = strings.TrimSpace(value)
	return value != "" && len(value) <= limit && !strings.ContainsAny(value, "\r\n\t")
}

func sanitizeEvidenceURL(raw string) string {
	parsed, err := url.Parse(raw)
	if err != nil {
		return sanitizeURLString(raw)
	}
	parsed.Fragment = ""
	return sanitizeURL(parsed)
}

func validateOptionalFingerprint(kind, value string) error {
	if value == "" {
		return nil
	}
	if len(value) > maxProbeIdentifierLength || strings.TrimSpace(value) != value || strings.ContainsAny(value, " \r\n\t") {
		return fmt.Errorf("%s fingerprint must be a bounded opaque identifier", kind)
	}
	return nil
}
