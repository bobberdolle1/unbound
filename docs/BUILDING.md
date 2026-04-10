# Building UNBOUND v2.0.0

Полное руководство по сборке UNBOUND на всех поддерживаемых платформах.

## Quick Start

```bash
# Unix / macOS / WSL — собрать всё
./build_all.sh all

# Windows PowerShell — собрать Windows
.\build_all.ps1 windows

# Linux через Docker
./build_all.sh linux
```

---

## Table of Contents

- [Quick Start](#quick-start)
- [Prerequisites](#prerequisites)
  - [Global Tools](#global-tools)
  - [Docker (for isolated builds)](#docker-for-isolated-builds)
- [Master Build Scripts](#master-build-scripts)
  - [Unix / macOS / Linux / WSL](#unix--macos--linux--wsl)
  - [Windows (PowerShell)](#windows-powershell)
- [Platform-Specific Scripts](#platform-specific-scripts)
- [Docker-Based Cross-Compilation](#docker-based-cross-compilation)
- [CI/CD (GitHub Actions)](#cicd-github-actions)
- [Build Output Locations](#build-output-locations)
- [Troubleshooting](#troubleshooting)

---

## Quick Start

```bash
# Unix / macOS / WSL — build everything
./build_all.sh all

# Windows PowerShell — build Windows binary
.\build_all.ps1 windows

# Linux via Docker (clean, no host pollution)
./build_all.sh linux-docker
```

---

## Prerequisites

### Global Tools

| Tool | Purpose | Install |
|------|---------|---------|
| **Go 1.23+** | Core engine (Go) | <https://go.dev/dl/> |
| **Node.js 20+** | Frontend build (npm) | <https://nodejs.org/> |
| **Wails CLI** | Desktop GUI builds (Win/Mac) | `go install github.com/wailsapp/wails/v2/cmd/wails@latest` |
| **Docker** | Isolated cross-compilation | <https://www.docker.com/products/docker-desktop/> |
| **JDK 17** | Android builds | `apt install openjdk-17-jdk` / `brew install openjdk@17` |
| **Gradle** | Android builds | Bundled as `gradlew` in `android/` |
| **zip / tar** | Packaging | Pre-installed on most systems |

Verify your setup:

```bash
go version          # ≥ 1.23
node --version      # ≥ 20
wails version       # ≥ 2.11
docker --version    # any recent
```

### Docker (for Isolated Builds)

Docker-based builds guarantee a clean environment — no toolchain pollution on your host OS.

1. Install **Docker Desktop** (Windows/macOS) or **Docker Engine** (Linux).
2. Verify:

```bash
docker run hello-world
```

3. (Optional) Increase Docker memory to **4 GB+** for Android SDK builds.

Available Docker build images:

| Dockerfile | Target | Base Image |
|------------|--------|------------|
| `scripts/docker/Dockerfile.linux` | Linux x86_64 binary | `golang:1.23-bookworm` |
| `scripts/docker/Dockerfile.openwrt` | OpenWrt IPK (mipsel) | `openwrt/sdk:23.05` |
| `scripts/docker/Dockerfile.android` | Android APK | `ubuntu:22.04` + SDK |
| `scripts/docker/Dockerfile.decky` | Decky Loader plugin | `node:20-bookworm-slim` |

---

## Master Build Scripts

### Unix / macOS / Linux / WSL

**File:** `build_all.sh`

```bash
chmod +x build_all.sh

# Show all targets
./build_all.sh --help

# Build a single target
./build_all.sh windows
./build_all.sh linux
./build_all.sh android
./build_all.sh openwrt
./build_all.sh decky
./build_all.sh magisk
./build_all.sh webos
./build_all.sh ios
./build_all.sh tvos
./build_all.sh darwin

# Build via Docker (no local Go required)
./build_all.sh linux-docker       # not applicable on this script, use below
./build_all.sh openwrt docker

# Build ALL available targets
./build_all.sh all

# Options
./build_all.sh windows --debug     # debug build (no strip, with symbols)
./build_all.sh all --clean         # clean before building
./build_all.sh windows --version 2.0.0  # override version string
```

### Windows (PowerShell)

**File:** `build_all.ps1`

```powershell
# Show help
.\build_all.ps1 -Help

# Build targets
.\build_all.ps1 windows
.\build_all.ps1 linux-docker
.\build_all.ps1 openwrt-docker
.\build_all.ps1 android
.\build_all.ps1 all

# Options
.\build_all.ps1 windows -Debug
.\build_all.ps1 all -Clean -Version 2.0.0
```

> **Note:** On Windows, you may need to set execution policy:
> ```powershell
> Set-ExecutionPolicy -Scope CurrentUser RemoteSigned
> ```

---

## Platform-Specific Scripts

Individual scripts live in `scripts/build/`. Use these for targeted builds.

| Script | Target | Usage |
|--------|--------|-------|
| `scripts/build/build_windows.ps1` | Windows (Go binary) | `.\scripts\build\build_windows.ps1 [-Debug]` |
| `scripts/build/build_linux.sh` | Linux (Go binary) | `./scripts/build/build_linux.sh [debug]` |
| `scripts/build/build_android.sh` | Android (APK) | `./scripts/build/build_android.sh [debug]` |
| `scripts/build/build_openwrt.sh` | OpenWrt (binary or IPK) | `./scripts/build/build_openwrt.sh [docker]` |
| `scripts/build/build_decky.sh` | Decky plugin | `./scripts/build/build_decky.sh [docker]` |
| `scripts/build/build_magisk.sh` | Magisk module ZIP | `./scripts/build/build_magisk.sh` |

Each script is standalone and prints colored status output.

---

## Docker-Based Cross-Compilation

### Why Docker?

- **Isolation:** No toolchain installed on host OS.
- **Reproducibility:** Same build every time, on any machine.
- **Cross-compile:** Build Linux/mipsel on a Windows laptop.

### Using Docker Compose

```bash
# Build a single target
docker compose -f scripts/docker/docker-compose.build.yml build linux
docker compose -f scripts/docker/docker-compose.build.yml build android
docker compose -f scripts/docker/docker-compose.build.yml build openwrt
docker compose -f scripts/docker/docker-compose.build.yml build decky

# Build multiple targets in parallel
docker compose -f scripts/docker/docker-compose.build.yml build --parallel linux android

# Build all Docker targets
docker compose -f scripts/docker/docker-compose.build.yml build
```

### Using Individual Dockerfiles

```bash
# Linux binary
docker build -t unbound-linux-builder \
    --build-arg VERSION=2.0.0 \
    -f scripts/docker/Dockerfile.linux .

docker run --rm -v $(pwd)/build/bin-linux:/output unbound-linux-builder

# OpenWrt IPK
docker build -t unbound-openwrt-builder \
    --build-arg VERSION=2.0.0 \
    -f scripts/docker/Dockerfile.openwrt openwrt/unbound-wrt/

# Android APK
docker build -t unbound-android-builder \
    --build-arg VERSION=2.0.0 \
    -f scripts/docker/Dockerfile.android .

docker run --rm -v $(pwd)/dist:/output unbound-android-builder
```

---

## CI/CD (GitHub Actions)

### Automatic Builds

The workflow at `.github/workflows/main.yml` triggers on:

- **Push** to `main` / `master`
- **Pull requests** to `main` / `master`
- **Tags** (`v*`) → creates a GitHub Release
- **Manual dispatch** from the Actions tab

### Manual Dispatch

1. Go to **Actions** → **Build & Release** → **Run workflow**
2. Choose targets (comma-separated): `windows,linux,android`
3. Toggle **Create a GitHub Release?** if desired
4. Click **Run workflow**

### Artifacts

Each platform uploads its artifacts independently. After a successful run:

| Artifact Name | Contents |
|---------------|----------|
| `unbound-windows` | `unbound-vX.Y.Z-win64.zip` |
| `unbound-linux` | `unbound-vX.Y.Z-linux-amd64.tar.gz` |
| `unbound-macos` | `unbound-vX.Y.Z-macos-universal.zip` |
| `unbound-android` | `*.apk` files |
| `unbound-openwrt` | `*.tar.gz` (binary) / IPK packages |
| `unbound-decky` | `unbound-vX.Y.Z-decky-plugin.tar.gz` |
| `unbound-magisk` | `unbound-vX.Y.Z-magisk.zip` |

Artifacts are retained for **30 days** by default.

### Release on Tag

When you push a tag like `v2.0.0`:

```bash
git tag v2.0.0 && git push origin v2.0.0
```

The workflow automatically creates a **GitHub Release** with all platform artifacts attached.

---

## Build Output Locations

| Platform | Binary Path | Release Archive |
|----------|-------------|-----------------|
| Windows | `build/bin/unbound.exe` | `dist/unbound-vX.Y.Z-win64/` |
| Linux | `build/bin/unbound-linux` | `dist/` |
| macOS | `build/bin/Unbound.app` | `dist/` |
| OpenWrt | `build/bin/unbound-openwrt-mipsle` | `dist/openwrt/` |
| Android | `android/app/build/outputs/apk/` | `dist/*.apk` |
| Decky | `dist/decky/` | `dist/` |
| Magisk | `dist/unbound-magisk-vX.Y.Z.zip` | `dist/` |
| webOS | `build/bin-webos/` | — |

---

## Troubleshooting

### "wails: command not found"

```bash
go install github.com/wailsapp/wails/v2/cmd/wails@latest
# Ensure GOPATH/bin is in PATH:
export PATH="$HOME/go/bin:$PATH"  # Unix
$env:PATH = "$env:USERPROFILE\go\bin;$env:PATH"  # PowerShell
```

### Frontend build fails

```bash
cd frontend
rm -rf node_modules package-lock.json
npm install
npm run build
```

### Docker build permission denied

```bash
# Linux: add user to docker group
sudo usermod -aG docker $USER
newgrp docker
```

### Android SDK license not accepted

```bash
yes | sdkmanager --licenses
```

### Go module download errors

```bash
go clean -modcache
go mod download
go mod tidy
```

### macOS: "Unbound.app is damaged"

```bash
xattr -cr build/bin/Unbound.app
```

### Windows: PowerShell script execution blocked

```powershell
Set-ExecutionPolicy -Scope CurrentUser RemoteSigned
```

---

## Platform Notes

### Windows
- Requires **Administrator privileges** at runtime for DPI bypass functionality.
- Built via **Wails** (WebView2-based GUI).

### macOS
- Universal binary (Intel + Apple Silicon).
- Built via **Wails** on a macOS host.

### Linux
- CLI mode by default (`--cli` flag).
- Can be paired with a systemd service for auto-start.

### Steam Deck / SteamOS
- Use the **Decky Loader** plugin for in-game overlay.
- Native binary runs in Desktop mode.

### OpenWrt
- Targets `mipsle` (softfloat) architecture.
- Package the binary as an IPK using the OpenWrt SDK (Docker).

### Android
- Requires Android SDK + NDK.
- Use `build_android.sh debug` for a debuggable APK.

### webOS (LG TVs)
- Requires `ares-cli` from LG.
- Package and sideload via Developer Mode.

### tvOS (Apple TV)
- Requires Xcode + Apple Developer account.
- Build via `tvos/build-tvos.sh` on macOS.

### Magisk Module
- Installs binaries and scripts to `/data/adb/modules/`.
- Flashes via Magisk Manager or TWRP.
