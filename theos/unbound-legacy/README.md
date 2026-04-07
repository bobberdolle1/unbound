# Unbound Legacy

> DPI/censorship bypass for jailbroken iOS devices.
> Supports **iOS 6.1.3** (iPhone 4s, skeuomorphic UI) and **iOS 15+** (ARM64, modern flat UI) in a single DEB package.

## Architecture

```
┌──────────────────────────────────────────────────┐
│              Unbound Legacy                       │
├────────────────┬─────────────────────────────────┤
│  Application   │ Skeuomorphic (iOS 6) / Modern   │
│                │ Objective-C, CoreGraphics, UIKit│
├────────────────┼─────────────────────────────────┤
│  Proxy Manager │ SCPreferences API -- SOCKS      │
│                │ proxy injection, no reboot      │
├────────────────┼─────────────────────────────────┤
│  Tweak         │ Cydia Substrate / ElleKit hooks │
│                │ SpringBoard + Preferences       │
├────────────────┼─────────────────────────────────┤
│  C Engine      │ Ported from bol-van/zapret      │
│  (tpws)        │ epoll→kqueue shim, ARMv7+ARM64  │
├────────────────┼─────────────────────────────────┤
│  Launch Daemon │ com.unbound.tpws.plist           │
│                │ Auto-start, KeepAlive            │
└────────────────┴─────────────────────────────────┘
```

## Project Structure

```
theos/unbound-legacy/
├── Makefile                          # Main Theos build (dual-arch)
├── control                           # DEB metadata
├── README.md                         # This file
│
├── engine/
│   ├── Makefile.tpws                 # tpws cross-compilation
│   └── tpws/
│       ├── darwin_compat.h           # epoll→kqueue, signalfd, timerfd shim
│       ├── tpws.h                    # tpws public API
│       ├── ios_main.c                # iOS daemon entry point
│       ├── Entitlements.xml          # ldid entitlements for binary
│       ├── epoll-shim/
│       │   ├── include/sys/epoll.h   # epoll API declarations
│       │   └── src/epoll_shim.c      # Full kqueue-backed implementation
│       └── macos/
│           ├── net/pfvar.h           # PF NAT lookup structures
│           └── sys/socket.h          # Darwin socket options
│
├── unboundApp/                       # Application
│   ├── UnboundAppDelegate.h/m        # Version-based UI routing
│   ├── UnboundProxyManager.h/m       # Shared proxy/daemon manager
│   ├── iOS6/                         # Skeuomorphic UI
│   │   ├── UnboundSkeuomorphicViewController.h/m
│   │   ├── UnboundLinenBackgroundView.h/m
│   │   ├── UnboundLeatherPanelView.h/m
│   │   ├── UnboundGlossyButton.h/m
│   │   └── UnboundSkeuomorphicSwitch.h/m
│   └── Modern/                       # Modern iOS 15+ UI
│       └── UnboundModernViewController.h/m
│
├── unboundTweak/
│   ├── Tweak.xm                      # Logos tweak (SpringBoard + Preferences)
│   └── UnboundProxyManager.m         # Tweak proxy manager (non-ARC)
│
├── layout/                           # Files installed to device
│   ├── DEBIAN/
│   │   ├── postinst                  # Post-install (PF rules, permissions)
│   │   └── prerm                     # Pre-remove cleanup
│   ├── Library/LaunchDaemons/
│   │   └── com.unbound.tpws.plist
│   └── Applications/Unbound.app/
│       ├── Info.plist
│       └── Entitlements.plist
│
└── scripts/
    ├── build.sh                      # Bash build (Linux/macOS/WSL)
    └── build.ps1                     # PowerShell build (Windows+WSL)
```

## Building

### Prerequisites

1. **Theos** -- https://theos.dev/docs/installation
2. **iOS 6.1 SDK** -- place at `$THEOS/sdks/iPhoneOS6.1.sdk`
3. **iOS 16.4+ SDK** -- usually bundled with Theos
4. **clang** with ARM cross-compilation
5. **ldid** (optional, for device signing)

### Quick Build

```bash
cd theos/unbound-legacy

# Both architectures
./scripts/build.sh

# Specific architecture
./scripts/build.sh armv7   # iOS 6 only
./scripts/build.sh arm64   # Modern iOS only

# Clean
./scripts/build.sh clean

# Deploy
./scripts/build.sh install 192.168.1.100
```

### Windows (via WSL)

```powershell
cd theos\unbound-legacy
.\scripts\build.ps1
.\scripts\build.ps1 -Deploy -DeviceIP 192.168.1.100
```

### Manual

```bash
export THEOS=$HOME/theos
cd theos/unbound-legacy
make -f engine/Makefile.tpws   # engine first
make package                    # then everything
```

Output: `packages/com.unbound.legacy_1.0.0_iphoneos-arm.deb`

## Installation

```bash
scp packages/com.unbound.legacy_1.0.0_iphoneos-arm.deb root@device:/tmp/
ssh root@device "dpkg -i /tmp/com.unbound.legacy_1.0.0_iphoneos-arm.deb"
```

## Usage

### From the App
1. Open **Unbound** on the device
2. Toggle **Engine** on
3. Configure port (default: 1993) and strategy
4. System SOCKS proxy is auto-set to `127.0.0.1:1993`

### From Preferences (iOS 6)
The tweak injects an "Unbound Legacy" entry into stock Settings.app

### From Darwin Notification
```objc
[[NSNotificationCenter defaultCenter] postNotificationName:@"com.unbound.toggle"
    object:nil userInfo:@{@"enabled": @YES, @"port": @1993}];
```

### CLI
```bash
unbound-tpws --port 1993        # start
killall unbound-tpws            # stop
cat /var/run/unbound-tpws.pid   # check PID
```

## How It Works

### tpws Engine
Port of bol-van/zapret `tpws` to iOS via:
- **epoll → kqueue** shim for async I/O
- **signalfd / timerfd / eventfd** stubs
- **SO_NOSIGPIPE** socket option for Darwin
- iOS daemon wrapper with PID file + signal handling

### Proxy Injection
Uses `SCPreferences` API (same as Apple's Settings app):
```objc
SCPreferencesRef ref = SCPreferencesCreate(NULL, CFSTR("com.unbound.legacy"), NULL);
/* ... modify SOCKSProxy, SOCKSPort, HTTPProxy, etc. ... */
SCPreferencesCommitChanges(ref);
SCPreferencesApplyChanges(ref);  // applies without reboot!
```

### PF Redirect Rules (transparent mode)
```
rdr pass on lo0 inet proto tcp from any to any port 80  -> 127.0.0.1 port 1993
rdr pass on lo0 inet proto tcp from any to any port 443 -> 127.0.0.1 port 1993
```

## Skeuomorphic UI (iOS 6)

All textures drawn procedurally with Core Graphics -- **no image assets**:

| Component | Technique |
|-----------|-----------|
| Linen Background | Fine-line cross-hatch texture + noise grain |
| Leather Panels | Multi-stop gradient, grain, stitched border, gloss overlay |
| Glossy Buttons | Gradient fill + top-half gloss reflection + inner shadow |
| On/Off Switch | Green/grey split track + glossy knob + embossed labels |

## Compatibility

| Device | iOS | Arch | UI |
|--------|-----|------|-----|
| iPhone 4s | 6.1.3 | ARMv7 (32-bit) | Skeuomorphic |
| iPhone 5/5c | 6-10 | ARMv7/ARM64 | Skeuomorphic |
| iPhone 5s-11 | 7-14 | ARM64 | Modern (transitional) |
| iPhone 12+ | 15-17 | ARM64 | Modern |

## Troubleshooting

- **Engine won't start**: check `/var/log/unbound-tpws.log`
- **Proxy not applying**: verify Wi-Fi is active network; check Settings → Wi-Fi → HTTP Proxy
- **PF rules**: `pfctl -s info` to check; `pfctl -f /etc/unbound/pf.conf` to reload

## Credits

- **tpws**: [bol-van/zapret](https://github.com/bol-van/zapret)
- **Proxy API**: [karajan/iOS-ProxyTool](https://github.com/karajanyp/iOS-ProxyTool)
- **Theos**: [theos/theos](https://github.com/theos/theos)
