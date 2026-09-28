// Package recommendation deterministically orders already-eligible candidates.
// It is pure: it neither executes, persists, nor grants activation authority.
package recommendation

import (
	"sort"

	"unbound/engine/attribution"
	"unbound/engine/diagnosis"
	"unbound/engine/outcomeledger"
	"unbound/engine/planner"
	"unbound/engine/strategyir"
)

const SchemaVersion = 1

type Disposition string

const (
	DispositionExperimentCandidates Disposition = "EXPERIMENT_CANDIDATES"
	DispositionNoAction             Disposition = "NO_ACTION"
	DispositionNoPacketStrategy     Disposition = "NO_PACKET_STRATEGY"
	DispositionInsufficientEvidence Disposition = "INSUFFICIENT_EVIDENCE"
	DispositionNoEligibleCandidates Disposition = "NO_ELIGIBLE_CANDIDATES"
)

type PriorityTier string

const (
	PriorityRecentVerified PriorityTier = "RECENT_VERIFIED"
	PriorityNeutral        PriorityTier = "NEUTRAL"
	PriorityRecentNegative PriorityTier = "RECENT_NEGATIVE"
)

type Candidate struct {
	StrategyID          string
	StrategyFingerprint string
	Safety              strategyir.SafetyPolicy
}

type Input struct {
	Diagnosis  diagnosis.Report
	Planner    planner.PlannerReport
	Candidates []Candidate
	// Matches are ledger query results keyed by strategy fingerprint. Only
	// COMPATIBLE matches are considered; all other states are deliberately inert.
	Matches map[string][]outcomeledger.Match
}

type CandidateRecommendation struct {
	StrategyID          string       `json:"strategy_id"`
	StrategyFingerprint string       `json:"strategy_fingerprint"`
	PriorityTier        PriorityTier `json:"priority_tier"`
	ReasonCodes         []string     `json:"reason_codes"`
	HistoryUsed         bool         `json:"history_used"`
}

type Report struct {
	SchemaVersion            int                       `json:"schema_version"`
	DiagnosisID              string                    `json:"diagnosis_id"`
	PlannerAttributionID     string                    `json:"planner_attribution_id"`
	Disposition              Disposition               `json:"disposition"`
	CandidateRecommendations []CandidateRecommendation `json:"candidate_recommendations,omitempty"`
	Limitations              []string                  `json:"limitations,omitempty"`
}

// Recommend preserves Planner as the structural authority. It can only reorder
// the supplied already-eligible candidates and falls back to V1 safety order.
func Recommend(input Input) Report {
	report := Report{SchemaVersion: SchemaVersion, DiagnosisID: input.Diagnosis.DiagnosisID, PlannerAttributionID: input.Planner.AttributionID}
	if input.Diagnosis.AttributionID == "" || input.Diagnosis.AttributionID != input.Planner.AttributionID {
		report.Disposition = DispositionInsufficientEvidence
		report.Limitations = []string{"DIAGNOSIS_PLANNER_IDENTITY_MISMATCH"}
		return report
	}
	if disposition, terminal := diagnosisDisposition(input.Diagnosis.Kind); terminal {
		report.Disposition = disposition
		return report
	}
	if input.Planner.Disposition != planner.DispositionCandidatesAvailable {
		report.Disposition = DispositionNoEligibleCandidates
		return report
	}
	eligible := eligibleCandidates(input)
	if len(eligible) == 0 {
		report.Disposition = DispositionNoEligibleCandidates
		return report
	}
	items := make([]recommendationItem, 0, len(eligible))
	for _, candidate := range eligible {
		tier, reasons, used := historySignal(input.Matches[candidate.StrategyFingerprint])
		items = append(items, recommendationItem{candidate: candidate, tier: tier, reasons: reasons, historyUsed: used})
	}
	sort.Slice(items, func(i, j int) bool {
		if tierRank(items[i].tier) != tierRank(items[j].tier) {
			return tierRank(items[i].tier) < tierRank(items[j].tier)
		}
		return safetyLess(items[i].candidate, items[j].candidate)
	})
	report.Disposition = DispositionExperimentCandidates
	for _, item := range items {
		report.CandidateRecommendations = append(report.CandidateRecommendations, CandidateRecommendation{
			StrategyID: item.candidate.StrategyID, StrategyFingerprint: item.candidate.StrategyFingerprint,
			PriorityTier: item.tier, ReasonCodes: item.reasons, HistoryUsed: item.historyUsed,
		})
	}
	return report
}

type recommendationItem struct {
	candidate   Candidate
	tier        PriorityTier
	reasons     []string
	historyUsed bool
}

func diagnosisDisposition(kind diagnosis.Kind) (Disposition, bool) {
	switch kind {
	case diagnosis.KindNoAnomaly:
		return DispositionNoAction, true
	case diagnosis.KindHTTPApplicationFailure, diagnosis.KindNetworkContextFailure:
		return DispositionNoPacketStrategy, true
	case diagnosis.KindUnknown, diagnosis.KindInsufficientEvidence:
		return DispositionInsufficientEvidence, true
	default:
		return "", false
	}
}

func eligibleCandidates(input Input) []Candidate {
	eligible := make(map[string]planner.CandidateAssessment, len(input.Planner.Candidates))
	for _, assessment := range input.Planner.Candidates {
		if assessment.Status == planner.StatusEligible && assessment.StrategyFingerprint != "" {
			eligible[assessment.StrategyFingerprint] = assessment
		}
	}
	out := make([]Candidate, 0, len(input.Candidates))
	seen := make(map[string]struct{}, len(input.Candidates))
	for _, candidate := range input.Candidates {
		assessment, ok := eligible[candidate.StrategyFingerprint]
		if !ok || candidate.StrategyID != assessment.StrategyID {
			continue
		}
		if _, duplicate := seen[candidate.StrategyFingerprint]; duplicate {
			continue
		}
		seen[candidate.StrategyFingerprint] = struct{}{}
		candidate.Safety = assessment.Safety
		out = append(out, candidate)
	}
	return out
}

func historySignal(matches []outcomeledger.Match) (PriorityTier, []string, bool) {
	compatible := make([]outcomeledger.Match, 0, len(matches))
	for _, match := range matches {
		if match.Status == outcomeledger.MatchCompatible {
			compatible = append(compatible, match)
		}
	}
	if len(compatible) == 0 {
		return PriorityNeutral, []string{"NO_COMPATIBLE_HISTORY"}, false
	}
	sort.Slice(compatible, func(i, j int) bool {
		left, right := compatible[i], compatible[j]
		if !left.Entry.RecordedAt.Equal(right.Entry.RecordedAt) {
			return left.Entry.RecordedAt.After(right.Entry.RecordedAt)
		}
		if confidenceRank(left.Confidence) != confidenceRank(right.Confidence) {
			return confidenceRank(left.Confidence) > confidenceRank(right.Confidence)
		}
		return left.Entry.EntryID < right.Entry.EntryID
	})
	newest := compatible[0]
	switch newest.Entry.Outcome {
	case "VERIFIED_FIXED":
		return PriorityRecentVerified, []string{"COMPATIBLE_RECENT_VERIFIED"}, true
	case "STILL_FAILING", "REGRESSION_OBSERVED":
		return PriorityRecentNegative, []string{"COMPATIBLE_RECENT_NEGATIVE"}, true
	default:
		return PriorityNeutral, []string{"COMPATIBLE_NON_EFFECTIVENESS_HISTORY"}, true
	}
}

func confidenceRank(value attribution.Confidence) int {
	switch value {
	case attribution.ConfidenceHigh:
		return 2
	case attribution.ConfidenceMedium:
		return 1
	default:
		return 0
	}
}
func tierRank(value PriorityTier) int {
	switch value {
	case PriorityRecentVerified:
		return 0
	case PriorityNeutral:
		return 1
	default:
		return 2
	}
}
func safetyLess(left, right Candidate) bool {
	a, b := left.Safety, right.Safety
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
	if aggressionRank(a.Aggressiveness) != aggressionRank(b.Aggressiveness) {
		return aggressionRank(a.Aggressiveness) < aggressionRank(b.Aggressiveness)
	}
	if left.StrategyID != right.StrategyID {
		return left.StrategyID < right.StrategyID
	}
	return left.StrategyFingerprint < right.StrategyFingerprint
}
func aggressionRank(value string) int {
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
