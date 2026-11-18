package main

import (
	"encoding/hex"
	"errors"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"unbound/engine"
	"unbound/engine/autotunevnext"
	"unbound/engine/backendcap"
	"unbound/engine/historyid"
	"unbound/engine/observatory"
	"unbound/engine/outcomeledger"
	"unbound/engine/recommendation"
)

const (
	vNextOutcomeLedgerFile = outcomeledger.FileName
	vNextContextKeyBytes   = 32
)

var vNextHistoryMu sync.Mutex

// VNextHistorySummary exposes privacy-safe counts and status for the local
// AutoTune vNext outcome ledger without mutating disk or identity state.
type VNextHistorySummary struct {
	RecordCount   int    `json:"record_count"`
	VerifiedCount int    `json:"verified_count"`
	NegativeCount int    `json:"negative_count"`
	UpdatedAt     string `json:"updated_at"`
	ContextReady  bool   `json:"context_ready"`
}

func getVNextOutcomeLedgerPath() (string, error) {
	directory, err := engine.GetConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(directory, vNextOutcomeLedgerFile), nil
}

// isVNextContextKeyReady inspects the local HMAC context key file read-only
// without creating, rotating, or caching key material.
func isVNextContextKeyReady(configDir string) bool {
	if strings.TrimSpace(configDir) == "" {
		return false
	}
	raw, err := os.ReadFile(filepath.Join(configDir, historyid.ContextKeyFileName))
	if err != nil || len(raw) == 0 {
		return false
	}
	if len(raw) == vNextContextKeyBytes {
		return true
	}
	trimmed := strings.TrimSpace(string(raw))
	if len(trimmed) == hex.EncodedLen(vNextContextKeyBytes) {
		decoded, decErr := hex.DecodeString(trimmed)
		return decErr == nil && len(decoded) == vNextContextKeyBytes
	}
	return false
}

func readVNextHistorySummary() VNextHistorySummary {
	vNextHistoryMu.Lock()
	defer vNextHistoryMu.Unlock()

	configDir, err := engine.GetConfigDir()
	if err != nil {
		return VNextHistorySummary{}
	}
	summary := VNextHistorySummary{
		ContextReady: isVNextContextKeyReady(configDir),
	}
	ledgerPath := filepath.Join(configDir, vNextOutcomeLedgerFile)
	loaded := outcomeledger.Load(ledgerPath)
	if loaded.State != outcomeledger.LoadValid {
		return summary
	}

	summary.RecordCount = len(loaded.Ledger.Entries)
	var latest time.Time
	for _, entry := range loaded.Ledger.Entries {
		if entry.RecordedAt.After(latest) {
			latest = entry.RecordedAt
		}
		switch entry.Outcome {
		case autotunevnext.OutcomeVerifiedFixed:
			if entry.InvalidatedAt == nil {
				summary.VerifiedCount++
			}
		case autotunevnext.OutcomeStillFailing, autotunevnext.OutcomeRegressionObserved, autotunevnext.OutcomeLifecycleFailure:
			summary.NegativeCount++
		}
	}
	if !latest.IsZero() {
		summary.UpdatedAt = latest.UTC().Format(time.RFC3339)
	}
	return summary
}

func clearVNextHistory() error {
	vNextHistoryMu.Lock()
	defer vNextHistoryMu.Unlock()

	path, err := getVNextOutcomeLedgerPath()
	if err != nil {
		return err
	}
	if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	return nil
}

func productProbeContracts(rawTarget string, target autotunevnext.Target, controls []autotunevnext.Target) (observatory.ProbeSpec, []observatory.ProbeSpec, error) {
	targetSpec, err := productProbeSpec(rawTarget, target, observatory.ControlRolePrimary)
	if err != nil {
		return observatory.ProbeSpec{}, nil, err
	}
	controlSpecs := make([]observatory.ProbeSpec, 0, len(controls))
	for _, control := range controls {
		spec, err := productProbeSpec(productVNextDefaultControl.Target, control, observatory.ControlRoleHealthy)
		if err != nil {
			return observatory.ProbeSpec{}, nil, err
		}
		controlSpecs = append(controlSpecs, spec)
	}
	return targetSpec, controlSpecs, nil
}

func productProbeSpec(raw string, target autotunevnext.Target, role observatory.ControlRole) (observatory.ProbeSpec, error) {
	parsed, err := url.Parse(target.URL)
	if err != nil {
		return observatory.ProbeSpec{}, err
	}
	parsed.RawQuery = ""
	parsed.ForceQuery = false
	parsed.Fragment = ""
	parsed.RawFragment = ""
	port := parsed.Port()
	if port == "" {
		port = "443"
	}
	id, service := "autotune-vnext-custom-https-v1", "custom-https"
	if role == observatory.ControlRoleHealthy {
		id, service = "autotune-vnext-cloudflare-control-v1", "cloudflare-control"
	} else {
		for _, preset := range productVNextExperimentTargets {
			if raw == preset.Target {
				id, service = "autotune-vnext-"+preset.ID+"-v1", preset.ID
				break
			}
		}
	}
	allowed := make([]int, 0, 200)
	for status := 200; status < 400; status++ {
		allowed = append(allowed, status)
	}
	spec := observatory.ProbeSpec{SchemaVersion: observatory.ProbeSpecSchemaVersion, ID: id, ServiceID: service, TargetContractRevision: "https-reachability-2xx-3xx-v1", Target: observatory.Target{URL: parsed.String(), Hostname: strings.ToLower(parsed.Hostname()), Port: port, RequestedProtocol: observatory.TransportTCP}, Transport: observatory.TransportTCP, AddressFamilyPolicy: target.AddressFamily, Mode: observatory.ProbeModeHTTPSGet, ExpectedResponse: observatory.ExpectedResponse{AllowedStatusCodes: allowed, RequirePathComplete: true}, ControlRole: role, Privacy: observatory.PrivacyModeRedacted}
	return spec, spec.Validate()
}

type productHistoryAdvisor struct {
	ledger  outcomeledger.Ledger
	probe   observatory.ProbeSpec
	backend backendcap.Backend
	// identities is the run's identity cohort. History may only reorder candidates
	// Planner already deemed eligible; it never adds one, never grants Apply and
	// never short-circuits current validation.
	identities historyid.Bundle
	now        func() time.Time
	onAdvise   func()
}

func (advisor productHistoryAdvisor) Advise(context autotunevnext.AdvisorContext) autotunevnext.Advice {
	if advisor.onAdvise != nil {
		advisor.onAdvise()
	}
	identity, err := advisor.probe.Identity()
	if err != nil {
		return autotunevnext.Advice{}
	}
	now := advisor.now().UTC()
	candidates := make([]recommendation.Candidate, 0, len(context.Eligible))
	matches := make(map[string][]outcomeledger.Match, len(context.Eligible))
	for _, candidate := range context.Eligible {
		candidates = append(candidates, recommendation.Candidate{StrategyID: candidate.StrategyID, StrategyFingerprint: candidate.StrategyFingerprint, Safety: candidate.Safety})
		matches[candidate.StrategyFingerprint] = outcomeledger.QueryLedger(advisor.ledger, outcomeledger.Query{ProbeIdentity: identity, ServiceID: advisor.probe.ServiceID, TargetContractRevision: advisor.probe.TargetContractRevision, Transport: advisor.probe.Transport, AddressFamily: context.Family, DiagnosisKind: context.Diagnosis.Kind, StrategyFingerprint: candidate.StrategyFingerprint, Backend: advisor.backend, BackendFingerprint: string(advisor.identities.BackendFingerprint), CapabilityFingerprint: string(advisor.identities.CapabilityFingerprint), ContextKey: string(advisor.identities.ContextKey), Now: now})
	}
	report := recommendation.Recommend(recommendation.Input{Diagnosis: context.Diagnosis, Planner: context.Planner, Candidates: candidates, Matches: matches})
	advice := autotunevnext.Advice{SchemaVersion: report.SchemaVersion, DiagnosisID: report.DiagnosisID, PlannerAttributionID: report.PlannerAttributionID, Disposition: autotunevnext.RecommendationDisposition(report.Disposition), Limitations: append([]string(nil), report.Limitations...)}
	for _, candidate := range report.CandidateRecommendations {
		advice.Candidates = append(advice.Candidates, autotunevnext.CandidateAdvice{StrategyFingerprint: candidate.StrategyFingerprint, ReasonCodes: append([]string(nil), candidate.ReasonCodes...), HistoryUsed: candidate.HistoryUsed})
	}
	return advice
}

type productOutcomeLedger struct {
	ledger      outcomeledger.Ledger
	writable    bool
	limitations []string
}

// loadProductOutcomeLedger attaches the run's ACTUAL identity availability to the
// loaded history. An identity that was produced must not be reported as
// unavailable; an identity that genuinely failed must be, so a user is never
// told history reuse is safe when it is not.
func loadProductOutcomeLedger(identities historyid.Bundle) productOutcomeLedger {
	vNextHistoryMu.Lock()
	defer vNextHistoryMu.Unlock()

	identityLimitations := identities.Limitations()
	withIdentities := func(base []string) []string {
		return append(append([]string(nil), base...), identityLimitations...)
	}
	path, err := getVNextOutcomeLedgerPath()
	if err != nil {
		return productOutcomeLedger{ledger: outcomeledger.Ledger{SchemaVersion: outcomeledger.SchemaVersion}, limitations: withIdentities([]string{"HISTORY_UNAVAILABLE_LOAD_FAILED"})}
	}
	loaded := outcomeledger.Load(path)
	switch loaded.State {
	case outcomeledger.LoadValid, outcomeledger.LoadEmpty:
		return productOutcomeLedger{ledger: loaded.Ledger, writable: true, limitations: withIdentities(nil)}
	case outcomeledger.LoadCorrupt:
		return productOutcomeLedger{ledger: outcomeledger.Ledger{SchemaVersion: outcomeledger.SchemaVersion}, limitations: withIdentities([]string{"HISTORY_UNAVAILABLE_CORRUPT"})}
	default:
		return productOutcomeLedger{ledger: outcomeledger.Ledger{SchemaVersion: outcomeledger.SchemaVersion}, limitations: withIdentities([]string{"HISTORY_UNAVAILABLE_UNSUPPORTED_VERSION"})}
	}
}

// persistProductOutcomes is deliberately after fresh current execution. A
// persistence failure changes only history availability, never the result,
// lifecycle, VERIFIED_FIXED finding, or Apply grant of that current run.
func persistProductOutcomes(result autotunevnext.Result, history productOutcomeLedger, identities historyid.Bundle) string {
	if len(result.OutcomeEvidence) == 0 {
		return ""
	}
	if !history.writable {
		return "HISTORY_PERSIST_SKIPPED_HISTORY_UNAVAILABLE"
	}
	if result.DiagnosisReport.DiagnosisID == "" || len(result.BaselineEvidence) == 0 {
		return "HISTORY_ENTRY_REJECTED"
	}
	byFingerprint := make(map[string]autotunevnext.CandidateExperiment, len(result.Experiments))
	for _, experiment := range result.Experiments {
		byFingerprint[experiment.Fingerprint] = experiment
	}
	updated := history.ledger
	changed := false
	for _, provenance := range result.OutcomeEvidence {
		experiment, exists := byFingerprint[provenance.StrategyFingerprint]
		if !exists {
			return "HISTORY_ENTRY_REJECTED"
		}
		entry, err := outcomeledger.NewEntry(outcomeledger.BuildInput{
			Evidence: result.BaselineEvidence, ValidationEvidence: provenance.Validation, Diagnosis: result.DiagnosisReport,
			StrategyID: experiment.StrategyID, StrategyFingerprint: experiment.Fingerprint, Backend: result.Backend,
			Outcome: provenance.Outcome, RecordedAt: result.DiagnosisReport.EffectiveAt,
			// The SAME cohort the history query used, so this entry is comparable
			// with what was just asked for.
			BackendFingerprint:    string(identities.BackendFingerprint),
			CapabilityFingerprint: string(identities.CapabilityFingerprint),
			ContextKey:            string(identities.ContextKey),
		}, outcomeledger.DefaultPolicy())
		if err != nil {
			return "HISTORY_ENTRY_REJECTED"
		}
		updated, err = outcomeledger.Add(updated, entry, outcomeledger.DefaultPolicy(), result.DiagnosisReport.EffectiveAt)
		if err != nil {
			return "HISTORY_PERSIST_FAILED"
		}
		changed = true
	}
	if !changed {
		return ""
	}
	vNextHistoryMu.Lock()
	defer vNextHistoryMu.Unlock()
	path, err := getVNextOutcomeLedgerPath()
	if err != nil || outcomeledger.Save(path, updated) != nil {
		return "HISTORY_PERSIST_FAILED"
	}
	return ""
}
