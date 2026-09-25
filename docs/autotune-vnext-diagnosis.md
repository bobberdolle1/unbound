# AutoTune vNext V2.2 diagnosis

V2.2 adds a pure layer above V2.1 evidence and V1 attribution:

- **Observatory** records redacted protocol facts.
- **Attribution** derives conservative low-level findings from bounded observations.
- **Diagnosis** validates their correlation and emits one normalized product-level failure class.

`diagnosis.Diagnose` performs no I/O, executor control, persistence, ranking, or activation. Its report ID and effective time derive from canonical evidence and attribution input.

Precedence is deterministic: unsupported transport, network-context finding, audited repeated partial transfer with completed controls, HTTP finding, edge finding, DNS/TCP/TLS finding, no anomaly, insufficient evidence, then unknown. Diagnosis never raises attribution confidence.

Terminology is intentionally limited:

- `DNS_PATH_ANOMALY` is not `DNS_INTERCEPTION`.
- `TLS_HANDSHAKE_PATH_FAILURE` is not `TLS_FINGERPRINT_FILTERING`.
- `PARTIAL_TRANSFER_ANOMALY` is not proven throttling.
- unsupported QUIC or UDP is `INSUFFICIENT_EVIDENCE`, not a QUIC/UDP path failure.

`UNKNOWN` means valid, correlated evidence had no justified deterministic rule. `INSUFFICIENT_EVIDENCE` means a required prerequisite is explicitly absent or unsupported. V2.2 does not add adaptive memory, planner feedback, automatic Apply, or a real QUIC/UDP/transfer observer.
