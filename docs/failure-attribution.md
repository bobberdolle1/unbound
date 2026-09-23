# Failure Attribution v1

Failure Attribution consumes persisted `observatory.ObservationResult` values and emits schema-versioned, deterministic analysis. It is a pure Go package at `engine/attribution`; its only product dependency is `engine/observatory`.

It performs no network requests and never starts or stops profiles or bypass engines. It does not modify DNS, firewall state, routes, proxies, Observatory records, Strategy Lab, or AutoTune.

## Facts and hypotheses

Observatory remains the factual measurement layer. Examples are `DNS_FAILURE`, `TCP_CONNECT_TIMEOUT`, `TLS_HANDSHAKE_TIMEOUT`, `HTTP_STATUS`, `QUIC_UNSUPPORTED`, an HTTP status, and per-attempt stage status.

Attribution reports facts as `Finding.kind = FACT` and path interpretations as `Finding.kind = INFERENCE`. Mechanism-level findings always use conservative vocabulary such as `TLS_PATH_FAILURE_SUSPECTED`; they do not prove censorship or operator intent. V1 never emits `DPI_BLOCK`, `DPI_CONFIRMED`, `SNI_BLOCK`, `RST_INJECTED`, `ALLOWLIST_MODE`, `TSPU_BLOCK`, or `VPN_DETECTED`.

`TLS_PATH_FAILURE_SUSPECTED` means the observed TLS path did not complete after TCP and ClientHello evidence. It cannot distinguish DPI, a remote edge, routing, a middlebox, local security software, or provider policy.

## Report schema

`AttributionReport.schema_version` is `1`. It contains a deterministic `attribution_id`, a creation timestamp derived from the latest input observation completion time, input run IDs, a privacy-minimized target reference, optional network-context key, primary finding, findings, evidence references, counterevidence, limitations, confidence, and applicability.

`TargetRef` serializes `scheme` when known, `hostname`, effective `port`, URL `path`, and `requested_protocol`. Endpoint identity uses these fields, so two observations for the same host but different paths are not pooled. It never serializes userinfo, query values, fragments, response bodies, cookies, authorization values, native error strings, or headers.

## Confidence rules

Confidence is rule-derived; there are no numeric probabilities.

- **LOW** — one relevant target failure, missing required prerequisite, heterogeneous target outcomes, incomplete control evidence, or a control cohort that does not establish a clean comparison.
- **MEDIUM** — the same target boundary repeats across at least two independent compatible target runs; at least two independent compatible control runs complete valid HTTP paths; no compatible control run fails at that boundary; and no target evidence completes HTTP.
- **HIGH** — direct factual observations such as completed HTTP, an observed reset, same-edge outcome variability, or a same-edge controlled A/B differential. HIGH is not used for suspected path mechanisms.

Run identity—not the number of attempts or repeated copies of the same evidence—establishes independence. Duplicate input never raises confidence. A valid HTTP response is path-complete. HTTP 4xx/5xx produces the factual `HTTP_APPLICATION_FAILURE_OBSERVED`; it is never reclassified as TLS or transport failure. Redirects are successful path evidence.

## Structural applicability

A finding never credits or blames a mechanism that cannot affect its failed boundary:

- DNS failure cannot be attributed to TLS/ClientHello handling.
- TCP connection failure occurs before ClientHello handling.
- TLS handshake failure cannot be classified using HTTP status.
- `QUIC_UNSUPPORTED` produces `INSUFFICIENT_EVIDENCE`; V1 makes no QUIC-path inference.
- An observed reset produces `TCP_RESET_OBSERVED` or `TLS_RESET_OBSERVED`, never `RST_INJECTED`.

`CouldStrategyAffectFailure(StrategyCapabilities, boundary, transport)` is an optional pure metadata helper for future StrategyIR work. A capability declares transports (`TCP`, `QUIC`) separately from affected stages (`DNS`, `CONNECT`, `HELLO`, `HANDSHAKE`, `HTTP`). The helper first checks transport applicability, then stage applicability. It only answers structural possibility; it does not score, choose, or run a strategy. For example, TCP ClientHello capability can affect TCP HELLO/HANDSHAKE but not DNS or QUIC; a QUIC handshake capability is the inverse. A DNS capability is not credited for an HTTP-status change merely because it is earlier in the path.

## Controls, repetition, and edges

Controls are supplied separately from target observations through `Cohort.Controls` or `--attribute-control`. A control is comparable only when platform, network label, address family, optional interface/gateway metadata, and a 30-minute observation window agree. Missing identity or an incompatible comparison is recorded as a limitation rather than silently pooled.

One compatible successful control **run** yields MEDIUM factual `CONTROL_PATH_HEALTHY`; at least two independent successful control runs yield HIGH. Multiple successful edge attempts inside one run do not count as independent runs. If compatible controls fail at the target-relevant boundary, they produce `CONTROL_PATH_DEGRADED` rather than a cleanly healthy cohort and cannot elevate target TLS confidence.

Resolved IP plus address family identifies an edge. Across the compatible target cohort, stable materially different outcomes on distinct edges yield LOW `EDGE_DEPENDENT_FAILURE_SUSPECTED`. If one exact edge produces materially different outcomes across independent runs, V1 emits HIGH factual `OUTCOME_VARIABILITY_OBSERVED` and does not attribute the cohort difference to edge identity. Completed HTTP and materially different failure boundaries are counterevidence to a uniform path claim; heterogeneous TLS and TCP outcomes therefore cannot receive the confidence of homogeneous TLS failures.

## Direct/profile comparisons

Observations with `execution_context.mode` of `direct` and `externally_active_profile` receive factual differential labels only when endpoint identity, network context, transport, time window, **and an exact resolved IP plus address family** are comparable. Resolver set equality or resolver order alone is not sufficient. When no same-edge pair exists, V1 emits no A/B label and records `same_resolved_edge` as a limitation.

- same-edge direct failure + profile success: `FIXED_BY_PROFILE`
- same-edge direct success + profile failure: `BROKEN_BY_PROFILE`
- same-edge both failure: `STILL_FAILING`
- same-edge both success: `REACHABLE_DIRECTLY`

The primary finding is the same-edge A/B fact when available, but `findings` retain direct-path boundary, edge, control, and counterevidence findings. These labels describe observed differentials only. They do not identify why a profile changed the result. The retained historical Windows YouTube evidence—direct, current Recommended, and published v0.6.9 Recommended all failing—therefore maps to `STILL_FAILING`, not a regression claim or `BROKEN_BY_PROFILE`.

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
