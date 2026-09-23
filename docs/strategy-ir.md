# StrategyIR v1 and backend capabilities

## Boundary

StrategyIR v1 is declarative data for a single traffic-selection and packet-operation chain. It records *what* a strategy means; backend compilers own both engine spelling and executor-owned packet acquisition.

A StrategyIR document never carries `zapret2_args`, `tpws_args`, raw argv, shell, PowerShell, batch, Lua source, Lua function names, filesystem paths, dynamic-library paths, remote URLs, or firewall commands. Existing production profile execution remains unchanged in this release. The compiler is pure and emits structured backend plans only.

`engine/strategyir` owns the schema. `engine/backendcap` owns static backend capability declarations and compilation. A compiled `Plan` separates `EngineArgv` (engine/profile flags) from `Capture` (typed executor input) and `AssetRefs`. `CapturePlan` carries backend kind, packet transport, direction, IP families, typed TCP/UDP port ranges, and reserved logical raw-capture references—never `nft`, `iptables`, or shell strings. Machine environment checks remain in the existing `engine.StrategyRequirements` path; compiler output derives that existing type rather than creating a competing environment model.

## Schema v1

`schema_version` is `1`. A strategy contains:

- exactly one `transport`: `TCP`, `UDP`, or `QUIC`;
- a `selector` with an exact application-protocol set (`HTTP`, `TLS`, `QUIC`, or the sole `ANY` value), IPv4/IPv6/any family, direction, transport-specific port ranges, and destination scope;
- an optional typed `range` filter for the entire ordered operation chain;
- ordered typed `operations`;
- declarative `safety`.

Port ranges are inclusive `{ "start": 443, "end": 8443 }`; a singleton has identical start/end values. Every declared transport requires a non-empty scope: `TCP` requires `tcp_ports`; `UDP` and `QUIC` require `udp_ports`. Empty ports never mean all ports. `OUTBOUND`, `INBOUND`, and `BOTH` are explicit rather than inferred from profile text.

`ANY` is exclusive in both selector sets. Application `QUIC` requires the sole `QUIC` transport, and `QUIC` transport requires the sole `QUIC` application protocol, which compiles as `--payload=quic_initial`. Thus it never becomes generic UDP capture. `HTTP` and `TLS` selectors require the sole `TCP` transport; the exact `{HTTP,TLS}` TCP union is supported. Generic `ANY` is permitted for TCP or UDP but not QUIC. Mixed transport sets, including UDP+QUIC, are invalid in V1 because a single backend payload filter cannot bind distinct L7 discriminators to distinct transports without separate profile sections.

IP-family `ANY` is likewise exclusive: `{ANY,IPv4}` and `{ANY,IPv6}` are invalid rather than redundant identities. Zapret2 engine profiles compile IPv4 and IPv6 as `--filter-l3=ipv4` and `--filter-l3=ipv6`; `ANY` compiles as the exact dual-stack `--filter-l3=ipv4,ipv6`. Windows renders the separate WinDivert `CapturePlan` as the corresponding `--wf-l3=` and directional `--wf-tcp-*`/`--wf-udp-*` constructor flags. Linux never renders `--wf-*`: its `CapturePlan` describes NFQUEUE ownership for the existing firewall executor. tpws uses `--filter-l3=` as a valid profile filter. A backend that cannot preserve either engine or capture family scope returns `UNSUPPORTED_IP_FAMILY`.

Zapret2 preserves a non-`ANY` application set as one exact comma-separated payload filter, for example `--payload=http_req,tls_client_hello`. A backend lacking that exact selector semantics returns `UNSUPPORTED_APPLICATION_PROTOCOL`; it never broadens to an unfiltered selector.

Destination scope is independent of packet operations:

- `ALL`;
- `EXPLICIT` host/domain literals;
- `MANAGED_LIST` logical IDs such as `youtube`;
- `AUTO_HOSTLIST` logical IDs such as `autodetect`;
- `IP_SET_REFERENCE` logical IDs such as `ipset-all`.

Exclude host/IP list IDs are also logical IDs. `C:\hosts.txt`, `/tmp/hosts`, and equivalent arbitrary paths are invalid. A `target_only` safety policy cannot use an `ALL` host scope, and a compiler never removes a non-global scope.

## Positions and operations

Positions are typed `PositionExpr` values: either an absolute byte offset with zero offset, or an anchor plus signed offset. V1 anchors are `HOST`, `ENDHOST`, `MIDSLD`, `SNIEXT`, and `METHOD`. This represents `1`, `midsld-2`, `host+1`, `endhost-1`, and `sniext+1` without carrying backend expression strings.

The minimal V1 operation vocabulary is:

- `SPLIT`, `MULTI_SPLIT`;
- `DISORDER`, `MULTI_DISORDER`;
- `TLS_RECORD_SPLIT`;
- `HTTP_HOST_CASE`;
- `FAKE_INJECTION` with a trusted payload reference;
- `HOST_FAKE_SPLIT`;
- `WINDOW_SHAPING`.

Operations are a strict tagged union. `SPLIT` requires exactly one position; `MULTI_SPLIT`, `MULTI_DISORDER`, `TLS_RECORD_SPLIT`, and `HOST_FAKE_SPLIT` require positions. Only `MULTI_SPLIT` accepts sequence overlap and its logical pattern. Only `FAKE_INJECTION` accepts `payload_ref`; it requires one. Fake modifiers are accepted only by fake injection, multi-disorder, and host-fake-split. Only window shaping accepts `window`. HTTP host case accepts none of those fields. A populated field outside its operation shape is invalid; no compiler field is ignored.

`SPLIT` is intentionally the semantic one-position form of `MULTI_SPLIT` for current Zapret2 and tpws targets. Both compile to the same one-position backend form; tests lock that equivalence. The distinct names preserve source-level intent and make future divergent backend mappings explicit.

Fake modifiers are independently capability-gated: repeat, TTL/hop, TCP sequence offset, TCP acknowledgment offset, TCP MD5-like fooling, and TCP timestamps. A `Strategy.range` is a typed bounded profile filter: `IN` or `OUT`, one of `PACKET_NUMBER`, `DATA_PACKET_NUMBER`, `RELATIVE_SEQUENCE`, or `DATA_POSITION`, plus a positive terminal value. It applies to the entire ordered operation chain and has no per-operation override or reset semantics in V1. Zapret2 emits the range after the packet/scope selectors and before the first `--lua-desync`, so the same filter governs every subsequent desync instance in that chain. It currently preserves `OUT + DATA_PACKET_NUMBER + 8` as `--out-range=-d8`, `OUT + DATA_PACKET_NUMBER + 3` as `--out-range=-d3`, and `IN + RELATIVE_SEQUENCE + 4096` as `--in-range=-s4096`. The names deliberately do not mislabel `d` as a generic packet count or `s` as sequence bytes. Unsupported direction/counter combinations fail closed.

Examples of trusted logical payload assets are `tls-clienthello-default`, `tls-google`, `quic-google`, and `fake-default-udp`. Resolution is executor responsibility and is constrained to pinned engine assets.

## Identity and strict loading

`Validate` enforces schema version, exact protocol-set semantics, mandatory transport port scopes, strict operation shapes, typed positions/ranges, logical IDs, and safety scope invariants. `UnmarshalStrict` uses unknown-field rejection and rejects trailing JSON. Unknown operation kinds are invalid.

`Canonicalize` deep-copies caller-owned slices and pointers before normalizing set-like fields—transport, app protocol and IP-family sets, port ranges, host literals, and logical scope IDs. It deliberately preserves operation order and position order. Invalid redundant `ANY` selector combinations never canonicalize; valid family and protocol set order cannot change identity. `Canonicalize`, `Marshal`, and `Fingerprint` never mutate input. `Fingerprint` is SHA-256 over canonical semantic JSON. IDs, display names, descriptions, and timestamps are excluded, so presentation changes do not alter the fingerprint.

## Static BackendCapabilities

Capabilities distinguish engine/profile support from packet acquisition support. `ProfileCapabilities` describes the flags and operations the pinned process can preserve; `CaptureCapabilities` describes the executor-owned acquisition backend, packet transports, directions, and IP families.

| Backend | Profile semantics | Capture capability |
|---|---|---|
| `zapret2/windows` | v1.0.5.1 / `a1bca5a85e25ab138e9617a560c262fcf53e969a`; TCP, UDP, QUIC; exact multi-protocol payload filters; IPv4/v6; logical host/IP lists; individually gated fake assets/modifiers; typed range cutoff; closed Zapret Lua operation table | `WINDIVERT`; inbound/outbound/both; IPv4/v6; TCP and UDP packet capture. The renderer emits `--wf-*` only for this capture constructor. |
| `zapret2/linux` | Same pinned nfqws2 profile subset, rendered as `--filter-l3`, `--filter-tcp`/`--filter-udp`, payload/scope/range, then ordered Lua operations | `NFQUEUE`; inbound/outbound/both; IPv4/v6; TCP and UDP packet capture. Firewall/NFQUEUE ownership stays in the existing Linux executor; compiler output contains no firewall commands and no `--wf-*`. |
| `zapret1-tpws/darwin` | base v72.13 / `d437963452674faadfd45adcd62466272b5a2fcd`; outbound TCP SOCKS-path split, TLS-record split, disorder, HTTP host case with an `ANY` selector only; preserves IPv4/IPv6 via `--filter-l3`; no host/IP scope or L7 payload-selector preservation | `SOCKS_TCP`; outbound IPv4/v6 TCP only. This is the existing runtime SOCKS path, not invented packet capture; UDP/QUIC remains unsupported. |
| `unbound-native/*` | no implementation | no implementation |

Environment facts such as enabled TCP timestamps, installed assets, privileges, and active IPv6 connectivity are not `BackendCapabilities`. Compiling Zapret2 derives the existing `engine.StrategyRequirements` (`EngineMinVersion`, `LuaModules`, inbound direction, QUIC, IPv6). V1 logical fake payloads are pinned `init_vars.lua` values: compiler capability checks them by logical ID and derives that Lua module, while `FakePayloads` remains reserved for future file-backed assets that the existing environment checker can verify.

## CompileResult contract

`Compile(strategy, backend)` returns one of:

- `COMPILED`: a deterministic `Plan`, logical required assets, fingerprint, and derived existing requirements;
- `UNSUPPORTED`: no plan, plus typed reasons;
- `INVALID`: invalid IR, plus `INVALID_IR`.

`Plan.EngineArgv` contains only process-supported profile filters and ordered operations. For Zapret2 it is `--filter-l3`, transport port filter, optional payload selector, destination scope, optional range, then the `--lua-desync` chain. `Plan.Capture` is separately deterministic typed data:

- Windows: `WINDIVERT`; `RenderWindowsCaptureArgv` derives the exact `--wf-l3` and `--wf-tcp-*`/`--wf-udp-*` constructor argv only at the Windows execution boundary.
- Linux: `NFQUEUE`; `EngineArgv` never contains any `--wf-*` option. The existing provider separately owns nftables/iptables rendering and NFQUEUE lifecycle; `CapturePlan` preserves its required semantics without coupling StrategyIR to that provider implementation.
- macOS: `SOCKS_TCP`; tpws has only profile filters in `EngineArgv` and remains bound to its existing SOCKS runtime executor.

Typed reasons include `UNSUPPORTED_OPERATION`, `UNSUPPORTED_TRANSPORT`, `UNSUPPORTED_APPLICATION_PROTOCOL`, `UNSUPPORTED_POSITION_ANCHOR`, `UNSUPPORTED_IP_FAMILY`, `UNSUPPORTED_PORT_SCOPE`, `UNSUPPORTED_SCOPE`, `UNSUPPORTED_DIRECTION`, `UNSUPPORTED_CAPTURE`, `UNSUPPORTED_FAKE_MODIFIER`, `UNSUPPORTED_CUTOFF`, `MISSING_ASSET`, `ENGINE_VERSION_TOO_OLD`, and `INVALID_IR`.

There is no best-effort translation. The compiler does not drop an operation, change a transport, broaden a protocol/host/IP scope, discard a position, remove a safety-relevant constraint, or erase capture requirements because an engine option is unavailable. For example, an explicit-host strategy compiled for tpws fails `UNSUPPORTED_SCOPE` because the current tpws compiler cannot truthfully preserve that scope. UDP and QUIC strategy requests for tpws are unsupported, not converted to TCP.

Logical asset placeholders in engine argv use `${asset:<id>}`. They are not executable paths and must be resolved by trusted product code before any future executor consumes a plan.

## Physical parser acceptance

`TestWindowsCompiledRepresentativePlansDryRun` is environment-gated physical parser acceptance for every representative Zapret2 Windows plan. It renders the structured WinDivert capture plan plus `EngineArgv`, invokes pinned `winws2.exe --dry-run`, and requires `command line parameters verified`.

`TestLinuxCompiledRepresentativePlansDryRun` runs automatically on Linux. It first checks pinned `nfqws2 --help` for `--dry-run`; without that documented parser-only mode it reports `SKIPPED_UNSUPPORTED`. When available, it invokes product-extracted `nfqws2 --dry-run` with representative Linux `EngineArgv` only—never firewall commands or NFQUEUE ownership. These checks prove argv syntax only, not packet-processing or profile semantic equivalence. No test in this boundary performs connectivity experiments.

## Inventory and shadow migration

The corpus inventory separates semantic operations from current backend spelling. It is **declared migration coverage** derived from reviewed source profiles, not a parser or verification result for arbitrary saved/custom data:

| Existing corpus | Semantic features observed | Current spellings / requirements | V1 disposition |
|---|---|---|---|
| Recommended / hostfakesplit | host fake split, QUIC/UDP fake, Steam exclusions, list/IP scope | `hostfakesplit`, `fake`, `--hostlist`, `--ipset`, `--wf-*` | partial: multi-section capture composition |
| Alternative 1 | TCP multisplit, sequence overlap, fake QUIC | `multisplit:pos=2:seqovl=652` | partial: multi-section capture composition |
| Alternative 2 | fake TLS, ack/timestamp fooling, multidisorder | `fake`, `multidisorder_legacy`, inline zero blob | partial: inline blob and legacy distinction |
| Universal / advanced | fake, host fake split, fake split, multi-protocol scopes | `fakedsplit`, multiple `--new` sections | partial |
| Discord | TLS TCP split and UDP voice fake | `multisplit`, `fake`, Discord/STUN L7 filter | TCP shadowed; voice remains partial |
| Steam-safe Game Filter | IP-targeted multi-split and UDP fake with cutoffs | `--ipset`, `multisplit`, `fake` | TCP shadowed; multi-section UDP remains partial |
| Adaptive | staged circular operation chain and inbound feedback | `zapret-auto.lua`, `circular` | unrepresentable: dynamic orchestration |
| AutoHostlist | target list plus feedback thresholds | `--hostlist-auto`, inbound sequence/event thresholds | partial |
| Strategy Lab / AutoTune candidates | split, fake, multidisorder, window shaping, syndata | opaque `Zapret2Args` | declared core subset only; raw candidates remain legacy |
| Saved discovered profiles | arbitrary persisted argv | opaque candidate/runtime args | unrepresentable; not parsed or verified |
| macOS tpws | TCP split, TLS record split, disorder, HTTP host case | `--split-pos`, `--tlsrec`, `--disorder`, `--hostcase` | TCP operation chain representable only where `ANY` selector is sufficient; PF QUIC fallback remains executor policy |

`RepresentativeFixtures` carries five reviewed semantic shadows: Recommended hostfakesplit, Alternative 1 multisplit, Alternative 2 fake TLS, Discord TCP, and Steam-safe Game Filter TCP. Golden compiler tests compare selector transport/ports/payload, scope and exclusions, ordered operations and positions, fake modifiers, typed range cutoffs, and logical assets to trusted legacy semantic fragments. This is semantic comparison, not byte-for-byte argv migration.

`CatalogCoverage` is the machine-readable declared migration report: `REPRESENTABLE=1`, `PARTIALLY_REPRESENTABLE=9`, `UNREPRESENTABLE=3`. It deliberately declares opaque custom Lua, dynamic adaptive orchestration, arbitrary discovered argv, and executor firewall ownership as missing capabilities rather than silently approximating them.

## Security and future planner boundary

Remote strategy packs may eventually carry only strict StrategyIR. They cannot select local paths, execute shell/Lua, name arbitrary Lua functions, or download executable code. Zapret compiler operation mapping is a closed table.

Attribution remains independent from backend compilers. `attribution.CouldStrategyAffectFailure` continues to accept neutral structural metadata. Candidate ranking, AutoTune changes, TransportIR/carriers, native execution, remote distribution, and profile runtime replacement are intentionally outside V1.
