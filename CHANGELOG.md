# Changelog - UNBOUND

Все значимые изменения проекта документируются в этом файле.

## [1.1.0] - 2026-04-07
### macOS Port — Cross-Platform Architecture
- **SpoofDPI Engine**: Новый движок обхода DPI для macOS на базе SpoofDPI (SOCKS5 прокси). Заменяет nfqws/pf. Полная замена `engine/providers/zapret_macos.go`.
- **Системная маршрутизация**: Автоматическая настройка SOCKS-прокси через `networksetup` с эскалацией привилегий через `osascript` (Touch ID / пароль).
- **Автозапуск через launchd**: Генерация `.plist` в `~/Library/LaunchAgents/com.bobberdolle1.unbound.plist` вместо Windows Task Scheduler.
- **Кроссплатформенные пути**: Конфиг перемещён из `%APPDATA%\Unbound` в `~/Library/Application Support/Unbound` (macOS) и `~/.config/Unbound` (Linux).
- **Discord Cache**: Очистка кэша теперь указывает на `~/Library/Application Support/discord/Cache` на macOS.
- **Детекция конфликтов**: macOS-специфичная проверка через `pgrep`/`pkill` (spoofdpi, v2ray, clash, shadowsocks, VPN).
- **Диагностика**: Проверка наличия SpoofDPI, доступности сетевых сервисов, прав администратора.
- **Graceful Shutdown**: При закрытии приложения SOCKS-прокси автоматически отключается через Wails `OnShutdown`, чтобы пользователь не потерял интернет.
- **CLI режим**: Headless-режим теперь работает и на macOS (без Windows-specific `AttachConsole`).

### Architectural Changes
- **`BypassProvider` Interface**: Унифицированный интерфейс для всех платформ. Автоподбор и healthcheck теперь работают через интерфейс, а не конкретный тип.
- **`BypassProviderWithCallbacks`**: Расширенный интерфейс для провайдеров с поддержкой callback'ов статуса и логов.
- **Build Tag Isolation**: Все платформенно-специфичные файлы изолированы через `//go:build windows` / `//go:build darwin` / `//go:build linux`. Кросс-компиляция не ломает другие платформы.
- **Конфигурация autostart**: `applyAutoStartSetting()` делегирована в платформенно-специфичные файлы (`config_windows.go`, `config_darwin.go`, `config_linux.go`).
- **Диагностика**: `diagnostics.go` содержит только общий тип `DiagnosticResult`. Реализации перенесены в `diagnostics_windows.go` и `diagnostics_darwin.go`.

### macOS Build
- Добавлен `macos/README.md` — полная документация модуля (зависимости, сборка, запуск, troubleshooting).
- Добавлен `macos/build.sh` — скрипт сборки macOS `.app` бандла (Intel, Apple Silicon, Universal).

### Fixed
- **Cross-platform compilation**: Windows, macOS (amd64/arm64), Linux код компилируется без ошибок в рамках своих build tags.
- **Health check**: Больше не ссылается на Windows-провайдер на других платформах.
- **Startup validator**: macOS больше не требует nfqws/dvtws; проверяет наличие spoofdpi (как warning, может быть в PATH).
- **Scanner**: Помечен как `//go:build windows`, не мешает кросс-компиляции.

## [1.0.5] — Unreleased
### Добавлено
- **OpenWRT-пакет (Unbound-WRT)**: Полная интеграция на уровне роутера — защита всей LAN без настройки клиентов.
  - Пакет `nfqws-unbound`: кросс-компиляция nfqws из zapret (bol-van), оптимизация `-Os` + strip для экономии flash.
  - `procd` init-скрипт с маппингом стратегий (multidisorder, split-tls, fake-ping, disorder+fake).
  - Правила `fw4/nftables`: перехват TCP 80/443 с `br-lan` в NFQUEUE 200, исключение RFC1918/broadcast.
  - UCI-конфиг по умолчанию в `/etc/config/unbound`.
  - `luci-app-unbound`: LuCI CBI-интерфейс — переключатель вкл/выкл, выбор стратегии, исключения доменов/IP.
  - Документация: сборка через OpenWrt SDK, установка `.ipk`, диагностика.

## [1.0.4] - 2026-04-07
### Добавлено
- **Русский интерфейс**: Полный перевод UI на русский язык — все кнопки, статусы, уведомления, настройки и сообщения об ошибках.
- **Улучшенный Автоподбор**: Расширенный список тестовых целей (YouTube, Discord, Instagram, Telegram, Twitter/X, RuTracker, NordVPN, Proton). Таймаут увеличен до 8 сек (аналог probe.trolling.website). HEAD-запросы для скорости. Умные веса: YouTube/Discord приносят больше очков.
- **Реальный LivePing**: Теперь тестирует сам YouTube и Discord вместо 1.1.1.1 — показывает реальный статус обхода DPI.
- **Расширенный детект конфликтов**: Обнаруживает ciadpi, ByeDPI, OpenVPN, Cloudflare WARP, ExpressVPN, NordVPN в дополнение к winws/goodbyedpi/nfqws.
- **Умный выход из Автоподбора**: Ранний выход если найден профиль, при котором работают и YouTube, и Discord одновременно.

### Исправлено
- **LivePing** больше не показывает пинг до 1.1.1.1 (что никак не связано с реальным DPI-обходом)
- **Конфликты** теперь отображаются на русском («⚠️ GoodbyeDPI запущен»)
- **Сообщения** о завершении конфликтующих процессов — на русском
- **Лог** теперь различает ошибки на русском (ключевые слова «ошибк», «запущ»)

### Изменено
- Версия приложения: `1.0.3` → `1.0.4`
- Интерфейс полностью на русском — основная аудитория RU

## [1.0.3] - 2026-03-23
### Added
- **Auto-Tune V2**: New parallel scanning engine for YouTube, Telegram, Discord, RuTracker, and Facebook.
- **System Health Check**: Built-in diagnostics for admin rights, process conflicts, and WinDivert status.
- **Discord Hygiene**: Option to auto-clean Discord cache on startup to prevent session poisoning.
- **TCP Timestamps**: System-wide toggle to improve compatibility with modern DPI bypass techniques.
- **Version Display**: Current app version now visible in Settings UI.
- **Full Kill**: Nuclear option to terminate all conflicting DPI bypass processes and reset drivers.

### Fixed
- **System Tray**: Fixed non-responsive menu items. Added `appicon.png` embedding for stable icon display on Windows.
- **Console Flashing**: All system calls now use `CREATE_NO_WINDOW`, eliminating black box flickering.
- **Window Management**: Improved "Show" from tray logic using `WindowUnminimise`.
- **Profiles**: Restored full list of 70+ presets from Zapret 2 reference materials.
- **Auto-Tune Stability**: Fixed log duplication and cancellation logic.
- **Launch Issues**: Fixed winws2.exe working directory and blob path resolution.
- **Build Errors**: Resolved circular dependencies and missing frontend exports.

### Changed
- **License**: Officially moved to **GNU GPL v3.0**.
- **UI**: Modernized Sketchy-style overlays for errors and warnings.
- **Architecture**: Improved provider management and status reporting.

## [1.0.1] - 2026-03-15
### Added
- **UAC Elevation**: Automatic request for administrator privileges on startup.
- **Task Scheduler**: Integration for silent auto-start with high privileges.
- **Unified Logging**: New scrollable "Dev Diary" for real-time engine feedback.

### Fixed
- **WinDivert Filters**: Fixed `--new` flags causing driver initialization errors on some Windows versions.
- **Asset Extraction**: Improved reliability of binary and Lua script extraction to `%APPDATA%`.

## [1.0.0] - 2026-02-28
### Added
- **Zapret 2 Integration**: Full migration to bol-van's Zapret 2 core with Lua-based desynchronization.
- **Doodle UI**: Complete redesign of the interface using hand-drawn sketchy aesthetics.
- **Multi-Engine Support**: Experimental support for Xray/VLESS and Shadowsocks.
- **Live Ping**: Real-time latency tracking for bypassed traffic.
- **Game Filter**: Optimized profiles for low-latency gaming (Discord Voice, Steam, etc.).

## [0.9.0] - 2026-01-10
### Added
- Initial implementation of the DPI Engine Orchestrator.
- Support for GoodbyeDPI and basic Zapret (v1) profiles.
- Automated hostlist synchronization from remote sources.
- System tray integration with status notifications.

---
*UNBOUND: Open source, community-driven, and ready for 2026.*
