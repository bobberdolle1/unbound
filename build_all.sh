#!/usr/bin/env bash
# ============================================================================
# UNBOUND — Master Build Script (Unix/macOS/Linux/WSL)
# ============================================================================
# Usage:
#   ./build_all.sh <target> [options]
#
# Targets:
#   windows          - Build Windows GUI binary via Wails (requires Wine + Go)
#   darwin           - Build macOS binary via Wails
#   linux            - Build Linux CLI/GUI binary
#   linux-steamdeck  - Build for Steam Deck (Decky Loader plugin + binary)
#   android          - Build Android APK via Gradle
#   ios              - Build iOS/macOS universal binary
#   tvos             - Build tvOS binary
#   openwrt          - Build OpenWrt IPK package
#   webos            - Build LG webOS app
#   decky            - Build Decky Loader plugin only
#   magisk           - Build Magisk module ZIP
#   all              - Build all available targets
#
# Options:
#   --debug          - Enable debug build mode
#   --clean          - Clean build artifacts before building
#   --version <ver>  - Override version string (default: from wails.json)
#   --help           - Show this help message
#
# Examples:
#   ./build_all.sh windows
#   ./build_all.sh android --debug
#   ./build_all.sh all --clean --version 1.0.5
# ============================================================================

set -euo pipefail

# ── Colors ───────────────────────────────────────────────────────────────────
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
CYAN='\033[0;36m'
BOLD='\033[1m'
NC='\033[0m' # No Color

# ── Globals ──────────────────────────────────────────────────────────────────
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$SCRIPT_DIR"
BUILD_DIR="$PROJECT_ROOT/build"
DIST_DIR="$PROJECT_ROOT/dist"
DEBUG_MODE=false
CLEAN_BUILD=false
VERSION_OVERRIDE=""

# ── Logging helpers ──────────────────────────────────────────────────────────
log_info()  { echo -e "${CYAN}[INFO]${NC} $*"; }
log_ok()    { echo -e "${GREEN}[OK]${NC} $*"; }
log_warn()  { echo -e "${YELLOW}[WARN]${NC} $*"; }
log_error() { echo -e "${RED}[ERROR]${NC} $*" >&2; }
log_step()  { echo -e "\n${BOLD}━━━ $* ━━━${NC}"; }

# ── Version resolution ───────────────────────────────────────────────────────
resolve_version() {
    if [ -n "$VERSION_OVERRIDE" ]; then
        echo "$VERSION_OVERRIDE"
        return
    fi
    if command -v jq &>/dev/null && [ -f "$PROJECT_ROOT/wails.json" ]; then
        jq -r '.info.productVersion // "0.0.0"' "$PROJECT_ROOT/wails.json" 2>/dev/null || echo "0.0.0"
    else
        grep -oP '"productVersion":\s*"\K[^"]+' "$PROJECT_ROOT/wails.json" 2>/dev/null || echo "0.0.0"
    fi
}

# ── Prerequisite checks ──────────────────────────────────────────────────────
require_cmd() {
    if ! command -v "$1" &>/dev/null; then
        log_error "$1 is required but not found in PATH."
        log_info "Install: $2"
        return 1
    fi
}

# ── Clean ────────────────────────────────────────────────────────────────────
do_clean() {
    log_step "Cleaning build artifacts"
    rm -rf "$BUILD_DIR/bin" "$BUILD_DIR/bin-linux" "$BUILD_DIR/bin-darwin"
    rm -rf "$DIST_DIR"/*.zip "$DIST_DIR"/*.apk "$DIST_DIR"/*.ipa "$DIST_DIR"/*.ipk
    rm -rf "$PROJECT_ROOT/frontend/dist" "$PROJECT_ROOT/frontend/node_modules/.cache"
    log_ok "Clean complete"
}

# ── Frontend build ───────────────────────────────────────────────────────────
build_frontend() {
    log_step "Building frontend assets"
    require_cmd npm "https://nodejs.org/"
    if [ -d "$PROJECT_ROOT/frontend" ]; then
        pushd "$PROJECT_ROOT/frontend" >/dev/null
        npm install --include=dev
        npm run build
        popd >/dev/null
        log_ok "Frontend built"
    else
        log_warn "frontend/ directory not found, skipping"
    fi
}

# ── Windows (via Wails + Wine cross-compile or native) ───────────────────────
build_windows() {
    local ver
    ver="$(resolve_version)"
    log_step "Building Windows binary (wails)"
    require_cmd go "https://go.dev/dl/"
    require_cmd wails "go install github.com/wailsapp/wails/v2/cmd/wails@latest"

    build_frontend

    local debug_flag=""
    [ "$DEBUG_MODE" = true ] && debug_flag="-debug"

    wails build -platform windows/amd64 -clean -o "unbound.exe" $debug_flag

    local out="$BUILD_DIR/bin"
    mkdir -p "$DIST_DIR/unbound-v${ver}-win64"
    cp -f "$out/unbound.exe" "$DIST_DIR/unbound-v${ver}-win64/" 2>/dev/null || \
    cp -f "$BUILD_DIR/bin/unbound.exe" "$DIST_DIR/unbound-v${ver}-win64/" 2>/dev/null || true

    log_ok "Windows binary built: $DIST_DIR/unbound-v${ver}-win64/"
}

# ── macOS / Darwin (native, must run on Mac) ────────────────────────────────
build_darwin() {
    local ver
    ver="$(resolve_version)"
    log_step "Building macOS binary (wails)"
    require_cmd go "brew install go"
    require_cmd wails "go install github.com/wailsapp/wails/v2/cmd/wails@latest"

    build_frontend

    local debug_flag=""
    [ "$DEBUG_MODE" = true ] && debug_flag="-debug"

    wails build -platform darwin/universal $debug_flag

    log_ok "macOS app built: $BUILD_DIR/bin/Unbound.app"
}

# ── Linux (native or Docker) ─────────────────────────────────────────────────
build_linux() {
    local ver
    ver="$(resolve_version)"
    log_step "Building Linux binary"
    require_cmd go "https://go.dev/dl/"

    build_frontend

    local debug_flag=""
    [ "$DEBUG_MODE" = true ] && debug_flag="-tags debug"

    # Build CLI mode binary for Linux
    CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o "$BUILD_DIR/bin/unbound-linux" $debug_flag .

    log_ok "Linux binary built: $BUILD_DIR/bin/unbound-linux"
}

# ── Steam Deck (SteamOS + Decky plugin) ──────────────────────────────────────
build_linux_steamdeck() {
    build_linux
    build_decky
}

# ── Android (Gradle) ────────────────────────────────────────────────────────
build_android() {
    local ver
    ver="$(resolve_version)"
    log_step "Building Android APK"

    if [ -d "$PROJECT_ROOT/android" ]; then
        if command -v ./gradlew &>/dev/null; then
            pushd "$PROJECT_ROOT/android" >/dev/null
            local variant="assembleRelease"
            [ "$DEBUG_MODE" = true ] && variant="assembleDebug"
            ./gradlew $variant
            popd >/dev/null
        elif command -v gradle &>/dev/null; then
            pushd "$PROJECT_ROOT/android" >/dev/null
            gradle assembleRelease
            popd >/dev/null
        else
            log_error "Gradle wrapper or gradle CLI not found"
            return 1
        fi

        mkdir -p "$DIST_DIR"
        find "$PROJECT_ROOT/android" -name "*.apk" -exec cp -v {} "$DIST_DIR/" \; 2>/dev/null || true
        log_ok "Android APK(s) copied to: $DIST_DIR/"
    else
        log_warn "android/ directory not found, skipping"
    fi
}

# ── iOS / tvOS (must run on Mac with Xcode) ─────────────────────────────────
build_ios() {
    log_step "Building iOS/tvOS binaries"
    if [ -f "$PROJECT_ROOT/macos/build.sh" ]; then
        bash "$PROJECT_ROOT/macos/build.sh"
    fi
    if [ -f "$PROJECT_ROOT/tvos/build-tvos.sh" ]; then
        bash "$PROJECT_ROOT/tvos/build-tvos.sh"
    fi
    log_ok "Apple platform builds complete"
}

build_tvos() {
    log_step "Building tvOS binary"
    if [ -f "$PROJECT_ROOT/tvos/build-tvos.sh" ]; then
        bash "$PROJECT_ROOT/tvos/build-tvos.sh"
    else
        log_warn "tvos/build-tvos.sh not found"
    fi
}

# ── OpenWrt (Docker or native) ──────────────────────────────────────────────
build_openwrt() {
    local ver
    ver="$(resolve_version)"
    log_step "Building OpenWrt IPK package"

    if [ -d "$PROJECT_ROOT/openwrt" ]; then
        # Build the Go binary for mipsle (OpenWrt typical arch)
        require_cmd go "https://go.dev/dl/"
        GOOS=linux GOARCH=mipsle GOMIPS=softfloat go build \
            -o "$BUILD_DIR/bin/unbound-openwrt-mipsle" \
            -ldflags="-s -w" .

        # Package as IPK
        if command -v docker &>/dev/null; then
            docker build -t unbound-openwrt \
                --build-arg VERSION="$ver" \
                "$PROJECT_ROOT/openwrt/unbound-wrt/" 2>/dev/null || \
            log_warn "Docker IPK build failed (may require SDK setup)"
        fi

        mkdir -p "$DIST_DIR"
        log_ok "OpenWrt binary built: $BUILD_DIR/bin/unbound-openwrt-mipsle"
    else
        log_warn "openwrt/ directory not found, skipping"
    fi
}

# ── webOS (LG) ───────────────────────────────────────────────────────────────
build_webos() {
    log_step "Building webOS app"
    if [ -d "$PROJECT_ROOT/webos" ]; then
        pushd "$PROJECT_ROOT/webos" >/dev/null
        if command -v ares-package &>/dev/null; then
            ares-package -o "$BUILD_DIR/bin-webos" .
        elif command -v npm &>/dev/null && [ -f package.json ]; then
            npm install
            npm run build 2>/dev/null || log_warn "webos npm build script not defined"
        fi
        popd >/dev/null
        log_ok "webOS build complete"
    else
        log_warn "webos/ directory not found, skipping"
    fi
}

# ── Decky Loader Plugin ─────────────────────────────────────────────────────
build_decky() {
    log_step "Building Decky Loader plugin"
    if [ -d "$PROJECT_ROOT/decky-plugin" ]; then
        pushd "$PROJECT_ROOT/decky-plugin" >/dev/null
        require_cmd npm "https://nodejs.org/"
        npm install
        npm run build 2>/dev/null || log_warn "Decky plugin build script not found"
        popd >/dev/null

        mkdir -p "$DIST_DIR"
        if [ -f "$PROJECT_ROOT/scripts/build-decky.sh" ]; then
            bash "$PROJECT_ROOT/scripts/build-decky.sh"
        fi
        log_ok "Decky plugin built"
    else
        log_warn "decky-plugin/ directory not found, skipping"
    fi
}

# ── Magisk Module ────────────────────────────────────────────────────────────
build_magisk() {
    local ver
    ver="$(resolve_version)"
    log_step "Building Magisk module"

    if [ -d "$PROJECT_ROOT/magisk-module" ]; then
        mkdir -p "$DIST_DIR"
        local zip="$DIST_DIR/unbound-magisk-v${ver}.zip"

        pushd "$PROJECT_ROOT/magisk-module" >/dev/null
        if command -v zip &>/dev/null; then
            zip -r "$zip" . -x "*.git*"
        else
            tar -czf "$DIST_DIR/unbound-magisk-v${ver}.tar.gz" .
        fi
        popd >/dev/null

        log_ok "Magisk module: $zip"
    else
        log_warn "magisk-module/ directory not found, skipping"
    fi
}

# ── All targets ──────────────────────────────────────────────────────────────
build_all() {
    log_step "Building ALL available targets"
    echo -e "  ${YELLOW}•${NC} Linux"
    echo -e "  ${YELLOW}•${NC} OpenWrt"
    echo -e "  ${YELLOW}•${NC} Android"
    echo -e "  ${CYAN}•${NC} Decky Plugin"
    echo -e "  ${YELLOW}•${NC} Magisk Module"
    echo -e "  ${YELLOW}•${NC} webOS"

    build_linux
    build_openwrt
    build_android
    build_decky
    build_magisk
    build_webos

    # macOS/iOS only on Mac
    if [[ "$(uname -s)" == "Darwin" ]]; then
        echo -e "  ${YELLOW}•${NC} macOS"
        echo -e "  ${YELLOW}•${NC} iOS/tvOS"
        build_darwin
        build_ios
    fi

    log_ok "All builds complete"
}

# ── Usage / Help ─────────────────────────────────────────────────────────────
show_help() {
    sed -n '2,/^#$/s/^# \?//p' "$0"
}

# ── Main dispatch ────────────────────────────────────────────────────────────
main() {
    local target=""

    while [[ $# -gt 0 ]]; do
        case "$1" in
            --debug)      DEBUG_MODE=true; shift ;;
            --clean)      CLEAN_BUILD=true; shift ;;
            --version)    VERSION_OVERRIDE="$2"; shift 2 ;;
            --help|-h)    show_help; exit 0 ;;
            -*)           log_error "Unknown option: $1"; exit 1 ;;
            *)            target="$1"; shift ;;
        esac
    done

    if [ -z "$target" ]; then
        show_help
        exit 1
    fi

    [ "$CLEAN_BUILD" = true ] && do_clean

    case "$target" in
        windows)          build_windows ;;
        darwin|macos)     build_darwin ;;
        linux)            build_linux ;;
        linux-steamdeck|steamdeck) build_linux_steamdeck ;;
        android)          build_android ;;
        ios)              build_ios ;;
        tvos)             build_tvos ;;
        openwrt)          build_openwrt ;;
        webos)            build_webos ;;
        decky|decky-plugin) build_decky ;;
        magisk)           build_magisk ;;
        all)              build_all ;;
        *)
            log_error "Unknown target: $target"
            echo ""
            show_help
            exit 1
            ;;
    esac

    log_step "Build complete: $target"
    echo -e " ${GREEN}Output:${NC} $DIST_DIR/"
    echo -e " ${GREEN}Binaries:${NC} $BUILD_DIR/bin/"
}

main "$@"
