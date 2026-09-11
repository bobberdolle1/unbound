# UNBOUND macOS: TCP SOCKS architecture

## Runtime path

```mermaid
flowchart LR
    App[Application that honors macOS SOCKS] -->|SOCKS5 CONNECT| Proxy[127.0.0.1:9888]
    Proxy -->|TCP desync| Tpws[tpws --socks]
    Tpws --> Internet
    Browser[HTTP/3-capable browser] -. Ultimate / YouTube only .->|UDP/443 blocked by PF| TCP
```

UNBOUND starts bundled `tpws` in upstream SOCKS mode:

```text
tpws --socks --port=9888 --bind-addr=127.0.0.1 …
```

The active default-route network service is configured with macOS `networksetup` to use SOCKS5 `127.0.0.1:9888`. The listener requires a SOCKS4/5 handshake; it is **not** a transparent proxy. UNBOUND therefore never redirects raw TCP/TLS into that listener with PF `rdr` or `route-to`.

`pf` is optional and is used only by the **Ultimate Bypass** and **YouTube QUIC Aggressive** profiles to block outbound UDP/443. This makes QUIC clients fall back to TCP, which then reaches `tpws` through the system SOCKS proxy. Other macOS profiles do not install the UDP/443 block.

## What macOS profiles cover

`tpws` operates on TCP streams. It does not proxy UDP or QUIC.

- **Ultimate Bypass (Multi-Strategy):** general HTTP/TCP HTTPS, with UDP/443 fallback to TCP.
- **YouTube QUIC Aggressive:** YouTube HTTP/TCP HTTPS, with UDP/443 fallback to TCP.
- **Discord TCP Bypass (Web / Gateway):** Discord HTTPS, REST, CDN and Gateway TCP/WebSocket traffic. It does **not** support Discord voice media.

Discord voice uses a separately negotiated UDP media path. Successful HTTPS or Gateway checks do not demonstrate voice support. A real UDP bypass needs a separate architecture, such as a Network Extension packet tunnel with supported UDP handling; it is not provided by `tpws --socks`.

## Diagnostics

With UNBOUND running, the active service should report:

```text
networksetup -getsocksfirewallproxy "Wi-Fi"
Enabled: Yes
Server: 127.0.0.1
Port: 9888
```

The effective proxy state is available via `scutil --proxy`; the listener can be inspected with:

```text
lsof -nP -iTCP:9888 -sTCP:LISTEN
```

A successful UNBOUND Doctor or AutoTune probe verifies the local SOCKS path used by that probe. It does not prove that every transport in a desktop application, including UDP voice, follows the proxy.

## Installation and build

Move `Unbound.app` to `/Applications`, launch it, then select a profile and connect. The application may request administrator approval only when a profile needs the PF UDP/443 fallback rule.

For source builds:

```bash
wails build -platform darwin/arm64
wails build -platform darwin/amd64
```
