# Observatory v1

Observatory is UNBOUND's read-only connectivity measurement subsystem. It records where an HTTPS attempt reached or stopped:

```text
resolve -> connect -> hello -> handshake -> HTTP -> carry
```

It describes evidence. It does not prove censorship intent, infer DPI mechanisms, select a bypass strategy, or control any bypass process.

## Boundary

`engine/observatory` depends only on Go's transport and OS network stack. It does not import Zapret providers, Strategy Lab, AutoTune, firewall management, DNS configuration, proxy configuration, routes, or hostlist code.

Observatory never starts or stops `winws2`, `nfqws2`, or `tpws`; it never changes host networking. Its only optional write is explicit evidence persistence through `SaveObservation`.

## CLI

```text
unbound --observe https://example.com --json
unbound --observe https://example.com --observe-ip-family=4 --observe-timeout=15s
unbound --observe https://example.com --observe-save
```

`--observe-protocol=tcp` is the implemented direct TCP/TLS/HTTP adapter. `--observe-protocol=quic` returns `SKIPPED_UNSUPPORTED` / `QUIC_UNSUPPORTED`; it does not treat opening a UDP socket as QUIC success.

`--json` emits exactly one `ObservationResult`. Observation is non-elevated and requires no administrator/root privileges.

## Schema

Every result has `schema_version: 1`, a random `run_id`, UTC bounds, `build_identity`, `platform`, target, network context, resolved addresses, ordered attempts, `final_boundary`, and a factual `classification`.

Each attempt records one pinned resolved IP, address family, TCP transport, local address when connected, elapsed time, and stage evidence. Stage evidence contains a status, a factual error class/code, bounded native detail, timing, and where relevant TLS or HTTP metadata.

Build identity always contains `version`, `commit`, `dirty`, `channel`, `os`, and `arch` so evidence identifies the producing binary.

## Stage semantics

| Stage | PASS means | Failure behavior |
|---|---|---|
| `resolve` | A usable address set was returned. | DNS resolver error is `DNS_FAILURE`; later stages are `NOT_REACHED`. |
| `connect` | A TCP socket reached the selected IP. | Timeout/refusal/reset/unreachable retain distinct TCP classes. |
| `hello` | TLS ClientHello bytes were emitted over the connected socket. | It is never inferred from creating a UDP socket or merely allocating TLS state. |
| `handshake` | Real `crypto/tls` handshake completed. | TLS timeout, reset, certificate, protocol, and other failures remain distinct. |
| `HTTP` | A bounded HTTP/1.1 response was received after TLS. | A response status such as 503 is factual `HTTP_STATUS`, not a TLS failure. |
| `carry` | The completed evidence was preserved. | Carry makes no network request and never rewrites an earlier boundary. |

Permitted statuses are `PASS`, `FAIL`, `TIMEOUT`, `RESET`, `SKIPPED`, `SKIPPED_UNSUPPORTED`, `NOT_REACHED`, and `CANCELLED`. `NOT_REACHED` is never converted into `FAIL`.

The structural attribution invariant is explicit in the ordered stages: a post-DNS TCP strategy cannot explain a DNS failure; a TLS ClientHello strategy cannot explain a TCP-connect failure; a TCP-only strategy cannot be credited or blamed for a QUIC result.

## Edge pinning

After resolution, every TCP attempt uses one resolved IP as its socket destination. TLS uses the original hostname as `ServerName`; HTTP uses the original hostname in `Host`. The observer does not hand the request to an HTTP transport that can perform another DNS lookup and switch CDN edge.

For example, an observation of `www.youtube.com -> X` records connect/TLS/HTTP against `X`, not an opaque later edge. The historical Windows YouTube case is evidence-shaped as resolve pass, connect pass, ClientHello attempted, handshake failure, HTTP not reached. It is not labelled "YouTube blocked".

## Error classes and limits

v1 classifications are factual: DNS, TCP, TLS, HTTP, QUIC-unsupported, transport, remote, unknown, and success classes. Typed context, `net.Error`, TLS, x509, and syscall errors normalize comparable meanings across platforms while retaining bounded native detail.

No `DPI_BLOCK`, `SNI_BLOCK`, `RST_INJECTED`, or allowlist conclusion exists in v1.

Attempts are sequential and bounded by DNS, connect, TLS, HTTP, and overall timeouts. Context cancellation closes in-flight HTTP reads and produces `CANCELLED` evidence.

## Read-only lab sample

The direct adapter was exercised at commit `3a9c91bc497e39ce7552d4d71940f8c341577a12` on Windows amd64, Linux amd64, and macOS arm64 without starting or stopping a bypass engine.

| Target | Windows amd64 | Linux amd64 | macOS arm64 |
|---|---|---|---|
| Cloudflare control (`www.cloudflare.com/cdn-cgi/trace`) | `SUCCESS` | `SUCCESS` | `SUCCESS` |
| YouTube (`www.youtube.com/generate_204`) | first selected edge: TLS handshake timeout; HTTP `NOT_REACHED` | TLS handshake timeout; HTTP `NOT_REACHED` | TLS handshake timeout; HTTP `NOT_REACHED` |
| Discord (`discord.com/api/v9/experiments`) | TLS handshake timeout; HTTP `NOT_REACHED` | TLS handshake timeout; HTTP `NOT_REACHED` | TLS handshake timeout; HTTP `NOT_REACHED` |

The resolved YouTube set included historical edge `142.251.152.4`; its current Windows attempt recorded `TCP_CONNECT_TIMEOUT`, then `NOT_REACHED` for TLS and HTTP. These are network observations, not claims about censorship intent or strategy correctness.

## Privacy and persistence

Observation does not retain response bodies. Captured response headers are allowlisted to `content-type` and `location`; cookies and `Authorization` headers are excluded. Persisted target URLs remove userinfo and query values. Private keys, passwords, and full bodies are never written.

`SaveObservation` writes only on explicit caller request under the owned user configuration path:

```text
<user-config>/Unbound/observations/<timestamp>-<run-id>.json
```

Files and directory use owner-only permissions where supported.

## Callback contract

`Options.Progress` receives stage evidence synchronously. Invocations are serialized for a single `Observe` call: Observatory never invokes the callback concurrently. Callbacks must return promptly to preserve bounded observation time.

## Known v1 limitations

- Direct HTTPS currently negotiates HTTP/1.1 deliberately to retain pinned, single-connection request control.
- QUIC requires a maintained implementation performing a real handshake and is intentionally unsupported until then.
- Default gateways and configured resolvers are optional context fields; v1 does not perform external ASN lookup.
- Observatory measures the active network environment but does not start or stop any profile. External A/B orchestration may supply `execution_context` metadata later.
