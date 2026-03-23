# Changelog - UNBOUND ClearFlow Engine

All notable changes to this project will be documented in this file.

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
