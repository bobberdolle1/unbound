package main

import (
	"net/url"
	"path/filepath"
	"strings"
	"time"

	"unbound/engine"
	"unbound/engine/autotunevnext"
	"unbound/engine/backendcap"
	"unbound/engine/observatory"
	"unbound/engine/outcomeledger"
	"unbound/engine/recommendation"
)

const vNextOutcomeLedgerFile = outcomeledger.FileName

func getVNextOutcomeLedgerPath() (string, error) {
	directory, err := engine.GetConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(directory, vNextOutcomeLedgerFile), nil
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
	now     func() time.Time
}

func (advisor productHistoryAdvisor) Advise(context autotunevnext.AdvisorContext) autotunevnext.Advice {
	identity, err := advisor.probe.Identity()
	if err != nil {
		return autotunevnext.Advice{}
	}
	candidates := make([]recommendation.Candidate, 0, len(context.Eligible))
	matches := make(map[string][]outcomeledger.Match, len(context.Eligible))
	for _, candidate := range context.Eligible {
		candidates = append(candidates, recommendation.Candidate{StrategyID: candidate.StrategyID, StrategyFingerprint: candidate.StrategyFingerprint, Safety: candidate.Safety})
		matches[candidate.StrategyFingerprint] = outcomeledger.QueryLedger(advisor.ledger, outcomeledger.Query{ProbeIdentity: identity, ServiceID: advisor.probe.ServiceID, TargetContractRevision: advisor.probe.TargetContractRevision, Transport: advisor.probe.Transport, AddressFamily: context.Family, StrategyFingerprint: candidate.StrategyFingerprint, Backend: advisor.backend, Now: advisor.now().UTC()})
	}
	report := recommendation.Recommend(recommendation.Input{Diagnosis: context.Diagnosis, Planner: context.Planner, Candidates: candidates, Matches: matches})
	advice := autotunevnext.Advice{SchemaVersion: report.SchemaVersion, DiagnosisID: report.DiagnosisID, PlannerAttributionID: report.PlannerAttributionID, Limitations: append([]string(nil), report.Limitations...)}
	for _, candidate := range report.CandidateRecommendations {
		advice.Candidates = append(advice.Candidates, autotunevnext.CandidateAdvice{StrategyFingerprint: candidate.StrategyFingerprint, ReasonCodes: append([]string(nil), candidate.ReasonCodes...), HistoryUsed: candidate.HistoryUsed})
	}
	return advice
}

func loadProductOutcomeLedger() (outcomeledger.Ledger, []string) {
	path, err := getVNextOutcomeLedgerPath()
	if err != nil {
		return outcomeledger.Ledger{SchemaVersion: outcomeledger.SchemaVersion}, []string{"HISTORY_UNAVAILABLE_LOAD_FAILED"}
	}
	loaded := outcomeledger.Load(path)
	switch loaded.State {
	case outcomeledger.LoadValid, outcomeledger.LoadEmpty:
		return loaded.Ledger, []string{"CONTEXT_IDENTITY_UNAVAILABLE", "CAPABILITY_IDENTITY_UNAVAILABLE"}
	case outcomeledger.LoadCorrupt:
		return outcomeledger.Ledger{SchemaVersion: outcomeledger.SchemaVersion}, []string{"HISTORY_UNAVAILABLE_CORRUPT", "CONTEXT_IDENTITY_UNAVAILABLE", "CAPABILITY_IDENTITY_UNAVAILABLE"}
	default:
		return outcomeledger.Ledger{SchemaVersion: outcomeledger.SchemaVersion}, []string{"HISTORY_UNAVAILABLE_UNSUPPORTED_VERSION", "CONTEXT_IDENTITY_UNAVAILABLE", "CAPABILITY_IDENTITY_UNAVAILABLE"}
	}
}

// persistProductOutcomes is deliberately after fresh current execution. A
// persistence failure changes only history availability, never the result,
// lifecycle, VERIFIED_FIXED finding, or Apply grant of that current run.
func persistProductOutcomes(result autotunevnext.Result, ledger outcomeledger.Ledger) string {
	if result.DiagnosisReport.DiagnosisID == "" || len(result.BaselineEvidence) == 0 || len(result.OutcomeEvidence) == 0 {
		return ""
	}
	byFingerprint := make(map[string]autotunevnext.CandidateExperiment, len(result.Experiments))
	for _, experiment := range result.Experiments {
		byFingerprint[experiment.Fingerprint] = experiment
	}
	updated := ledger
	changed := false
	for _, provenance := range result.OutcomeEvidence {
		experiment, exists := byFingerprint[provenance.StrategyFingerprint]
		if !exists {
			continue
		}
		entry, err := outcomeledger.NewEntry(outcomeledger.BuildInput{
			Evidence: result.BaselineEvidence, ValidationEvidence: provenance.Validation, Diagnosis: result.DiagnosisReport,
			StrategyID: experiment.StrategyID, StrategyFingerprint: experiment.Fingerprint, Backend: result.Backend,
			Outcome: provenance.Outcome, RecordedAt: result.DiagnosisReport.EffectiveAt,
		}, outcomeledger.DefaultPolicy())
		if err != nil {
			continue
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
	path, err := getVNextOutcomeLedgerPath()
	if err != nil || outcomeledger.Save(path, updated) != nil {
		return "HISTORY_PERSIST_FAILED"
	}
	return ""
}
