# Unbound for tvOS (Apple TV)

DPI/censorship bypass engine for Apple TV running tvOS 17+.

## Architecture

```
UnboundTV (SwiftUI App)
    │
    ├── UnboundViewModel.swift     # State management
    ├── ContentView.swift          # Main UI with D-pad navigation
    └── SettingsView.swift         # Settings overlay
         │
         ▼
PacketTunnel (NetworkExtension)
    │
    ├── PacketTunnelProvider.swift # NEPacketTunnelProvider implementation
    └── UnboundTunnelEngine.swift  # Swift wrapper for C engine
         │
         ▼
UnboundEngine (C Library)
    │
    └── UnboundTunnelEngine.h      # C API for DPI bypass engine
         (Adapted from theos/unbound-legacy/engine/tpws)
```

## How It Works

1. **User taps CONNECT** in the SwiftUI app
2. **ViewModel** configures and starts the `NEPacketTunnelProvider`
3. **PacketTunnelProvider** creates a virtual network interface
4. **All traffic** is routed through this virtual interface
5. **UnboundEngine** (tpws) intercepts and manipulates packets to bypass DPI
6. **Modified packets** are sent to their destinations, bypassing censorship

## Prerequisites

- **Xcode 15+** with tvOS 17+ SDK
- **Apple Developer Account** (for NetworkExtension entitlements)
- **Apple TV** running tvOS 17 or later
- **macOS** for building (cross-compilation not supported)

## Building

### Option 1: Swift Package Manager (Recommended)

```bash
cd tvos/UnboundTV
swift build --arch arm64
```

### Option 2: Xcode

1. Open `UnboundTV.xcodeproj` in Xcode
2. Select the "UnboundTV" scheme (not PacketTunnel)
3. Build for your Apple TV device (⌘B)
4. Run on device (⌘R)

### Option 3: xcodebuild (CI/CD)

```bash
xcodebuild -project UnboundTV.xcodeproj \
  -scheme UnboundTV \
  -destination 'platform=tvOS,name=Apple TV' \
  -configuration Release \
  build
```

## Installing on Apple TV

### Development Mode

1. Enable Developer Mode on your Apple TV:
   - Settings → Remotes and Devices → Remote App and Devices
   - Pair with Xcode via Network

2. In Xcode, select your Apple TV as the run destination
3. Run the app (⌘R)

### Distribution (TestFlight/App Store)

1. Archive the project: Product → Archive
2. Distribute via TestFlight or direct IPA installation
3. Requires NetworkExtension entitlement approval from Apple

## Configuration

### Profiles

Three DPI bypass profiles are available:

| Profile | Description | Use Case |
|---------|-------------|----------|
| **Default** | Balanced split desync (6 repeats) | Most ISPs |
| **Aggressive** | Fake + split with auto-TTL (11 repeats) | Stubborn DPI systems |
| **Lite** | Minimal split desync (3 repeats) | Light censorship |

### Domain Lists

The engine uses domain lists to target specific services. Edit the YouTube domain list in:
```
tvos/UnboundTV/UnboundTV/Resources/youtube.txt
```

## Platform Limitations

### tvOS Sandbox Restrictions

Unlike the WebOS implementation (which has root access), tvOS apps run in a strict sandbox:

- **No iptables access**: Cannot use NFQUEUE-based `nfqws` like WebOS
- **Packet Tunnel API only**: Must use `NEPacketTunnelProvider` for traffic interception
- **Local proxy mode**: Uses `tpws` in SOCKS proxy mode instead of transparent mode

### Engine Adaptation

The original `nfqws` (Linux netfilter queue) engine is replaced with:
- **tpws** (transparent proxy with SOCKS mode)
- Adapted from `theos/unbound-legacy/engine/tpws/`
- Cross-compiled for tvOS ARM64 using Xcode toolchain

## Troubleshooting

### Tunnel won't start

- Check that NetworkExtension entitlement is enabled
- Verify the app is signed with a provisioning profile that includes the entitlement
- Check Console.app for PacketTunnel extension logs

### YouTube still blocked

- Try the "Aggressive" profile
- Verify DNS settings (use 8.8.8.8 to avoid ISP DNS hijacking)
- Check that all YouTube domains are in the hostlist file

### Performance issues

- Switch to "Lite" profile for lower overhead
- Check active connection count in Settings
- Restart the tunnel if memory pressure is high

## Development Notes

### Adapting tpws for tvOS

The existing tpws engine from the iOS Theos project is adapted:

1. **Remove daemonization code** (tvOS extensions can't fork/daemonize)
2. **Replace syslog** with os.Logger (Apple unified logging)
3. **Remove PID file management** (not needed in extension sandbox)
4. **Adapt epoll-shim** for tvOS (uses kqueue on Darwin)
5. **Link against NetworkExtension** framework

### Memory Constraints

tvOS extensions have strict memory limits:
- Keep connection pool small (max 100 concurrent)
- Release buffers promptly
- Monitor memory in Xcode Debug Navigator

### Testing

- Use the tvOS Simulator for UI testing
- Test actual tunnel functionality on physical Apple TV
- Use Network Link Conditioner to simulate slow networks

## File Structure

```
tvos/UnboundTV/
├── Package.swift                      # Swift Package Manager config
├── UnboundTV/                         # Main app target
│   ├── UnboundTVApp.swift            # @main entry point
│   ├── ContentView.swift             # Main UI view
│   ├── UnboundViewModel.swift        # Connection state manager
│   ├── SettingsView.swift            # Settings overlay
│   └── Resources/
│       └── Assets.xcassets/          # App icons and images
├── PacketTunnel/                      # NetworkExtension target
│   ├── PacketTunnelProvider.swift    # VPN tunnel implementation
│   ├── UnboundTunnelEngine.swift     # Swift C wrapper
│   ├── Info.plist                    # Extension configuration
│   └── Resources/
└── UnboundEngine/                     # C library target
    ├── include/
    │   └── UnboundTunnelEngine.h     # Public C API
    ├── UnboundEngine.h               # Umbrella header
    ├── module.modulemap              # Clang module definition
    └── tpws/                         # Adapted tpws source
        ├── tpws.c
        ├── tpws_conn.c
        ├── helpers.c
        └── ... (other tpws source files)
```

## Comparison with Other Platforms

| Feature | WebOS (rooted) | tvOS (sandboxed) |
|---------|----------------|------------------|
| **Engine** | nfqws (NFQUEUE) | tpws (SOCKS proxy) |
| **Traffic interception** | iptables NFQUEUE | NEPacketTunnelProvider |
| **Root required** | Yes (webosbrew) | No |
| **System-wide** | Yes | Yes (when tunnel active) |
| **Build toolchain** | WebOS NDK (Yocto) | Xcode (clang) |
| **UI framework** | Enact (React) | SwiftUI |

## Future Work

- [ ] Implement full packet manipulation in PacketTunnelProvider
- [ ] Add QUIC/UDP bypass support (currently TCP-only)
- [ ] Integrate auto-tune profile selection
- [ ] Add connection metrics dashboard
- [ ] Support custom domain lists beyond YouTube
