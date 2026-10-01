package historyid

import (
	"crypto/sha256"
	"fmt"
	"strings"

	"unbound/engine"
	"unbound/engine/backendcap"
)

// BackendFingerprintFor derives the opaque backend implementation identity.
//
// It answers exactly one question: is this materially the same implementation
// and runtime identity responsible for packet execution? It is NOT a version
// check, NOT a licensing check, and NOT a statement that the backend is
// currently healthy.
//
// INPUTS ARE FACTS, NOT HINTS. The provenance triple comes from the pinned
// upstream bundle, and the two content identities come from assets that were
// verified byte-for-byte immediately before hashing. A missing input is
// reported as unavailable rather than replaced with a placeholder, because a
// fingerprint derived from absent data would claim a compatibility that was
// never established.
func BackendFingerprintFor(backend backendcap.Backend, assets *engine.AssetPaths) (BackendFingerprint, error) {
	return backendFingerprintFor(backend, assets, ExecutionContractRevision)
}

// backendFingerprintFor is the testable seam: the contract revision is an
// explicit parameter so a change to it can be observed without editing the
// source file, and the runtime asset resolver is indirected for the same reason.
func backendFingerprintFor(backend backendcap.Backend, assets *engine.AssetPaths, contractRevision string) (BackendFingerprint, error) {
	return backendFingerprintWith(backendcap.Get(backend), assets, contractRevision, engine.VerifiedRuntimeAssetIdentity)
}

func backendFingerprintWith(
	capabilities backendcap.Capabilities,
	assets *engine.AssetPaths,
	contractRevision string,
	resolveRuntimeAssets func(*engine.AssetPaths) (string, error),
) (BackendFingerprint, error) {
	if assets == nil {
		return "", &ErrUnavailable{
			Identity: "backend",
			Reason:   "engine runtime assets are not extracted, so no verified implementation identity exists",
		}
	}
	if strings.TrimSpace(assets.EngineSHA256) == "" {
		return "", &ErrUnavailable{
			Identity: "backend",
			Reason:   "the platform engine binary hash is unknown, so no verified implementation identity exists",
		}
	}
	if resolveRuntimeAssets == nil {
		return "", fmt.Errorf("backend identity: runtime asset resolver is not provided")
	}
	runtimeAssets, err := resolveRuntimeAssets(assets)
	if err != nil {
		return "", &ErrUnavailable{
			Identity: "backend",
			Reason:   "the verified runtime asset identity could not be established: " + err.Error(),
		}
	}
	if strings.TrimSpace(runtimeAssets) == "" {
		return "", &ErrUnavailable{
			Identity: "backend",
			Reason:   "the verified runtime asset identity is empty, so no verified implementation identity exists",
		}
	}
	canonical, err := backendCanonical(capabilities, assets.EngineSHA256, runtimeAssets, contractRevision)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(canonical)
	return BackendFingerprint(formatIdentity(backendPrefix, sum[:])), nil
}

// backendCanonical renders the backend implementation material into a stable
// byte string. It is hand written and line oriented for the same reasons as
// contextSource.canonical: no struct tags to drift, no omitempty ambiguity, and
// no possibility of a map iteration order leaking into the digest.
//
// It deliberately contains no filesystem path, no timestamp, no process id, no
// queue number and no other host state. The extraction root differs on every
// run and on every machine, so a path here would make the identity unstable;
// the assets themselves are already fully identified by their verified content
// hashes.
//
// It deliberately excludes the git master SHA, any build timestamp and the
// frontend version: none of them describe what executes packets, and all three
// would invalidate every historical result on a routine rebuild.
//
// An EMPTY PROVENANCE IS FACTUAL, NOT AN ERROR. The native backends are
// placeholders with no upstream bundle behind them, and that is a true property
// of the backend. Their identity still differs from a real engine backend
// because the backend ID, the engine hash and the runtime asset identity
// differ, so a placeholder can never be mistaken for a verified engine.
func backendCanonical(
	capabilities backendcap.Capabilities,
	engineSHA256 string,
	runtimeAssets string,
	contractRevision string,
) ([]byte, error) {
	provenance := capabilities.Provenance
	if err := lineEncodable("provenance_tag", provenance.Tag); err != nil {
		return nil, err
	}
	if err := lineEncodable("provenance_base_tag", provenance.BaseTag); err != nil {
		return nil, err
	}
	if err := lineEncodable("provenance_commit", provenance.Commit); err != nil {
		return nil, err
	}
	if err := lineEncodable("execution_contract", contractRevision); err != nil {
		return nil, err
	}

	var b strings.Builder
	b.WriteString("backend-source-v1\n")
	b.WriteString("backend=" + string(capabilities.Backend) + "\n")
	b.WriteString("provenance_tag=" + provenance.Tag + "\n")
	b.WriteString("provenance_base_tag=" + provenance.BaseTag + "\n")
	b.WriteString("provenance_commit=" + provenance.Commit + "\n")
	b.WriteString("engine_sha256=" + engineSHA256 + "\n")
	b.WriteString("runtime_assets=" + runtimeAssets + "\n")
	b.WriteString("execution_contract=" + contractRevision + "\n")
	return []byte(b.String()), nil
}

// lineEncodable rejects source material that cannot survive the line oriented
// canonical form. A value containing a line break would let two different
// backend identities render to identical bytes, which would silently merge
// unrelated implementations instead of failing closed.
func lineEncodable(name, value string) error {
	if strings.ContainsAny(value, "\r\n") {
		return fmt.Errorf("backend identity field %s contains a line break", name)
	}
	return nil
}
