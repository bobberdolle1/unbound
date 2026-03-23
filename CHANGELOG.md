# Changelog - UNBOUND ClearFlow Engine

## [1.0.3] - 2026-03-23
### Added
- **Auto-Tune V2**: New parallel scanning engine for YouTube, Telegram, Discord, RuTracker, and Facebook.
- **System Health Check**: Built-in diagnostics for admin rights, process conflicts, and WinDivert status.
- **Discord Hygiene**: Option to auto-clean Discord cache on startup to prevent session poisoning.
- **TCP Timestamps**: System-wide toggle to improve compatibility with modern DPI bypass techniques.
- **Version Display**: Current app version now visible in Settings.
- **Full Kill**: Nuclear option to terminate all conflicting DPI bypass processes and reset drivers.

### Fixed
- **Console Flashing**: All system calls now use `CREATE_NO_WINDOW`, eliminating black box flickering.
- **Profiles**: Restored full list of 70+ presets from Zapret 2 reference materials.
- **Auto-Tune Stability**: Fixed log duplication and cancellation logic.
- **Launch Issues**: Fixed winws2.exe working directory and blob path resolution.
- **Build Errors**: Resolved circular dependencies and missing frontend exports.

### Changed
- **License**: Officially moved to GNU GPL v3.0.
- **UI**: Modernized Sketchy-style overlays for errors and warnings.
- **Architecture**: Improved provider management and status reporting.

---
*UNBOUND: Open source, community-driven, and ready for 2026.*
