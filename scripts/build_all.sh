#!/usr/bin/env bash
# ============================================================================
# UNBOUND — Master Build Script (Unix/macOS/Linux/WSL)
# ============================================================================
# Usage:
#   ./build_all.sh <target> [options]
#
# Targets:
#   windows          - Build the native Windows Wails application
#   darwin           - Build the native macOS Universal Wails application
#   linux            - Cross-build the Linux CLI for amd64
#   all              - Build Linux plus the native GUI target available on this host
#
# Options:
#   --debug          - Enable debug build mode
#   --clean          - Clean build artifacts before building
#   --version <ver>  - Override version string (default: from wails.json)
#   --help           - Show this help message
#
# Examples:
#   ./build_all.sh windows
#   ./build_all.sh linux --debug
#   ./build_all.sh all --clean --version 0.2.1
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
PROJECT_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
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

# go_ldflags injects the resolved version into the binary.
#
# resolve_version() already worked out the version (including --version), but
# nothing passed it to the compiler, so `--version 2.6.0` produced a binary that
# still reported the value compiled into engine.Version. With Actions
# unavailable this script is the release path, so the drift would ship.
go_ldflags() {
    echo "-s -w -X unbound/engine.Version=$(resolve_version)"
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


# ── Native/cross-platform builders ──────────────────────────────────────────
host_os() {
    case "$(uname -s)" in
        Darwin) echo darwin ;;
        MINGW*|MSYS*|CYGWIN*) echo windows ;;
        Linux) echo linux ;;
        *) echo unsupported ;;
    esac
}

build_windows() {
    local ver
    ver="$(resolve_version)"
    log_step "Building native Windows Wails application"
    if [ "$(host_os)" != "windows" ]; then
        log_error "Windows Wails builds must run natively on Windows"
        return 1
    fi
    require_cmd powershell.exe "Windows PowerShell"

    local args=()
    [ "$DEBUG_MODE" = true ] && args+=("-DebugBuild")
    UNBOUND_VERSION="$ver" powershell.exe -NoProfile -ExecutionPolicy Bypass \
        -File "$PROJECT_ROOT/scripts/build/build_windows.ps1" "${args[@]}"
    log_ok "Windows app built: $BUILD_DIR/bin/unbound.exe"
}

build_darwin() {
    local ver
    ver="$(resolve_version)"
    log_step "Building native macOS Universal Wails application"
    if [ "$(host_os)" != "darwin" ]; then
        log_error "macOS Wails builds must run natively on macOS"
        return 1
    fi

    local args=()
    if [ "$DEBUG_MODE" = true ]; then
        UNBOUND_VERSION="$ver" "$PROJECT_ROOT/scripts/build/build_darwin.sh" universal debug
    else
        UNBOUND_VERSION="$ver" "$PROJECT_ROOT/scripts/build/build_darwin.sh" universal
    fi
}

build_linux() {
    local ver
    ver="$(resolve_version)"
    log_step "Building Linux amd64 CLI"

    if [ "$DEBUG_MODE" = true ]; then
        GOARCH=amd64 UNBOUND_VERSION="$ver" "$PROJECT_ROOT/scripts/build/build_linux.sh" debug
    else
        GOARCH=amd64 UNBOUND_VERSION="$ver" "$PROJECT_ROOT/scripts/build/build_linux.sh"
    fi
    log_ok "Linux CLI built: $BUILD_DIR/bin/unbound-linux-amd64"
}

build_all() {
    log_step "Building every target available on this host"
    build_linux
    case "$(host_os)" in
        windows) build_windows ;;
        darwin) build_darwin ;;
        linux) log_info "Linux has no additional native release target" ;;
        *) log_error "Unsupported build host"; return 1 ;;
    esac
    log_ok "Host-supported builds complete"
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
