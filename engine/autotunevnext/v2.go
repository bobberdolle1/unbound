package autotunevnext

import (
	"fmt"
	"sort"

	"unbound/engine/attribution"
	"unbound/engine/diagnosis"
	"unbound/engine/observatory"
	"unbound/engine/planner"
)

const advisorSchemaVersion = 1

func buildBaselineDiagnosis(request Request, attributionReport attribution.AttributionReport, baseline observatory.ObservationResult, controls []controlBaseline) (diagnosis.Report, []observatory.EvidenceRecord, error) {
	if request.TargetProbe == nil {
		return diagnosis.Report{}, nil, fmt.Errorf("target probe contract is absent")
	}
	if len(request.ControlProbes) != len(controls) {
		return diagnosis.Report{}, nil, fmt.Errorf("control probe contracts do not match controls")
	}
	targetRecord, err := observatory.BuildEvidenceRecord(*request.TargetProbe, observatory.EvidenceInput{Observations: []observatory.ObservationResult{baseline}})
	if err != nil {
		return diagnosis.Report{}, nil, fmt.Errorf("target evidence: %w", err)
	}
	controlRecords := make([]observatory.EvidenceRecord, 0, len(controls))
	for i, control := range controls {
		record, err := observatory.BuildEvidenceRecord(request.ControlProbes[i], observatory.EvidenceInput{Observations: []observatory.ObservationResult{control.observation}})
		if err != nil {
			return diagnosis.Report{}, nil, fmt.Errorf("control evidence: %w", err)
		}
		controlRecords = append(controlRecords, record)
	}
	diagnosisReport, err := diagnosis.Diagnose(diagnosis.Input{Target: []observatory.EvidenceRecord{targetRecord}, Controls: controlRecords, Attribution: attributionReport})
	if err != nil {
		return diagnosis.Report{}, nil, fmt.Errorf("diagnosis: %w", err)
	}
	return diagnosisReport, append([]observatory.EvidenceRecord{targetRecord}, controlRecords...), nil
}

func advisorContext(report diagnosis.Report, plan planner.PlannerReport, candidates []candidateInput, family observatory.AddressFamily) AdvisorContext {
	context := AdvisorContext{Diagnosis: report, Planner: plan, Family: family}
	for _, candidate := range candidates {
		if candidate.assessment.Status == planner.StatusEligible {
			context.Eligible = append(context.Eligible, CandidateIdentity{StrategyID: candidate.assessment.StrategyID, StrategyFingerprint: candidate.assessment.StrategyFingerprint, Safety: candidate.assessment.Safety})
		}
	}
	return context
}

func diagnosisDisposition(report diagnosis.Report) RecommendationDisposition {
	switch report.Kind {
	case diagnosis.KindNoAnomaly:
		return RecommendationNoAction
	case diagnosis.KindHTTPApplicationFailure, diagnosis.KindNetworkContextFailure:
		return RecommendationNoPacketStrategy
	case diagnosis.KindUnknown, diagnosis.KindInsufficientEvidence:
		return RecommendationInsufficientEvidence
	default:
		return RecommendationExperimentCandidates
	}
}

func terminalStatus(disposition RecommendationDisposition) Status {
	switch disposition {
	case RecommendationNoAction:
		return StatusCompletedNoActionNeeded
	case RecommendationNoEligibleCandidates:
		return StatusCompletedNoEligibleCandidates
	default:
		return StatusInconclusive
	}
}

func validateAndOrderAdvice(advice Advice, context AdvisorContext, candidates []candidateInput) ([]candidateInput, RecommendationDisposition, bool) {
	if advice.SchemaVersion != advisorSchemaVersion || advice.DiagnosisID == "" || advice.DiagnosisID != context.Diagnosis.DiagnosisID || advice.PlannerAttributionID == "" || advice.PlannerAttributionID != context.Planner.AttributionID {
		return candidates, "", false
	}
	switch advice.Disposition {
	case RecommendationNoAction, RecommendationNoPacketStrategy, RecommendationInsufficientEvidence, RecommendationNoEligibleCandidates:
		if len(advice.Candidates) != 0 {
			return candidates, "", false
		}
		return candidates, advice.Disposition, true
	case RecommendationExperimentCandidates:
	default:
		return candidates, "", false
	}
	if len(advice.Candidates) != len(context.Eligible) {
		return candidates, "", false
	}
	byFingerprint := make(map[string]CandidateAdvice, len(advice.Candidates))
	for _, candidate := range advice.Candidates {
		if candidate.StrategyFingerprint == "" {
			return candidates, "", false
		}
		if _, duplicate := byFingerprint[candidate.StrategyFingerprint]; duplicate {
			return candidates, "", false
		}
		byFingerprint[candidate.StrategyFingerprint] = candidate
	}
	eligible := make(map[string]CandidateIdentity, len(context.Eligible))
	for _, candidate := range context.Eligible {
		eligible[candidate.StrategyFingerprint] = candidate
	}
	for fingerprint := range byFingerprint {
		if _, ok := eligible[fingerprint]; !ok {
			return candidates, "", false
		}
	}
	ordered := append([]candidateInput(nil), candidates...)
	rank := make(map[string]int, len(advice.Candidates))
	for i, adviceCandidate := range advice.Candidates {
		rank[adviceCandidate.StrategyFingerprint] = i
	}
	for i := range ordered {
		if adviceCandidate, ok := byFingerprint[ordered[i].assessment.StrategyFingerprint]; ok {
			ordered[i].recommendationReason = append([]string(nil), adviceCandidate.ReasonCodes...)
		}
	}
	sort.SliceStable(ordered, func(i, j int) bool {
		left, leftOK := rank[ordered[i].assessment.StrategyFingerprint]
		right, rightOK := rank[ordered[j].assessment.StrategyFingerprint]
		if leftOK && rightOK {
			return left < right
		}
		if leftOK != rightOK {
			return leftOK
		}
		return safetyLess(ordered[i], ordered[j])
	})
	return ordered, advice.Disposition, true
}

func buildCandidateValidationEvidence(request Request, fingerprint string, target []observatory.ObservationResult, controls []controlBaseline, results []ControlResult) ([]observatory.EvidenceRecord, error) {
	if request.TargetProbe == nil || len(target) != 3 {
		return nil, nil
	}
	targetRecord, err := observatory.BuildEvidenceRecord(*request.TargetProbe, observatory.EvidenceInput{Observations: target, ExperimentID: "candidate-" + fingerprint, StrategyFingerprint: fingerprint})
	if err != nil {
		return nil, fmt.Errorf("target validation evidence: %w", err)
	}
	records := []observatory.EvidenceRecord{targetRecord}
	if len(controls) != len(results) || len(request.ControlProbes) != len(controls) {
		if len(controls) == 0 {
			return records, nil
		}
		return nil, fmt.Errorf("control validation contracts do not match observations")
	}
	for index := range controls {
		active := results[index].activeObservation
		if active == nil {
			return nil, fmt.Errorf("control active validation observation is absent")
		}
		record, err := observatory.BuildEvidenceRecord(request.ControlProbes[index], observatory.EvidenceInput{
			Observations: []observatory.ObservationResult{controls[index].observation, *active},
			ExperimentID: "candidate-" + fingerprint, StrategyFingerprint: fingerprint,
		})
		if err != nil {
			return nil, fmt.Errorf("control validation evidence: %w", err)
		}
		records = append(records, record)
	}
	return records, nil
}
