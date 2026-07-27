#!/usr/bin/env bash
# ============================================================================
# UNBOUND — Master Build Script (Unix/macOS/Linux/WSL)
# ============================================================================
# Usage:
#   ./build_all.sh <target> [options]
#
# Targets:
#   windows          - Build Windows GUI binary via Wails
#   darwin           - Build macOS binary via Wails
#   linux            - Build Linux CLI/GUI binary
#   all              - Build all desktop targets (Windows, Linux, macOS)
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
#   ./build_all.sh all --clean --version 0.1.0-refresh
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

    wails build -platform windows/amd64 -clean -o "unbound.exe" $debug_flag \
        -ldflags "-X unbound/engine.Version=${ver}"

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

    wails build -platform darwin/universal $debug_flag \
        -ldflags "-X unbound/engine.Version=${ver}"

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
    CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -trimpath \
        -ldflags="$(go_ldflags)" -o "$BUILD_DIR/bin/unbound-linux" $debug_flag .

    log_ok "Linux binary built: $BUILD_DIR/bin/unbound-linux"
}

# ── All targets ──────────────────────────────────────────────────────────────
build_all() {
    log_step "Building ALL desktop targets"
    echo -e "  ${CYAN}•${NC} Linux"
    build_linux

    if command -v wails &>/dev/null; then
        echo -e "  ${CYAN}•${NC} Windows"
        build_windows
    fi

    if [[ "$(uname -s)" == "Darwin" ]]; then
        echo -e "  ${CYAN}•${NC} macOS"
        build_darwin
    fi

    log_ok "Desktop builds complete"
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
