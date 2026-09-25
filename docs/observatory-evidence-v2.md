# AutoTune vNext V2.1 Observatory evidence

V2.1 adds a versioned observation contract. It collects facts for later stages; it does not diagnose a failure, rank strategies, learn across sessions, or activate an executor.

## ProbeSpec

`observatory.ProbeSpec` defines one bounded measurement:

- `schema_version`, stable probe `id`, service ID, and target-contract revision;
- a redacted HTTPS target, transport, and IPv4/IPv6/any policy;
- `HTTPS_GET` mode and the exact allowed HTTP status/path-completion semantics;
- primary-target, healthy-control, or neutral-control role; and
- optional audited `TransferContract` identity, revision, and positive expected byte count. `AuditID` is trusted configuration metadata, not cryptographic proof that an endpoint was audited.

`ProbeSpec.Identity()` canonically fingerprints every semantic field, including the target-contract revision. Unknown schema versions are rejected. TCP is implemented through the existing direct TCP/TLS/HTTP observer. QUIC and UDP remain representable contracts but the observer records their measurement as `UNSUPPORTED`; V2.1 does not claim a QUIC or UDP handshake observation.

## EvidenceRecord

`BuildEvidenceRecord` wraps one or more existing `ObservationResult` values. V1 consumers keep using `ObservationResult` unchanged. Runs are a canonical chronological sequence: `StartedAt`, then `FinishedAt`, then RunID and canonical observation content break ties. Each redacted evidence run preserves the factual selected edge, actual address family, execution context, timestamps, protocol stages, and a typed measurement status:

- `COMPLETED`
- `PARTIAL`
- `FAILED`
- `UNSUPPORTED`
- `NOT_REQUESTED`

Completed HTTP paths separately state whether the observed response matches the `ProbeSpec` expectation. This is evidence only, not a censorship conclusion.

Optional transfer facts use the same explicit status and carry expected/actual bytes, completion, first-byte time, and total time. A completed transfer must exactly satisfy its audited byte contract. V2.1 never requests arbitrary padded bodies. The normal direct observer reports an audited transfer as `UNSUPPORTED` until a controlled audited transfer reader exists; a probe without a transfer contract reports `NOT_REQUESTED`.

## Conservative semantics and privacy

DNS evidence can record resolver failure, no usable answer, resolver-selected edge, and actual family. It does not encode `DNS_INTERCEPTION`. TLS evidence can record TCP connection, ClientHello emission, handshake/certificate facts, and normalized failures. It does not encode `TLS_FINGERPRINT_FILTERING`.

`EvidenceRecord` serialization rejects unsafe data. It removes URL userinfo/query/fragment, target display names, local interface/address/gateway/resolver/network-label identifiers, attempt local addresses, stage details, redirects, and headers except bounded `content-type`. The target hostname, port, and path remain intentionally as **sanitized**, not anonymized, probe identity needed to interpret the evidence. It never contains raw response bodies, cookies, authorization headers, arbitrary request payloads, packet dumps, or certificate private material. `MarshalEvidenceRecord`, `ParseEvidenceRecord`, and `CanonicalJSON` validate schema, chronological run order, probe identity, and a stable SHA-256-based evidence-event fingerprint. The fingerprint identifies this serialized evidence event; it is not a cross-session outcome identity.

## Evidence limits

`HERMETIC_CORRECTNESS_EVIDENCE != REAL_WORLD_EFFECTIVENESS_EVIDENCE`.

V2.1 tests prove deterministic contract construction, redaction, serialization, unsupported-status handling, and V1 compatibility. They do not prove a DPI bypass works against a real natural failure. V2.2 will consume evidence for conservative diagnosis; V2.3 and V2.4 remain out of scope.
