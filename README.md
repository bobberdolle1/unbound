# UNBOUND

Ultimate DPI bypass engine combining Zapret 2, GoodbyeDPI, and cross-platform support.

## Features

- **Multi-Engine Support**: Zapret (Linux/macOS/Windows), GoodbyeDPI (Windows)
- **Cross-Platform**: Windows, Linux, macOS, Android (Magisk module)
- **Modern UI**: React + TailwindCSS + Framer Motion
- **Multiple Profiles**: Optimized for Discord, YouTube, Telegram, and more

## Platform Setup

### Windows
1. Download latest release
2. Run as Administrator
3. Select engine (GoodbyeDPI or Zapret 2) and profile
4. Click Start

### Linux
```bash
# Install dependencies
sudo apt install libnetfilter-queue-dev iptables

# Download nfqws from zapret releases
# Place in engine/core_bin/linux/

# Run with sudo
sudo ./unbound
```

### macOS
```bash
# Install dependencies
brew install libnetfilter-queue

# Download nfqws from zapret releases
# Place in engine/core_bin/macos/

# Run with sudo
sudo ./unbound
```

### Android (Magisk)
1. Flash `android/magisk/unbound-magisk.zip` via Magisk Manager
2. Configure via terminal: `su -c unbound-config`
3. Reboot

## Build from Source

### Prerequisites
- Go 1.23+
- Node.js 18+
- Wails v2: `go install github.com/wailsapp/wails/v2/cmd/wails@latest`

### Build
```bash
# Install frontend dependencies
cd frontend && npm install && cd ..

# Build for current platform
wails build

# Build for specific platform
wails build -platform windows/amd64
wails build -platform linux/amd64
wails build -platform darwin/universal
```

## Engines

### Zapret (nfqws)
Advanced DPI bypass using packet manipulation, NFQUEUE, and Lua scripting.

### GoodbyeDPI (Windows)
WinDivert-based DPI bypass optimized for Windows.

## Profiles

- **Ultimate Bypass**: Multi-strategy for maximum compatibility
- **Discord Voice Optimized**: Low-latency UDP optimization
- **YouTube QUIC Aggressive**: QUIC/HTTP3 bypass
- **Telegram API Bypass**: Optimized for Telegram protocols
- **Standard HTTPS/QUIC**: Basic HTTPS + QUIC bypass
- **HTTP + HTTPS Split**: Split packet strategy

## Credits

- [Zapret](https://github.com/bol-van/zapret) by bol-van
- [GoodbyeDPI](https://github.com/ValdikSS/GoodbyeDPI) by ValdikSS
- Built with [Wails](https://wails.io)

## License

MIT License - see LICENSE file

**Note**: Zapret and GoodbyeDPI are separate projects with their own licenses.
