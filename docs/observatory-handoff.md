# Observatory v1 handoff

**Scope:** design only. No Observatory implementation is included in this branch.

## Pipeline

```text
resolve -> connect -> hello -> handshake -> HTTP -> carry
```

The collector records one structured observation per attempt, plus the selected transport and the target/edge identity. `carry` preserves completed stage evidence for later comparison; it must not convert an earlier failure into a strategy verdict.

| Stage | Observable outcomes |
| --- | --- |
| `resolve` | DNS success, resolver failure, address set |
| `connect` | TCP connect success, timeout, refusal, reset/RST |
| `hello` | transport-specific first-flight emission (TCP ClientHello or QUIC Initial) |
| `handshake` | TLS success/failure/timeout/reset; QUIC success/failure/reset |
| `HTTP` | application status, timeout, redirect, protocol error |
| `carry` | retained endpoint, transport, timing, and preceding failure boundary |

## Core invariant

**Never penalize a packet strategy for a failure it could not structurally affect.**

A strategy applied after TCP connection cannot be blamed for DNS failure. A TCP-only strategy cannot be credited or blamed for a QUIC-only failure. A remote reset recorded before the strategy's controllable boundary is evidence about the network edge/upstream, not a strategy score.

Classify independently:

- DNS
- TCP connect
- RST/reset
- TLS
- QUIC
- HTTP/application
- remote/upstream
- transport

The UI and scorer must retain the raw stage boundary rather than reduce every failed request to `blocked`.

## Initial physical regression fixture

```json
{
  "case": "windows-youtube-google-edge-2026-09-22",
  "platform": "windows/amd64",
  "target": "https://www.youtube.com/generate_204",
  "edge": "142.251.152.4:443",
  "direct": {"tcp": "PASS", "tls": "FAIL", "http": "FAIL"},
  "current_recommended": {"tcp": "PASS", "tls": "FAIL", "http": "FAIL"},
  "published_v0_6_9_recommended": {"tcp": "PASS", "tls": "FAIL", "http": "FAIL"},
  "classification": "CURRENT_NETWORK_EDGE_OR_STRATEGY_MISMATCH"
}
```

This is a physical evidence example and future regression fixture. It is not a hardcoded strategy rule and does not downgrade `WINDOWS_PLATFORM_DATAPLANE=PASS`.

## First implementation boundary

Start with a transport-neutral observation schema and adapters for direct TCP/TLS/HTTP and QUIC. Keep packet-strategy attribution separate from measurement. Add strategy scoring only after the stage model can prove which boundary the strategy can affect.
