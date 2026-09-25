package outcomeledger

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"unbound/engine/attribution"
	"unbound/engine/autotunevnext"
	"unbound/engine/backendcap"
	"unbound/engine/diagnosis"
	"unbound/engine/observatory"
)

func TestNewEntryDeterminismAndCorrelation(t *testing.T) {
	at := time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)
	input := testInput(t, at, autotunevnext.OutcomeVerifiedFixed)
	left, err := NewEntry(input, DefaultPolicy())
	if err != nil {
		t.Fatal(err)
	}
	right, err := NewEntry(input, DefaultPolicy())
	if err != nil || left.EntryID != right.EntryID {
		t.Fatalf("determinism: %#v %v", right, err)
	}
	input.Diagnosis.EvidenceFingerprints = []string{"evidence-v1-missing"}
	if _, err := NewEntry(input, DefaultPolicy()); err == nil {
		t.Fatal("uncorrelated diagnosis accepted")
	}
	input = testInput(t, at, autotunevnext.OutcomeNotRunBudget)
	if _, err := NewEntry(input, DefaultPolicy()); err == nil {
		t.Fatal("not-run outcome persisted as effectiveness evidence")
	}
	input = testInput(t, at, autotunevnext.OutcomeVerifiedFixed)
	input.StrategyFingerprint = ""
	if _, err := NewEntry(input, DefaultPolicy()); err == nil {
		t.Fatal("executed outcome without strategy fingerprint accepted")
	}

	input = testInput(t, at, autotunevnext.OutcomeVerifiedFixed)
	input.ContextKey = "raw-network-label"
	if _, err := NewEntry(input, DefaultPolicy()); err == nil {
		t.Fatal("raw context key accepted")
	}
}

func TestTTLDecayAndCompatibility(t *testing.T) {
	at := time.Date(2026, 1, 2, 3, 0, 0, 0, time.UTC)
	policy := DefaultPolicy()
	policy.VerifiedFixedTTL = 9 * time.Hour
	entry, err := NewEntry(testInput(t, at, autotunevnext.OutcomeVerifiedFixed), policy)
	if err != nil {
		t.Fatal(err)
	}
	query := queryFor(entry, at.Add(time.Hour))
	if match := MatchEntry(entry, query); match.Status != MatchCompatible || match.Confidence != attribution.ConfidenceHigh {
		t.Fatalf("fresh=%#v", match)
	}
	query.Now = at.Add(4 * time.Hour)
	if match := MatchEntry(entry, query); match.Confidence != attribution.ConfidenceMedium {
		t.Fatalf("middle=%#v", match)
	}
	query.Now = at.Add(7 * time.Hour)
	if match := MatchEntry(entry, query); match.Confidence != attribution.ConfidenceLow {
		t.Fatalf("old=%#v", match)
	}
	query.Now = at.Add(9 * time.Hour)
	if match := MatchEntry(entry, query); match.Status != MatchExpired {
		t.Fatalf("expired=%#v", match)
	}
	query.Now = at.Add(time.Hour)
	query.AddressFamily = observatory.AddressFamilyIPv6
	if match := MatchEntry(entry, query); match.Status != MatchIncompatible || match.Reasons[0] != ReasonFamilyChanged {
		t.Fatalf("family=%#v", match)
	}
	query = queryFor(entry, at.Add(time.Hour))
	query.TargetContractRevision = "v2"
	if match := MatchEntry(entry, query); match.Status != MatchIncompatible || match.Reasons[0] != ReasonContractChanged {
		t.Fatalf("contract=%#v", match)
	}
	query = queryFor(entry, at.Add(time.Hour))
	query.StrategyFingerprint = fingerprint("strategy", "b")
	if match := MatchEntry(entry, query); match.Status != MatchIncompatible || match.Reasons[0] != ReasonStrategyChanged {
		t.Fatalf("strategy=%#v", match)
	}
	query = queryFor(entry, at.Add(time.Hour))
	query.BackendFingerprint = fingerprint("backend", "c")
	if match := MatchEntry(entry, query); match.Status != MatchIncompatible || match.Reasons[0] != ReasonBackendChanged {
		t.Fatalf("backend=%#v", match)
	}
	entry.ContextKey = ""
	entry.EntryID, err = entryID(entry)
	if err != nil {
		t.Fatal(err)
	}
	query = queryFor(entry, at.Add(time.Hour))
	query.ContextKey = ""
	if match := MatchEntry(entry, query); match.Status != MatchStale || match.Reasons[0] != ReasonContextMissing {
		t.Fatalf("empty context=%#v", match)
	}
}

func TestContradictionAndBoundedEviction(t *testing.T) {
	at := time.Date(2026, 1, 2, 3, 0, 0, 0, time.UTC)
	policy := DefaultPolicy()
	policy.MaxEntries = 2
	positive, err := NewEntry(testInput(t, at, autotunevnext.OutcomeVerifiedFixed), policy)
	if err != nil {
		t.Fatal(err)
	}
	ledger, err := Add(Ledger{}, positive, policy, at)
	if err != nil {
		t.Fatal(err)
	}
	negativeInput := testInput(t, at.Add(time.Hour), autotunevnext.OutcomeRegressionObserved)
	negative, err := NewEntry(negativeInput, policy)
	if err != nil {
		t.Fatal(err)
	}
	ledger, err = Add(ledger, negative, policy, at.Add(time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	if len(ledger.Entries) != 2 || ledger.Entries[0].InvalidationReason != InvalidationContradicted {
		t.Fatalf("contradiction=%#v", ledger)
	}
	matches := QueryLedger(ledger, queryFor(positive, at.Add(2*time.Hour)))
	if matches[0].Status != MatchCompatible || matches[1].Status != MatchStale {
		t.Fatalf("query=%#v", matches)
	}
	inconclusive, err := NewEntry(testInput(t, at.Add(2*time.Hour), autotunevnext.OutcomeInconclusive), policy)
	if err != nil {
		t.Fatal(err)
	}
	ledger, err = Add(ledger, inconclusive, policy, at.Add(2*time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	if len(ledger.Entries) != 2 {
		t.Fatalf("bound=%#v", ledger)
	}
	for _, entry := range ledger.Entries {
		if entry.EntryID == positive.EntryID {
			t.Fatal("older invalidated evidence survived deterministic eviction")
		}
	}
	invalidated, err := Invalidate(ledger, negative.EntryID, InvalidationExplicit, at.Add(3*time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range invalidated.Entries {
		if entry.EntryID == negative.EntryID && entry.InvalidatedAt == nil {
			t.Fatal("explicit invalidation lost")
		}
	}
}

func TestDirectReachabilityInvalidatesPositiveReuse(t *testing.T) {
	at := time.Date(2026, 1, 2, 3, 0, 0, 0, time.UTC)
	positive, err := NewEntry(testInput(t, at, autotunevnext.OutcomeVerifiedFixed), DefaultPolicy())
	if err != nil {
		t.Fatal(err)
	}
	ledger, err := Add(Ledger{}, positive, DefaultPolicy(), at)
	if err != nil {
		t.Fatal(err)
	}
	direct, err := NewEntry(testInput(t, at.Add(time.Hour), autotunevnext.OutcomeDirectBecameReachable), DefaultPolicy())
	if err != nil {
		t.Fatal(err)
	}
	ledger, err = Add(ledger, direct, DefaultPolicy(), at.Add(time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range ledger.Entries {
		if entry.EntryID == positive.EntryID && entry.InvalidationReason != InvalidationContradicted {
			t.Fatalf("direct reachability did not invalidate positive: %#v", entry)
		}
	}
}

func TestPersistenceCorruptionAndPrivacy(t *testing.T) {
	at := time.Date(2026, 1, 2, 3, 0, 0, 0, time.UTC)
	entry, err := NewEntry(testInput(t, at, autotunevnext.OutcomeVerifiedFixed), DefaultPolicy())
	if err != nil {
		t.Fatal(err)
	}
	ledger, err := Add(Ledger{}, entry, DefaultPolicy(), at)
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	managed := filepath.Join(dir, "autotune_vnext_state.json")
	if err := os.WriteFile(managed, []byte("managed intent"), 0600); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, FileName)
	if err := Save(path, ledger); err != nil {
		t.Fatal(err)
	}
	loaded := Load(path)
	if loaded.State != LoadValid || len(loaded.Ledger.Entries) != 1 {
		t.Fatalf("load=%#v", loaded)
	}
	managedData, err := os.ReadFile(managed)
	if err != nil || string(managedData) != "managed intent" {
		t.Fatalf("managed state changed: %q %v", managedData, err)
	}
	data, err := json.Marshal(ledger)
	if err != nil {
		t.Fatal(err)
	}
	for _, forbidden := range []string{"Authorization", "Cookie", "?", "@", "interface", "gateway", "resolver", "local_address", "selected_edge", "192.0.2.1"} {
		if strings.Contains(strings.ToLower(string(data)), strings.ToLower(forbidden)) {
			t.Fatalf("serialized ledger leaks %q: %s", forbidden, data)
		}
	}
	if err := os.WriteFile(path, []byte("{"), 0600); err != nil {
		t.Fatal(err)
	}
	if result := Load(path); result.State != LoadCorrupt || len(result.Ledger.Entries) != 0 {
		t.Fatalf("corrupt=%#v", result)
	}
	if err := os.WriteFile(path, []byte(`{"schema_version":99}`), 0600); err != nil {
		t.Fatal(err)
	}
	if result := Load(path); result.State != LoadUnsupportedVersion {
		t.Fatalf("future=%#v", result)
	}

	tamperedLedger := ledger
	tamperedLedger.Fingerprint = "ledger-v1-tampered"
	tamperedData, err := json.Marshal(tamperedLedger)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, tamperedData, 0600); err != nil {
		t.Fatal(err)
	}
	if result := Load(path); result.State != LoadCorrupt {
		t.Fatalf("tampered ledger=%#v", result)
	}
	ledger.Entries[0].EntryID = "tampered"
	if _, err := Add(Ledger{}, ledger.Entries[0], DefaultPolicy(), at); err == nil {
		t.Fatal("corrupt entry ID accepted")
	}
}

func testInput(t *testing.T, at time.Time, outcome autotunevnext.Outcome) BuildInput {
	t.Helper()
	evidence := testEvidence(t, at)
	diagnosisReport := diagnosis.Report{SchemaVersion: diagnosis.SchemaVersion, DiagnosisID: "diagnosis-v1-test", ProbeIdentity: evidence.ProbeIdentity, Kind: diagnosis.KindTCPPathFailure, Confidence: attribution.ConfidenceHigh, EvidenceFingerprints: []string{evidence.Fingerprint}, EffectiveAt: at}
	return BuildInput{Evidence: []observatory.EvidenceRecord{evidence}, Diagnosis: diagnosisReport, StrategyID: "strategy", StrategyFingerprint: fingerprint("strategy", "a"), Backend: backendcap.Zapret2Windows, BackendFingerprint: fingerprint("backend", "b"), CapabilityFingerprint: fingerprint("capability", "c"), Outcome: outcome, RecordedAt: at, ContextKey: fingerprint("context", "d")}
}

func testEvidence(t *testing.T, at time.Time) observatory.EvidenceRecord {
	t.Helper()
	primary := 0
	port := "443"
	spec := observatory.ProbeSpec{SchemaVersion: observatory.ProbeSpecSchemaVersion, ID: "probe", ServiceID: "service", TargetContractRevision: "v1", Target: observatory.Target{URL: "https://example.test/ok", Hostname: "example.test", Port: port, RequestedProtocol: observatory.TransportTCP}, Transport: observatory.TransportTCP, AddressFamilyPolicy: observatory.AddressFamilyIPv4, Mode: observatory.ProbeModeHTTPSGet, ExpectedResponse: observatory.ExpectedResponse{AllowedStatusCodes: []int{204}, RequirePathComplete: true}, ControlRole: observatory.ControlRolePrimary, Privacy: observatory.PrivacyModeRedacted}
	stages := []observatory.StageEvidence{{Stage: observatory.StageResolve, Status: observatory.StatusPass}, {Stage: observatory.StageConnect, Status: observatory.StatusPass}, {Stage: observatory.StageHello, Status: observatory.StatusPass}, {Stage: observatory.StageHandshake, Status: observatory.StatusPass}, {Stage: observatory.StageHTTP, Status: observatory.StatusPass, HTTPStatus: 204, PathComplete: true}}
	observation := observatory.ObservationResult{SchemaVersion: observatory.SchemaVersion, RunID: "run", StartedAt: at, FinishedAt: at.Add(time.Second), Target: spec.Target, Attempts: []observatory.ConnectionAttempt{{ResolvedIP: "192.0.2.1", AddressFamily: observatory.AddressFamilyIPv4, Transport: observatory.TransportTCP, Stages: stages}}, PrimaryAttemptIndex: &primary, FinalBoundary: observatory.StageHTTP, Classification: observatory.ClassSuccess, ExecutionContext: observatory.ExecutionContext{Mode: "direct"}}
	record, err := observatory.BuildEvidenceRecord(spec, observatory.EvidenceInput{Observations: []observatory.ObservationResult{observation}})
	if err != nil {
		t.Fatal(err)
	}
	return record
}
func queryFor(entry OutcomeEntry, now time.Time) Query {
	return Query{ProbeIdentity: entry.ProbeIdentity, ServiceID: entry.ServiceID, TargetContractRevision: entry.TargetContractRevision, Transport: entry.Transport, AddressFamily: entry.AddressFamily, StrategyFingerprint: entry.StrategyFingerprint, Backend: entry.Backend, BackendFingerprint: entry.BackendFingerprint, CapabilityFingerprint: entry.CapabilityFingerprint, ContextKey: entry.ContextKey, Now: now}
}
func fingerprint(kind, digit string) string { return kind + "-v1-" + strings.Repeat(digit, 64) }
