package recommendation

import (
	"testing"
	"time"

	"unbound/engine/attribution"
	"unbound/engine/diagnosis"
	"unbound/engine/outcomeledger"
	"unbound/engine/planner"
	"unbound/engine/strategyir"
)

func TestRecommendOrdersCompatibleHistoryWithoutChangingEligibility(t *testing.T) {
	input := testInput()
	input.Matches = map[string][]outcomeledger.Match{
		"b": {{Status: outcomeledger.MatchCompatible, Confidence: attribution.ConfidenceHigh, Entry: outcomeledger.OutcomeEntry{EntryID: "positive", Outcome: "VERIFIED_FIXED", RecordedAt: testNow}}},
		"a": {{Status: outcomeledger.MatchStale, Entry: outcomeledger.OutcomeEntry{EntryID: "stale", Outcome: "VERIFIED_FIXED", RecordedAt: testNow.Add(time.Hour)}}},
	}
	report := Recommend(input)
	if report.Disposition != DispositionExperimentCandidates {
		t.Fatalf("disposition = %s", report.Disposition)
	}
	if len(report.CandidateRecommendations) != 2 || report.CandidateRecommendations[0].StrategyFingerprint != "b" {
		t.Fatalf("unexpected order: %#v", report.CandidateRecommendations)
	}
	if report.CandidateRecommendations[1].HistoryUsed {
		t.Fatal("stale history influenced recommendation")
	}
	if input.Planner.Candidates[2].Status != planner.StatusStructurallyInapplicable {
		t.Fatal("planner status changed")
	}
}

func TestRecommendNegativeNewestEvidenceDemotesAndNeverBans(t *testing.T) {
	input := testInput()
	input.Matches = map[string][]outcomeledger.Match{
		"a": {
			{Status: outcomeledger.MatchCompatible, Confidence: attribution.ConfidenceHigh, Entry: outcomeledger.OutcomeEntry{EntryID: "old-positive", Outcome: "VERIFIED_FIXED", RecordedAt: testNow}},
			{Status: outcomeledger.MatchCompatible, Confidence: attribution.ConfidenceMedium, Entry: outcomeledger.OutcomeEntry{EntryID: "new-negative", Outcome: "STILL_FAILING", RecordedAt: testNow.Add(time.Minute)}},
		},
	}
	report := Recommend(input)
	if report.CandidateRecommendations[1].StrategyFingerprint != "a" || report.CandidateRecommendations[1].PriorityTier != PriorityRecentNegative {
		t.Fatalf("newest negative did not demote without exclusion: %#v", report.CandidateRecommendations)
	}
}

func TestRecommendSafetyTieBreakAndPermutations(t *testing.T) {
	input := testInput()
	input.Candidates[0], input.Candidates[1] = input.Candidates[1], input.Candidates[0]
	report := Recommend(input)
	if report.CandidateRecommendations[0].StrategyFingerprint != "a" {
		t.Fatalf("safety ordering lost: %#v", report.CandidateRecommendations)
	}
	input.Matches = map[string][]outcomeledger.Match{
		"a": {{Status: outcomeledger.MatchExpired, Entry: outcomeledger.OutcomeEntry{Outcome: "VERIFIED_FIXED"}}},
		"b": {{Status: outcomeledger.MatchInvalid, Entry: outcomeledger.OutcomeEntry{Outcome: "VERIFIED_FIXED"}}},
	}
	report = Recommend(input)
	if report.CandidateRecommendations[0].StrategyFingerprint != "a" {
		t.Fatal("expired or invalid history influenced order")
	}
}

func TestRecommendConservativeDiagnosisAndMalformedIdentity(t *testing.T) {
	for _, kind := range []diagnosis.Kind{diagnosis.KindUnknown, diagnosis.KindInsufficientEvidence, diagnosis.KindNetworkContextFailure, diagnosis.KindHTTPApplicationFailure, diagnosis.KindNoAnomaly} {
		input := testInput()
		input.Diagnosis.Kind = kind
		if report := Recommend(input); len(report.CandidateRecommendations) != 0 {
			t.Fatalf("%s returned experiments", kind)
		}
	}
	input := testInput()
	input.Diagnosis.AttributionID = "other"
	if report := Recommend(input); report.Disposition != DispositionInsufficientEvidence {
		t.Fatalf("identity mismatch = %s", report.Disposition)
	}
}

func TestRecommendUsesNewestEffectivenessEventAndConfidencePolicy(t *testing.T) {
	input := testInput()
	input.Matches = map[string][]outcomeledger.Match{
		"a": {
			{Status: outcomeledger.MatchCompatible, Confidence: attribution.ConfidenceHigh, Entry: outcomeledger.OutcomeEntry{EntryID: "negative", Outcome: "STILL_FAILING", RecordedAt: testNow}},
			{Status: outcomeledger.MatchCompatible, Confidence: attribution.ConfidenceHigh, Entry: outcomeledger.OutcomeEntry{EntryID: "inconclusive", Outcome: "INCONCLUSIVE", RecordedAt: testNow.Add(time.Minute)}},
		},
		"b": {{Status: outcomeledger.MatchCompatible, Confidence: attribution.ConfidenceLow, Entry: outcomeledger.OutcomeEntry{EntryID: "weak-positive", Outcome: "VERIFIED_FIXED", RecordedAt: testNow.Add(time.Minute)}}},
	}
	report := Recommend(input)
	if report.CandidateRecommendations[0].StrategyFingerprint != "b" || report.CandidateRecommendations[0].HistoryUsed {
		t.Fatalf("low-confidence verified history must remain neutral: %#v", report.CandidateRecommendations)
	}
	if report.CandidateRecommendations[1].StrategyFingerprint != "a" || report.CandidateRecommendations[1].PriorityTier != PriorityRecentNegative {
		t.Fatalf("newer non-effectiveness history masked negative evidence: %#v", report.CandidateRecommendations)
	}
}

var testNow = time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)

func testInput() Input {
	candidates := []Candidate{{StrategyID: "safe", StrategyFingerprint: "a"}, {StrategyID: "aggressive", StrategyFingerprint: "b"}, {StrategyID: "ineligible", StrategyFingerprint: "c"}}
	return Input{
		Diagnosis: diagnosis.Report{DiagnosisID: "diagnosis", AttributionID: "attribution", Kind: diagnosis.KindTCPPathFailure},
		Planner: planner.PlannerReport{AttributionID: "attribution", Disposition: planner.DispositionCandidatesAvailable, Candidates: []planner.CandidateAssessment{
			{StrategyID: "safe", StrategyFingerprint: "a", Status: planner.StatusEligible, Safety: strategyir.SafetyPolicy{TargetOnly: true, Aggressiveness: "LOW"}},
			{StrategyID: "aggressive", StrategyFingerprint: "b", Status: planner.StatusEligible, Safety: strategyir.SafetyPolicy{TargetOnly: false, Aggressiveness: "HIGH"}},
			{StrategyID: "ineligible", StrategyFingerprint: "c", Status: planner.StatusStructurallyInapplicable},
		}}, Candidates: candidates,
	}
}
