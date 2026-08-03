# UNBOUND v0.3.0-rc.1 — Windows 11 Smoke Test Matrix

| № | Check Item | Status | Notes / Actual Behavior |
| :-: | :--- | :-: | :--- |
| 1 | Clean git checkout on Windows 11 | **NOT TESTED** | |
| 2 | `npm ci` execution without errors | **NOT TESTED** | |
| 3 | `go test ./...` execution | **NOT TESTED** | |
| 4 | `npx tsc --noEmit` execution | **NOT TESTED** | |
| 5 | Vitest test suite (`npm test`) | **NOT TESTED** | |
| 6 | Vite production build (`npm run build`) | **NOT TESTED** | |
| 7 | `wails doctor` check | **NOT TESTED** | |
| 8 | `wails build` production packaging | **NOT TESTED** | |
| 9 | Application launch (`.exe`) | **NOT TESTED** | |
| 10 | File version & Product version properties (`0.3.0-rc.1`) | **NOT TESTED** | |
| 11 | Multi-resolution Taskbar app icon | **NOT TESTED** | |
| 12 | UAC Administrator prompt on startup | **NOT TESTED** | |
| 13 | Launch without Administrator (graceful error overlay) | **NOT TESTED** | |
| 14 | Platform-aware Windows Titlebar (right-aligned `─` `✕`) | **NOT TESTED** | |
| 15 | Window Minimize behavior | **NOT TESTED** | |
| 16 | Close to system tray behavior | **NOT TESTED** | |
| 17 | Restore from system tray behavior | **NOT TESTED** | |
| 18 | Application Quit | **NOT TESTED** | |
| 19 | Drag titlebar & non-drag interactive elements | **NOT TESTED** | |
| 20 | Monolith theme rendering | **NOT TESTED** | |
| 21 | Paper theme rendering | **NOT TESTED** | |
| 22 | Graphite theme rendering | **NOT TESTED** | |
| 23 | Wide layout (720px) | **NOT TESTED** | |
| 24 | Compact layout (360px) | **NOT TESTED** | |
| 25 | Strategy selection (`UISelect`) | **NOT TESTED** | |
| 26 | Desync engine start / WinDivert driver load | **NOT TESTED** | |
| 27 | Desync engine stop / process cleanup | **NOT TESTED** | |
| 28 | Reconnect lifecycle | **NOT TESTED** | |
| 29 | `winws2.exe` process lifecycle & orphan check | **NOT TESTED** | |
| 30 | ICMP/TCP ping polling & sparkline display | **NOT TESTED** | |
| 31 | Log journal polling (`[STDOUT]/[STDERR]`) | **NOT TESTED** | |
| 32 | AutoTune scanning progress modal | **NOT TESTED** | |
| 33 | System diagnostics modal | **NOT TESTED** | |
| 34 | Driver conflict detection overlay | **NOT TESTED** | |
| 35 | Domain bypass list editing | **NOT TESTED** | |
| 36 | LUA strategy editor | **NOT TESTED** | |
| 37 | Modal dismissal via Escape key | **NOT TESTED** | |
| 38 | System reboot & persistent configuration test | **NOT TESTED** | |
