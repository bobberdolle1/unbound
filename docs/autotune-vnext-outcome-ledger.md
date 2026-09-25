# AutoTune vNext V2.3 outcome ledger

V2.3 provides `engine/outcomeledger`: local, versioned, bounded history of redacted, evidence-linked experiment outcomes. It is separate from `autotune_vnext_state.json`, which remains managed activation **intent** only.

The ledger records only validated effectiveness outcomes: `VERIFIED_FIXED`, `STILL_FAILING`, `REGRESSION_OBSERVED`, `DIRECT_BECAME_REACHABLE`, `INCONCLUSIVE`, and `LIFECYCLE_FAILURE`. It rejects capability, policy, and not-run states as effectiveness evidence.

Each entry binds canonical evidence fingerprints, a V2.2 diagnosis ID, probe/service/target-contract identity, transport, factual address family, and (when a strategy executed) strategy, backend, and capability fingerprints. Entry IDs and ledger fingerprints are deterministic hashes. The contract stores no target URL, hostname, edge, interface, gateway, resolver, local address, response body, credentials, or raw network label. Optional context keys must be trusted `context-v1-<sha256>` values.

`Policy` applies conservative outcome-specific TTLs and deterministic discrete confidence decay: diagnosis confidence is an upper bound; age can only reduce it. Expired, invalidated, drifted, mismatched, and context-unsafe entries are not compatible. A positive entry without a context key is never a strong cross-network reuse signal.

Contradictory later failures or direct reachability invalidate older compatible positive entries for reuse while retaining them until deterministic bounded eviction. Storage evicts expired, invalidated/old, and low-value evidence before fresh positive evidence. `Save` writes `autotune_vnext_outcomes.json` atomically with restrictive permissions; malformed, tampered, or future-schema files load as unavailable history and never produce reusable results.

Historical edge data is not stored. Current execution must always resolve and observe the current edge/family. V2.3's query API returns compatibility evidence only; it cannot rank a strategy, recommend activation, Apply anything, alter planner behavior, or skip current validation.

`HISTORICAL_VERIFIED_FIXED != CURRENT_VERIFIED_FIXED`.

`HISTORY != APPLY_PERMISSION`.

Future reuse requires a fresh observation, current edge/family discovery, current capability and planner checks, bounded candidate validation, direct-before/active/direct-after semantics, controls, and verified restoration. Explicit managed Revert still clears only managed intent; V2.3 exposes invalidation primitives but does not wire Revert to ledger mutation.
