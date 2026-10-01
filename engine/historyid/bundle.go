package historyid

import (
	"context"
	"errors"

	"unbound/engine"
	"unbound/engine/backendcap"
)

// Bundle is the coherent identity cohort for ONE run.
//
// One run must have one coherent cohort. The same ContextKey,
// BackendFingerprint and CapabilityFingerprint must be used for both the history
// query and the outcome persistence of that run; recomputing them midway, with
// the network having changed underneath, would write entries under a namespace
// the query never consulted.
//
// INDEPENDENT AVAILABILITY. Each identity carries its own error. A missing
// context key must never prevent a backend fingerprint, and neither must
// prevent a fresh measurement. Failure of any identity here makes history
// reuse more conservative and nothing else: a fresh AutoTune run does not
// consult this bundle for eligibility, execution, Apply, or discovery.
type Bundle struct {
	ContextKey              NetworkContextIdentity
	ContextErr              error
	BackendFingerprint      BackendFingerprint
	BackendErr              error
	CapabilityFingerprint   CapabilityFingerprint
	CapabilityErr           error
	ContextProviderOverride NetworkContextProvider
}

// ForRun assembles the identity cohort for a single run against one backend.
//
// No single failure aborts the bundle. Each identity is attempted
// independently so a partial cohort is normal and expected, not an error.
func ForRun(ctx context.Context, backend backendcap.Backend, assets *engine.AssetPaths, configDir string) Bundle {
	var bundle Bundle

	// Context identity. The key is local HMAC material and never leaves the
	// machine except as the opaque fingerprint. A test may supply its own
	// provider; it then owns its own key handling entirely.
	if bundle.ContextProviderOverride != nil {
		bundle.ContextKey, bundle.ContextErr = bundle.ContextProviderOverride.Identity(ctx)
	} else {
		key, keyErr := CachedContextKey(configDir)
		if keyErr != nil {
			bundle.ContextErr = keyErr
		} else {
			bundle.ContextKey, bundle.ContextErr = SystemNetworkContextProvider{Key: key}.Identity(ctx)
		}
	}

	// Backend implementation identity.
	bundle.BackendFingerprint, bundle.BackendErr = BackendFingerprintFor(backend, assets)

	// Capability identity, from the backend's advertised static capabilities.
	// This is deliberately independent of the current host: a machine that
	// cannot currently capture is a current-availability fact owned by Planner
	// and preflight, not a capability identity.
	bundle.CapabilityFingerprint, bundle.CapabilityErr = CapabilityFingerprintFor(backendcap.Get(backend))

	return bundle
}

// Limitations reports which identities are genuinely unavailable, so the
// product can report them dynamically instead of unconditionally.
//
// A limitation is reported for an identity that is BOTH unavailable AND
// required for a safe reuse decision. Reporting "context unavailable" when the
// context producer succeeded would be a false statement about the product.
func (b Bundle) Limitations() []string {
	limitations := make([]string, 0, 3)
	if b.ContextErr != nil && !b.ContextKey.Available() {
		limitations = append(limitations, "CONTEXT_IDENTITY_UNAVAILABLE")
	}
	if b.BackendErr != nil && !b.BackendFingerprint.Available() {
		limitations = append(limitations, "BACKEND_IDENTITY_UNAVAILABLE")
	}
	if b.CapabilityErr != nil && !b.CapabilityFingerprint.Available() {
		limitations = append(limitations, "CAPABILITY_IDENTITY_UNAVAILABLE")
	}
	return limitations
}

// IsUnavailable reports whether the error represents a factual unavailability
// rather than, say, a cancelled context.
func IsUnavailable(err error) bool {
	var unavailable *ErrUnavailable
	return errors.As(err, &unavailable)
}
