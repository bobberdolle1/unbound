# Linux v0.6.9 physical acceptance record

## Decision

Linux engine lifecycle is **PASS**; the NFQUEUE packet path and firewall ownership/cleanup are **PASS**. No repeatable, release-grade strategy for both YouTube and Discord was found on the physical Linux acceptance host. **No Linux binary asset is published for v0.6.9.**

This is a release-scope decision, not a claim that Linux is universally broken.

## Environment and provenance

- Physical private Linux acceptance host; root-run `nfqws2` with `/usr/sbin/iptables` 1.8.10 (nf_tables backend).
- Zapret2 provenance: v1.0.5.1, commit `a1bca5a85e25ab138e9617a560c262fcf53e969a`.
- Tested `nfqws2` SHA-256: `f451dc5c438cc721da181cffadb47ec6eb09aad09255de3f29eee724d485629a`.
- The v1.0.5.1 release archive SHA-256 was `92212b110c4e1a2613567e2821823e058fc408a5cfcc6548e4aa568f632a6c8e`.

## Established runtime facts

- Commit `122a4422` makes the extracted runtime root searchable after the privilege drop (`0711`) and Lua directories/files readable (`0755`/`0644`). This fixed Lua asset visibility and remains required.
- The Linux provider inserts an owned mangle `POSTROUTING` NFQUEUE rule, limits interception to initial packets, and uses queue 200 with bypass behavior.
- A scoped Cloudflare probe increased the NFQUEUE counter from `0/0` to `6/917`, completed TLS and returned HTTP 301. The rule could be checked with `iptables -C`; stop removed the rule and left no `nfqws2` process.
- Hostlist scoping isolated collateral traffic: the no-desync Cloudflare pass-through and cleanup checks passed.

## Strategy research evidence

- Plain `tcp_md5` was unsafe on the tested Cloudflare route. Capture showed a forged MD5 segment followed by the genuine ClientHello; the remote peer did not respond. It is not a candidate for release.
- YouTube `multidisorder:pos=midsld` was edge-dependent in upstream blockcheck exploration. It was not repeatable enough for a product profile.
- A complex Discord `multidisorder` candidate was repeatable on the tested edges:
  `multidisorder:pos=1,sniext+1,host+1,midsld-2,midsld,midsld+2,endhost-1`.
  It does not establish a release-grade combined profile.
- The targeted YouTube/Discord profile from `f2931b1` was physically exercised with its intended two hostlist blocks. Discord returned HTTP 200 and Steam/Cloudflare passed, but pinned YouTube (`142.251.154.4`) still timed out during TLS. Commit `ac74e004` reverted that profile; the revert remains intentional.

## Follow-up

Future Linux work belongs in a post-v0.6.9 strategy-discovery issue. Start from the retained evidence above: investigate route/edge stability for YouTube, re-evaluate the promising Discord candidate independently, preserve hostlist collateral isolation, and do not reuse plain `tcp_md5` on the tested Cloudflare route.
