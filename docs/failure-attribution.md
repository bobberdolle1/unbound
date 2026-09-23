# Failure Attribution v1

Failure Attribution consumes persisted `observatory.ObservationResult` values and emits schema-versioned, deterministic analysis. It is a pure Go package at `engine/attribution`; its only product dependency is `engine/observatory`.

It performs no network requests and never starts or stops profiles or bypass engines. It does not modify DNS, firewall state, routes, proxies, Observatory records, Strategy Lab, or AutoTune.

## Facts and hypotheses

Observatory remains the factual measurement layer. Examples are `DNS_FAILURE`, `TCP_CONNECT_TIMEOUT`, `TLS_HANDSHAKE_TIMEOUT`, `HTTP_STATUS`, `QUIC_UNSUPPORTED`, an HTTP status, and per-attempt stage status.

Attribution reports facts as `Finding.kind = FACT` and path interpretations as `Finding.kind = INFERENCE`. Mechanism-level findings always use conservative vocabulary such as `TLS_PATH_FAILURE_SUSPECTED`; they do not prove censorship or operator intent. V1 never emits `DPI_BLOCK`, `DPI_CONFIRMED`, `SNI_BLOCK`, `RST_INJECTED`, `ALLOWLIST_MODE`, `TSPU_BLOCK`, or `VPN_DETECTED`.

`TLS_PATH_FAILURE_SUSPECTED` means the observed TLS path did not complete after TCP and ClientHello evidence. It cannot distinguish DPI, a remote edge, routing, a middlebox, local security software, or provider policy.

## Report schema

`AttributionReport.schema_version` is `1`. It contains a deterministic `attribution_id`, a creation timestamp derived from the latest input observation completion time, input run IDs, a privacy-minimized target reference, optional network-context key, primary finding, findings, evidence references, counterevidence, limitations, confidence, and applicability.

Evidence references contain only `run_id`, attempt index, and stage. Reports do not copy response bodies, cookies, authorization values, URL queries, userinfo, or native error strings.

## Confidence rules

Confidence is rule-derived; there are no numeric probabilities.

- **LOW** — one relevant target failure, a missing required prerequisite, contradictory completed-path evidence, or a control cohort that does not establish a healthy comparison.
- **MEDIUM** — the same target boundary repeats across at least two attempts or edges and at least two compatible control attempts complete valid HTTP paths. It is still a suspected path anomaly, not a mechanism claim.
- **HIGH** — direct factual observations such as completed HTTP, an observed reset, or a controlled A/B differential label. HIGH is not used merely because a timeout repeated.

A valid HTTP response is path-complete. HTTP 4xx/5xx produces the factual `HTTP_APPLICATION_FAILURE_OBSERVED`; it is never reclassified as TLS or transport failure. Redirects are successful path evidence.

## Structural applicability

A finding never credits or blames a mechanism that cannot affect its failed boundary:

- DNS failure cannot be attributed to TLS/ClientHello handling.
- TCP connection failure occurs before ClientHello handling.
- TLS handshake failure cannot be classified using HTTP status.
- `QUIC_UNSUPPORTED` produces `INSUFFICIENT_EVIDENCE`; V1 makes no QUIC-path inference.
- An observed reset produces `TCP_RESET_OBSERVED` or `TLS_RESET_OBSERVED`, never `RST_INJECTED`.

`CouldStrategyAffectFailure(StrategyCapabilities, boundary, transport)` is an optional pure metadata helper for future StrategyIR work. A capability declares affected stages (`DNS`, `CONNECT`, `HELLO`, `HANDSHAKE`, `HTTP`, `QUIC`) and supported transports. The helper only answers structural possibility; it does not score, choose, or run a strategy. For example, a TCP ClientHello capability can affect HELLO/HANDSHAKE but not DNS or QUIC. A DNS capability is not credited for an HTTP-status change merely because it is earlier in the path.

## Controls, repetition, and edges

Controls are supplied separately from target observations through `Cohort.Controls` or `--attribute-control`. A control is comparable only when platform, network label, address family, optional interface/gateway metadata, and a 30-minute observation window agree. Missing identity or an incompatible comparison is recorded as a limitation rather than silently pooled.

Compatible controls with at least two successful HTTP attempts yield `CONTROL_PATH_HEALTHY`. Repeated target failures at a stage plus that healthy control may raise the target path hypothesis to MEDIUM. If compatible controls fail at the same boundary, V1 emits at most LOW `NETWORK_CONTEXT_FAILURE_SUSPECTED` instead of a target-specific elevation.

Different resolved IPs with materially different stage outcomes produce LOW `EDGE_DEPENDENT_FAILURE_SUSPECTED`. The per-edge facts remain available by evidence reference. Completed HTTP from another edge is explicit counterevidence and prevents a uniform path claim. Lower-boundary failure on a different edge is retained as edge-specific evidence but does not refute a later-stage finding for edges that reached it.

## Direct/profile comparisons

Observations with `execution_context.mode` of `direct` and `externally_active_profile` can receive factual differential labels only when target, network context, transport, and time window are comparable. Different resolved sets are recorded as a limitation; they do not prove a profile effect.

- direct failure + profile success: `FIXED_BY_PROFILE`
- direct success + profile failure: `BROKEN_BY_PROFILE`
- both failure: `STILL_FAILING`
- both success: `REACHABLE_DIRECTLY`

These labels describe observed differentials. They do not identify why a profile changed the result. The retained historical Windows YouTube evidence—direct, current Recommended, and published v0.6.9 Recommended all failing—therefore maps to `STILL_FAILING`, not a regression claim or `BROKEN_BY_PROFILE`.

## CLI

The CLI reads existing JSON only:

```text
unbound --attribute observation.json --json
unbound --attribute target-a.json --attribute target-b.json \
  --attribute-control cloudflare.json --json
```

Input may be one `ObservationResult`, an array, or an evidence envelope containing observation objects. This command needs no administrative privilege and performs no new observation or profile action.

## Explicit non-goals

V1 does not implement StrategyIR compilation, AutoTune scoring, automatic strategy selection, Strategy Lab execution, gaming/R6 graph analysis, packet-level injection attribution, or QUIC inference without a real QUIC handshake.

Failure Attribution identifies likely failure boundaries and path anomalies. It does not prove censorship intent or operator motivation.
