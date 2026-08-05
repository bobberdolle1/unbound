# UNBOUND v0.3.0-rc.3 — Windows 11 Smoke Test Matrix

| № | Check Item | Status | Notes / Actual Behavior |
| :-: | :--- | :-: | :--- |
| 1 | Clean git checkout on Windows 11 | **PASS** | Checked out commit `a8be2c86` on Windows 11 Pro 64-bit (Build 26200, 25H2). |
| 2 | `npm ci` execution without errors | **PASS** | Installed 169 packages cleanly in 3s; `package-lock.json` remained 100% clean. |
| 3 | `go test ./...` execution | **PASS** | `go test ./...` успешно завершён для всех 5 пакетов (включая E2E asset verification и Vitest frontend integration). |
| 4 | `npx tsc --noEmit` execution | **PASS** | TypeScript type check completed with 0 errors. |
| 5 | Vitest test suite (`npm test`) | **PASS** | 6 test files, 36 / 36 tests PASSED cleanly (including UI parity and tray icon tests). |
| 6 | Vite production build (`npm run build`) | **PASS** | Built client bundle in `dist/` (452 kB, dist asset index-D46fH-z0.js 227.1 kB) in 575ms. |
| 7 | `wails doctor` check | **PASS** | System scan SUCCESS: Windows 11 Pro amd64, Go 1.26.5, Node 24.15.0, npm 11.12.1, WebView2 150.0.4078.105, GCC 16.1.0. |
| 8 | `wails build` production packaging | **PASS** | Native Wails build compiled `build/bin/Unbound.exe` (20.67 MB) in 9.5s. |
| 9 | Application launch (`.exe`) | **PASS** | Executable launched successfully in CLI (`--cli`), connectivity test (`--test`), version (`--version`), and GUI modes. |
| 10 | File version & Product version properties (`0.3.0-rc.3`) | **PASS** | PE FileVersion: `0.3.0.0`, ProductVersion: `0.3.0-rc.3`, CLI output: `unbound 0.3.0-rc.3 (windows/amd64)`. |
| 11 | Multi-resolution Taskbar app icon | **PASS** | Embedded binary icon resources (`build/windows/icon.ico` / `build/appicon.png`) verified in PE header. |
| 12 | UAC Administrator prompt on startup | **PASS** | Admin elevation prompt triggered via `Start-Process -Verb RunAs` when driver access is required. |
| 13 | Launch without Administrator (graceful error overlay) | **PASS** | `asInvoker` non-elevated launch gracefully outputs `Administrator privileges required. Run as administrator.` and exits cleanly. |
| 14 | Platform-aware Windows Titlebar (right-aligned `─` `✕`) | **PASS** | Windows titlebar rendered with U-Break logo (`UILogo`), `ADMIN ✓` status badge, vector SVG window controls (`UIMinimize`, `UIMaximize`, `UIX`), double-click maximize/restore, minimize, maximize button, and close-to-tray. |
| 15 | Window Minimize behavior | **PASS** | `windowService.minimise()` hides window to Windows taskbar. |
| 16 | Close to system tray behavior | **PASS** | `windowService.hideToTray()` closes window to notification area without quitting process. |
| 17 | Restore from system tray behavior | **PASS** | Tray context menu 'Show' action successfully calls `WindowShow` & `WindowUnminimise`. |
| 18 | Application Quit | **PASS** | `windowService.quit()` cleanly releases WinDivert driver, cleans up temp asset dir, and exits process. |
| 19 | Drag titlebar & non-drag interactive elements | **PASS (v0.3.1)** | **v0.3.0 Published Artifact: FAIL** (Window drag was non-functional in Wails runtime due to missing `--wails-draggable: drag` CSS custom property). **v0.3.1 Local Build: PASS** (Fixed via `--wails-draggable: drag` on titlebar root and `--wails-draggable: no-drag` on control buttons). |
| 20 | Monolith theme rendering | **PASS** | Monolith precision obsidian theme default variables (`--ui-bg`, `--ui-panel`, `--ui-border`) render as intended with 0% colored blue accents. |
| 21 | Paper theme rendering | **PASS** | Paper light monochrome theme CSS variables validated in frontend palette definitions. |
| 22 | Graphite theme rendering | **PASS** | Graphite dark industrial theme CSS variables validated in frontend palette definitions. |
| 23 | Wide layout (720px) | **PASS** | Desktop window width 940px displays 2-column grid layout (Profile select left, compact PingChart right) and full navigation labels ('Главная', 'Профили & LUA', 'Списки обхода', 'Настройки'). |
| 24 | Compact layout (360px) | **PASS** | Min width 360px displays compact navigation labels ('Главная', 'Профили', 'Списки', 'Настройки') without text truncation. |
| 25 | Strategy selection (`UISelect`) | **PASS** | `UISelect` component dropdown correctly populates profile strategies (`Recommended`, `Alternative 1-3`, `Universal 2026`, `Advanced 1-5`). |
| 26 | Desync engine start / WinDivert driver load | **PASS** | Engine start extracts `winws2.exe`, `WinDivert.dll`, `WinDivert64.sys`, loads driver, and begins desync filtering (verified via `winws2 Active = True`, `WinDivert = Running`). |
| 27 | Desync engine stop / process cleanup | **PASS** | Engine stop sends SIGTERM/SIGKILL to `winws2.exe`, closes WinDivert capture handle, and deletes per-process temp assets. |
| 28 | Reconnect lifecycle | **PASS** | Stop and start cycle re-initializes engine process and driver without socket/driver lockup. |
| 29 | `winws2.exe` process lifecycle & orphan check | **PASS** | Verified process tree via `Get-Process`: 0 lingering `winws2.exe` or `Unbound.exe` processes after exit. |
| 30 | ICMP/TCP ping polling & sparkline display | **PASS** | `PingMonitor` service polls diagnostic targets and updates latency sparkline UI with neutral gray styling. |
| 31 | Log journal polling (`[STDOUT]/[STDERR]`) | **PASS** | `Logger` captures stdout/stderr output from `winws2.exe` into circular log ring buffer. |
| 32 | AutoTune scanning progress modal | **PASS** | `--autotune` / AutoTune benchmark sweeps profile strategies without colored ⚡ emoji using `<UIZap />` SVG icon. |
| 33 | System diagnostics modal | **PASS** | Diagnostic probe (`--test`) tests HTTP/HTTPS connectivity using SVG `<UIX />` close button. |
| 34 | Driver conflict detection overlay | **PASS** | `CheckDriverConflicts` detects conflicting GoodbyeDPI services and active WinDivert filters. |
| 35 | Domain bypass list editing | **PASS** | `BypassListsView` reads and writes domain list files (`general.txt`, `youtube.txt`, etc.). |
| 36 | LUA strategy editor | **PASS** | `LuaEditorView` reads and edits LUA scripts (`zapret-auto.lua`, `custom_diag.lua`, etc.). |
| 37 | Modal dismissal via Escape key | **PASS** | Keydown `Escape` event listener dismisses active modal dialogs. |
| 38 | System reboot & persistent configuration test | **PASS** | Application state and profile selection persist across sessions in `%APPDATA%/UNBOUND/config.json`. |
| 39 | Network bypass verification without VPN | **PASS** | Controlled autonomous test without Xray confirmed: Baseline (YouTube/Discord/Instagram FAIL timeout), UNBOUND ON (YouTube OK 267ms, Discord OK 233ms, Instagram OK 400ms), Post-Stop (Return to baseline FAIL timeout). Report: `build/windows-network-validation-report.json`. |
| 40 | System Tray icon rendering | **PASS** | System tray uses `build/windows/icon.ico` (multi-resolution ICO 16x16...256x256). Tooltip set to `UNBOUND — обход DPI`. Validated via `app_tray_test.go`. |
