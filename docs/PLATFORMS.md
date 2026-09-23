# Supported platforms and acceptance matrix

UNBOUND uses distinct bypass mechanisms on the three desktop platforms. Profiles and capabilities are not interchangeable across operating systems.

## Authoritative physical acceptance matrix

Lab cycle: **2026-09-22–2026-09-23**. Each result used a dedicated lab target, one owned candidate at a time, and before/after cleanup evidence. It records dataplane/lifecycle acceptance, not a guarantee that every service is reachable from every provider edge.

| Target | Runtime path | Physical dataplane | Scope and network context |
| --- | --- | --- | --- |
| `windows/amd64` | WinDivert + `winws2.exe` | **PASS** | Dedicated Windows lab. Engine lifecycle and cleanup passed. The YouTube service/strategy case is tracked separately; it does not downgrade platform dataplane acceptance. |
| `linux/amd64` | NFQUEUE + `nfqws2` | **PASS** | Dedicated Linux lab. NFQUEUE counters advanced and owned firewall rules were restored. |
| `darwin/arm64` | Universal `tpws --socks` + system SOCKS | **PASS** | macOS 27.0 on Apple M1. Local SOCKS dataplane, lifecycle, and exact proxy-state restoration passed. |
| `darwin/amd64` | Same Universal `tpws` artifact | **NOT VERIFIED** | Universal x86_64 slice and build are verified, but no physical Intel macOS acceptance was run. |

`PLATFORM_DATAPLANE_READY=YES`

Windows YouTube evidence from 2026-09-22 resolved `www.youtube.com` to Google edge `142.251.152.4:443`: TCP passed, while direct, current Recommended, and exact published v0.6.9 Recommended runs failed at TLS/HTTP. Classification: `CURRENT_NETWORK_EDGE_OR_STRATEGY_MISMATCH`. See [`observatory-handoff.md`](observatory-handoff.md).

| Платформа | Движок / перехват | Релизный формат | Поддерживаемый трафик |
|-----------|-------------------|-----------------|------------------------|
| 🪟 **Windows 10/11 x64** | Zapret 2 `winws2.exe` + WinDivert | Wails GUI + CLI | TCP и UDP/QUIC согласно профилю |
| 🐧 **Linux amd64/arm64** | Zapret 2 `nfqws2` + NFQUEUE | CLI | TCP и UDP/QUIC согласно профилю |
| 🍎 **macOS 11+ Intel/Apple Silicon** | Zapret `tpws --socks` + system SOCKS; profile-specific PF UDP/443 fallback | Universal Wails GUI + CLI | TCP only |

---

## 🪟 Windows

**Требования:** Windows 10/11 x64, WebView2 для GUI и права администратора для WinDivert.

- Встроенный `winws2.exe` перехватывает трафик через драйвер WinDivert.
- Доступны нативные Lua-профили Zapret 2, включая отдельные TCP и UDP/QUIC-фильтры.
- GUI, CLI, Control Center, Task Scheduler autostart и `.cmd`-запускатели входят в релиз.
- GUI при необходимости сам запрашивает UAC. Консольные команды запускаются без принудительного UAC: `--version`, `--list-profiles` и `--test` работают без повышения, а запуск движка/Control Center/управление службой требует уже повышенный терминал или комплект `.cmd`-запускателей.
- Microsoft Defender может эвристически блокировать неподписанные сетевые инструменты. Исключение добавляется только явным действием пользователя; это не цифровая подпись и не гарантия доверия.

## 🐧 Linux

**Требования:** ядро с `NFQUEUE`, права root, `nft` либо `iptables`; для запуска GUI из исходников дополнительно нужны зависимости Wails/WebKit.

- Встроенный `nfqws2` создаёт NFQUEUE-процесс, а UNBOUND устанавливает изолированные правила `nftables` с фолбэком на `iptables`.
- Linux остаётся экспериментальным: на физическом хосте подтверждены жизненный цикл `nfqws2`/NFQUEUE и очистка firewall, но для v0.6.9 не найдена повторяемая релизная стратегия YouTube + Discord.
- Для v0.6.9 бинарные Linux-архивы не публикуются. Подробная запись физической приёмки: [`release/linux-v0.6.9-acceptance.md`](release/linux-v0.6.9-acceptance.md).
- systemd autostart и `.sh`-запускатели остаются исходниковой функциональностью, но не являются обещанием релизного архива v0.6.9.

## 🍎 macOS

**Требования:** macOS 11+, права администратора только для профилей, блокирующих UDP/443 через `pf`, WebKit в составе системы.

- Universal `tpws` (`x86_64` + `arm64`) работает как локальный SOCKS4/5 TCP-прокси на `127.0.0.1:9888`; UNBOUND применяет SOCKS только к сетевому сервису активного default route.
- PF не перенаправляет TCP в SOCKS listener. Для Ultimate/YouTube-профилей он ограниченно блокирует UDP/443, чтобы браузеры откатывались с QUIC на TCP; остальные профили не меняют UDP.
- `tpws` не обрабатывает UDP/QUIC. Профиль `Discord TCP Bypass (Web / Gateway)` охватывает HTTPS/Gateway TCP, но не Discord voice media.
- Приложение не подписано Apple Developer ID и не notarized. После загрузки GitHub Gatekeeper может потребовать снять quarantine через включённый `fix_gatekeeper.command`; это осознанное локальное действие пользователя.

---

## Целостность и происхождение

- Zapret 2 закреплён на `v1.0.3` (`b78b52c4…`) для Windows/Linux.
- macOS `tpws` собирается как Universal (`x86_64` + `arm64`) из exact upstream commit [`d437963452674faadfd45adcd62466272b5a2fcd`](https://github.com/bol-van/zapret/commit/d437963452674faadfd45adcd62466272b5a2fcd), следующего за base tag `v72.13`; это не upstream release tag. Причина pin: upstream исправление совместимости macOS resolver stack. Не маркировать как `v72.14`.
- Полные URL, commit SHA, SHA256 исходного архива и артефакта, лицензии и список вендоренных путей находятся в [`../engine/ENGINE_PROVENANCE.json`](../engine/ENGINE_PROVENANCE.json).
- Встроенные runtime-ассеты сверяются с [`../engine/ENGINE_ASSETS.sha256`](../engine/ENGINE_ASSETS.sha256) перед привилегированным запуском. Каждый релизный архив содержит `BUNDLE_SHA256SUMS.txt`; GitHub Release содержит `SHA256SUMS.txt`.

## Ограничения поддержки

- DPI-поведение зависит от провайдера, маршрута и времени. Ни один профиль и AutoTune не гарантируют доступность конкретного сервиса.
- Secure DNS меняет системный DNS и не заменяет DPI-обход, VPN или шифрованный туннель.
- Приложение не перенаправляет трафик на серверы UNBOUND и не устанавливает Root CA.
- Android, Magisk, OpenWrt, Steam Deck, браузерные расширения, iOS/tvOS и Smart TV не входят в текущий desktop-релиз; старый код хранится в `legacy/v2.x`.
