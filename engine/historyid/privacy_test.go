package historyid

// Privacy and fail-closed tests for the history-identity layer.
//
// FROZEN PRINCIPLE UNDER TEST. HISTORY != ELIGIBILITY, HISTORY != VERIFIED_FIXED,
// HISTORY != APPLY_PERMISSION. CURRENT_VALIDATION > HISTORICAL_SUCCESS. These
// tests therefore assert that the identity layer can only ever make historical
// reuse MORE conservative: it must never emit a fabricated value, never widen
// reuse, and never reveal the local network facts or the HMAC key that it
// reduces them through.
//
// HERMETIC BY CONSTRUCTION. No test here reads the host's real network, touches
// a real key file, or opens a socket. Every contextSource is synthetic, every
// key is a fixed local value, and every filesystem path comes from t.TempDir().
// SystemNetworkContextProvider and Bundle.ForRun are deliberately NOT exercised
// here: they observe the live machine, which would make the result depend on
// whatever network the test happens to run on.

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"

	"unbound/engine/attribution"
	"unbound/engine/autotunevnext"
	"unbound/engine/backendcap"
	"unbound/engine/diagnosis"
	"unbound/engine/observatory"
	"unbound/engine/outcomeledger"
	"unbound/engine/strategyir"
)

// contextIdentityFormat is the ONLY shape a network context identity may take in
// any caller: a version prefix plus a 64 character lowercase hex digest.
var contextIdentityFormat = regexp.MustCompile(`^context-v1-[0-9a-f]{64}$`)

// Synthetic secrets. Each is shaped like the real fact it stands in for, and
// each is chosen so it could never occur naturally: if any of these strings
// appears in any derived value, the value is leaking, not coinciding.
const (
	privacySecretSSID      = "SECRET_SSID_ABC123"
	privacySecretGateway   = "10.99.88.1"
	privacySecretResolver  = "resolver-secret.example"
	privacySecretAdapter   = "adapter-secret-guid"
	privacySecretInterface = "iface-secret-name"
)

func privacySecrets() []string {
	return []string{
		privacySecretSSID,
		privacySecretGateway,
		privacySecretResolver,
		privacySecretAdapter,
		privacySecretInterface,
	}
}

// privacySecretSource is a complete, sufficient local network context whose
// every member is a synthetic secret.
//
// Interface members carry the "|index" suffix the real readers append, so the
// secrets below are substrings of what the OS would actually report.
func privacySecretSource() contextSource {
	return contextSource{
		Platform:         "windows",
		DefaultInterface: []string{privacySecretInterface + "|7", privacySecretAdapter + "|11"},
		Gateway:          []string{privacySecretGateway},
		Resolver:         []string{privacySecretResolver},
		Profile:          privacySecretSSID,
	}
}

// privacyFixedKey returns a deterministic, valid 32 byte local key. Tests that
// vary exactly one thing use it so that only that thing can move the identity.
func privacyFixedKey(seed byte) []byte {
	key := make([]byte, contextKeyBytes)
	for i := range key {
		key[i] = seed ^ byte(i*7+1)
	}
	return key
}

func mustPrivacyIdentity(t *testing.T, key []byte, source contextSource) NetworkContextIdentity {
	t.Helper()
	identity, err := ContextKey(key, source)
	if err != nil {
		t.Fatalf("ContextKey returned error: %v", err)
	}
	if !contextIdentityFormat.MatchString(string(identity)) {
		t.Fatalf("identity %q is not context-v1- plus 64 lowercase hex characters", identity)
	}
	if !identity.Available() {
		t.Fatalf("identity %q reports itself unavailable", identity)
	}
	return identity
}

// privacyEncodings renders one secret under every representation a careless
// implementation might plausibly emit: verbatim, case folded, hex, both base64
// alphabets, and URL escaped. A leak is a leak in any of them.
func privacyEncodings(value string) map[string]string {
	return map[string]string{
		"raw":              value,
		"lowercase":        strings.ToLower(value),
		"uppercase":        strings.ToUpper(value),
		"hex":              hex.EncodeToString([]byte(value)),
		"base64_std":       base64.StdEncoding.EncodeToString([]byte(value)),
		"base64_url":       base64.URLEncoding.EncodeToString([]byte(value)),
		"url_query_escape": url.QueryEscape(value),
		"url_path_escape":  url.PathEscape(value),
	}
}

// assertNoPrivacySecrets fails if any secret survives, in any encoding, inside
// the named surface. Every surface here is something a caller could persist or
// show to a user.
func assertNoPrivacySecrets(t *testing.T, surface, content string, secrets []string) {
	t.Helper()
	if content == "" {
		return
	}
	for _, secret := range secrets {
		for encoding, needle := range privacyEncodings(secret) {
			if strings.Contains(content, needle) {
				t.Errorf("%s leaks secret %q as %s: %q found in %q", surface, secret, encoding, needle, content)
			}
		}
	}
}

// ---------------------------------------------------------------------------
// 1. Raw source values never appear.
// ---------------------------------------------------------------------------

func TestContextIdentityNeverContainsRawNetworkFacts(t *testing.T) {
	source := privacySecretSource()
	identity := mustPrivacyIdentity(t, privacyFixedKey(0x11), source)

	if !contextIdentityFormat.MatchString(identity.String()) {
		t.Fatalf("identity %q is not exactly context-v1- plus 64 lowercase hex characters", identity)
	}

	// The opaque form must not merely be formatted correctly; it must not carry
	// any source fact at all.
	assertNoPrivacySecrets(t, "identity", identity.String(), privacySecrets())

	// Nothing in the returned value is the input under any plausible identity.
	for _, secret := range privacySecrets() {
		if strings.Contains(identity.String(), secret) {
			t.Fatalf("identity %q contains the raw source value %q", identity, secret)
		}
	}

	// The canonical source form is HMAC input only. Prove it is not reachable
	// from the identity by confirming the identity is not the plain digest of
	// the canonical bytes, which is what an unkeyed or echoing implementation
	// would return.
	plain := sha256.Sum256(source.canonical())
	if identity.String() == formatIdentity(contextPrefix, plain[:]) {
		t.Fatalf("identity is an unkeyed digest of the canonical source: %q", identity)
	}
}

// ---------------------------------------------------------------------------
// 2. Raw source values never appear anywhere downstream.
// ---------------------------------------------------------------------------

// mustPrivacyLedgerEntry builds a real outcomeledger.OutcomeEntry carrying the
// given context key, then marshals it. This is the surface that actually
// reaches disk and the user interface, so it is where a leak would matter.
func mustPrivacyLedgerEntry(t *testing.T, contextKey, capabilityFingerprint string) outcomeledger.OutcomeEntry {
	t.Helper()
	at := time.Date(2026, 3, 4, 5, 6, 7, 0, time.UTC)

	primary := 0
	port := "443"
	spec := observatory.ProbeSpec{
		SchemaVersion:          observatory.ProbeSpecSchemaVersion,
		ID:                     "probe",
		ServiceID:              "service",
		TargetContractRevision: "v1",
		Target:                 observatory.Target{URL: "https://example.test/ok", Hostname: "example.test", Port: port, RequestedProtocol: observatory.TransportTCP},
		Transport:              observatory.TransportTCP,
		AddressFamilyPolicy:    observatory.AddressFamilyIPv4,
		Mode:                   observatory.ProbeModeHTTPSGet,
		ExpectedResponse:       observatory.ExpectedResponse{AllowedStatusCodes: []int{204}, RequirePathComplete: true},
		ControlRole:            observatory.ControlRolePrimary,
		Privacy:                observatory.PrivacyModeRedacted,
	}
	stages := []observatory.StageEvidence{
		{Stage: observatory.StageResolve, Status: observatory.StatusPass},
		{Stage: observatory.StageConnect, Status: observatory.StatusFail, Class: observatory.ClassTCPConnectTimeout},
	}
	observation := observatory.ObservationResult{
		SchemaVersion:       observatory.SchemaVersion,
		RunID:               "run",
		StartedAt:           at,
		FinishedAt:          at.Add(time.Second),
		Target:              spec.Target,
		Attempts:            []observatory.ConnectionAttempt{{ResolvedIP: "192.0.2.1", AddressFamily: observatory.AddressFamilyIPv4, Transport: observatory.TransportTCP, Stages: stages}},
		PrimaryAttemptIndex: &primary,
		FinalBoundary:       observatory.StageConnect,
		Classification:      observatory.ClassTCPConnectTimeout,
		ExecutionContext:    observatory.ExecutionContext{Mode: "direct"},
	}
	evidence, err := observatory.BuildEvidenceRecord(spec, observatory.EvidenceInput{Observations: []observatory.ObservationResult{observation}})
	if err != nil {
		t.Fatalf("build evidence: %v", err)
	}

	runID := evidence.Runs[0].Observation.RunID
	finding := attribution.Finding{
		Code:               attribution.FindingTCPPathFailureSuspected,
		Confidence:         attribution.ConfidenceHigh,
		Stage:              observatory.StageConnect,
		SupportingEvidence: []attribution.EvidenceRef{{RunID: runID, AttemptIndex: 0, Stage: observatory.StageConnect}},
	}
	report, err := diagnosis.Diagnose(diagnosis.Input{
		Target: []observatory.EvidenceRecord{evidence},
		Attribution: attribution.AttributionReport{
			SchemaVersion: attribution.SchemaVersion,
			AttributionID: "attribution-v1-privacy",
			InputRunIDs:   []string{runID},
			Target: attribution.TargetRef{
				Scheme: "https", Hostname: evidence.Probe.Target.Hostname, Port: evidence.Probe.Target.Port,
				Path: "/ok", RequestedProtocol: evidence.Probe.Transport,
			},
			PrimaryFinding: finding,
			Findings:       []attribution.Finding{finding},
		},
	})
	if err != nil {
		t.Fatalf("diagnose: %v", err)
	}

	strategy := strategyir.RepresentativeFixtures()["alternative-multisplit"]
	strategyFingerprint, err := strategyir.Fingerprint(strategy)
	if err != nil {
		t.Fatalf("strategy fingerprint: %v", err)
	}

	entry, err := outcomeledger.NewEntry(outcomeledger.BuildInput{
		Evidence:              []observatory.EvidenceRecord{evidence},
		ValidationEvidence:    []observatory.EvidenceRecord{evidence},
		Diagnosis:             report,
		StrategyID:            strategy.ID,
		StrategyFingerprint:   strategyFingerprint,
		Backend:               backendcap.Zapret2Windows,
		BackendFingerprint:    "backend-v1-" + strings.Repeat("a", 64),
		CapabilityFingerprint: capabilityFingerprint,
		Outcome:               autotunevnext.OutcomeVerifiedFixed,
		RecordedAt:            report.EffectiveAt,
		ContextKey:            contextKey,
	}, outcomeledger.DefaultPolicy())
	if err != nil {
		t.Fatalf("NewEntry with the derived context key was rejected: %v", err)
	}
	return entry
}

func TestContextIdentityAndEverythingDerivedFromItAreFreeOfRawNetworkFacts(t *testing.T) {
	secrets := privacySecrets()
	identity := mustPrivacyIdentity(t, privacyFixedKey(0x22), privacySecretSource())
	backend := backendcap.Zapret2Windows
	capability := mustCapabilityFingerprint(t, backendcap.Get(backend))

	entry := mustPrivacyLedgerEntry(t, identity.String(), capability.String())
	if entry.ContextKey != identity.String() {
		t.Fatalf("the ledger stored context key %q, want %q", entry.ContextKey, identity)
	}
	encoded, err := json.Marshal(entry)
	if err != nil {
		t.Fatalf("marshal ledger entry: %v", err)
	}

	surfaces := map[string]string{
		"context identity":         identity.String(),
		"backend label":            string(backend),
		"capability fingerprint":   capability.String(),
		"ledger entry JSON":        string(encoded),
		"ledger entry context_key": entry.ContextKey,
	}
	for surface, content := range surfaces {
		assertNoPrivacySecrets(t, surface, content, secrets)
	}

	// The opaque identities must survive the round trip unchanged, so the
	// ledger is holding the fingerprint and not something derived from it.
	if !strings.Contains(string(encoded), identity.String()) {
		t.Fatalf("ledger JSON does not carry the context identity %q: %s", identity, encoded)
	}
	if !strings.Contains(string(encoded), capability.String()) {
		t.Fatalf("ledger JSON does not carry the capability fingerprint %q: %s", capability, encoded)
	}
}

// TestLedgerRefusesRawNetworkFactsAsContextKey proves that the raw facts are
// not merely absent from the serialized entry, but are rejected outright by the
// consumer. An identity that merely hid them while a raw label would have been
// accepted would not be a privacy boundary.
func TestLedgerRefusesRawNetworkFactsAsContextKey(t *testing.T) {
	for _, secret := range privacySecrets() {
		if entry, err := privacyLedgerEntryFor(t, secret); err == nil {
			t.Fatalf("the ledger accepted the raw network fact %q as a context key: %#v", secret, entry)
		}
	}
}

// privacyLedgerEntryFor builds the same entry with an arbitrary context key and
// reports the outcome, so a rejected key does not abort the test.
func privacyLedgerEntryFor(t *testing.T, contextKey string) (outcomeledger.OutcomeEntry, error) {
	t.Helper()
	at := time.Date(2026, 3, 4, 5, 6, 7, 0, time.UTC)
	primary := 0
	spec := observatory.ProbeSpec{
		SchemaVersion:          observatory.ProbeSpecSchemaVersion,
		ID:                     "probe",
		ServiceID:              "service",
		TargetContractRevision: "v1",
		Target:                 observatory.Target{URL: "https://example.test/ok", Hostname: "example.test", Port: "443", RequestedProtocol: observatory.TransportTCP},
		Transport:              observatory.TransportTCP,
		AddressFamilyPolicy:    observatory.AddressFamilyIPv4,
		Mode:                   observatory.ProbeModeHTTPSGet,
		ExpectedResponse:       observatory.ExpectedResponse{AllowedStatusCodes: []int{204}, RequirePathComplete: true},
		ControlRole:            observatory.ControlRolePrimary,
		Privacy:                observatory.PrivacyModeRedacted,
	}
	stages := []observatory.StageEvidence{
		{Stage: observatory.StageResolve, Status: observatory.StatusPass},
		{Stage: observatory.StageConnect, Status: observatory.StatusFail, Class: observatory.ClassTCPConnectTimeout},
	}
	observation := observatory.ObservationResult{
		SchemaVersion:       observatory.SchemaVersion,
		RunID:               "run",
		StartedAt:           at,
		FinishedAt:          at.Add(time.Second),
		Target:              spec.Target,
		Attempts:            []observatory.ConnectionAttempt{{ResolvedIP: "192.0.2.1", AddressFamily: observatory.AddressFamilyIPv4, Transport: observatory.TransportTCP, Stages: stages}},
		PrimaryAttemptIndex: &primary,
		FinalBoundary:       observatory.StageConnect,
		Classification:      observatory.ClassTCPConnectTimeout,
		ExecutionContext:    observatory.ExecutionContext{Mode: "direct"},
	}
	evidence, err := observatory.BuildEvidenceRecord(spec, observatory.EvidenceInput{Observations: []observatory.ObservationResult{observation}})
	if err != nil {
		t.Fatalf("build evidence: %v", err)
	}
	runID := evidence.Runs[0].Observation.RunID
	finding := attribution.Finding{
		Code:               attribution.FindingTCPPathFailureSuspected,
		Confidence:         attribution.ConfidenceHigh,
		Stage:              observatory.StageConnect,
		SupportingEvidence: []attribution.EvidenceRef{{RunID: runID, AttemptIndex: 0, Stage: observatory.StageConnect}},
	}
	report, err := diagnosis.Diagnose(diagnosis.Input{
		Target: []observatory.EvidenceRecord{evidence},
		Attribution: attribution.AttributionReport{
			SchemaVersion: attribution.SchemaVersion,
			AttributionID: "attribution-v1-privacy",
			InputRunIDs:   []string{runID},
			Target: attribution.TargetRef{
				Scheme: "https", Hostname: evidence.Probe.Target.Hostname, Port: evidence.Probe.Target.Port,
				Path: "/ok", RequestedProtocol: evidence.Probe.Transport,
			},
			PrimaryFinding: finding,
			Findings:       []attribution.Finding{finding},
		},
	})
	if err != nil {
		t.Fatalf("diagnose: %v", err)
	}
	strategy := strategyir.RepresentativeFixtures()["alternative-multisplit"]
	strategyFingerprint, err := strategyir.Fingerprint(strategy)
	if err != nil {
		t.Fatalf("strategy fingerprint: %v", err)
	}
	return outcomeledger.NewEntry(outcomeledger.BuildInput{
		Evidence:            []observatory.EvidenceRecord{evidence},
		ValidationEvidence:  []observatory.EvidenceRecord{evidence},
		Diagnosis:           report,
		StrategyID:          strategy.ID,
		StrategyFingerprint: strategyFingerprint,
		Backend:             backendcap.Zapret2Windows,
		BackendFingerprint:  "backend-v1-" + strings.Repeat("a", 64),
		Outcome:             autotunevnext.OutcomeVerifiedFixed,
		RecordedAt:          report.EffectiveAt,
		ContextKey:          contextKey,
	}, outcomeledger.DefaultPolicy())
}

// ---------------------------------------------------------------------------
// 3. Order independence.
// ---------------------------------------------------------------------------

func TestContextIdentityIgnoresEnumerationOrder(t *testing.T) {
	key := privacyFixedKey(0x33)
	baseline := mustPrivacyIdentity(t, key, privacySecretSource())

	shuffled := privacySecretSource()
	shuffled.DefaultInterface = reversed(shuffled.DefaultInterface)
	shuffled.Gateway = reversed(shuffled.Gateway)
	shuffled.Resolver = reversed(shuffled.Resolver)

	reordered := mustPrivacyIdentity(t, key, shuffled)
	if reordered != baseline {
		t.Fatalf("identity depends on OS enumeration order: %q != %q", reordered, baseline)
	}

	// Repeating a member is the same set, not a different network.
	duplicated := privacySecretSource()
	duplicated.DefaultInterface = append(duplicated.DefaultInterface, privacySecretInterface+"|7")
	duplicated.Resolver = append(duplicated.Resolver, privacySecretResolver)
	if got := mustPrivacyIdentity(t, key, duplicated); got != baseline {
		t.Fatalf("identity depends on repeated set members: %q != %q", got, baseline)
	}

	// A nil set and an empty set both mean absent, and must not be
	// distinguishable from each other.
	absent := privacySecretSource()
	absent.Resolver = nil
	explicitlyEmpty := privacySecretSource()
	explicitlyEmpty.Resolver = []string{}
	if a, b := mustPrivacyIdentity(t, key, absent), mustPrivacyIdentity(t, key, explicitlyEmpty); a != b {
		t.Fatalf("nil and empty resolver sets disagree: %q != %q", a, b)
	}
}

// ---------------------------------------------------------------------------
// 4. Change detection.
// ---------------------------------------------------------------------------

func TestContextIdentityChangesWithEveryRelevantFact(t *testing.T) {
	key := privacyFixedKey(0x44)
	baseline := mustPrivacyIdentity(t, key, privacySecretSource())

	cases := map[string]func(*contextSource){
		"gateway_set_changed": func(s *contextSource) {
			s.Gateway = []string{"10.99.88.2"}
		},
		"resolver_set_changed": func(s *contextSource) {
			s.Resolver = append(s.Resolver, "resolver-other.example")
		},
		"extra_gateway_added": func(s *contextSource) {
			s.Gateway = append(s.Gateway, "10.99.88.2")
		},
		"gateway_absent": func(s *contextSource) {
			s.Gateway = nil
		},
		"resolver_absent": func(s *contextSource) {
			s.Resolver = nil
		},
		"default_interface_set_changed": func(s *contextSource) {
			s.DefaultInterface = append(s.DefaultInterface, "third-secret-iface|19")
		},
		// Dropping the interface set entirely is deliberately not a change
		// detection case: it makes the source insufficient, so it is asserted to
		// fail closed in TestContextIdentityFailsClosedWithoutSufficientFacts.
		"platform_changed": func(s *contextSource) {
			s.Platform = "linux"
		},
		"interface_renamed": func(s *contextSource) {
			s.DefaultInterface = []string{privacySecretInterface + "|9", privacySecretAdapter + "|11"}
		},
		"profile_changed": func(s *contextSource) {
			s.Profile = "SECRET_SSID_DEF456"
		},
	}
	for name, mutate := range cases {
		t.Run(name, func(t *testing.T) {
			changed := privacySecretSource()
			mutate(&changed)
			identity := mustPrivacyIdentity(t, key, changed)
			if identity == baseline {
				t.Fatalf("identity %q did not change when %s changed", identity, name)
			}
			// A distinct identity must still be opaque, never an echo.
			assertNoPrivacySecrets(t, "identity", identity.String(), privacySecrets())
		})
	}
}

// ---------------------------------------------------------------------------
// 5. Insufficient facts fail closed.
// ---------------------------------------------------------------------------

func TestContextIdentityFailsClosedWithoutSufficientFacts(t *testing.T) {
	key := privacyFixedKey(0x55)

	// The value a fabricated implementation would return instead of an error.
	fabricated := func(literal string) NetworkContextIdentity {
		mac := hmac.New(sha256.New, key)
		mac.Write([]byte(literal))
		return NetworkContextIdentity(formatIdentity(contextPrefix, mac.Sum(nil)))
	}

	cases := map[string]contextSource{
		"nil_default_interface": {
			Platform: "windows", Gateway: []string{privacySecretGateway},
			Resolver: []string{privacySecretResolver}, Profile: privacySecretSSID,
		},
		"empty_default_interface": {
			Platform: "windows", DefaultInterface: []string{},
			Gateway: []string{privacySecretGateway}, Resolver: []string{privacySecretResolver},
			Profile: privacySecretSSID,
		},
		"gateway_only": {
			Platform: "windows", Gateway: []string{privacySecretGateway},
		},
		"everything_absent": {},
		// A blank interface member is deliberately not listed. readContextSource
		// builds every member as name|index from an interface that already
		// passed the up, non-loopback and global-unicast filters, so no
		// production reader can hand ContextKey a blank member, and sufficient()
		// only promises to require a non-empty interface set.
	}
	for name, source := range cases {
		t.Run(name, func(t *testing.T) {
			identity, err := ContextKey(key, source)
			if err == nil {
				t.Fatalf("insufficient facts produced identity %q instead of an error", identity)
			}
			var unavailable *ErrUnavailable
			if !errors.As(err, &unavailable) {
				t.Fatalf("error %v (%T) is not an *ErrUnavailable", err, err)
			}
			if unavailable.Identity != "context" {
				t.Fatalf("ErrUnavailable.Identity = %q, want %q", unavailable.Identity, "context")
			}
			if unavailable.Reason == "" {
				t.Fatal("ErrUnavailable.Reason is empty; the caller must know why history became unavailable")
			}

			// The identity must be absent, not merely unavailable-looking.
			if identity != "" {
				t.Fatalf("failed derivation returned identity %q, want empty", identity)
			}
			if identity.Available() {
				t.Fatalf("empty identity %q reports itself available", identity)
			}
			if contextIdentityFormat.MatchString(identity.String()) {
				t.Fatalf("failed derivation returned a valid looking identity %q", identity)
			}

			// And specifically: it is not a hash of a literal placeholder.
			for _, literal := range []string{"unknown", "unknown_default_interface", "", "none", "0.0.0.0"} {
				if identity == fabricated(literal) {
					t.Fatalf("failed derivation returned the hash of the literal %q", literal)
				}
			}
		})
	}
}

// ---------------------------------------------------------------------------
// 6. Key rotation.
// ---------------------------------------------------------------------------

func TestContextIdentityIsNamespacedByTheLocalKey(t *testing.T) {
	source := privacySecretSource()
	keyA := privacyFixedKey(0x66)
	keyB := privacyFixedKey(0x67)
	if bytes.Equal(keyA, keyB) {
		t.Fatal("the two rotation keys are identical; the test proves nothing")
	}

	underA := mustPrivacyIdentity(t, keyA, source)
	underB := mustPrivacyIdentity(t, keyB, source)
	if underA == underB {
		t.Fatalf("rotating the local key did not change the identity: both %q", underA)
	}

	// Rotation is safe precisely because it is stable per key and never
	// collides back: the same key must still reproduce the same identity.
	if again := mustPrivacyIdentity(t, keyA, source); again != underA {
		t.Fatalf("identity under key A is not stable: %q != %q", again, underA)
	}
	assertNoPrivacySecrets(t, "identity", underA.String(), privacySecrets())
	assertNoPrivacySecrets(t, "identity", underB.String(), privacySecrets())
}

// ---------------------------------------------------------------------------
// 7. HMAC key bytes never leak.
// ---------------------------------------------------------------------------

func TestContextIdentityNeverLeaksTheHMACKey(t *testing.T) {
	key := privacyFixedKey(0x77)
	identity := mustPrivacyIdentity(t, key, privacySecretSource())
	capability := mustCapabilityFingerprint(t, backendcap.Get(backendcap.Zapret2Windows))
	entry := mustPrivacyLedgerEntry(t, identity.String(), capability.String())
	encoded, err := json.Marshal(entry)
	if err != nil {
		t.Fatalf("marshal ledger entry: %v", err)
	}

	surfaces := map[string]string{
		"context identity":  identity.String(),
		"ledger entry JSON": string(encoded),
	}

	needles := map[string]string{
		"key_hex":        hex.EncodeToString(key),
		"key_base64_std": base64.StdEncoding.EncodeToString(key),
		"key_base64_url": base64.URLEncoding.EncodeToString(key),
		"key_raw":        string(key),
	}
	// Every 8 byte window of the key, in the encodings a partial leak would
	// most plausibly take.
	for offset := 0; offset+8 <= len(key); offset++ {
		window := key[offset : offset+8]
		needles["window_hex_"+itoa(offset)] = hex.EncodeToString(window)
		needles["window_base64_"+itoa(offset)] = base64.StdEncoding.EncodeToString(window)
	}

	for surface, content := range surfaces {
		for name, needle := range needles {
			if needle == "" {
				continue
			}
			if strings.Contains(content, needle) {
				t.Errorf("%s leaks key material (%s): %q found in %q", surface, name, needle, content)
			}
		}
	}
}

// ---------------------------------------------------------------------------
// 8. Empty / weak keys are rejected.
// ---------------------------------------------------------------------------

func TestContextIdentityRejectsWeakKeys(t *testing.T) {
	source := privacySecretSource()
	cases := map[string][]byte{
		"nil_key":      nil,
		"empty_key":    {},
		"one_byte_key": {0x01},
		"short_key":    make([]byte, 31),
	}
	for name, key := range cases {
		t.Run(name, func(t *testing.T) {
			identity, err := ContextKey(key, source)
			if err == nil {
				t.Fatalf("weak key produced identity %q instead of an error", identity)
			}
			var unavailable *ErrUnavailable
			if !errors.As(err, &unavailable) {
				t.Fatalf("error %v (%T) is not an *ErrUnavailable", err, err)
			}
			if identity != "" {
				t.Fatalf("weak key returned identity %q, want empty", identity)
			}
			if identity.Available() || contextIdentityFormat.MatchString(identity.String()) {
				t.Fatalf("weak key returned a valid looking identity %q", identity)
			}
			// An identity derived from a guessable key is guessable offline,
			// so the failure must not be silently repaired by any fallback.
			mac := hmac.New(sha256.New, key)
			mac.Write(source.canonical())
			if identity == NetworkContextIdentity(formatIdentity(contextPrefix, mac.Sum(nil))) {
				t.Fatalf("identity was derived from the weak key: %q", identity)
			}
		})
	}

	// A 32 byte key is the documented minimum and must be accepted, otherwise
	// the rejection above would be vacuous.
	if got := mustPrivacyIdentity(t, make([]byte, 32), source); !got.Available() {
		t.Fatalf("a valid 32 byte key produced %q", got)
	}
}

// ---------------------------------------------------------------------------
// 9. Key file behaviour.
// ---------------------------------------------------------------------------

func TestLoadOrCreateContextKeyCreatesThenIsStable(t *testing.T) {
	ForgetCachedContextKey()
	dir := t.TempDir()

	first, err := LoadOrCreateContextKey(dir)
	if err != nil {
		t.Fatalf("first LoadOrCreateContextKey: %v", err)
	}
	if len(first) != contextKeyBytes {
		t.Fatalf("key length = %d, want %d", len(first), contextKeyBytes)
	}

	second, err := LoadOrCreateContextKey(dir)
	if err != nil {
		t.Fatalf("second LoadOrCreateContextKey: %v", err)
	}
	if !bytes.Equal(first, second) {
		t.Fatal("a second load returned different key bytes; one namespace became two")
	}

	// The persisted form must decode back to exactly the returned key.
	path := filepath.Join(dir, ContextKeyFileName)
	onDisk, err := readContextKeyFile(path)
	if err != nil {
		t.Fatalf("readContextKeyFile: %v", err)
	}
	if !bytes.Equal(first, onDisk) {
		t.Fatal("the persisted key file does not decode to the returned key")
	}
}

// TestLoadOrCreateContextKeyConcurrentCreatesAgree exercises the real create
// race.
//
// ABOUT THE PROCESS CACHE. CachedContextKey memoises per process, but
// LoadOrCreateContextKey deliberately does not consult that cache: it is the
// cold path every process and every restart takes. Calling it directly,
// concurrently, on one fresh directory, therefore exercises the genuine
// O_CREATE|O_EXCL winner/loser path rather than a memoised short circuit.
// ForgetCachedContextKey is still called so that no key cached by an earlier
// test can leak into this one.
//
// WHAT THIS TEST ASSERTS. Whatever happens inside the create window, the local
// history namespace must never split: every racer that obtains a key obtains
// the SAME key, and that key is the one on disk. That invariant is
// deterministic and holds today.
//
// A racer can currently fail with errContextKeyCorrupt by reading the key file
// between the winner's O_EXCL create and its first write. That failure is
// asserted deterministically, on every run, by the companion test below, so
// this test reports it instead of failing on a scheduling coin flip.
func TestLoadOrCreateContextKeyConcurrentCreatesAgree(t *testing.T) {
	ForgetCachedContextKey()
	dir := t.TempDir()

	const racers = 16
	keys := make([][]byte, racers)
	errs := make([]error, racers)
	start := make(chan struct{})
	var wg sync.WaitGroup
	for i := range racers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			keys[i], errs[i] = LoadOrCreateContextKey(dir)
		}()
	}
	close(start)
	wg.Wait()

	var agreed []byte
	for i := range racers {
		if errs[i] != nil {
			// The in-progress file window, asserted and failing on every run by
			// TestLoadOrCreateContextKeyRecoversFromAHalfCreatedKeyFile.
			t.Logf("racer %d entered the create window and failed: %v", i, errs[i])
			continue
		}
		if len(keys[i]) != contextKeyBytes {
			t.Fatalf("racer %d observed %d key bytes, want %d", i, len(keys[i]), contextKeyBytes)
		}
		if agreed == nil {
			agreed = keys[i]
			continue
		}
		if !bytes.Equal(keys[i], agreed) {
			t.Fatalf("racer %d observed different key bytes: the local history namespace split", i)
		}
	}
	if agreed == nil {
		t.Fatal("no racer obtained a key at all")
	}

	// Exactly one key file, holding that one key.
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("read config dir: %v", err)
	}
	if len(entries) != 1 || entries[0].Name() != ContextKeyFileName {
		names := make([]string, 0, len(entries))
		for _, entry := range entries {
			names = append(names, entry.Name())
		}
		t.Fatalf("config dir contains %v, want exactly [%s]", names, ContextKeyFileName)
	}
	onDisk, err := readContextKeyFile(filepath.Join(dir, ContextKeyFileName))
	if err != nil {
		t.Fatalf("readContextKeyFile: %v", err)
	}
	if !bytes.Equal(onDisk, agreed) {
		t.Fatal("the persisted key does not match the key every successful racer observed")
	}
}

// TestLoadOrCreateContextKeyRecoversFromAHalfCreatedKeyFile pins the exact
// window that makes the race above observable, without depending on timing.
//
// os.OpenFile with O_CREATE|O_EXCL makes the key path exist the instant the
// winner wins, and the key bytes only reach the file on the following
// WriteString. A reader arriving inside that window — a concurrent process, or
// the next start after a crash between create and write — sees a file that
// exists but is zero length. readContextKeyFile classifies that as
// errContextKeyCorrupt, and LoadOrCreateContextKey turns it into a hard error
// instead of re-reading the winner's key.
//
// This contradicts the atomicity contract in context_key.go, which states that
// the loser of the create race "does NOT overwrite, it re-reads the winner's
// key". It fails closed, so history reuse only becomes more conservative and no
// namespace ever splits, but two processes starting together — or one start
// after a crash mid write — can lose context identity, and after a crash the
// zero length file is permanent until someone deletes it by hand.
func TestLoadOrCreateContextKeyRecoversFromAHalfCreatedKeyFile(t *testing.T) {
	ForgetCachedContextKey()
	dir := t.TempDir()
	path := filepath.Join(dir, ContextKeyFileName)

	// Exactly the state left behind between O_EXCL create and the first write.
	if err := os.WriteFile(path, nil, 0o600); err != nil {
		t.Fatalf("seed half-created key file: %v", err)
	}

	key, err := LoadOrCreateContextKey(dir)
	if err != nil {
		t.Fatalf("a half-created key file was reported as %v instead of being completed by its creator", err)
	}
	if len(key) != contextKeyBytes {
		t.Fatalf("recovered key length = %d, want %d", len(key), contextKeyBytes)
	}
	onDisk, err := readContextKeyFile(path)
	if err != nil {
		t.Fatalf("readContextKeyFile: %v", err)
	}
	if !bytes.Equal(onDisk, key) {
		t.Fatal("the recovered key does not match the persisted key")
	}
}

func TestLoadOrCreateContextKeyRefusesToReinterpretACorruptKeyFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, ContextKeyFileName)

	// Garbage of a length that is neither the hex form nor the raw byte form.
	garbage := []byte("corrupt-key-file")
	if err := os.WriteFile(path, garbage, 0o600); err != nil {
		t.Fatalf("seed corrupt key file: %v", err)
	}
	ForgetCachedContextKey()

	key, err := LoadOrCreateContextKey(dir)
	if err == nil {
		t.Fatalf("a corrupt key file was accepted as %x; old history would silently change namespace", key)
	}
	if !errors.Is(err, errContextKeyCorrupt) {
		t.Fatalf("error %v is not errContextKeyCorrupt", err)
	}
	if key != nil {
		t.Fatalf("a corrupt key file returned key bytes %x", key)
	}

	// The corrupt file must be left exactly as the user left it: history must
	// not be silently reinterpreted under a namespace the user never chose.
	after, readErr := os.ReadFile(path)
	if readErr != nil {
		t.Fatalf("the corrupt key file was removed or made unreadable: %v", readErr)
	}
	if !bytes.Equal(after, garbage) {
		t.Fatalf("the corrupt key file was rewritten as %q, want %q", after, garbage)
	}

	// It stays unusable, and stays untouched, on every later read too.
	for range 3 {
		ForgetCachedContextKey()
		if _, err := LoadOrCreateContextKey(dir); err == nil {
			t.Fatal("a corrupt key file was accepted on a later read")
		}
		final, readErr := os.ReadFile(path)
		if readErr != nil || !bytes.Equal(final, garbage) {
			t.Fatalf("the corrupt key file changed on a later read: %q (%v)", final, readErr)
		}
	}
}

func TestContextKeyFileIsOwnerOnly(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Windows relies on the user-owned config directory DACL rather than Unix mode bits")
	}
	ForgetCachedContextKey()
	dir := t.TempDir()
	if _, err := LoadOrCreateContextKey(dir); err != nil {
		t.Fatalf("LoadOrCreateContextKey: %v", err)
	}
	info, err := os.Stat(filepath.Join(dir, ContextKeyFileName))
	if err != nil {
		t.Fatalf("stat key file: %v", err)
	}
	if mode := info.Mode().Perm(); mode != 0o600 {
		t.Fatalf("key file mode = %#o, want 0600: the key must not be readable by other users", mode)
	}
}

// ---------------------------------------------------------------------------
// 10. Key file location.
// ---------------------------------------------------------------------------

func TestLoadOrCreateContextKeyRequiresAnOwnedConfigDirectory(t *testing.T) {
	ForgetCachedContextKey()

	_, statBefore := os.Stat(ContextKeyFileName)
	key, err := LoadOrCreateContextKey("")
	if err == nil {
		t.Fatalf("an empty config dir produced key %x; the key would land in an arbitrary working directory", key)
	}
	if key != nil {
		t.Fatalf("an empty config dir returned key bytes %x", key)
	}

	// An empty config dir resolves to the bare file name, i.e. the process
	// working directory. Nothing may be created there. The check is conditional
	// so that an unrelated pre-existing file of the same name, created by some
	// other test, cannot make this flaky.
	if errors.Is(statBefore, os.ErrNotExist) {
		if _, statErr := os.Stat(ContextKeyFileName); !errors.Is(statErr, os.ErrNotExist) {
			t.Fatalf("a key file was created in the working directory (%v)", statErr)
		}
	}
}
