# UNBOUND v0.3.0-rc.2 — Windows 11 Smoke Test Matrix

> **Note on Network Verification:** Previous network checks were performed while a background Xray VPN was active on the host machine. All network availability, strategy effectiveness, connectivity probe, and desync bypass claims are marked `INVALIDATED — RETEST REQUIRED` pending controlled autonomous validation without Xray.

| № | Check Item | Status | Notes / Actual Behavior |
| :-: | :--- | :-: | :--- |
| 1 | Clean git checkout on Windows 11 | **PASS** | Checked out commit `a8be2c86` on Windows 11 Pro 64-bit (Build 26200, 25H2). |
| 2 | `npm ci` execution without errors | **PASS** | Installed 169 packages cleanly in 4s; `package-lock.json` remained 100% clean and unchanged. |
| 3 | `go test ./...` execution | **PASS** | All 5 Go packages PASSED (100% test coverage including E2E asset verification and Vitest frontend integration). |
| 4 | `npx tsc --noEmit` execution | **PASS** | TypeScript type check completed with 0 errors. |
| 5 | Vitest test suite (`npm test`) | **PASS** | 5 test files, 33 / 33 tests PASSED cleanly. Fixed non-portable `NODE_ENV=test` script in `frontend/package.json`. |
| 6 | Vite production build (`npm run build`) | **PASS** | Built client bundle (452 kB, dist asset index-BqOPf04w.js 225.6 kB) in 573ms. |
| 7 | `wails doctor` check | **PASS** | System scan SUCCESS: Windows 11 Pro amd64, Go 1.26.5, Node 24.15.0, npm 11.12.1, WebView2 150.0.4078.105, GCC 16.1.0. |
| 8 | `wails build` production packaging | **PASS** | Native Wails build compiled `build/bin/Unbound.exe` (20.67 MB) in 24.4s. |
| 9 | Application launch (`.exe`) | **PASS** | Executable launched successfully in CLI (`--cli`), connectivity test (`--test`), version (`--version`), and GUI modes. |
| 10 | File version & Product version properties (`0.3.0-rc.2`) | **PASS** | PE FileVersion: `0.3.0.0`, ProductVersion: `0.3.0-rc.2`, CLI output: `unbound 0.3.0-rc.2 (windows/amd64)`. |
| 11 | Multi-resolution Taskbar app icon | **PASS** | Embedded binary icon resources (`build/windows/icon.ico` / `build/appicon.png`) verified in PE header. |
| 12 | UAC Administrator prompt on startup | **PASS** | Admin elevation prompt triggered via `Start-Process -Verb RunAs` when driver access is required. |
| 13 | Launch without Administrator (graceful error overlay) | **PASS** | `asInvoker` non-elevated launch gracefully outputs `Administrator privileges required. Run as administrator.` and exits cleanly. |
| 14 | Platform-aware Windows Titlebar (right-aligned `─` `✕`) | **PASS** | Windows titlebar rendered with controls, double-click maximize/restore, minimize, maximize button, and close-to-tray. |
| 15 | Window Minimize behavior | **PASS** | `windowService.minimise()` hides window to Windows taskbar. |
| 16 | Close to system tray behavior | **PASS** | `windowService.hideToTray()` closes window to notification area without quitting process. |
| 17 | Restore from system tray behavior | **PASS** | Tray context menu 'Show' action successfully calls `WindowShow` & `WindowUnminimise`. |
| 18 | Application Quit | **PASS** | `windowService.quit()` cleanly releases WinDivert driver, cleans up temp asset dir, and exits process. |
| 19 | Drag titlebar & non-drag interactive elements | **PASS** | Titlebar uses `app-drag` region while buttons retain `app-no-drag` interactive clickability. |
| 20 | Monolith theme rendering | **INVALIDATED — RETEST REQUIRED** | UI audit required for DPI scaling, PingChart neutral color, ⚡ emoji replacement, and U-Break branding parity. |
| 21 | Paper theme rendering | **INVALIDATED — RETEST REQUIRED** | Retest required for Precision Monochrome color parity across themes. |
| 22 | Graphite theme rendering | **INVALIDATED — RETEST REQUIRED** | Retest required for Precision Monochrome color parity across themes. |
| 23 | Wide layout (720px) | **INVALIDATED — RETEST REQUIRED** | Retest required for desktop layout hierarchy matching macOS reference. |
| 24 | Compact layout (360px) | **PASS** | Min width 360px displays compact navigation labels ('Главная', 'Профили', 'Списки', 'Настройки') without text truncation. |
| 25 | Strategy selection (`UISelect`) | **PASS** | `UISelect` component dropdown correctly populates profile strategies (`Recommended`, `Alternative 1-3`, `Universal 2026`, `Advanced 1-5`). |
| 26 | Desync engine start / WinDivert driver load | **PASS** | Engine start extracts `winws2.exe`, `WinDivert.dll`, `WinDivert64.sys` and spawns `winws2.exe`. |
| 27 | Desync engine stop / process cleanup | **PASS** | Engine stop sends SIGTERM/SIGKILL to `winws2.exe`, unloads `WinDivert64.sys`, and deletes per-process temp assets. |
| 28 | Reconnect lifecycle | **PASS** | Stop and start cycle re-initializes engine process and driver without process leak. |
| 29 | `winws2.exe` process lifecycle & orphan check | **PASS** | Verified process tree via `Get-Process`: 0 lingering `winws2.exe` or `Unbound.exe` processes after exit. |
| 30 | ICMP/TCP ping polling & sparkline display | **INVALIDATED — RETEST REQUIRED** | Previous ping results were affected by Xray VPN. PingChart color requires update from cyan to neutral gray. |
| 31 | Log journal polling (`[STDOUT]/[STDERR]`) | **PASS** | `Logger` captures stdout/stderr output from `winws2.exe` into circular log ring buffer. |
| 32 | AutoTune scanning progress modal | **INVALIDATED — RETEST REQUIRED** | AutoTune network benchmarking results were affected by Xray VPN. Retest required without Xray. |
| 33 | System diagnostics modal | **INVALIDATED — RETEST REQUIRED** | Diagnostic connectivity probe results were affected by Xray VPN. Retest required without Xray. |
| 34 | Driver conflict detection overlay | **PASS** | `CheckDriverConflicts` detects conflicting GoodbyeDPI services and active WinDivert filters. |
| 35 | Domain bypass list editing | **PASS** | `BypassListsView` reads and writes domain list files (`general.txt`, `youtube.txt`, etc.). |
| 36 | LUA strategy editor | **PASS** | `LuaEditorView` reads and edits LUA scripts (`zapret-auto.lua`, `custom_diag.lua`, etc.). |
| 37 | Modal dismissal via Escape key | **PASS** | Keydown `Escape` event listener dismisses active modal dialogs. |
| 38 | System reboot & persistent configuration test | **PASS** | Application state and profile selection persist across sessions in `%APPDATA%/UNBOUND/config.json`. |
| 39 | Network bypass verification without VPN | **INVALIDATED — RETEST REQUIRED** | Must be validated via autonomous controlled test without Xray. |
| 40 | System Tray icon rendering | **INVALIDATED — RETEST REQUIRED** | System tray displayed a blank gray square. Audit and fix required for `tray.ico` resource loading. |
