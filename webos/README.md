# Unbound for WebOS (LG Smart TVs)

DPI/censorship bypass engine for rooted LG WebOS TVs using the webosbrew homebrew platform.

## Architecture

```
Unbound WebOS App (Enact/React)
    │
    ├── UnboundPanel.js            # Main UI with D-pad navigation
    └── services/UnboundService.js # Luna service client
         │
         ▼
webosbrew Root Execution Service
    (org.webosbrew.hbchannel.service)
         │
         ▼
Shell Service Scripts
    ├── unbound-service.sh         # Engine management daemon
    └── unbound-init.sh            # Boot-time initialization
         │
         ▼
nfqws Engine (C binary)
    (Cross-compiled from bol-van/zapret)
         │
         ▼
iptables NFQUEUE Rules
    (Routes YouTube traffic through DPI bypass)
```

## How It Works

1. **User taps CONNECT** in the Enact UI (navigated via TV remote D-pad)
2. **UnboundService.js** calls the webosbrew root execution service
3. **Root service** starts the `nfqws` binary with profile-specific arguments
4. **iptables rules** redirect port 443 traffic to NFQUEUE 200
5. **nfqws** intercepts packets and applies DPI bypass techniques
6. **Modified packets** bypass DPI inspection, unblocking YouTube and other services

## Prerequisites

- **Rooted LG WebOS TV** (via RootMyTV or similar exploit)
- **webosbrew Homebrew Channel** installed
- **SSH access** to the TV (for deployment and debugging)
- **WebOS NDK** installed at `/opt/webos-sdk-x86_64` (for building nfqws)
- **Node.js 18+** and npm (for building the Enact frontend)

## Building

### Step 1: Build the nfqws Engine (Linux/macOS only)

The nfqws binary must be cross-compiled for WebOS ARM using the WebOS NDK:

```bash
cd webos/native/nfqws

# Option A: Using Make (requires WebOS NDK)
make WEBOS_SDK_PATH=/opt/webos-sdk-x86_64 package

# Option B: Using CMake
mkdir build && cd build
cmake -DCMAKE_TOOLCHAIN_FILE=/opt/webos-sdk-x86_64/1.0.g/sysroots/x86_64-webossdk-linux/usr/share/cmake/OEToolchainConfig.cmake ..
make
```

**Dependencies** (must be cross-compiled first):
- `libnetfilter_queue`
- `libnfnetlink`
- `libmnl`

The Makefile will automatically download and build these dependencies.

### Step 2: Build the Enact Frontend

```bash
cd webos
npm install
npm run build
```

This produces a packaged app in the `dist/` directory.

### Step 3: Package for webosbrew

```bash
# Install webOS CLI tools
npm install -g @webosose/ares-cli

# Package the app
ares-package ./dist com.unbound.app

# This creates: com.unbound.app_1.0.0_all.ipk
```

## Installing on TV

### Via SSH/SCP

```bash
# Transfer the IPK to the TV
scp com.unbound.app_1.0.0_all.ipk root@<TV_IP>:/media/developer/

# SSH into the TV
ssh root@<TV_IP>

# Install the app
luna-send-pub -n 1 luna://com.webos.appInstallService/dev/install '{"id":"com.unbound.app","ipkUrl":"/media/developer/com.unbound.app_1.0.0_all.ipk"}'

# Verify installation
ls -la /media/developer/apps/usr/palm/applications/com.unbound.app/
```

### Via webosbrew Homebrew Channel

If you have the Homebrew Channel installed, you can sideload apps through its interface.

## Setting Up Boot-Time Service

To ensure Unbound starts on TV boot:

```bash
# SSH into the TV
ssh root@<TV_IP>

# Copy the init script to webosbrew init.d
cp services/unbound-init.sh /var/lib/webosbrew/init.d/unbound
chmod +x /var/lib/webosbrew/init.d/unbound

# Copy the service script
cp services/unbound-service.sh /media/developer/apps/usr/palm/applications/com.unbound.app/services/
chmod +x /media/developer/apps/usr/palm/applications/com.unbound.app/services/unbound-service.sh
```

**Important**: Disable Fast Boot in TV settings to ensure init.d scripts run reliably.

## Usage

### Navigating the UI

The interface is fully navigable with the TV remote's D-pad:

1. **CONNECT/DISCONNECT** button — Main toggle (auto-focused)
2. **Profile buttons** — Select bypass strategy
3. **Settings** button — View engine info

### Profiles

| Profile | nfqws Arguments | Use Case |
|---------|----------------|----------|
| **Default** | `--dpi-desync=split --dpi-desync-pos=2 --dpi-desync-repeats=6` | Most ISPs |
| **Aggressive** | `--dpi-desync=fake,split --dpi-desync-pos=1,midsld --dpi-desync-repeats=11 --fake-ttl=1` | Stubborn DPI |
| **Lite** | `--dpi-desync=split --dpi-desync-pos=2 --dpi-desync-repeats=3` | Light censorship |

### Domain Lists

Edit the YouTube domain list to add/remove domains:
```
webos/native/nfqws/lists/youtube.txt
```

After editing, rebuild and reinstall the app.

## Troubleshooting

### App won't launch

- Check webOS Developer Mode is enabled
- Verify the app is installed correctly:
  ```bash
  luna-send-pub -n 1 luna://com.webos.applicationManager/dev/listApps
  ```

### nfqws fails to start

- Check root access is working:
  ```bash
  luna-send-pub -n 1 luna://org.webosbrew.hbchannel.service/exec '{"command":"whoami"}'
  ```
  Should return "root"

- Check nfqws binary exists and is executable:
  ```bash
  ls -la /media/developer/apps/usr/palm/applications/com.unbound.app/bin/nfqws
  ```

### iptables rules not applied

- Check iptables status:
  ```bash
  iptables -L -n -v
  ```
- Look for UNBOUND_CHAIN in the output

### YouTube still blocked

- Try the "Aggressive" profile
- Check nfqws logs:
  ```bash
  tail -f /var/log/messages | grep nfqws
  ```
- Verify the hostlist file path is correct

## Platform Limitations

### WebOS-Specific Considerations

1. **Root access required**: Unlike tvOS, WebOS requires a rooted TV to manipulate iptables
2. **Early boot execution**: init.d scripts run before network is ready; the script waits up to 30s
3. **Fast Boot interference**: TV's Fast Boot feature may skip init.d execution
4. **System updates**: May break root access; re-root after major updates

### Engine Choice: nfqws vs tpws

WebOS uses **nfqws** (netfilter queue) instead of tpws because:
- WebOS runs Linux with full iptables support
- Root access allows NFQUEUE manipulation
- More efficient than SOCKS proxy mode (transparent interception)
- Lower memory footprint (important for TV hardware)

## Development Notes

### Enact Framework

The UI uses the Enact framework, which provides:
- **Spotlight**: D-pad focus management
- **Moonstone**: TV-optimized UI components
- **Panels**: Navigation structure

Key concepts:
- Every focusable element needs a `spotlightId`
- Use `@enact/spotlight` for focus state management
- Test with actual TV remote (simulator D-pad ≠ real remote)

### Luna Service Communication

The app communicates with the system via Luna services:

```javascript
// Call root execution service
const bridge = new PalmServiceBridge();
bridge.call('org.webosbrew.hbchannel.service/exec', 
  JSON.stringify({ command: 'iptables -L' }), 
  (response) => { /* handle */ });
```

### Cross-Compilation Gotchas

1. **Sysroot mismatch**: WebOS NDK uses an older glibc (2.26); avoid newer C features
2. **Missing libraries**: Many standard libs not in SDK; build dependencies manually
3. **FPU settings**: WebOS TVs use `softfp` ABI, not `hardfp`
4. **Strip the binary**: `arm-webos-linux-gnueabi-strip` reduces binary size by ~60%

## File Structure

```
webos/
├── appinfo/
│   └── appinfo.json                 # WebOS app metadata
├── src/
│   ├── index.js                     # Entry point
│   ├── App.js                       # Root component with Spotlight
│   └── components/
│       ├── UnboundPanel.js          # Main UI panel
│       └── UnboundPanel.module.less # Styles
├── services/
│   ├── unbound-service.sh           # Management daemon
│   └── unbound-init.sh              # Boot-time setup
├── native/
│   └── nfqws/
│       ├── Makefile                 # Cross-compilation build
│       ├── CMakeLists.txt           # Alternative CMake build
│       └── lists/
│           └── youtube.txt          # Domain hostlist
├── web/
│   └── index.html                   # HTML entry point
├── package.json                     # npm dependencies
└── README.md                        # This file
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

- [ ] Add support for nftables (newer WebOS versions)
- [ ] Implement Luna service as proper Node.js daemon
- [ ] Add autotune profile detection
- [ ] Support custom hostlists via USB import
- [ ] Add connection metrics dashboard
- [ ] QUIC/UDP bypass support (currently TCP-only)
