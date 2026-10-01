package main

// Adversarial tests for the history-identity layer.
//
// FROZEN PRINCIPLE under test, stated once so every assertion below is read
// against it:
//
//	HISTORY != ELIGIBILITY
//	HISTORY != VERIFIED_FIXED
//	HISTORY != APPLY_PERMISSION
//	CURRENT_VALIDATION > HISTORICAL_SUCCESS
//
// Every test here is hermetic: no network, no packet path, no managed
// activation, and nothing written to disk. History cohorts are synthesised in
// memory with outcomeledger.NewEntry and the ledger is passed by value.

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/url"
	"slices"
	"strings"
	"testing"
	"time"

	"unbound/engine/attribution"
	"unbound/engine/autotunevnext"
	"unbound/engine/backendcap"
	"unbound/engine/diagnosis"
	"unbound/engine/historyid"
	"unbound/engine/observatory"
	"unbound/engine/outcomeledger"
	"unbound/engine/planner"
	"unbound/engine/recommendation"
	"unbound/engine/strategyir"
)

// historyIdentityHash produces a valid opaque identity value for one identity
// namespace. Values are opaque to the ledger: only equality ever matters, and
// no test here ever needs to know what the digest was derived from.
func historyIdentityHash(namespace, digit string) string {
	return namespace + "-v1-" + strings.Repeat(digit, 64)
}

// historyIdentityRecordAt is the fixed instant synthetic cohorts are recorded
// at. A fixed time keeps TTL reasoning exact.
var historyIdentityRecordAt = time.Date(2026, 1, 2, 3, 0, 0, 0, time.UTC)

// ---------------------------------------------------------------------------
// Synthetic history cohorts
// ---------------------------------------------------------------------------

// historyIdentityStandaloneProbeSpec is a probe identity that belongs to no
// product path. Cohort-level tests use it because they only exercise the
// ledger, and nothing there should depend on the product's probe.
func historyIdentityStandaloneProbeSpec() observatory.ProbeSpec {
	return observatory.ProbeSpec{
		SchemaVersion:          observatory.ProbeSpecSchemaVersion,
		ID:                     "history-identity-probe",
		ServiceID:              "history-identity-service",
		TargetContractRevision: "v1",
		Target: observatory.Target{
			URL:               "https://history-identity.test/ok",
			Hostname:          "history-identity.test",
			Port:              "443",
			RequestedProtocol: observatory.TransportTCP,
		},
		Transport:           observatory.TransportTCP,
		AddressFamilyPolicy: observatory.AddressFamilyIPv4,
		Mode:                observatory.ProbeModeHTTPSGet,
		ExpectedResponse:    observatory.ExpectedResponse{AllowedStatusCodes: []int{204}, RequirePathComplete: true},
		ControlRole:         observatory.ControlRolePrimary,
		Privacy:             observatory.PrivacyModeRedacted,
	}
}

// historyIdentityEvidenceFor synthesises one primary-target record showing a
// factual TCP connect failure against an arbitrary probe contract. Nothing is
// observed; the record is built from literal values.
func historyIdentityEvidenceFor(t *testing.T, spec observatory.ProbeSpec, at time.Time, runID string) observatory.EvidenceRecord {
	t.Helper()
	primary := 0
	observation := observatory.ObservationResult{
		SchemaVersion:       observatory.SchemaVersion,
		RunID:               runID,
		StartedAt:           at,
		FinishedAt:          at.Add(time.Second),
		Target:              spec.Target,
		Platform:            "test/platform",
		NetworkContext:      observatory.NetworkContext{AddressFamily: observatory.AddressFamilyIPv4, NetworkLabel: "history-identity"},
		PrimaryAttemptIndex: &primary,
		FinalBoundary:       observatory.StageConnect,
		Classification:      observatory.ClassTCPConnectTimeout,
		Attempts: []observatory.ConnectionAttempt{{
			ResolvedIP:    "192.0.2.1",
			AddressFamily: observatory.AddressFamilyIPv4,
			Transport:     observatory.TransportTCP,
			Stages: []observatory.StageEvidence{
				{Stage: observatory.StageResolve, Status: observatory.StatusPass},
				{Stage: observatory.StageConnect, Status: observatory.StatusFail, Class: observatory.ClassTCPConnectTimeout},
			},
		}},
		ExecutionContext: observatory.ExecutionContext{Mode: "direct"},
	}
	record, err := observatory.BuildEvidenceRecord(spec, observatory.EvidenceInput{Observations: []observatory.ObservationResult{observation}})
	if err != nil {
		t.Fatalf("synthetic evidence: %v", err)
	}
	return record
}

func historyIdentityEvidence(t *testing.T, at time.Time) observatory.EvidenceRecord {
	t.Helper()
	return historyIdentityEvidenceFor(t, historyIdentityStandaloneProbeSpec(), at, "history-identity-run")
}

func historyIdentityDiagnosisFor(t *testing.T, evidence observatory.EvidenceRecord) diagnosis.Report {
	t.Helper()
	runID := evidence.Runs[0].Observation.RunID
	parsedTarget, parseErr := url.Parse(evidence.Probe.Target.URL)
	if parseErr != nil {
		t.Fatalf("probe target URL: %v", parseErr)
	}
	finding := attribution.Finding{
		Code:       attribution.FindingTCPPathFailureSuspected,
		Confidence: attribution.ConfidenceHigh,
		Stage:      observatory.StageConnect,
		SupportingEvidence: []attribution.EvidenceRef{
			{RunID: runID, AttemptIndex: 0, Stage: observatory.StageConnect},
		},
	}
	report, err := diagnosis.Diagnose(diagnosis.Input{
		Target: []observatory.EvidenceRecord{evidence},
		Attribution: attribution.AttributionReport{
			SchemaVersion: attribution.SchemaVersion,
			AttributionID: "attribution-v1-history-identity",
			InputRunIDs:   []string{runID},
			Target: attribution.TargetRef{
				Scheme:            "https",
				Hostname:          evidence.Probe.Target.Hostname,
				Port:              evidence.Probe.Target.Port,
				Path:              parsedTarget.Path,
				RequestedProtocol: evidence.Probe.Transport,
			},
			PrimaryFinding: finding,
			Findings:       []attribution.Finding{finding},
		},
	})
	if err != nil {
		t.Fatalf("synthetic diagnosis: %v", err)
	}
	if report.Kind != diagnosis.KindTCPPathFailure {
		t.Fatalf("synthetic diagnosis anchored to the wrong kind: %q", report.Kind)
	}
	return report
}

func historyIdentityDiagnosis(t *testing.T, at time.Time) diagnosis.Report {
	t.Helper()
	return historyIdentityDiagnosisFor(t, historyIdentityEvidence(t, at))
}

// historyIdentityCohort fully describes one recorded historical outcome. Every
// field is an independent knob so a test can move exactly one thing.
type historyIdentityCohort struct {
	outcome               autotunevnext.Outcome
	strategyID            string
	strategyFingerprint   string
	backend               backendcap.Backend
	backendFingerprint    string
	capabilityFingerprint string
	contextKey            string
	withValidation        bool
}

func historyIdentityDefaultCohort() historyIdentityCohort {
	return historyIdentityCohort{
		outcome:               autotunevnext.OutcomeVerifiedFixed,
		strategyID:            "history-identity-strategy",
		strategyFingerprint:   testFingerprint("1"),
		backend:               backendcap.Zapret2Windows,
		backendFingerprint:    historyIdentityHash("backend", "2"),
		capabilityFingerprint: historyIdentityHash("capability", "3"),
		contextKey:            historyIdentityHash("context", "4"),
		withValidation:        true,
	}
}

func historyIdentityBuildInput(cohort historyIdentityCohort, evidence observatory.EvidenceRecord, report diagnosis.Report) outcomeledger.BuildInput {
	validation := []observatory.EvidenceRecord(nil)
	if cohort.withValidation {
		validation = []observatory.EvidenceRecord{evidence}
	}
	return outcomeledger.BuildInput{
		Evidence:              []observatory.EvidenceRecord{evidence},
		ValidationEvidence:    validation,
		Diagnosis:             report,
		StrategyID:            cohort.strategyID,
		StrategyFingerprint:   cohort.strategyFingerprint,
		Backend:               cohort.backend,
		BackendFingerprint:    cohort.backendFingerprint,
		CapabilityFingerprint: cohort.capabilityFingerprint,
		Outcome:               cohort.outcome,
		RecordedAt:            report.EffectiveAt,
		ContextKey:            cohort.contextKey,
	}
}

// historyIdentityEntryFor records a cohort against an arbitrary probe contract,
// so a test can produce history that is genuinely about the product's probe.
func historyIdentityEntryFor(t *testing.T, cohort historyIdentityCohort, spec observatory.ProbeSpec) outcomeledger.OutcomeEntry {
	t.Helper()
	evidence := historyIdentityEvidenceFor(t, spec, historyIdentityRecordAt, "history-identity-recorded-"+cohort.strategyID)
	entry, err := outcomeledger.NewEntry(historyIdentityBuildInput(cohort, evidence, historyIdentityDiagnosisFor(t, evidence)), outcomeledger.DefaultPolicy())
	if err != nil {
		t.Fatalf("synthetic history entry: %v", err)
	}
	return entry
}

func historyIdentityEntry(t *testing.T, cohort historyIdentityCohort) outcomeledger.OutcomeEntry {
	t.Helper()
	return historyIdentityEntryFor(t, cohort, historyIdentityStandaloneProbeSpec())
}

func historyIdentityEntryExpectingError(t *testing.T, cohort historyIdentityCohort) {
	t.Helper()
	evidence := historyIdentityEvidence(t, historyIdentityRecordAt)
	if _, err := outcomeledger.NewEntry(historyIdentityBuildInput(cohort, evidence, historyIdentityDiagnosisFor(t, evidence)), outcomeledger.DefaultPolicy()); err == nil {
		t.Fatal("history accepted a cohort the ledger must refuse")
	}
}

// historyIdentityQuery builds the query a LATER run makes with its own cohort.
// It is always derived from the recorded entry, so a test moves identity by
// editing the returned query rather than by touching unrelated fields.
func historyIdentityQuery(entry outcomeledger.OutcomeEntry, now time.Time) outcomeledger.Query {
	return outcomeledger.Query{
		ProbeIdentity:          entry.ProbeIdentity,
		ServiceID:              entry.ServiceID,
		TargetContractRevision: entry.TargetContractRevision,
		Transport:              entry.Transport,
		AddressFamily:          entry.AddressFamily,
		DiagnosisKind:          entry.DiagnosisKind,
		StrategyFingerprint:    entry.StrategyFingerprint,
		Backend:                entry.Backend,
		BackendFingerprint:     entry.BackendFingerprint,
		CapabilityFingerprint:  entry.CapabilityFingerprint,
		ContextKey:             entry.ContextKey,
		Now:                    now,
	}
}

func historyIdentityLedger(t *testing.T, entries ...outcomeledger.OutcomeEntry) outcomeledger.Ledger {
	t.Helper()
	ledger := outcomeledger.Ledger{SchemaVersion: outcomeledger.SchemaVersion}
	var err error
	for _, entry := range entries {
		ledger, err = outcomeledger.Add(ledger, entry, outcomeledger.DefaultPolicy(), entry.RecordedAt)
		if err != nil {
			t.Fatalf("synthetic ledger: %v", err)
		}
	}
	return ledger
}

// historyIdentityExpect asserts one match outcome. It is deliberately strict:
// a test that wants "not compatible" must say which of the two refusal
// statuses it means, so STALE and INCOMPATIBLE can never be confused.
func historyIdentityExpect(t *testing.T, label string, match outcomeledger.Match, status outcomeledger.MatchStatus, reason outcomeledger.Reason) {
	t.Helper()
	if match.Status != status || len(match.Reasons) != 1 || match.Reasons[0] != reason {
		t.Fatalf("INVARIANT BROKEN [%s]: expected %s/%s, got %s/%v (confidence %q). A different historical positive must not be reusable as if it were current evidence.",
			label, status, reason, match.Status, match.Reasons, match.Confidence)
	}
}

func withBackendFingerprint(query outcomeledger.Query, value string) outcomeledger.Query {
	query.BackendFingerprint = value
	return query
}

func withCapabilityFingerprint(query outcomeledger.Query, value string) outcomeledger.Query {
	query.CapabilityFingerprint = value
	return query
}

func withContextKey(query outcomeledger.Query, value string) outcomeledger.Query {
	query.ContextKey = value
	return query
}

func withDiagnosisKind(query outcomeledger.Query, kind diagnosis.Kind) outcomeledger.Query {
	query.DiagnosisKind = kind
	return query
}

func withStrategyFingerprint(query outcomeledger.Query, value string) outcomeledger.Query {
	query.StrategyFingerprint = value
	return query
}

func withAddressFamily(query outcomeledger.Query, family observatory.AddressFamily) outcomeledger.Query {
	query.AddressFamily = family
	return query
}

func withProbeIdentity(query outcomeledger.Query, value string) outcomeledger.Query {
	query.ProbeIdentity = value
	return query
}

func withContractRevision(query outcomeledger.Query, value string) outcomeledger.Query {
	query.TargetContractRevision = value
	return query
}

// ---------------------------------------------------------------------------
// 1-3. Identity change must refuse reuse
// ---------------------------------------------------------------------------

// TestHistoryIdentityMismatchRefusesHistoricalPositive is the core adversarial
// cohort. A recorded VERIFIED_FIXED is compared against a query whose identity
// cohort differs in exactly ONE dimension. Every row must refuse.
//
// INVARIANT: a historical positive is reusable only inside the exact identity
// cohort that produced it. Identity is the namespace of reuse, not a hint.
func TestHistoryIdentityMismatchRefusesHistoricalPositive(t *testing.T) {
	base := historyIdentityDefaultCohort()
	recorded := historyIdentityEntry(t, base)
	compatible := historyIdentityQuery(recorded, recorded.RecordedAt.Add(time.Hour))

	cases := []struct {
		name   string
		query  outcomeledger.Query
		status outcomeledger.MatchStatus
		reason outcomeledger.Reason
	}{
		{
			name:   "backend fingerprint changed on the later run",
			query:  withBackendFingerprint(compatible, historyIdentityHash("backend", "5")),
			status: outcomeledger.MatchIncompatible,
			reason: outcomeledger.ReasonBackendChanged,
		},
		{
			name:   "capability fingerprint changed on the later run",
			query:  withCapabilityFingerprint(compatible, historyIdentityHash("capability", "6")),
			status: outcomeledger.MatchIncompatible,
			reason: outcomeledger.ReasonCapabilityChanged,
		},
		{
			name:   "context key changed on the later run",
			query:  withContextKey(compatible, historyIdentityHash("context", "7")),
			status: outcomeledger.MatchIncompatible,
			reason: outcomeledger.ReasonContextChanged,
		},
		{
			name:   "backend enum changed while the fingerprint did not",
			query:  withBackend(compatible, backendcap.NativeWindows),
			status: outcomeledger.MatchIncompatible,
			reason: outcomeledger.ReasonBackendChanged,
		},
		{
			name:  "all three identities identical",
			query: compatible,
			// Control: identical identity is the ONLY way a historical positive
			// may ever be compatible. If this row ever refuses, the layer is
			// merely dead and the end-to-end tests below would be vacuous.
			status: outcomeledger.MatchCompatible,
		},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			match := outcomeledger.MatchEntry(recorded, testCase.query)
			if testCase.status == outcomeledger.MatchCompatible {
				if match.Status != outcomeledger.MatchCompatible {
					t.Fatalf("INVARIANT BROKEN [%s]: identical identity refused (%s/%v). History reuse is meant to work inside one identity cohort so the refusal rows stay meaningful.", testCase.name, match.Status, match.Reasons)
				}
				return
			}
			historyIdentityExpect(t, testCase.name, match, testCase.status, testCase.reason)
		})
	}

	// The mirror image: the later run keeps its own cohort and the RECORDED
	// entry is the odd one out. Identity mismatch must refuse from both sides.
	mirrorCases := []struct {
		name   string
		mutate func(*historyIdentityCohort)
		reason outcomeledger.Reason
	}{
		{name: "recorded backend fingerprint differs", mutate: func(c *historyIdentityCohort) { c.backendFingerprint = historyIdentityHash("backend", "8") }, reason: outcomeledger.ReasonBackendChanged},
		{name: "recorded capability fingerprint differs", mutate: func(c *historyIdentityCohort) { c.capabilityFingerprint = historyIdentityHash("capability", "9") }, reason: outcomeledger.ReasonCapabilityChanged},
		{name: "recorded context key differs", mutate: func(c *historyIdentityCohort) { c.contextKey = historyIdentityHash("context", "a") }, reason: outcomeledger.ReasonContextChanged},
	}
	for _, testCase := range mirrorCases {
		t.Run("recorded side: "+testCase.name, func(t *testing.T) {
			recordedCohort := historyIdentityDefaultCohort()
			testCase.mutate(&recordedCohort)
			entry := historyIdentityEntry(t, recordedCohort)
			historyIdentityExpect(t, testCase.name, outcomeledger.MatchEntry(entry, compatible), outcomeledger.MatchIncompatible, testCase.reason)
		})
	}

	t.Run("QueryLedger agrees with MatchEntry on every mismatch", func(t *testing.T) {
		// The product path goes through QueryLedger, not MatchEntry. A refusal
		// that only existed on the single-entry helper would be theatre.
		ledger := historyIdentityLedger(t, recorded)

		matches := outcomeledger.QueryLedger(ledger, withContextKey(compatible, historyIdentityHash("context", "b")))
		if len(matches) != 1 {
			t.Fatalf("expected exactly one match, got %d", len(matches))
		}
		historyIdentityExpect(t, "QueryLedger context changed", matches[0], outcomeledger.MatchIncompatible, outcomeledger.ReasonContextChanged)

		unchanged := outcomeledger.QueryLedger(ledger, compatible)
		if len(unchanged) != 1 || unchanged[0].Status != outcomeledger.MatchCompatible {
			t.Fatalf("QueryLedger refused an identical identity cohort: %#v", unchanged)
		}
	})
}

func withBackend(query outcomeledger.Query, value backendcap.Backend) outcomeledger.Query {
	query.Backend = value
	return query
}

// ---------------------------------------------------------------------------
// 4. Identities unchanged: every other gate must still refuse
// ---------------------------------------------------------------------------

// TestHistoryIdentityUnchangedStillRefusesRemainingGates proves that identity
// is the ONLY thing the mismatch rows moved. With all three identities held
// identical the historical positive is compatible — and every other
// pre-existing gate still refuses on its own.
//
// INVARIANT: identity compatibility is necessary but nowhere near sufficient.
func TestHistoryIdentityUnchangedStillRefusesRemainingGates(t *testing.T) {
	entry := historyIdentityEntry(t, historyIdentityDefaultCohort())
	compatibleQuery := historyIdentityQuery(entry, entry.RecordedAt.Add(time.Hour))

	t.Run("identical identity alone is compatible", func(t *testing.T) {
		if match := outcomeledger.MatchEntry(entry, compatibleQuery); match.Status != outcomeledger.MatchCompatible {
			t.Fatalf("INVARIANT BROKEN: identical identity refused (%s/%v)", match.Status, match.Reasons)
		}
	})

	cases := []struct {
		name   string
		query  outcomeledger.Query
		status outcomeledger.MatchStatus
		reason outcomeledger.Reason
	}{
		{
			name:   "TTL expired",
			query:  historyIdentityQuery(entry, entry.ExpiresAt),
			status: outcomeledger.MatchExpired,
			reason: outcomeledger.ReasonExpired,
		},
		{
			name:   "current diagnosis kind differs",
			query:  withDiagnosisKind(compatibleQuery, diagnosis.KindDNSPathAnomaly),
			status: outcomeledger.MatchIncompatible,
			reason: outcomeledger.ReasonDiagnosisChanged,
		},
		{
			name:   "current strategy fingerprint differs",
			query:  withStrategyFingerprint(compatibleQuery, testFingerprint("c")),
			status: outcomeledger.MatchIncompatible,
			reason: outcomeledger.ReasonStrategyChanged,
		},
		{
			name:   "address family differs",
			query:  withAddressFamily(compatibleQuery, observatory.AddressFamilyIPv6),
			status: outcomeledger.MatchIncompatible,
			reason: outcomeledger.ReasonFamilyChanged,
		},
		{
			name:   "probe identity differs",
			query:  withProbeIdentity(compatibleQuery, "history-identity-test-other"),
			status: outcomeledger.MatchIncompatible,
			reason: outcomeledger.ReasonProbeChanged,
		},
		{
			name:   "target contract revision differs",
			query:  withContractRevision(compatibleQuery, "v2"),
			status: outcomeledger.MatchIncompatible,
			reason: outcomeledger.ReasonContractChanged,
		},
	}

	for _, testCase := range cases {
		t.Run("gate still refuses: "+testCase.name, func(t *testing.T) {
			historyIdentityExpect(t, testCase.name, outcomeledger.MatchEntry(entry, testCase.query), testCase.status, testCase.reason)
		})
	}

	t.Run("gate still refuses: validation evidence was never recorded", func(t *testing.T) {
		// The write boundary refuses a VERIFIED_FIXED without validation
		// evidence at all, so a positive lacking it cannot enter history.
		unvalidated := historyIdentityDefaultCohort()
		unvalidated.withValidation = false
		historyIdentityEntryExpectingError(t, unvalidated)

		// Defence in depth: even if such an entry reached the matcher (an old
		// file, a future writer), reuse is stale, never compatible.
		stripped := historyIdentityWithoutValidationEvidence(t, entry)
		historyIdentityExpect(t, "missing validation evidence", outcomeledger.MatchEntry(stripped, compatibleQuery),
			outcomeledger.MatchStale, outcomeledger.ReasonValidationEvidenceRequired)
	})
}

// historyIdentityCanonicalEntryID reproduces the ledger's own event identity
// over an exported struct so a test can construct an entry shape NewEntry
// deliberately refuses to build. It is a canary: it must agree with the ID
// NewEntry produced, or the canonical derivation has moved.
func historyIdentityCanonicalEntryID(t *testing.T, entry outcomeledger.OutcomeEntry) string {
	t.Helper()
	entry.EntryID = ""
	entry.InvalidatedAt = nil
	entry.InvalidationReason = ""
	data, err := json.Marshal(entry)
	if err != nil {
		t.Fatalf("canonical entry encoding: %v", err)
	}
	sum := sha256.Sum256(data)
	return "outcome-v1-" + hex.EncodeToString(sum[:])
}

func historyIdentityWithoutValidationEvidence(t *testing.T, entry outcomeledger.OutcomeEntry) outcomeledger.OutcomeEntry {
	t.Helper()
	if got := historyIdentityCanonicalEntryID(t, entry); got != entry.EntryID {
		t.Fatalf("canonical entry ID derivation drifted: ledger produced %q, test reproduced %q", entry.EntryID, got)
	}
	stripped := entry
	stripped.ValidationEvidenceFingerprints = nil
	stripped.EntryID = historyIdentityCanonicalEntryID(t, stripped)
	if err := stripped.Validate(); err != nil {
		t.Fatalf("stripped entry is not a structurally valid ledger entry: %v", err)
	}
	return stripped
}

// ---------------------------------------------------------------------------
// 6. Empty identity is not trust
// ---------------------------------------------------------------------------

// TestEmptyHistoryIdentityIsNotTrust covers the degenerate cohorts: an identity
// that was never produced.
//
// INVARIANT: an absent identity is a refusal, never a wildcard. The dangerous
// comparison is empty == empty, which satisfies every naive equality check.
func TestEmptyHistoryIdentityIsNotTrust(t *testing.T) {
	base := historyIdentityDefaultCohort()

	t.Run("empty context key recorded and queried", func(t *testing.T) {
		cohort := base
		cohort.contextKey = ""
		entry := historyIdentityEntry(t, cohort)
		query := historyIdentityQuery(entry, entry.RecordedAt.Add(time.Hour))
		if query.ContextKey != "" {
			t.Fatal("expected an empty query context key")
		}
		// empty == empty satisfies the mismatch comparison, so this row
		// specifically proves the positive-reuse rule catches it anyway.
		historyIdentityExpect(t, "empty context key", outcomeledger.MatchEntry(entry, query), outcomeledger.MatchStale, outcomeledger.ReasonContextMissing)
	})

	t.Run("empty context key recorded, real one queried", func(t *testing.T) {
		cohort := base
		cohort.contextKey = ""
		entry := historyIdentityEntry(t, cohort)
		query := withContextKey(historyIdentityQuery(entry, entry.RecordedAt.Add(time.Hour)), base.contextKey)
		historyIdentityExpect(t, "empty recorded context key", outcomeledger.MatchEntry(entry, query), outcomeledger.MatchIncompatible, outcomeledger.ReasonContextChanged)
	})

	t.Run("empty capability fingerprint recorded and queried", func(t *testing.T) {
		cohort := base
		cohort.capabilityFingerprint = ""
		entry := historyIdentityEntry(t, cohort)
		query := historyIdentityQuery(entry, entry.RecordedAt.Add(time.Hour))
		if query.CapabilityFingerprint != "" {
			t.Fatal("expected an empty query capability fingerprint")
		}
		// The capability mismatch comparison short-circuits on an empty
		// ENTRY value, so this row proves the later rule is what refuses.
		historyIdentityExpect(t, "empty capability fingerprint", outcomeledger.MatchEntry(entry, query), outcomeledger.MatchStale, outcomeledger.ReasonCapabilityMissing)
	})

	t.Run("empty capability fingerprint recorded, real one queried", func(t *testing.T) {
		cohort := base
		cohort.capabilityFingerprint = ""
		entry := historyIdentityEntry(t, cohort)
		query := withCapabilityFingerprint(historyIdentityQuery(entry, entry.RecordedAt.Add(time.Hour)), base.capabilityFingerprint)
		historyIdentityExpect(t, "empty recorded capability fingerprint", outcomeledger.MatchEntry(entry, query), outcomeledger.MatchStale, outcomeledger.ReasonCapabilityMissing)
	})

	t.Run("both identities empty", func(t *testing.T) {
		cohort := base
		cohort.contextKey = ""
		cohort.capabilityFingerprint = ""
		entry := historyIdentityEntry(t, cohort)
		query := historyIdentityQuery(entry, entry.RecordedAt.Add(time.Hour))

		// The context rule is evaluated first and must be the one that fires.
		historyIdentityExpect(t, "both identities empty", outcomeledger.MatchEntry(entry, query), outcomeledger.MatchStale, outcomeledger.ReasonContextMissing)

		// Neither empty identity may hide behind the other: give the entry a
		// real context key and the capability rule must then fire on its own.
		onlyCapabilityEmpty := base
		onlyCapabilityEmpty.capabilityFingerprint = ""
		capabilityEntry := historyIdentityEntry(t, onlyCapabilityEmpty)
		historyIdentityExpect(t, "empty capability behind a real context", outcomeledger.MatchEntry(capabilityEntry, historyIdentityQuery(capabilityEntry, capabilityEntry.RecordedAt.Add(time.Hour))),
			outcomeledger.MatchStale, outcomeledger.ReasonCapabilityMissing)
	})

	t.Run("an identity-less positive is never a usable positive", func(t *testing.T) {
		// Belt and braces over the whole chain. Even if some future change made
		// the matcher return COMPATIBLE here, the ranking must still never treat
		// the entry as evidence.
		cohort := base
		cohort.contextKey = ""
		cohort.capabilityFingerprint = ""
		entry := historyIdentityEntry(t, cohort)
		match := outcomeledger.MatchEntry(entry, historyIdentityQuery(entry, entry.RecordedAt.Add(time.Hour)))
		report := recommendation.Recommend(recommendation.Input{
			Diagnosis:  historyIdentityDiagnosis(t, historyIdentityRecordAt),
			Planner:    historyIdentityPlannerReport(base.strategyFingerprint, base.strategyID),
			Candidates: []recommendation.Candidate{{StrategyID: base.strategyID, StrategyFingerprint: base.strategyFingerprint}},
			Matches:    map[string][]outcomeledger.Match{base.strategyFingerprint: {match}},
		})
		if report.Disposition != recommendation.DispositionExperimentCandidates || len(report.CandidateRecommendations) != 1 {
			t.Fatalf("control broken: the eligible candidate vanished: %#v", report)
		}
		if report.CandidateRecommendations[0].HistoryUsed {
			t.Fatalf("INVARIANT BROKEN: an identity-less historical positive drove the ranking (match status %s): %#v", match.Status, report.CandidateRecommendations[0])
		}
		for _, code := range report.CandidateRecommendations[0].ReasonCodes {
			if code != "NO_COMPATIBLE_EFFECTIVENESS_HISTORY" {
				t.Fatalf("INVARIANT BROKEN: an identity-less match produced reason %q: %#v", code, report.CandidateRecommendations[0])
			}
		}
	})

	t.Run("an identity-less direct-reachability record cannot rank anything", func(t *testing.T) {
		// DIRECT_BECAME_REACHABLE is the one persisted outcome that is neither
		// an effectiveness positive nor gated by the positive-reuse identity
		// rules, because it asserts something about the DIRECT path rather
		// than about a strategy. Record the real behaviour and pin the reason
		// it is harmless: it is inert for ordering by construction.
		cohort := base
		cohort.outcome = autotunevnext.OutcomeDirectBecameReachable
		cohort.contextKey = ""
		cohort.capabilityFingerprint = ""
		entry := historyIdentityEntry(t, cohort)
		match := outcomeledger.MatchEntry(entry, historyIdentityQuery(entry, entry.RecordedAt.Add(time.Hour)))
		t.Logf("direct-reachability entry with both identities absent matches as %s/%v", match.Status, match.Reasons)

		report := recommendation.Recommend(recommendation.Input{
			Diagnosis:  historyIdentityDiagnosis(t, historyIdentityRecordAt),
			Planner:    historyIdentityPlannerReport(base.strategyFingerprint, base.strategyID),
			Candidates: []recommendation.Candidate{{StrategyID: base.strategyID, StrategyFingerprint: base.strategyFingerprint}},
			Matches:    map[string][]outcomeledger.Match{base.strategyFingerprint: {match}},
		})
		if len(report.CandidateRecommendations) != 1 {
			t.Fatalf("control broken: the eligible candidate vanished: %#v", report)
		}
		if report.CandidateRecommendations[0].HistoryUsed {
			t.Fatalf("INVARIANT BROKEN: an identity-less record drove the ranking (match status %s): %#v", match.Status, report.CandidateRecommendations[0])
		}
		for _, code := range report.CandidateRecommendations[0].ReasonCodes {
			if code != "NO_COMPATIBLE_EFFECTIVENESS_HISTORY" {
				t.Fatalf("INVARIANT BROKEN: an identity-less match produced reason %q: %#v", code, report.CandidateRecommendations[0])
			}
		}
	})
}

// ---------------------------------------------------------------------------
// Hermetic run harness
// ---------------------------------------------------------------------------

// historyIdentityObservation builds one literal observation. Platform and
// network context are set so the engine's direct/profile comparison is
// applicable; otherwise every run would be INCONCLUSIVE for fixture reasons.
func historyIdentityObservation(id string, success bool) observatory.ObservationResult {
	observation := managedVNextObservation(success)
	observation.RunID = id
	observation.Platform = "test/platform"
	observation.NetworkContext = observatory.NetworkContext{
		Interface: "history-identity-test", LocalAddress: "192.0.2.2",
		AddressFamily: observatory.AddressFamilyIPv4, NetworkLabel: "history-identity-test",
	}
	return observation
}

// historyIdentityRecorder captures exactly what the advisor returned during a
// run, so a test can prove history really was consulted.
type historyIdentityRecorder struct {
	inner  autotunevnext.CandidateAdvisor
	advice autotunevnext.Advice
	calls  int
}

func (r *historyIdentityRecorder) Advise(context autotunevnext.AdvisorContext) autotunevnext.Advice {
	r.calls++
	r.advice = r.inner.Advise(context)
	return r.advice
}

func historyIdentityProductProbe(t *testing.T) (autotunevnext.Target, observatory.ProbeSpec, []observatory.ProbeSpec, []strategyir.Strategy) {
	t.Helper()
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
	return target, probe, controls, catalog
}

// historyIdentityRun drives one hermetic in-memory run. Observations are
// supplied literally; nothing is captured, executed, or persisted.
func historyIdentityRun(t *testing.T, advisor autotunevnext.CandidateAdvisor, observations ...bool) autotunevnext.Result {
	t.Helper()
	target, probe, controls, catalog := historyIdentityProductProbe(t)
	results := make([]observatory.ObservationResult, 0, len(observations))
	for index, success := range observations {
		results = append(results, historyIdentityObservation("history-identity-"+string(rune('a'+index)), success))
	}
	result := autotunevnext.Run(context.Background(), autotunevnext.Request{
		Target: target, Strategies: catalog, Backend: backendcap.Zapret2Windows,
		TargetProbe: &probe, ControlProbes: controls, Advisor: advisor,
		Policy: autotunevnext.Policy{MaxCandidates: 1, MaxDuration: time.Minute, PerObservationTimeout: time.Second},
	}, &productHistorySequenceObserver{results: results}, &productVNextNoopExecutor{}, productVNextSupportedPreflight{}, productVNextAssets{})
	return result
}

// historyIdentityPositiveLedger records the previous run's VERIFIED_FIXED into
// an in-memory ledger using exactly the cohort the later query will use. This
// is the strongest possible historical positive: same strategy, same target,
// same diagnosis kind, same identity.
func historyIdentityPositiveLedger(t *testing.T, previous autotunevnext.Result, identities historyid.Bundle) (outcomeledger.Ledger, outcomeledger.OutcomeEntry) {
	t.Helper()
	if len(previous.OutcomeEvidence) == 0 || len(previous.Experiments) == 0 {
		t.Fatalf("previous run produced no effectiveness evidence: %#v", previous)
	}
	provenance := previous.OutcomeEvidence[0]
	var executed autotunevnext.CandidateExperiment
	found := false
	for _, candidate := range previous.Experiments {
		if candidate.Fingerprint == provenance.StrategyFingerprint {
			executed, found = candidate, true
		}
	}
	if !found {
		t.Fatalf("provenance has no matching experiment: %#v", previous)
	}
	entry, err := outcomeledger.NewEntry(outcomeledger.BuildInput{
		Evidence: previous.BaselineEvidence, ValidationEvidence: provenance.Validation,
		Diagnosis: previous.DiagnosisReport, StrategyID: executed.StrategyID,
		StrategyFingerprint: executed.Fingerprint, Backend: previous.Backend,
		Outcome: provenance.Outcome, RecordedAt: previous.DiagnosisReport.EffectiveAt,
		BackendFingerprint:    string(identities.BackendFingerprint),
		CapabilityFingerprint: string(identities.CapabilityFingerprint),
		ContextKey:            string(identities.ContextKey),
	}, outcomeledger.DefaultPolicy())
	if err != nil {
		t.Fatalf("recording the previous positive: %v", err)
	}
	return historyIdentityLedger(t, entry), entry
}

func historyIdentityRunBundle() historyid.Bundle {
	identities := historyid.Bundle{
		ContextKey:            historyid.NetworkContextIdentity(historyIdentityHash("context", "d")),
		BackendFingerprint:    historyid.BackendFingerprint(historyIdentityHash("backend", "e")),
		CapabilityFingerprint: historyid.CapabilityFingerprint(historyIdentityHash("capability", "f")),
	}
	if len(identities.Limitations()) != 0 {
		panic("test identity cohort must be fully available: " + strings.Join(identities.Limitations(), ","))
	}
	return identities
}

// historyIdentityBundleFor reconstructs the run identity cohort that produced
// a recorded entry, so a test can hand the advisor exactly the cohort its
// history was recorded under.
func historyIdentityBundleFor(t *testing.T, entry outcomeledger.OutcomeEntry) historyid.Bundle {
	t.Helper()
	identities := historyid.Bundle{
		ContextKey:            historyid.NetworkContextIdentity(entry.ContextKey),
		BackendFingerprint:    historyid.BackendFingerprint(entry.BackendFingerprint),
		CapabilityFingerprint: historyid.CapabilityFingerprint(entry.CapabilityFingerprint),
	}
	if limitations := identities.Limitations(); len(limitations) != 0 {
		t.Fatalf("cohort under test is not fully available: %v", limitations)
	}
	return identities
}

// ---------------------------------------------------------------------------
// 5. Current evidence still wins
// ---------------------------------------------------------------------------

// TestHistoryIdentityCompatiblePositiveCannotOverrideCurrentEvidence is the
// end-to-end adversarial test: history says VERIFIED_FIXED and matches the
// current identity cohort exactly, and the CURRENT run still disagrees.
//
// INVARIANT: CURRENT_VALIDATION > HISTORICAL_SUCCESS. A compatible historical
// positive may reorder already-eligible candidates. It may not select, may not
// grant Apply, and may not turn an inconclusive current result into a
// selection.
func TestHistoryIdentityCompatiblePositiveCannotOverrideCurrentEvidence(t *testing.T) {
	identities := historyIdentityRunBundle()

	// Baseline fails, active succeeds, direct after fails again: the engine's
	// own fact that the profile FIXED it.
	previous := historyIdentityRun(t, nil, false, false, true, false)
	if len(previous.OutcomeEvidence) == 0 || previous.OutcomeEvidence[0].Outcome != autotunevnext.OutcomeVerifiedFixed {
		t.Fatalf("control broken: no VERIFIED_FIXED to record: %#v", previous.OutcomeEvidence)
	}
	ledger, entry := historyIdentityPositiveLedger(t, previous, identities)

	_, probe, _, _ := historyIdentityProductProbe(t)
	probeIdentity, err := probe.Identity()
	if err != nil {
		t.Fatal(err)
	}
	if entry.ProbeIdentity != probeIdentity {
		t.Fatalf("control broken: the recorded entry does not describe the product probe (%q vs %q)", entry.ProbeIdentity, probeIdentity)
	}

	newAdvisor := func() *historyIdentityRecorder {
		_, productProbe, _, _ := historyIdentityProductProbe(t)
		return &historyIdentityRecorder{inner: productHistoryAdvisor{
			ledger: ledger, probe: productProbe, backend: backendcap.Zapret2Windows, identities: identities,
			now: func() time.Time { return entry.RecordedAt.Add(time.Minute) },
		}}
	}
	target, _, err := normalizeVNextTarget("https://target.test/")
	if err != nil {
		t.Fatal(err)
	}

	t.Run("control: the recorded positive is genuinely compatible", func(t *testing.T) {
		// If this fails, every assertion below would pass for the wrong
		// reason: history would simply not have matched at all.
		query := outcomeledger.Query{
			ProbeIdentity: probeIdentity, ServiceID: entry.ServiceID,
			TargetContractRevision: entry.TargetContractRevision, Transport: entry.Transport,
			AddressFamily: entry.AddressFamily, DiagnosisKind: entry.DiagnosisKind,
			StrategyFingerprint: entry.StrategyFingerprint, Backend: entry.Backend,
			BackendFingerprint:    string(identities.BackendFingerprint),
			CapabilityFingerprint: string(identities.CapabilityFingerprint),
			ContextKey:            string(identities.ContextKey),
			Now:                   entry.RecordedAt.Add(time.Minute),
		}
		if match := outcomeledger.MatchEntry(entry, query); match.Status != outcomeledger.MatchCompatible {
			t.Fatalf("control broken: the historical positive is not compatible (%s/%v), so the rows below prove nothing", match.Status, match.Reasons)
		}
	})

	t.Run("current failing run is not rescued by history", func(t *testing.T) {
		recorder := newAdvisor()
		// Same target, same probe, same strategy, same identity — and the
		// current active measurement fails. History said this worked before.
		current := historyIdentityRun(t, recorder, false, false, false, false)

		if recorder.calls != 1 || len(recorder.advice.Candidates) == 0 {
			t.Fatalf("control broken: history was never consulted (calls=%d advice=%#v)", recorder.calls, recorder.advice)
		}
		// The advisor really did see the compatible positive. Whether it was
		// then trusted for ordering or deliberately held back at low
		// confidence, it was neither ignored nor treated as proof.
		sawCompatibleHistory := false
		for _, candidate := range recorder.advice.Candidates {
			for _, code := range candidate.ReasonCodes {
				if code == "COMPATIBLE_RECENT_VERIFIED" || code == "COMPATIBLE_LOW_CONFIDENCE_VERIFIED" {
					sawCompatibleHistory = true
				}
			}
		}
		if !sawCompatibleHistory {
			t.Fatalf("control broken: the compatible historical positive never reached the advisor: %#v", recorder.advice)
		}

		if len(current.Experiments) == 0 || current.Experiments[0].Outcome == autotunevnext.OutcomeVerifiedFixed {
			t.Fatalf("fixture broken: the current run did not disagree with history: %#v", current.Experiments)
		}
		if current.SelectedStrategyID != "" || current.SelectedFingerprint != "" {
			t.Fatalf("INVARIANT BROKEN [HISTORY != VERIFIED_FIXED]: a failing current run was turned into a selection by history alone: %#v", current)
		}

		mapped := mapAutoTuneVNextResult(current, "https://target.test/")
		if mapped.ApplyAvailable || mapped.ApplyToken != "" {
			t.Fatalf("INVARIANT BROKEN [HISTORY != APPLY_PERMISSION]: history produced an apply grant: %#v", mapped)
		}
		if mapped.SelectedStrategyID != "" {
			t.Fatalf("INVARIANT BROKEN [HISTORY != APPLY_PERMISSION]: the product result carries a selection history invented: %#v", mapped)
		}

		service := &productVNextService{grants: map[string]verifiedSelectionGrant{}}
		if token := service.issueVerifiedGrant(current, target, "https://target.test/", nil); token != "" {
			t.Fatalf("INVARIANT BROKEN [HISTORY != APPLY_PERMISSION]: an unverified current result received grant %q", token)
		}
	})

	t.Run("control: a genuinely verified current run still selects and grants", func(t *testing.T) {
		// Non-vacuity. Apply permission is not dead code; it is gated on
		// CURRENT evidence, which is the whole point.
		recorder := newAdvisor()
		current := historyIdentityRun(t, recorder, false, false, true, false)
		if current.SelectedStrategyID == "" {
			t.Fatalf("control broken: a current VERIFIED_FIXED run did not select: %#v", current)
		}
		mapped := mapAutoTuneVNextResult(current, "https://target.test/")
		if mapped.SelectedStrategyID != current.SelectedStrategyID {
			t.Fatalf("control broken: the mapped product result lost the current selection: %#v", mapped)
		}
		service := &productVNextService{grants: map[string]verifiedSelectionGrant{}}
		if token := service.issueVerifiedGrant(current, target, "https://target.test/", nil); token == "" {
			t.Fatalf("control broken: a current VERIFIED_FIXED run received no grant: %#v", current)
		}
	})
}

// ---------------------------------------------------------------------------
// 7. History cannot grant eligibility
// ---------------------------------------------------------------------------

// historyIdentityPlannerReport marks exactly one fingerprint eligible.
func historyIdentityPlannerReport(eligibleFingerprint, eligibleID string) planner.PlannerReport {
	return planner.PlannerReport{
		AttributionID: "attribution-v1-history-identity",
		Disposition:   planner.DispositionCandidatesAvailable,
		Candidates: []planner.CandidateAssessment{{
			StrategyID: eligibleID, StrategyFingerprint: eligibleFingerprint, Status: planner.StatusEligible,
		}},
	}
}

// TestHistoryCannotGrantEligibilityOrReorderingAuthority asserts the structural
// authority split on real product types.
//
// INVARIANT: HISTORY != ELIGIBILITY. A compatible historical positive affects
// the ORDER of candidates Planner already deemed eligible and nothing else.
func TestHistoryCannotGrantEligibilityOrReorderingAuthority(t *testing.T) {
	base := historyIdentityDefaultCohort()
	eligibleID := "history-identity-eligible"
	blockedID := "history-identity-blocked"
	eligibleFingerprint := testFingerprint("1")
	blockedFingerprint := testFingerprint("2")

	_, probe, _, _ := historyIdentityProductProbe(t)

	// Two perfectly compatible historical positives, recorded against the very
	// probe the advisor will query: one for a candidate Planner deemed
	// eligible, one for a candidate it rejected outright.
	eligibleCohort := base
	eligibleCohort.strategyID = eligibleID
	eligibleCohort.strategyFingerprint = eligibleFingerprint
	blockedCohort := base
	blockedCohort.strategyID = blockedID
	blockedCohort.strategyFingerprint = blockedFingerprint
	eligibleEntry := historyIdentityEntryFor(t, eligibleCohort, probe)
	blockedEntry := historyIdentityEntryFor(t, blockedCohort, probe)

	ledger := historyIdentityLedger(t, eligibleEntry, blockedEntry)
	cohortIdentities := historyIdentityBundleFor(t, eligibleEntry)

	plannerReport := planner.PlannerReport{
		AttributionID: "attribution-v1-history-identity",
		Disposition:   planner.DispositionCandidatesAvailable,
		Candidates: []planner.CandidateAssessment{
			{StrategyID: eligibleID, StrategyFingerprint: eligibleFingerprint, Status: planner.StatusEligible},
			{StrategyID: blockedID, StrategyFingerprint: blockedFingerprint, Status: planner.StatusStructurallyInapplicable},
		},
	}
	advisorContext := autotunevnext.AdvisorContext{
		Diagnosis: historyIdentityDiagnosis(t, historyIdentityRecordAt),
		Planner:   plannerReport,
		Family:    observatory.AddressFamilyIPv4,
		// The advisor's own input is already restricted to what Planner
		// accepted. Even when it is not, the second row below proves the
		// restriction is enforced again downstream.
		Eligible: []autotunevnext.CandidateIdentity{{StrategyID: eligibleID, StrategyFingerprint: eligibleFingerprint}},
	}
	advisor := productHistoryAdvisor{
		ledger: ledger, probe: probe, backend: backendcap.Zapret2Windows, identities: cohortIdentities,
		now: func() time.Time { return blockedEntry.RecordedAt.Add(time.Hour) },
	}

	t.Run("matches really are compatible for both candidates", func(t *testing.T) {
		// Non-vacuity: the blocked candidate's history is not refused for
		// identity reasons, so its absence below is a PLANNER decision, not an
		// accident of the fixture.
		for _, entry := range []outcomeledger.OutcomeEntry{eligibleEntry, blockedEntry} {
			query := outcomeledger.Query{
				ProbeIdentity: entry.ProbeIdentity, ServiceID: entry.ServiceID,
				TargetContractRevision: entry.TargetContractRevision, Transport: entry.Transport,
				AddressFamily: entry.AddressFamily, DiagnosisKind: diagnosis.KindTCPPathFailure,
				StrategyFingerprint: entry.StrategyFingerprint, Backend: entry.Backend,
				BackendFingerprint: entry.BackendFingerprint, CapabilityFingerprint: entry.CapabilityFingerprint,
				ContextKey: entry.ContextKey, Now: entry.RecordedAt.Add(time.Hour),
			}
			if match := outcomeledger.MatchEntry(entry, query); match.Status != outcomeledger.MatchCompatible {
				t.Fatalf("control broken: %s history is not compatible (%s/%v)", entry.StrategyID, match.Status, match.Reasons)
			}
		}
	})

	t.Run("a blocked candidate never appears", func(t *testing.T) {
		advice := advisor.Advise(advisorContext)
		if len(advice.Candidates) != 1 || advice.Candidates[0].StrategyFingerprint != eligibleFingerprint {
			t.Fatalf("INVARIANT BROKEN [HISTORY != ELIGIBILITY]: advice contains candidates Planner did not mark eligible: %#v", advice.Candidates)
		}
		if slices.ContainsFunc(advice.Candidates, func(c autotunevnext.CandidateAdvice) bool { return c.StrategyFingerprint == blockedFingerprint }) {
			t.Fatalf("INVARIANT BROKEN [HISTORY != ELIGIBILITY]: a Planner-rejected candidate appeared in advice: %#v", advice.Candidates)
		}
		// Non-vacuity: history WAS in play for the one eligible candidate.
		if !advice.Candidates[0].HistoryUsed {
			t.Fatalf("control broken: history was not used for the eligible candidate, so nothing was proven: %#v", advice.Candidates[0])
		}
	})

	t.Run("history cannot promote a candidate the advisor was handed anyway", func(t *testing.T) {
		// Defence in depth: even a buggy caller that leaks a blocked candidate
		// into Eligible does not get it into the advice.
		leaked := advisorContext
		leaked.Eligible = []autotunevnext.CandidateIdentity{
			{StrategyID: eligibleID, StrategyFingerprint: eligibleFingerprint},
			{StrategyID: blockedID, StrategyFingerprint: blockedFingerprint},
		}
		advice := advisor.Advise(leaked)
		if slices.ContainsFunc(advice.Candidates, func(c autotunevnext.CandidateAdvice) bool { return c.StrategyFingerprint == blockedFingerprint }) {
			t.Fatalf("INVARIANT BROKEN [HISTORY != ELIGIBILITY]: a Planner-rejected candidate survived into advice: %#v", advice.Candidates)
		}
	})

	t.Run("history only reorders, and the ranking package says so", func(t *testing.T) {
		report := historyIdentityDiagnosis(t, historyIdentityRecordAt)

		// A non-compatible match must be completely inert.
		neutral := recommendation.Recommend(recommendation.Input{
			Diagnosis:  report,
			Planner:    plannerReport,
			Candidates: []recommendation.Candidate{{StrategyID: eligibleID, StrategyFingerprint: eligibleFingerprint}},
			Matches: map[string][]outcomeledger.Match{eligibleFingerprint: {
				{Entry: eligibleEntry, Status: outcomeledger.MatchIncompatible, Reasons: []outcomeledger.Reason{outcomeledger.ReasonContextChanged}},
			}},
		})
		if neutral.Disposition != recommendation.DispositionExperimentCandidates || len(neutral.CandidateRecommendations) != 1 {
			t.Fatalf("control broken: a single eligible candidate vanished: %#v", neutral)
		}
		if neutral.CandidateRecommendations[0].HistoryUsed {
			t.Fatalf("INVARIANT BROKEN: a non-compatible match was treated as history: %#v", neutral.CandidateRecommendations[0])
		}
		for _, code := range neutral.CandidateRecommendations[0].ReasonCodes {
			if code != "NO_COMPATIBLE_EFFECTIVENESS_HISTORY" {
				t.Fatalf("INVARIANT BROKEN: an incompatible match produced reason %q: %#v", code, neutral.CandidateRecommendations[0])
			}
		}

		// A structurally ineligible Planner candidate contributes nothing even
		// when its matches are maximally positive.
		ineligible := recommendation.Recommend(recommendation.Input{
			Diagnosis: report,
			Planner:   plannerReport,
			Candidates: []recommendation.Candidate{
				{StrategyID: eligibleID, StrategyFingerprint: eligibleFingerprint},
				{StrategyID: blockedID, StrategyFingerprint: blockedFingerprint},
			},
			Matches: map[string][]outcomeledger.Match{
				eligibleFingerprint: {{Entry: eligibleEntry, Status: outcomeledger.MatchCompatible}},
				blockedFingerprint:  {{Entry: blockedEntry, Status: outcomeledger.MatchCompatible}},
			},
		})
		if slices.ContainsFunc(ineligible.CandidateRecommendations, func(c recommendation.CandidateRecommendation) bool {
			return c.StrategyFingerprint == blockedFingerprint
		}) {
			t.Fatalf("INVARIANT BROKEN [HISTORY != ELIGIBILITY]: ranking promoted a Planner-ineligible candidate: %#v", ineligible.CandidateRecommendations)
		}
	})
}

// TestHistoryIdentityUnavailableIsReportedNotFabricated pins the failure mode
// the identities exist to prevent: a fabricated fallback that would make every
// machine share one namespace.
//
// INVARIANT: an unavailable identity must be reported as unavailable, never
// substituted with a value that would make unrelated history look compatible.
func TestHistoryIdentityUnavailableIsReportedNotFabricated(t *testing.T) {
	bundle := historyid.Bundle{ContextErr: &historyid.ErrUnavailable{Identity: "CONTEXT", Reason: "no default route"}}
	if limitations := bundle.Limitations(); !slices.Contains(limitations, "CONTEXT_IDENTITY_UNAVAILABLE") {
		t.Fatalf("an unavailable context identity was not reported: %#v", limitations)
	}
	if bundle.ContextKey.Available() {
		t.Fatalf("an unavailable context identity was fabricated: %q", bundle.ContextKey)
	}
	if !historyid.IsUnavailable(bundle.ContextErr) {
		t.Fatal("a factual unavailability did not classify as unavailable")
	}

	// The empty identity then behaves as a refusal in the ledger, not as a
	// wildcard that silently un-narrows the history namespace.
	cohort := historyIdentityDefaultCohort()
	cohort.contextKey = ""
	entry := historyIdentityEntry(t, cohort)
	historyIdentityExpect(t, "unavailable context identity", outcomeledger.MatchEntry(entry, historyIdentityQuery(entry, entry.RecordedAt.Add(time.Hour))),
		outcomeledger.MatchStale, outcomeledger.ReasonContextMissing)
}

// TestHistoryIdentityCohortIsSelfConsistent guards the end-to-end fixture: the
// strategy fingerprint history is recorded under must be the real digest of a
// real catalog strategy, otherwise "compatible" could be an accident of a
// malformed fingerprint rather than a statement about identity.
func TestHistoryIdentityCohortIsSelfConsistent(t *testing.T) {
	result := historyIdentityRun(t, nil, false, false, true, false)
	executed := ""
	for _, experiment := range result.Experiments {
		if experiment.ExperimentExecuted {
			executed = experiment.Fingerprint
		}
	}
	if executed == "" {
		t.Fatalf("no candidate was executed: %#v", result.Experiments)
	}
	_, _, _, catalog := historyIdentityProductProbe(t)
	for _, strategy := range catalog {
		computed, err := strategyir.Fingerprint(strategy)
		if err != nil {
			t.Fatal(err)
		}
		if computed == executed {
			return
		}
	}
	t.Fatalf("executed candidate is not a catalog strategy: %q", executed)
}
