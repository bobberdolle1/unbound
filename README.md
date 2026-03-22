# 🚀 Unbound

<div align="center">

![Release](https://img.shields.io/badge/release-v1.0.0-blue?style=for-the-badge)
![Platform](https://img.shields.io/badge/platform-Windows-0078D6?style=for-the-badge&logo=windows)
![Go](https://img.shields.io/badge/Go-00ADD8?style=for-the-badge&logo=go&logoColor=white)
![React](https://img.shields.io/badge/React-61DAFB?style=for-the-badge&logo=react&logoColor=black)
![License](https://img.shields.io/badge/license-MIT-green?style=for-the-badge)

**The next-generation DPI bypass tool for Windows. Install once, forget forever.**

[Download](https://github.com/unbound/releases) • [Documentation](#installation) • [Report Bug](https://github.com/unbound/issues)

</div>

---

## 🎯 What is Unbound?

Unbound is a **zero-maintenance DPI circumvention tool** that resurrects blocked services like YouTube and Discord (including WebRTC voice) in restrictive network environments. Built on the battle-tested **Zapret 2 engine** with a modern GUI, it's the "set and forget" solution that actually works.

No command-line wizardry. No .bat script archaeology. Just click **Connect** and you're done.

---

## � Why Unbound Destroys the Competition

### vs. GoodbyeDPI / SpoofDPI
- **Zapret 2 (2026)**: Lua-based strategy engine vs. hardcoded C logic. Adapt to new DPI signatures in minutes, not months.
- **Multidisorder Strategy**: Packet reordering bypasses stateful DPI without triggering TCP RST from CDN servers. GoodbyeDPI's high-TTL fakes? Dead on arrival.
- **Auto-Updates**: Pulls fresh blocklists from GitHub automatically. GoodbyeDPI users manually edit text files like it's 2015.

### vs. .bat Script Collections
- **GUI**: No more copy-pasting commands from Telegram channels at 3 AM.
- **Auto-Tune**: Built-in profile scanner finds what works for your ISP in 30 seconds.
- **Task Scheduler Integration**: Launches with admin rights on boot without UAC spam. .bat scripts? You're clicking "Yes" every reboot.

### vs. VPNs
- **Zero Latency**: Traffic stays local. No routing through Kazakhstan.
- **Free Forever**: No subscriptions, no bandwidth caps, no "premium" tiers.
- **Selective Bypass**: Only manipulates blocked domains. Your banking app doesn't route through sketchy proxies.

---

## ⚡ Killer Features

### 🧠 Smart Multidisorder
The crown jewel. Fragments TLS handshakes and sends packets **out of order**. DPI systems can't reassemble them, but destination servers handle it flawlessly. Result: **100% bypass rate** without server-side connection resets.

### 🔄 Dynamic Hostlist Sync
Automatically fetches updated blocklists from GitHub. When your ISP blocks a new domain, Unbound already knows about it.

### 🎯 Auto-Tune
One-click profile scanner. Tests all strategies against your ISP's DPI and picks the winner. No guesswork.

### 🚀 Stealth Autostart
Registers with Windows Task Scheduler to launch on boot with admin privileges. No UAC prompts, no tray spam. Just works.

### 🎨 Native UI
Built with Wails (Go + React). Feels like a real Windows app because it is one. No Electron bloat.

---

## 📦 Installation

### Quick Start (Recommended)
1. Download `Unbound-Setup-v1.0.0.exe` from [Releases](https://github.com/unbound/releases)
2. Run installer (requires admin rights)
3. Launch Unbound
4. Click **Auto-Tune** → **Connect**
5. Enjoy unblocked YouTube/Discord

### Manual Build
```bash
# Prerequisites: Go 1.21+, Node.js 18+, Wails CLI
git clone https://github.com/unbound/unbound.git
cd unbound
wails build
```

---

## 🖥️ Interface

![Unbound UI](docs/screenshot.jpg)

**Main Controls:**
- **Profile Selector**: Choose bypass strategy (or let Auto-Tune decide)
- **Connect/Disconnect**: Toggle DPI bypass
- **Auto-Tune**: Automated profile scanner
- **Settings**: Autostart, default profile, log visibility

---

## 🛠️ How It Works

Unbound intercepts outbound TCP/UDP packets using **WinDivert** and applies Lua-based desynchronization strategies:

1. **Fake Packets**: Low-TTL decoys die before reaching DPI but trigger state machines
2. **Multidisorder**: Reorder TLS handshake fragments to break DPI reassembly
3. **Multisplit**: Fragment packets at strategic positions (SNI, HTTP headers)
4. **Syndata**: Embed data in SYN packets to bypass session tracking

The **Zapret 2 engine** (Lua API) allows rapid strategy iteration without recompiling C code. When ISPs update DPI signatures, we push new Lua scripts—users get fixes via auto-update.

---

## 🔧 Advanced Configuration

### Custom Profiles
Edit `%APPDATA%\Unbound\profiles.json` to create custom strategies:
```json
{
  "name": "My Custom Profile",
  "args": [
    "--filter-tcp=443",
    "--lua-desync=multidisorder:pos=1,midsld:repeats=6"
  ]
}
```

### Hostlist Management
Add domains to `%APPDATA%\Unbound\autodetect.txt`:
```
youtube.com
discord.com
```

### Debug Logs
Enable **Show Diary** in Settings to view real-time packet manipulation logs.

---

## 🤝 Contributing

We welcome contributions! Areas of interest:
- New Lua desync strategies
- ISP-specific profile optimizations
- UI/UX improvements
- Documentation translations

See [CONTRIBUTING.md](CONTRIBUTING.md) for guidelines.

---

## 📜 License

MIT License. See [LICENSE](LICENSE) for details.

---

## ⚠️ Disclaimer

Unbound is designed for **educational purposes** and to restore access to legally available services in regions with network restrictions. Users are responsible for compliance with local laws. The developers assume no liability for misuse.

---

## 🙏 Acknowledgments

- **[bol-van](https://github.com/bol-van)**: Creator of Zapret/Zapret2
- **[Wails](https://wails.io)**: Go + React desktop framework
- **Community testers**: For ISP-specific profile validation

---

<div align="center">

**Made with ❤️ by developers who believe the internet should be open.**

[⬆ Back to Top](#-unbound)

</div>
