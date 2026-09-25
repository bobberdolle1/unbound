// Package outcomeledger stores bounded, redacted historical experiment outcomes.
// It is deliberately pure apart from explicitly requested local file I/O: it
// neither executes, ranks, recommends, nor activates strategies.
package outcomeledger

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"unbound/engine/attribution"
	"unbound/engine/autotunevnext"
	"unbound/engine/backendcap"
	"unbound/engine/diagnosis"
	"unbound/engine/observatory"
)

const (
	SchemaVersion = 1
	FileName      = "autotune_vnext_outcomes.json"
)

type InvalidationReason string

const (
	InvalidationExplicit       InvalidationReason = "EXPLICIT"
	InvalidationContradicted   InvalidationReason = "CONTRADICTED"
	InvalidationContextChanged InvalidationReason = "CONTEXT_CHANGED"
)

type Ledger struct {
	SchemaVersion int            `json:"schema_version"`
	Entries       []OutcomeEntry `json:"entries"`
	Fingerprint   string         `json:"fingerprint"`
}

// OutcomeEntry contains only redacted identifiers. It intentionally has no
// edge, hostname, URL, interface, gateway, resolver, local address, or body.
type OutcomeEntry struct {
	EntryID                string                    `json:"entry_id"`
	ProbeIdentity          string                    `json:"probe_identity"`
	ServiceID              string                    `json:"service_id"`
	TargetContractRevision string                    `json:"target_contract_revision"`
	Transport              observatory.Transport     `json:"transport"`
	AddressFamily          observatory.AddressFamily `json:"address_family"`
	StrategyID             string                    `json:"strategy_id,omitempty"`
	StrategyFingerprint    string                    `json:"strategy_fingerprint,omitempty"`
	Backend                backendcap.Backend        `json:"backend,omitempty"`
	BackendFingerprint     string                    `json:"backend_fingerprint,omitempty"`
	CapabilityFingerprint  string                    `json:"capability_fingerprint,omitempty"`
	DiagnosisID            string                    `json:"diagnosis_id"`
	DiagnosisKind          diagnosis.Kind            `json:"diagnosis_kind"`
	DiagnosisConfidence    attribution.Confidence    `json:"diagnosis_confidence"`
	EvidenceFingerprints   []string                  `json:"evidence_fingerprints"`
	Outcome                autotunevnext.Outcome     `json:"outcome"`
	RecordedAt             time.Time                 `json:"recorded_at"`
	ExpiresAt              time.Time                 `json:"expires_at"`
	ContextKey             string                    `json:"context_key,omitempty"`
	InvalidatedAt          *time.Time                `json:"invalidated_at,omitempty"`
	InvalidationReason     InvalidationReason        `json:"invalidation_reason,omitempty"`
}

type Policy struct {
	MaxEntries       int
	VerifiedFixedTTL time.Duration
	NegativeTTL      time.Duration
	InconclusiveTTL  time.Duration
	LifecycleTTL     time.Duration
}

func DefaultPolicy() Policy {
	return Policy{MaxEntries: 128, VerifiedFixedTTL: 24 * time.Hour, NegativeTTL: 12 * time.Hour, InconclusiveTTL: 6 * time.Hour, LifecycleTTL: 6 * time.Hour}
}

func (policy Policy) normalized() (Policy, error) {
	defaults := DefaultPolicy()
	if policy.MaxEntries == 0 {
		policy.MaxEntries = defaults.MaxEntries
	}
	if policy.VerifiedFixedTTL == 0 {
		policy.VerifiedFixedTTL = defaults.VerifiedFixedTTL
	}
	if policy.NegativeTTL == 0 {
		policy.NegativeTTL = defaults.NegativeTTL
	}
	if policy.InconclusiveTTL == 0 {
		policy.InconclusiveTTL = defaults.InconclusiveTTL
	}
	if policy.LifecycleTTL == 0 {
		policy.LifecycleTTL = defaults.LifecycleTTL
	}
	if policy.MaxEntries < 1 || policy.VerifiedFixedTTL <= 0 || policy.NegativeTTL <= 0 || policy.InconclusiveTTL <= 0 || policy.LifecycleTTL <= 0 {
		return Policy{}, fmt.Errorf("ledger policy requires positive bounds and TTLs")
	}
	return policy, nil
}

type BuildInput struct {
	Evidence              []observatory.EvidenceRecord
	Diagnosis             diagnosis.Report
	StrategyID            string
	StrategyFingerprint   string
	Backend               backendcap.Backend
	BackendFingerprint    string
	CapabilityFingerprint string
	Outcome               autotunevnext.Outcome
	RecordedAt            time.Time
	ContextKey            string
}

// NewEntry creates one immutable, evidence-linked historical event. RecordedAt
// is the effective time of the complete diagnosis evidence cohort; requiring
// exact equality prevents callers from extending TTL with an arbitrary time.
func NewEntry(input BuildInput, policy Policy) (OutcomeEntry, error) {
	policy, err := policy.normalized()
	if err != nil {
		return OutcomeEntry{}, err
	}
	if input.RecordedAt.IsZero() {
		return OutcomeEntry{}, fmt.Errorf("recorded time is required")
	}
	if !persistedOutcome(input.Outcome) {
		return OutcomeEntry{}, fmt.Errorf("outcome %q is not durable effectiveness history", input.Outcome)
	}
	if err := input.Diagnosis.Validate(); err != nil {
		return OutcomeEntry{}, fmt.Errorf("invalid diagnosis report: %w", err)
	}
	if !input.RecordedAt.UTC().Equal(input.Diagnosis.EffectiveAt.UTC()) {
		return OutcomeEntry{}, fmt.Errorf("recorded time must equal diagnosis effective time")
	}
	if err := safeContextKey(input.ContextKey); err != nil {
		return OutcomeEntry{}, err
	}
	if err := safeOpaqueFingerprint("backend", input.BackendFingerprint); err != nil {
		return OutcomeEntry{}, err
	}
	if err := safeOpaqueFingerprint("capability", input.CapabilityFingerprint); err != nil {
		return OutcomeEntry{}, err
	}
	if len(input.Evidence) == 0 {
		return OutcomeEntry{}, fmt.Errorf("outcome entry requires evidence")
	}
	fingerprints := make([]string, 0, len(input.Evidence))
	var target *observatory.EvidenceRecord
	var latest time.Time
	family := observatory.AddressFamily("")
	for index := range input.Evidence {
		record := &input.Evidence[index]
		if err := record.Validate(); err != nil {
			return OutcomeEntry{}, fmt.Errorf("invalid evidence: %w", err)
		}
		if record.CollectedAt.After(latest) {
			latest = record.CollectedAt
		}
		fingerprints = append(fingerprints, record.Fingerprint)
		if record.ProbeIdentity == input.Diagnosis.ProbeIdentity {
			if record.Probe.ControlRole != observatory.ControlRolePrimary {
				return OutcomeEntry{}, fmt.Errorf("diagnosis target evidence is not a primary target")
			}
			if target == nil {
				target = record
			}
			for _, run := range record.Runs {
				if run.AddressFamily == "" {
					continue
				}
				if family != "" && family != run.AddressFamily {
					return OutcomeEntry{}, fmt.Errorf("target evidence mixes selected address families")
				}
				family = run.AddressFamily
			}
		}
		if record.StrategyFingerprint != "" && input.StrategyFingerprint != "" && record.StrategyFingerprint != input.StrategyFingerprint {
			return OutcomeEntry{}, fmt.Errorf("strategy fingerprint does not match evidence")
		}
		if record.BackendFingerprint != "" && input.BackendFingerprint != "" && record.BackendFingerprint != input.BackendFingerprint {
			return OutcomeEntry{}, fmt.Errorf("backend fingerprint does not match evidence")
		}
	}
	if target == nil || family == "" {
		return OutcomeEntry{}, fmt.Errorf("diagnosis target requires one factual address family")
	}
	if !input.Diagnosis.EffectiveAt.UTC().Equal(latest.UTC()) {
		return OutcomeEntry{}, fmt.Errorf("diagnosis effective time does not match supplied evidence")
	}
	sort.Strings(fingerprints)
	if !sameStrings(fingerprints, sortedStrings(input.Diagnosis.EvidenceFingerprints)) {
		return OutcomeEntry{}, fmt.Errorf("diagnosis does not reference supplied evidence")
	}
	if !outcomeCompatibleDiagnosis(input.Outcome, input.Diagnosis.Kind) {
		return OutcomeEntry{}, fmt.Errorf("outcome %q is inconsistent with diagnosis %q", input.Outcome, input.Diagnosis.Kind)
	}
	if requiresStrategy(input.Outcome) {
		if input.StrategyID == "" || input.StrategyFingerprint == "" || input.Backend == "" {
			return OutcomeEntry{}, fmt.Errorf("executed outcome requires strategy and backend identities")
		}
		if err := safeStrategyFingerprint(input.StrategyFingerprint); err != nil {
			return OutcomeEntry{}, err
		}
	}
	if input.Outcome == autotunevnext.OutcomeDirectBecameReachable {
		input.StrategyID = ""
		input.StrategyFingerprint = ""
		input.Backend = ""
		input.BackendFingerprint = ""
		input.CapabilityFingerprint = ""
	}
	entry := OutcomeEntry{ProbeIdentity: target.ProbeIdentity, ServiceID: target.Probe.ServiceID, TargetContractRevision: target.Probe.TargetContractRevision, Transport: target.Probe.Transport, AddressFamily: family, StrategyID: input.StrategyID, StrategyFingerprint: input.StrategyFingerprint, Backend: input.Backend, BackendFingerprint: input.BackendFingerprint, CapabilityFingerprint: input.CapabilityFingerprint, DiagnosisID: input.Diagnosis.DiagnosisID, DiagnosisKind: input.Diagnosis.Kind, DiagnosisConfidence: input.Diagnosis.Confidence, EvidenceFingerprints: fingerprints, Outcome: input.Outcome, RecordedAt: input.RecordedAt.UTC(), ExpiresAt: input.RecordedAt.UTC().Add(policy.ttl(input.Outcome)), ContextKey: input.ContextKey}
	entry.EntryID, err = entryID(entry)
	if err != nil {
		return OutcomeEntry{}, err
	}
	return entry, nil
}

func persistedOutcome(outcome autotunevnext.Outcome) bool {
	switch outcome {
	case autotunevnext.OutcomeVerifiedFixed, autotunevnext.OutcomeStillFailing, autotunevnext.OutcomeRegressionObserved, autotunevnext.OutcomeDirectBecameReachable, autotunevnext.OutcomeInconclusive, autotunevnext.OutcomeLifecycleFailure:
		return true
	default:
		return false
	}
}
func requiresStrategy(outcome autotunevnext.Outcome) bool {
	return outcome != autotunevnext.OutcomeDirectBecameReachable
}

func outcomeCompatibleDiagnosis(outcome autotunevnext.Outcome, kind diagnosis.Kind) bool {
	switch outcome {
	case autotunevnext.OutcomeVerifiedFixed, autotunevnext.OutcomeStillFailing, autotunevnext.OutcomeRegressionObserved, autotunevnext.OutcomeDirectBecameReachable:
		return kind != diagnosis.KindNoAnomaly && kind != diagnosis.KindInsufficientEvidence && kind != diagnosis.KindUnknown
	default:
		return true
	}
}
func (policy Policy) ttl(outcome autotunevnext.Outcome) time.Duration {
	switch outcome {
	case autotunevnext.OutcomeVerifiedFixed:
		return policy.VerifiedFixedTTL
	case autotunevnext.OutcomeInconclusive:
		return policy.InconclusiveTTL
	case autotunevnext.OutcomeLifecycleFailure:
		return policy.LifecycleTTL
	default:
		return policy.NegativeTTL
	}
}

type MatchStatus string
type Reason string

const (
	MatchCompatible         MatchStatus = "COMPATIBLE"
	MatchStale              MatchStatus = "STALE"
	MatchIncompatible       MatchStatus = "INCOMPATIBLE"
	MatchExpired            MatchStatus = "EXPIRED"
	MatchInvalid            MatchStatus = "INVALID"
	ReasonExpired           Reason      = "EXPIRED"
	ReasonFuture            Reason      = "FUTURE_RECORDED_AT"
	ReasonInvalid           Reason      = "INVALID_HISTORY"
	ReasonInvalidated       Reason      = "INVALIDATED"
	ReasonProbeChanged      Reason      = "PROBE_CHANGED"
	ReasonContractChanged   Reason      = "TARGET_CONTRACT_CHANGED"
	ReasonServiceChanged    Reason      = "SERVICE_CHANGED"
	ReasonTransportChanged  Reason      = "TRANSPORT_CHANGED"
	ReasonFamilyChanged     Reason      = "ADDRESS_FAMILY_CHANGED"
	ReasonStrategyChanged   Reason      = "STRATEGY_CHANGED"
	ReasonBackendChanged    Reason      = "BACKEND_CHANGED"
	ReasonCapabilityChanged Reason      = "CAPABILITY_CHANGED"
	ReasonCapabilityMissing Reason      = "CAPABILITY_IDENTITY_REQUIRED_FOR_POSITIVE_REUSE"
	ReasonContextChanged    Reason      = "CONTEXT_CHANGED"
	ReasonContextMissing    Reason      = "CONTEXT_REQUIRED_FOR_POSITIVE_REUSE"
)

type Query struct {
	ProbeIdentity          string
	ServiceID              string
	TargetContractRevision string
	Transport              observatory.Transport
	AddressFamily          observatory.AddressFamily
	StrategyFingerprint    string
	Backend                backendcap.Backend
	BackendFingerprint     string
	CapabilityFingerprint  string
	ContextKey             string
	Now                    time.Time
}
type Match struct {
	Entry      OutcomeEntry
	Status     MatchStatus
	Confidence attribution.Confidence
	Reasons    []Reason
}

// MatchEntry supplies only compatibility evidence for future callers. It never
// returns an execution target, edge, recommendation, or Apply instruction.
func MatchEntry(entry OutcomeEntry, query Query) Match {
	match := Match{Entry: entry, Confidence: attribution.ConfidenceLow}
	if err := entry.Validate(); err != nil {
		match.Status = MatchInvalid
		match.Reasons = []Reason{ReasonInvalid}
		return match
	}
	if err := validateQuery(query); err != nil {
		match.Status = MatchInvalid
		match.Reasons = []Reason{ReasonInvalid}
		return match
	}
	match.Confidence = decayedConfidence(entry, query.Now)
	if query.Now.Before(entry.RecordedAt) {
		match.Status = MatchStale
		match.Reasons = []Reason{ReasonFuture}
		return match
	}
	if !entry.ExpiresAt.After(query.Now) {
		match.Status = MatchExpired
		match.Reasons = []Reason{ReasonExpired}
		return match
	}
	if entry.InvalidatedAt != nil {
		match.Status = MatchStale
		match.Reasons = []Reason{ReasonInvalidated}
		return match
	}
	for _, check := range []struct {
		same   bool
		reason Reason
	}{{entry.ProbeIdentity == query.ProbeIdentity, ReasonProbeChanged}, {entry.ServiceID == query.ServiceID, ReasonServiceChanged}, {entry.TargetContractRevision == query.TargetContractRevision, ReasonContractChanged}, {entry.Transport == query.Transport, ReasonTransportChanged}, {entry.AddressFamily == query.AddressFamily, ReasonFamilyChanged}} {
		if !check.same {
			match.Status = MatchIncompatible
			match.Reasons = []Reason{check.reason}
			return match
		}
	}
	if entry.ContextKey != query.ContextKey {
		match.Status = MatchIncompatible
		match.Reasons = []Reason{ReasonContextChanged}
		return match
	}
	if entry.Outcome != autotunevnext.OutcomeDirectBecameReachable {
		if entry.StrategyFingerprint != query.StrategyFingerprint {
			match.Status = MatchIncompatible
			match.Reasons = []Reason{ReasonStrategyChanged}
			return match
		}
		if entry.Backend != query.Backend || (entry.BackendFingerprint != "" && entry.BackendFingerprint != query.BackendFingerprint) {
			match.Status = MatchIncompatible
			match.Reasons = []Reason{ReasonBackendChanged}
			return match
		}
		if entry.CapabilityFingerprint != "" && entry.CapabilityFingerprint != query.CapabilityFingerprint {
			match.Status = MatchIncompatible
			match.Reasons = []Reason{ReasonCapabilityChanged}
			return match
		}
	}
	if entry.Outcome == autotunevnext.OutcomeVerifiedFixed && entry.ContextKey == "" {
		match.Status = MatchStale
		match.Reasons = []Reason{ReasonContextMissing}
		return match
	}
	if entry.Outcome == autotunevnext.OutcomeVerifiedFixed && entry.CapabilityFingerprint == "" {
		match.Status = MatchStale
		match.Reasons = []Reason{ReasonCapabilityMissing}
		return match
	}
	match.Status = MatchCompatible
	return match
}

func validateQuery(query Query) error {
	if query.Now.IsZero() || query.ProbeIdentity == "" || query.ServiceID == "" || query.TargetContractRevision == "" || !validTransport(query.Transport) || !validFamily(query.AddressFamily) {
		return fmt.Errorf("query has missing required identity")
	}
	if err := safeContextKey(query.ContextKey); err != nil {
		return err
	}
	if query.StrategyFingerprint != "" {
		if err := safeStrategyFingerprint(query.StrategyFingerprint); err != nil {
			return err
		}
	}
	if err := safeOpaqueFingerprint("backend", query.BackendFingerprint); err != nil {
		return err
	}
	return safeOpaqueFingerprint("capability", query.CapabilityFingerprint)
}

func QueryLedger(ledger Ledger, query Query) []Match {
	if err := ledger.Validate(); err != nil {
		return nil
	}
	out := make([]Match, 0, len(ledger.Entries))
	for _, entry := range ledger.Entries {
		out = append(out, MatchEntry(entry, query))
	}
	sort.Slice(out, func(i, j int) bool {
		if !out[i].Entry.RecordedAt.Equal(out[j].Entry.RecordedAt) {
			return out[i].Entry.RecordedAt.After(out[j].Entry.RecordedAt)
		}
		return out[i].Entry.EntryID < out[j].Entry.EntryID
	})
	return out
}

func decayedConfidence(entry OutcomeEntry, now time.Time) attribution.Confidence {
	if now.IsZero() || now.Before(entry.RecordedAt) || !entry.ExpiresAt.After(now) {
		return attribution.ConfidenceLow
	}
	base := entry.DiagnosisConfidence
	lifetime := entry.ExpiresAt.Sub(entry.RecordedAt)
	age := now.Sub(entry.RecordedAt)
	if age < lifetime/3 {
		return base
	}
	if age < 2*lifetime/3 {
		if base == attribution.ConfidenceHigh {
			return attribution.ConfidenceMedium
		}
		return attribution.ConfidenceLow
	}
	return attribution.ConfidenceLow
}

// Add retains contradictory evidence but marks older reusable positive entries
// stale. It then applies deterministic expired-first bounded compaction.
func Add(ledger Ledger, entry OutcomeEntry, policy Policy, now time.Time) (Ledger, error) {
	if err := entry.Validate(); err != nil {
		return Ledger{}, err
	}
	if now.IsZero() {
		return Ledger{}, fmt.Errorf("compaction time is required")
	}
	if len(ledger.Entries) == 0 && ledger.Fingerprint == "" && (ledger.SchemaVersion == 0 || ledger.SchemaVersion == SchemaVersion) {
		ledger.SchemaVersion = SchemaVersion
	} else if err := ledger.Validate(); err != nil {
		return Ledger{}, fmt.Errorf("invalid source ledger: %w", err)
	}
	entries := append([]OutcomeEntry(nil), ledger.Entries...)
	for index := range entries {
		if entries[index].EntryID == entry.EntryID {
			return compactEntries(entries, policy, now)
		}
	}
	for index := range entries {
		if contradicts(entries[index], entry) {
			at := entry.RecordedAt.UTC()
			entries[index].InvalidatedAt = &at
			entries[index].InvalidationReason = InvalidationContradicted
		}
	}
	entries = append(entries, entry)
	return compactEntries(entries, policy, now)
}

func contradicts(old, newer OutcomeEntry) bool {
	if old.Outcome != autotunevnext.OutcomeVerifiedFixed || !newerEvent(old, newer) {
		return false
	}
	if newer.Outcome == autotunevnext.OutcomeDirectBecameReachable {
		return sameTargetScope(old, newer)
	}
	return (newer.Outcome == autotunevnext.OutcomeRegressionObserved || newer.Outcome == autotunevnext.OutcomeStillFailing) && sameStrategyScope(old, newer)
}

func newerEvent(old, newer OutcomeEntry) bool {
	return newer.RecordedAt.After(old.RecordedAt) || newer.RecordedAt.Equal(old.RecordedAt) && newer.EntryID > old.EntryID
}

func sameTargetScope(left, right OutcomeEntry) bool {
	return left.ProbeIdentity == right.ProbeIdentity && left.ServiceID == right.ServiceID && left.TargetContractRevision == right.TargetContractRevision && left.Transport == right.Transport && left.AddressFamily == right.AddressFamily && left.ContextKey == right.ContextKey
}

func sameStrategyScope(left, right OutcomeEntry) bool {
	return sameTargetScope(left, right) && left.StrategyFingerprint == right.StrategyFingerprint && left.Backend == right.Backend && left.BackendFingerprint == right.BackendFingerprint && left.CapabilityFingerprint == right.CapabilityFingerprint
}

func Invalidate(ledger Ledger, entryID string, reason InvalidationReason, at time.Time) (Ledger, error) {
	if err := ledger.Validate(); err != nil {
		return Ledger{}, fmt.Errorf("invalid source ledger: %w", err)
	}
	if entryID == "" || !validInvalidationReason(reason) || at.IsZero() {
		return Ledger{}, fmt.Errorf("valid entry ID, invalidation reason, and time are required")
	}
	copy := Ledger{SchemaVersion: ledger.SchemaVersion, Entries: append([]OutcomeEntry(nil), ledger.Entries...)}
	for index := range copy.Entries {
		if copy.Entries[index].EntryID != entryID {
			continue
		}
		value := at.UTC()
		if value.Before(copy.Entries[index].RecordedAt) || copy.Entries[index].InvalidatedAt != nil && value.Before(*copy.Entries[index].InvalidatedAt) {
			return Ledger{}, fmt.Errorf("invalidation time precedes recorded history")
		}
		copy.Entries[index].InvalidatedAt = &value
		copy.Entries[index].InvalidationReason = reason
		return finalize(copy)
	}
	return Ledger{}, fmt.Errorf("outcome entry %q not found", entryID)
}

func validInvalidationReason(reason InvalidationReason) bool {
	return reason == InvalidationExplicit || reason == InvalidationContradicted || reason == InvalidationContextChanged
}

func Compact(ledger Ledger, policy Policy, now time.Time) (Ledger, error) {
	if err := ledger.Validate(); err != nil {
		return Ledger{}, fmt.Errorf("invalid source ledger: %w", err)
	}
	return compactEntries(ledger.Entries, policy, now)
}

func compactEntries(entries []OutcomeEntry, policy Policy, now time.Time) (Ledger, error) {
	policy, err := policy.normalized()
	if err != nil {
		return Ledger{}, err
	}
	if now.IsZero() {
		return Ledger{}, fmt.Errorf("compaction time is required")
	}
	entries = append([]OutcomeEntry(nil), entries...)
	sort.Slice(entries, func(i, j int) bool {
		left, right := evictionRank(entries[i], now), evictionRank(entries[j], now)
		if left != right {
			return left < right
		}
		if !entries[i].RecordedAt.Equal(entries[j].RecordedAt) {
			return entries[i].RecordedAt.Before(entries[j].RecordedAt)
		}
		return entries[i].EntryID < entries[j].EntryID
	})
	if len(entries) > policy.MaxEntries {
		entries = entries[len(entries)-policy.MaxEntries:]
	}
	return finalize(Ledger{SchemaVersion: SchemaVersion, Entries: entries})
}
func evictionRank(entry OutcomeEntry, now time.Time) int {
	if !entry.ExpiresAt.After(now) {
		return 0
	}
	if entry.InvalidatedAt != nil {
		return 1
	}
	if entry.Outcome == autotunevnext.OutcomeInconclusive || entry.Outcome == autotunevnext.OutcomeLifecycleFailure {
		return 2
	}
	if entry.Outcome != autotunevnext.OutcomeVerifiedFixed {
		return 3
	}
	return 4
}

func (entry OutcomeEntry) Validate() error {
	if entry.EntryID == "" || entry.ProbeIdentity == "" || entry.ServiceID == "" || entry.TargetContractRevision == "" || entry.Transport == "" || entry.AddressFamily == "" || entry.DiagnosisID == "" || entry.DiagnosisKind == "" || entry.DiagnosisConfidence == "" || len(entry.EvidenceFingerprints) == 0 || !persistedOutcome(entry.Outcome) || entry.RecordedAt.IsZero() || entry.ExpiresAt.IsZero() || !entry.ExpiresAt.After(entry.RecordedAt) {
		return fmt.Errorf("outcome entry has missing or invalid required fields")
	}
	if !validTransport(entry.Transport) || !validFamily(entry.AddressFamily) || !validDiagnosisKind(entry.DiagnosisKind) || !outcomeCompatibleDiagnosis(entry.Outcome, entry.DiagnosisKind) {
		return fmt.Errorf("outcome entry has unsupported or inconsistent diagnosis identity")
	}
	if requiresStrategy(entry.Outcome) && (entry.StrategyID == "" || entry.StrategyFingerprint == "" || entry.Backend == "") {
		return fmt.Errorf("executed outcome lacks strategy or backend identity")
	}
	if entry.Outcome == autotunevnext.OutcomeDirectBecameReachable && (entry.StrategyID != "" || entry.StrategyFingerprint != "" || entry.Backend != "" || entry.BackendFingerprint != "" || entry.CapabilityFingerprint != "") {
		return fmt.Errorf("direct reachability must not carry strategy or backend identity")
	}
	if entry.DiagnosisConfidence != attribution.ConfidenceLow && entry.DiagnosisConfidence != attribution.ConfidenceMedium && entry.DiagnosisConfidence != attribution.ConfidenceHigh {
		return fmt.Errorf("invalid diagnosis confidence")
	}
	if entry.InvalidatedAt != nil && (entry.InvalidatedAt.Before(entry.RecordedAt) || !validInvalidationReason(entry.InvalidationReason)) {
		return fmt.Errorf("invalid outcome invalidation metadata")
	}
	if entry.InvalidatedAt == nil && entry.InvalidationReason != "" {
		return fmt.Errorf("outcome invalidation reason lacks timestamp")
	}
	if err := safeContextKey(entry.ContextKey); err != nil {
		return err
	}
	if entry.StrategyFingerprint != "" {
		if err := safeStrategyFingerprint(entry.StrategyFingerprint); err != nil {
			return err
		}
	}
	if err := safeOpaqueFingerprint("backend", entry.BackendFingerprint); err != nil {
		return err
	}
	if err := safeOpaqueFingerprint("capability", entry.CapabilityFingerprint); err != nil {
		return err
	}
	if err := safePrefixedHash("diagnosis", entry.DiagnosisID); err != nil {
		return err
	}
	for _, fingerprint := range entry.EvidenceFingerprints {
		if err := safePrefixedHash("evidence", fingerprint); err != nil {
			return err
		}
	}
	if !sameStrings(entry.EvidenceFingerprints, sortedStrings(entry.EvidenceFingerprints)) {
		return fmt.Errorf("evidence fingerprints are not canonical")
	}
	id, err := entryID(entry)
	if err != nil {
		return err
	}
	if entry.EntryID != id {
		return fmt.Errorf("outcome entry ID does not match canonical event")
	}
	return nil
}

func validTransport(value observatory.Transport) bool {
	return value == observatory.TransportTCP || value == observatory.TransportQUIC || value == observatory.TransportUDP
}

func validFamily(value observatory.AddressFamily) bool {
	return value == observatory.AddressFamilyIPv4 || value == observatory.AddressFamilyIPv6
}

func validDiagnosisKind(value diagnosis.Kind) bool {
	switch value {
	case diagnosis.KindNoAnomaly, diagnosis.KindDNSPathAnomaly, diagnosis.KindTCPPathFailure, diagnosis.KindTLSHandshakePathFailure, diagnosis.KindHTTPApplicationFailure, diagnosis.KindPartialTransferAnomaly, diagnosis.KindEdgeDependentFailure, diagnosis.KindNetworkContextFailure, diagnosis.KindInsufficientEvidence, diagnosis.KindUnknown:
		return true
	default:
		return false
	}
}

func (ledger Ledger) Validate() error {
	if ledger.SchemaVersion != SchemaVersion {
		return fmt.Errorf("unsupported ledger schema version %d", ledger.SchemaVersion)
	}
	seen := make(map[string]struct{}, len(ledger.Entries))
	for index, entry := range ledger.Entries {
		if err := entry.Validate(); err != nil {
			return err
		}
		if _, exists := seen[entry.EntryID]; exists {
			return fmt.Errorf("duplicate outcome entry ID %q", entry.EntryID)
		}
		seen[entry.EntryID] = struct{}{}
		if index > 0 && entryOrder(ledger.Entries[index], ledger.Entries[index-1]) {
			return fmt.Errorf("ledger entries are not canonical")
		}
	}
	fingerprint, err := ledgerFingerprint(ledger)
	if err != nil {
		return err
	}
	if ledger.Fingerprint != fingerprint {
		return fmt.Errorf("ledger fingerprint does not match canonical content")
	}
	return nil
}

func entryOrder(left, right OutcomeEntry) bool {
	return left.RecordedAt.Before(right.RecordedAt) || left.RecordedAt.Equal(right.RecordedAt) && left.EntryID < right.EntryID
}

func finalize(ledger Ledger) (Ledger, error) {
	ledger.SchemaVersion = SchemaVersion
	ledger.Entries = append([]OutcomeEntry(nil), ledger.Entries...)
	sort.Slice(ledger.Entries, func(i, j int) bool { return entryOrder(ledger.Entries[i], ledger.Entries[j]) })
	fingerprint, err := ledgerFingerprint(ledger)
	if err != nil {
		return Ledger{}, err
	}
	ledger.Fingerprint = fingerprint
	if err := ledger.Validate(); err != nil {
		return Ledger{}, err
	}
	return ledger, nil
}

func entryID(entry OutcomeEntry) (string, error) {
	copy := entry
	copy.EntryID = ""
	copy.InvalidatedAt = nil
	copy.InvalidationReason = ""
	data, err := json.Marshal(copy)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(data)
	return "outcome-v1-" + hex.EncodeToString(sum[:]), nil
}

func ledgerFingerprint(ledger Ledger) (string, error) {
	copy := ledger
	copy.Fingerprint = ""
	data, err := json.Marshal(copy)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(data)
	return "ledger-v1-" + hex.EncodeToString(sum[:]), nil
}

func sortedStrings(values []string) []string {
	out := append([]string(nil), values...)
	sort.Strings(out)
	result := out[:0]
	for _, value := range out {
		if value == "" || len(result) > 0 && result[len(result)-1] == value {
			continue
		}
		result = append(result, value)
	}
	return result
}

func sameStrings(left, right []string) bool {
	if len(left) != len(right) {
		return false
	}
	for index := range left {
		if left[index] != right[index] {
			return false
		}
	}
	return true
}

func safeStrategyFingerprint(value string) error {
	if len(value) != 64 {
		return fmt.Errorf("invalid strategy fingerprint")
	}
	for _, char := range value {
		if !strings.ContainsRune("0123456789abcdef", char) {
			return fmt.Errorf("invalid strategy fingerprint")
		}
	}
	return nil
}

func safePrefixedHash(kind, value string) error {
	prefix := kind + "-v1-"
	if len(value) != len(prefix)+64 || !strings.HasPrefix(value, prefix) {
		return fmt.Errorf("invalid %s fingerprint", kind)
	}
	for _, char := range strings.TrimPrefix(value, prefix) {
		if !strings.ContainsRune("0123456789abcdef", char) {
			return fmt.Errorf("invalid %s fingerprint", kind)
		}
	}
	return nil
}

func safeOpaqueFingerprint(kind, value string) error {
	if value == "" {
		return nil
	}
	if len(value) > 128 || strings.TrimSpace(value) != value || strings.ContainsAny(value, " \r\n\t") {
		return fmt.Errorf("%s fingerprint must be a bounded opaque identifier", kind)
	}
	return nil
}

func safeContextKey(value string) error {
	if value == "" {
		return nil
	}
	return safePrefixedHash("context", value)
}

type LoadState string

const (
	LoadEmpty              LoadState = "EMPTY"
	LoadValid              LoadState = "VALID"
	LoadCorrupt            LoadState = "CORRUPT"
	LoadUnsupportedVersion LoadState = "UNSUPPORTED_VERSION"
)

type LoadResult struct {
	State  LoadState
	Ledger Ledger
}

func Load(path string) LoadResult {
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return LoadResult{State: LoadEmpty, Ledger: Ledger{SchemaVersion: SchemaVersion}}
	}
	if err != nil {
		return LoadResult{State: LoadCorrupt}
	}
	var ledger Ledger
	if err := json.Unmarshal(data, &ledger); err != nil {
		return LoadResult{State: LoadCorrupt}
	}
	if ledger.SchemaVersion != SchemaVersion {
		return LoadResult{State: LoadUnsupportedVersion}
	}
	if err := ledger.Validate(); err != nil {
		return LoadResult{State: LoadCorrupt}
	}
	return LoadResult{State: LoadValid, Ledger: ledger}
}

// Save atomically persists only validated canonical ledger data. It never reads
// or modifies managed activation intent files.
func Save(path string, ledger Ledger) error {
	if err := ledger.Validate(); err != nil {
		return err
	}
	directory := filepath.Dir(path)
	if err := os.MkdirAll(directory, 0700); err != nil {
		return err
	}
	data, err := json.Marshal(ledger)
	if err != nil {
		return err
	}
	temporary, err := os.CreateTemp(directory, ".autotune-vnext-outcomes-*.tmp")
	if err != nil {
		return err
	}
	name := temporary.Name()
	defer os.Remove(name)
	if err := temporary.Chmod(0600); err != nil {
		temporary.Close()
		return err
	}
	if _, err := temporary.Write(data); err != nil {
		temporary.Close()
		return err
	}
	if err := temporary.Sync(); err != nil {
		temporary.Close()
		return err
	}
	if err := temporary.Close(); err != nil {
		return err
	}
	return os.Rename(name, path)
}
