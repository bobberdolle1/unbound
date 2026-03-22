# 🚀 UNBOUND

**Next-Gen DPI Bypass Engine with Auto-Tune Scanner & Multi-Protocol Support**

![Windows](https://img.shields.io/badge/Windows-0078D6?style=for-the-badge&logo=windows&logoColor=white)
![Go](https://img.shields.io/badge/Go-00ADD8?style=for-the-badge&logo=go&logoColor=white)
![React](https://img.shields.io/badge/React-20232A?style=for-the-badge&logo=react&logoColor=61DAFB)
![TypeScript](https://img.shields.io/badge/TypeScript-007ACC?style=for-the-badge&logo=typescript&logoColor=white)

---

## 🎯 What is UNBOUND?

**Unbound** is a premium GUI wrapper for **Zapret 2** (nfqws.exe + WinDivert) with automatic ISP-specific strategy detection. Forget manual parameter tuning — just press **CONNECT**.

### ⚡ Key Features

- **🧠 Smart Auto-Tune Scanner** — automatically tests all profiles with TLS certificate verification to protect against TSPU MITM
- **📝 Advanced Lua Editor** — write and save custom Zapret 2 scripts directly in the app
- **🔄 Auto-Update System** — GitHub API integration with glassmorphic notification toast
- **🎨 Premium Dark UI** — glassmorphic interface with real-time telemetry and live ping indicator
- **🔒 Zero-Zombie Engine** — proper WinDivert driver termination on close/minimize to tray
- **📊 Live Telemetry** — real-time engine monitoring with log filtering
- **🚀 Multi-Protocol Ready** — architecture supports Zapret 2, Xray VLESS/Reality, AmneziaWG (coming soon)
- **🔐 Smart Prober** — DPI interference detection via TLS handshake and TTFB measurements
- **📡 Live Ping Indicator** — real-time connection latency display (updates every 5s)
- **🖥️ Headless CLI Mode** — run on servers without GUI (`--cli --profile="Standard Split"`)
- **🌍 Cross-Platform Builds** — Linux/macOS CLI binaries via Makefile

---

## 📥 Installation

1. Download `unbound.exe` from [Releases](https://github.com/bobberdolle1/unbound/releases/latest)
2. Run **as Administrator** (required for WinDivert)
3. Press **CONNECT** or run **Auto-Tune**

> ⚠️ **Important:** Close all other DPI-bypass tools (GoodbyeDPI, Zapret CLI, etc.) before running Unbound

---

## 🎮 Usage

### Quick Start
1. Select a profile from the list (e.g., `Standard Split`)
2. Press the large **TAP TO CONNECT** button
3. Check access to blocked resources

### Auto-Tune (Recommended)
1. Press the **Auto-Tune** button (radar icon)
2. Wait 2-3 minutes while the system tests all profiles
3. Unbound automatically selects and starts the best option

### Advanced Lua Editor
1. Press the **Code** icon in the top right corner
2. Write your Lua script for Zapret 2
3. Press **Save & Apply** — the `Custom Profile` will be automatically selected
4. Script is saved to `%APPDATA%/Unbound/custom_profile.lua`

### Headless CLI Mode
```bash
# Windows
unbound.exe --cli --profile="Standard Split" --debug

# Linux (after cross-compile)
./unbound-cli-linux --cli --profile="Standard Split"
```

---

## 🛠️ Built-in Profiles

| Profile | Description |
|---------|-------------|
| **Standard Split** | Classic split at position 1 |
| **Fake Packets + BadSeq** | Fake packets + incorrect sequence |
| **Disorder** | Packet fragment reordering |
| **Split Handshake** | Split at middle of domain (midsld) |
| **Flowseal Legacy** | Compatibility with old Zapret versions |
| **Xray VLESS/Reality** | Subscription-based VLESS proxy (Sprint 5) |
| **AmneziaWG (VPN Mode)** | WireGuard with obfuscation (Sprint 5) |
| **Custom Profile** | Your own Lua script |

---

## 🏗️ Architecture

```
Unbound (Wails v2)
├── Go Backend
│   ├── engine/
│   │   ├── engine.go          # Unified DPIEngine interface
│   │   ├── prober.go          # Smart Prober with TLS cert verification
│   │   ├── orchestrator.go    # Multi-engine orchestrator
│   │   ├── subscription.go    # Xray vless:// URI parser & config generator
│   │   ├── updater.go         # GitHub API auto-update checker
│   │   ├── scanner.go         # Auto-Tune with Smart Prober integration
│   │   ├── list_manager.go    # Dynamic Discord/Telegram list fetcher
│   │   └── engines/
│   │       ├── xray.go        # Xray VLESS/Reality engine
│   │       └── amneziawg.go   # AmneziaWG engine stub
│   ├── app.go                 # Wails bindings + API methods
│   └── app_windows.go         # System Tray integration
│
└── React Frontend (TypeScript + Tailwind)
    └── src/
        └── App.tsx            # Dynamic modal: Lua/Xray/AmneziaWG
```

### 🔬 Smart Prober Technology

Smart Prober uses multi-level verification to detect DPI interference:

1. **TLS Handshake Verification** — validates TLS certificate authenticity
2. **Certificate Chain Validation** — compares against known CAs (DigiCert, Let's Encrypt, Google Trust Services)
3. **TTFB Measurement** — measures Time-To-First-Byte as quality metric
4. **Connection Reset Detection** — detects ECONNRESET from DPI
5. **MITM Detection** — protects against TSPU certificate spoofing

---

## 🔧 Building from Source

### Requirements
- Go 1.21+
- Node.js 18+
- Wails CLI v2.11.0+

### Commands
```bash
# Install dependencies
go mod download
cd frontend && npm install

# Dev mode
wails dev

# Production build (Windows GUI)
wails build -clean

# Run tests
.\scripts\run_qa_suite.ps1      # Full QA suite
go test ./...                    # Unit tests only
go run .\scripts\test_bypass_debug.go  # Real bypass test
```

Ready `unbound.exe` will appear in `build/bin/`

---

## 🐛 Troubleshooting

### "WinDivert Error/Binding Failure"
- Close all other DPI-bypass tools
- Restart Unbound as Administrator
- Check that WinDivert driver is not blocked by antivirus

### "Administrator/root privileges required"
- Run `unbound.exe` via Right-click → "Run as administrator"

### Profile doesn't work
- Try **Auto-Tune** — it will automatically find a working option
- Check logs in the bottom panel (Telemetry)
- Run `.\scripts\test_bypass_debug.go` to verify bypass effectiveness

### Auto-update notification not showing
- Check Settings → Enable Auto-Update Checks
- Manually check: Help → Check for Updates

---

## 📚 Documentation

- [Testing Guide](docs/TESTING.md) — QA procedures and test suite
- [Release Notes](docs/release-notes.txt) — version history

---

## � License

MIT License — do what you want, but without warranties.

---

## 🙏 Credits

- **[Zapret](https://github.com/bol-van/zapret)** — powerful DPI bypass engine
- **[Wails](https://wails.io)** — desktop GUI with Go + React
- **[WinDivert](https://reqrypt.org/windivert.html)** — low-level packet interception
- **[Xray-core](https://github.com/XTLS/Xray-core)** — VLESS/Reality protocol

---

## 🔗 Links

- [Releases](https://github.com/bobberdolle1/unbound/releases)
- [Issues](https://github.com/bobberdolle1/unbound/issues)
- [Zapret Documentation](https://github.com/bol-van/zapret)

---

**Made with 🔥 by bobberdolle1**
