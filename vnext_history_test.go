package main

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
	"unbound/engine/historyid"

	"unbound/engine"
	"unbound/engine/autotunevnext"
	"unbound/engine/backendcap"
	"unbound/engine/diagnosis"
	"unbound/engine/observatory"
	"unbound/engine/outcomeledger"
	"unbound/engine/planner"
)

func TestProductOutcomeLedgerUnavailableIsReadOnly(t *testing.T) {
	restore := engine.SetConfigDirForTest(t.TempDir())
	defer restore()
	path, err := getVNextOutcomeLedgerPath()
	if err != nil {
		t.Fatal(err)
	}
	for _, contents := range [][]byte{[]byte("not-json"), []byte(`{"schema_version":99}`)} {
		if err := os.WriteFile(path, contents, 0600); err != nil {
			t.Fatal(err)
		}
		history := loadProductOutcomeLedger(historyid.Bundle{})
		if history.writable {
			t.Fatalf("unavailable history became writable: %#v", history)
		}
		before, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		limitation := persistProductOutcomes(autotunevnext.Result{OutcomeEvidence: []autotunevnext.OutcomeEvidence{{}}}, history, historyid.Bundle{})
		if limitation != "HISTORY_PERSIST_SKIPPED_HISTORY_UNAVAILABLE" {
			t.Fatalf("limitation=%q", limitation)
		}
		after, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		if string(after) != string(before) {
			t.Fatalf("unavailable ledger was overwritten: before=%q after=%q", before, after)
		}
	}
}

func TestProductOutcomeEntryRejectionIsReported(t *testing.T) {
	history := productOutcomeLedger{writable: true}
	result := autotunevnext.Result{OutcomeEvidence: []autotunevnext.OutcomeEvidence{{}}}
	if limitation := persistProductOutcomes(result, history, historyid.Bundle{}); limitation != "HISTORY_ENTRY_REJECTED" {
		t.Fatalf("entry rejection was silent: %q", limitation)
	}
}

func TestProductHistoryAdvisorSnapshotsClockOnce(t *testing.T) {
	restore := engine.SetConfigDirForTest(t.TempDir())
	defer restore()
	target, _, err := normalizeVNextTarget("https://example.test/")
	if err != nil {
		t.Fatal(err)
	}
	probe, err := productProbeSpec("", target, observatory.ControlRolePrimary)
	if err != nil {
		t.Fatal(err)
	}
	calls := 0
	advisor := productHistoryAdvisor{ledger: loadProductOutcomeLedger(historyid.Bundle{}).ledger, probe: probe, backend: backendcap.Zapret2Windows, now: func() time.Time { calls++; return time.Date(2026, 1, 2, 3, 4, calls, 0, time.UTC) }}
	context := autotunevnext.AdvisorContext{Diagnosis: diagnosis.Report{DiagnosisID: "diagnosis", AttributionID: "attribution", Kind: diagnosis.KindTCPPathFailure}, Planner: planner.PlannerReport{AttributionID: "attribution", Disposition: planner.DispositionCandidatesAvailable, Candidates: []planner.CandidateAssessment{{StrategyID: "a", StrategyFingerprint: testFingerprint("a"), Status: planner.StatusEligible}, {StrategyID: "b", StrategyFingerprint: testFingerprint("b"), Status: planner.StatusEligible}}}, Family: observatory.AddressFamilyIPv4, Eligible: []autotunevnext.CandidateIdentity{{StrategyID: "a", StrategyFingerprint: testFingerprint("a")}, {StrategyID: "b", StrategyFingerprint: testFingerprint("b")}}}
	advice := advisor.Advise(context)
	if calls != 1 || advice.Disposition != autotunevnext.RecommendationExperimentCandidates {
		t.Fatalf("advisor clock/disposition=%d %#v", calls, advice)
	}
}

func TestProductPersistenceWritesFreshOutcome(t *testing.T) {
	restore := engine.SetConfigDirForTest(t.TempDir())
	defer restore()
	target, _, err := normalizeVNextTarget("https://target.test/")
	if err != nil {
		t.Fatal(err)
	}
	catalog, err := productionVNextStrategyCatalog("target.test")
	if err != nil {
		t.Fatal(err)
	}
	probe, controls, err := productProbeContracts(target.URL, target, nil)
	if err != nil {
		t.Fatal(err)
	}
	observer := &productHistorySequenceObserver{results: []observatory.ObservationResult{
		productHistoryObservation("baseline", false), productHistoryObservation("before", false), productHistoryObservation("active", true), productHistoryObservation("after", false),
	}}
	result := autotunevnext.Run(context.Background(), autotunevnext.Request{Target: target, Strategies: catalog, Backend: backendcap.Zapret2Windows, TargetProbe: &probe, ControlProbes: controls, Policy: autotunevnext.Policy{MaxCandidates: 1, MaxDuration: time.Minute, PerObservationTimeout: time.Second}}, observer, &productVNextNoopExecutor{}, productVNextSupportedPreflight{}, productVNextAssets{})
	if len(result.OutcomeEvidence) != 1 || len(result.OutcomeEvidence[0].Validation) == 0 {
		t.Fatalf("fresh run did not produce evidence-linked outcome: %#v", result)
	}
	history := loadProductOutcomeLedger(historyid.Bundle{})
	if !history.writable {
		t.Fatalf("empty product ledger is not writable: %#v", history)
	}
	if limitation := persistProductOutcomes(result, history, historyid.Bundle{}); limitation != "" {
		t.Fatalf("fresh persistence limitation=%q", limitation)
	}
	path, err := getVNextOutcomeLedgerPath()
	if err != nil {
		t.Fatal(err)
	}
	loaded := outcomeledger.Load(path)
	if loaded.State != outcomeledger.LoadValid || len(loaded.Ledger.Entries) != 1 || len(loaded.Ledger.Entries[0].ValidationEvidenceFingerprints) == 0 {
		t.Fatalf("fresh validated result was not persisted: %#v", loaded)
	}
	if _, err := os.Stat(filepath.Join(filepath.Dir(path), vNextManagedStateFile)); !os.IsNotExist(err) {
		t.Fatalf("outcome persistence modified managed intent: %v", err)
	}
}

type productHistorySequenceObserver struct {
	results []observatory.ObservationResult
}

func (observer *productHistorySequenceObserver) Observe(context.Context, string, observatory.Options) (observatory.ObservationResult, error) {
	if len(observer.results) == 0 {
		return observatory.ObservationResult{}, os.ErrNotExist
	}
	result := observer.results[0]
	observer.results = observer.results[1:]
	return result, nil
}

func productHistoryObservation(id string, success bool) observatory.ObservationResult {
	result := managedVNextObservation(success)
	result.RunID = id
	result.NetworkContext = observatory.NetworkContext{Interface: "test", LocalAddress: "192.0.2.2", AddressFamily: observatory.AddressFamilyIPv4, NetworkLabel: "test"}
	return result
}

func testFingerprint(digit string) string { return strings.Repeat(digit, 64) }
