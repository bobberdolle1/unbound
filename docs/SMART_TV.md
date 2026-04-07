# Unbound for Smart TVs — Implementation Guide

This document covers the Smart TV expansion of the Unbound DPI bypass engine, bringing censorship circumvention directly to LG WebOS and Apple tvOS devices **without requiring a configured router**.

## Overview

| Platform | Minimum OS | Root Required | Engine | UI Framework |
|----------|-----------|---------------|--------|--------------|
| **LG WebOS** | 4.0+ (2018+) | Yes (webosbrew) | nfqws (NFQUEUE) | Enact (React) |
| **Apple tvOS** | 17.0+ (2023+) | No | tpws (SOCKS proxy) | SwiftUI |

## Architecture Comparison

### WebOS (Rooted LG TVs)

```
┌─────────────────────────────────────────┐
│           Enact Frontend                │
│    (React UI with D-pad navigation)     │
└──────────────────┬──────────────────────┘
                   │ Luna Service calls
                   ▼
┌─────────────────────────────────────────┐
│    webosbrew Root Execution Service     │
│      (org.webosbrew.hbchannel)          │
└──────────────────┬──────────────────────┘
                   │ Shell commands
                   ▼
┌─────────────────────────────────────────┐
│         unbound-service.sh              │
│     (Engine management daemon)          │
└───────┬──────────────────┬──────────────┘
        │                  │
        ▼                  ▼
┌───────────────┐  ┌──────────────────────┐
│    nfqws      │  │   iptables rules     │
│  (C binary)   │  │  NFQUEUE → port 200  │
└───────┬───────┘  └──────────┬───────────┘
        │                     │
        └──────────┬──────────┘
                   ▼
          ┌────────────────┐
          │   YouTube DNS  │
          │   (port 443)   │
          └────────────────┘
```

**Key Points:**
- Uses **netfilter NFQUEUE** for transparent packet interception
- All HTTPS traffic to YouTube domains routed through userspace engine
- Root access via webosbrew homebrew channel
- Boot-time initialization via `/var/lib/webosbrew/init.d/`

### tvOS (Apple TV)

```
┌─────────────────────────────────────────┐
│          SwiftUI Frontend               │
│   (Elegant toggle with focus engine)    │
└──────────────────┬──────────────────────┘
                   │ NEVPNManager API
                   ▼
┌─────────────────────────────────────────┐
│       NEPacketTunnelProvider            │
│      (NetworkExtension framework)       │
└──────────────────┬──────────────────────┘
                   │ Virtual interface
                   ▼
┌─────────────────────────────────────────┐
│        UnboundTunnelEngine              │
│    (tpws adapted for tvOS ARM64)        │
└──────────────────┬──────────────────────┘
                   │ SOCKS proxy on 127.0.0.1:1993
                   ▼
          ┌────────────────┐
          │   YouTube DNS  │
          │   (port 443)   │
          └────────────────┘
```

**Key Points:**
- Uses **NEPacketTunnelProvider** (available in tvOS 17+)
- Creates virtual network interface, all traffic routed through tunnel
- No root required — uses official Apple API
- tpws engine runs in SOCKS proxy mode

## Quick Start

### WebOS (LG TV)

**Prerequisites:**
- Rooted WebOS TV (via RootMyTV)
- webosbrew Homebrew Channel installed
- WebOS NDK installed (`/opt/webos-sdk-x86_64`)

**Build:**
```bash
# Build nfqws engine (Linux/macOS)
cd webos/native/nfqws
make WEBOS_SDK_PATH=/opt/webos-sdk-x86_64 package

# Build Enact frontend
cd ../../
npm install && npm run build

# Package for webosbrew
ares-package ./dist com.unbound.app
```

**Install:**
```bash
# Transfer to TV
scp com.unbound.app_1.0.0_all.ipk root@<TV_IP>:/media/developer/

# Install via SSH
ssh root@<TV_IP>
luna-send-pub -n 1 luna://com.webos.appInstallService/dev/install \
  '{"id":"com.unbound.app","ipkUrl":"/media/developer/com.unbound.app_1.0.0_all.ipk"}'
```

### tvOS (Apple TV)

**Prerequisites:**
- Xcode 15+ with tvOS 17 SDK
- Apple Developer Account
- Apple TV running tvOS 17+

**Build:**
```bash
cd tvos/UnboundTV

# Build tpws engine (requires macOS)
./build-tvos.sh

# Build Swift app
swift build --arch arm64
```

**Install:**
1. Open in Xcode
2. Select Apple TV as run destination
3. Press ⌘R to build and run

## DPI Bypass Strategies

Both platforms support three profiles optimized for different censorship scenarios:

### Default Profile
```
--dpi-desync=split
--dpi-desync-pos=2
--dpi-desync-repeats=6
```
**Use case**: Most ISPs with basic DPI inspection

### Aggressive Profile
```
--dpi-desync=fake,split
--dpi-desync-pos=1,midsld
--dpi-desync-repeats=11
--dpi-desync-autottl
--fake-ttl=1
```
**Use case**: Stubborn DPI systems (Russia, China, Iran)

### Lite Profile
```
--dpi-desync=split
--dpi-desync-pos=2
--dpi-desync-repeats=3
```
**Use case**: Light censorship, minimal overhead

## Supported Services

Out of the box, the following services are unblocked:

| Service | WebOS | tvOS |
|---------|-------|------|
| YouTube | ✅ | ✅ |
| YouTube Music | ✅ | ✅ |
| YouTube TV | ✅ | ✅ |
| Google Video | ✅ | ✅ |

To add more services, edit the domain list:
- **WebOS**: `webos/native/nfqws/lists/youtube.txt`
- **tvOS**: `tvos/UnboundTV/UnboundTV/Resources/youtube.txt`

## Technical Deep Dive

### WebOS: nfqws and Netfilter

The WebOS implementation uses the **nfqws** engine from bol-van/zapret, which leverages Linux's netfilter subsystem:

1. **iptables** rules redirect matching packets to NFQUEUE 200
2. **nfqws** receives packets in userspace via `libnetfilter_queue`
3. **DPI bypass** techniques applied (split desync, fake packets, etc.)
4. **Modified packets** reinjected into the network stack

This approach is highly efficient because it's **transparent** — the TV's apps don't need any configuration.

### tvOS: tpws and Packet Tunnel

tvOS requires a different approach due to sandbox restrictions:

1. **NEPacketTunnelProvider** creates a virtual TUN interface
2. **All traffic** is routed through this interface
3. **tpws engine** processes packets in SOCKS proxy mode
4. **Modified packets** written back to the packet flow

The tpws engine is adapted from the existing iOS Theos project (`theos/unbound-legacy/engine/tpws/`), with:
- Daemonization code removed (tvOS extensions can't fork)
- syslog replaced with os.Logger
- epoll-shim adapted for tvOS kqueue

### Cross-Compilation

#### WebOS (ARMv7a)
```bash
# Toolchain: arm-webos-linux-gnueabi-gcc
# Sysroot: WebOS NDK (Yocto-based)
# CFLAGS: -march=armv7-a -mfpu=neon -mfloat-abi=softfp
```

#### tvOS (ARM64)
```bash
# Toolchain: Xcode clang
# SDK: AppleTVOS17.0.sdk
# CFLAGS: -target arm64-apple-tvos17.0
```

## Limitations

### WebOS
- **Requires root**: Only works on rooted TVs
- **System updates**: May break root access
- **Fast Boot**: Must be disabled for boot-time scripts
- **Early boot timing**: Network may not be ready immediately

### tvOS
- **No iptables**: Cannot use NFQUEUE; must use SOCKS proxy
- **Memory limits**: Extensions have strict memory constraints
- **Apple review**: NetworkExtension requires entitlement approval
- **Sandbox restrictions**: Limited filesystem access

## File Structure

```
Unbound/
├── webos/                          # LG WebOS implementation
│   ├── src/                        # Enact/React frontend
│   ├── services/                   # Shell service scripts
│   ├── native/nfqws/               # C engine build system
│   └── README.md                   # WebOS-specific docs
│
├── tvos/                           # Apple tvOS implementation
│   ├── UnboundTV/                  # Swift app + extensions
│   │   ├── UnboundTV/             # Main SwiftUI app
│   │   ├── PacketTunnel/          # NEPacketTunnelProvider
│   │   └── UnboundEngine/         # C engine wrapper
│   ├── build-tvos.sh              # Build script
│   └── README.md                  # tvOS-specific docs
│
└── docs/SMART_TV.md               # This file
```

## Future Roadmap

### Phase 1 (Current)
- ✅ WebOS Enact UI with D-pad navigation
- ✅ WebOS nfqws cross-compilation
- ✅ WebOS boot-time service
- ✅ tvOS SwiftUI interface
- ✅ tvOS PacketTunnelProvider skeleton
- ✅ tvOS tpws engine adaptation

### Phase 2
- [ ] Full packet manipulation in tvOS tunnel
- [ ] QUIC/UDP bypass support
- [ ] Auto-tune profile detection
- [ ] Custom domain list import (USB/network)
- [ ] Connection metrics dashboard
- [ ] nftables support (newer WebOS)

### Phase 3
- [ ] Samsung Tizen implementation
- [ ] Android TV / Google TV port
- [ ] Roku channel (limited by Roku OS)
- [ ] Unified build system

## Contributing

When adding features:

1. **Test on real hardware** — TV remotes behave differently than simulators
2. **Focus on D-pad navigation** — No mouse/touch on TVs
3. **Keep memory low** — TV hardware is resource-constrained
4. **Follow existing patterns** — Enact for WebOS, SwiftUI for tvOS

## Credits

- **bol-van/zapret** — Original DPI bypass engine
- **webosbrew** — WebOS homebrew platform
- **RootMyTV** — WebOS rooting exploit
- **Apple NetworkExtension** — tvOS tunnel framework
