// Package historyid produces the three opaque identities that gate whether a
// historical AutoTune outcome may be reused: a local network context key, a
// backend implementation fingerprint, and a backend capability fingerprint.
//
// FROZEN PRINCIPLE. History is not eligibility. History is not a verified fix.
// History is not permission to apply anything, not a saved edge, and not graph
// discovery. These identities exist only so that a historical positive outcome
// is reused *less* often, never more often, than a current measurement would
// justify. CURRENT_VALIDATION > HISTORICAL_SUCCESS always holds.
//
// PRIVACY. Nothing in this package ever returns, logs, or persists raw network
// facts. Local network characteristics are read into memory, canonicalized, and
// immediately reduced through a locally keyed HMAC. Only the resulting opaque
// fingerprint leaves this package.
//
// NON-AUTHORITY. Nothing in this package executes, captures, mutates a
// machine, or grants permission. It is pure read-only inspection plus hashing.
package historyid

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"sort"
	"strings"
)

// Format prefixes. The version is part of the persisted value so a future
// derivation can never be silently confused with a current one.
const (
	contextPrefix     = "context-v1-"
	backendPrefix     = "backend-v1-"
	capabilityPrefix  = "capability-v1-"
	hexDigestLength   = 64
	executionContract = "strategyir-execution-v1"
)

// ErrUnavailable is returned when an identity cannot be produced factually. It
// is never replaced with a fabricated value: a caller that receives it must
// treat the identity as absent, and history reuse becomes more conservative.
type ErrUnavailable struct {
	Identity string
	Reason   string
}

func (e *ErrUnavailable) Error() string {
	return fmt.Sprintf("%s identity unavailable: %s", e.Identity, e.Reason)
}

// Identity values are opaque versioned fingerprints. They are the ONLY form in
// which any of this data may be persisted or shown to a user.
type (
	// NetworkContextIdentity answers exactly one question: does this historical
	// result appear to come from the same relevant local network context?
	//
	// It does NOT assert the same ISP, the same censorship mechanism, the same
	// public IP, the same DPI equipment, the same remote path, or the same
	// physical location. It is a local opaque namespace marker.
	NetworkContextIdentity string

	// BackendFingerprint answers: is this materially the same implementation and
	// runtime identity responsible for packet execution?
	BackendFingerprint string

	// CapabilityFingerprint answers: does this backend advertise the same
	// semantic ability set?
	CapabilityFingerprint string
)

// String satisfies fmt.Stringer without leaking anything beyond the opaque value.
func (n NetworkContextIdentity) String() string { return string(n) }
func (b BackendFingerprint) String() string     { return string(b) }
func (c CapabilityFingerprint) String() string  { return string(c) }

// Available reports whether the identity was actually produced.
func (n NetworkContextIdentity) Available() bool { return validOpaque(contextPrefix, string(n)) }
func (b BackendFingerprint) Available() bool     { return validOpaque(backendPrefix, string(b)) }
func (c CapabilityFingerprint) Available() bool  { return validOpaque(capabilityPrefix, string(c)) }

func validOpaque(prefix, value string) bool {
	rest, ok := strings.CutPrefix(value, prefix)
	if !ok || len(rest) != hexDigestLength {
		return false
	}
	for _, c := range rest {
		if (c < '0' || c > '9') && (c < 'a' || c > 'f') {
			return false
		}
	}
	return true
}

func formatIdentity(prefix string, sum []byte) string {
	return prefix + hex.EncodeToString(sum[:])
}

// contextSource is the canonicalized, in-memory-only description of the local
// network context. Every field is a SORTED set, because the order in which an
// OS enumerates interfaces, gateways or resolvers is not semantic and must not
// change the identity.
//
// A nil slice and an empty slice mean the same thing here: the set is absent.
// Absent is represented explicitly in the canonical form as an empty value, so
// "no gateway" and "field omitted" can never be confused.
type contextSource struct {
	Platform         string
	DefaultInterface []string
	Gateway          []string
	Resolver         []string
	Profile          string
}

// canonical renders the source material into a stable byte string. It is
// deliberately a hand-written, line-oriented, fully ordered format rather than
// JSON: there are no struct tags to drift, no omitempty ambiguity, and no
// possibility of a map iteration order leaking into the digest.
//
// The output contains raw local network facts and MUST NOT be persisted,
// logged, or shown to a user. It exists only as the HMAC input.
func (c contextSource) canonical() []byte {
	var b strings.Builder
	b.WriteString("context-source-v1\n")
	writeSet := func(name string, values []string) {
		b.WriteString(name)
		b.WriteByte('=')
		sorted := sortedUnique(values)
		if len(sorted) == 0 {
			// Explicit absent marker. Never omit the field entirely.
			b.WriteString("<none>")
		} else {
			b.WriteString(strings.Join(sorted, ","))
		}
		b.WriteByte('\n')
	}
	b.WriteString("platform=" + c.Platform + "\n")
	writeSet("default_interface", c.DefaultInterface)
	writeSet("gateway", c.Gateway)
	writeSet("resolver", c.Resolver)
	b.WriteString("profile=" + c.Profile + "\n")
	return []byte(b.String())
}

// sufficient reports whether these facts are strong enough to identify a
// context. A default-route interface set is the minimum: without it there is
// no evidence of an effective local network at all, and fabricating an identity
// from weaker or empty information would be exactly the universal-fallback
// behaviour this package must never have.
func (c contextSource) sufficient() bool {
	return len(sortedUnique(c.DefaultInterface)) > 0
}

func sortedUnique(values []string) []string {
	out := append([]string(nil), values...)
	sort.Strings(out)
	filtered := out[:0]
	for i, v := range out {
		if i == 0 || v != out[i-1] {
			filtered = append(filtered, v)
		}
	}
	return filtered
}

// ContextKey derives the opaque local network context identity.
//
// The derivation is a keyed HMAC-SHA256 over the canonicalized source
// material. A plain SHA-256 would not be privacy-safe: common SSIDs, gateway
// addresses and resolver addresses are dictionary-guessable, so an unkeyed
// digest of them can be brute-forced offline by anyone holding the ledger. The
// local random key is what makes the value non-guessable, which is why key
// rotation safely invalidates old identities rather than needing to migrate
// them.
func ContextKey(key []byte, source contextSource) (NetworkContextIdentity, error) {
	if len(key) < 32 {
		return "", &ErrUnavailable{Identity: "context", Reason: "local context key is missing or too short"}
	}
	if !source.sufficient() {
		return "", &ErrUnavailable{Identity: "context", Reason: "insufficient local network facts"}
	}
	mac := hmac.New(sha256.New, key)
	mac.Write(source.canonical())
	return NetworkContextIdentity(formatIdentity(contextPrefix, mac.Sum(nil))), nil
}

// ExecutionContractRevision is the explicit, versioned contract of the
// StrategyIR compiler and renderer whose output the engine binary executes.
//
// Backend behaviour is not only the upstream binary: a compiler or renderer
// change can alter executed semantics while the engine binary is byte-identical.
// Pinning the contract here means a future semantic change must consciously
// bump this revision, instead of silently leaving old VERIFIED_FIXED results
// looking compatible.
//
// This is a constant. Source files are never hashed at runtime.
const ExecutionContractRevision = executionContract
