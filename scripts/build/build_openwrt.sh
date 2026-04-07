#!/usr/bin/env bash
# ============================================================================
# UNBOUND — OpenWrt Package Build Script
# ============================================================================
# Usage:
#   ./scripts/build/build_openwrt.sh [docker]
#
# Without 'docker': builds Go binary for mipsel locally.
# With 'docker':   builds full IPK package via Docker SDK.
# ============================================================================

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(dirname "$(dirname "$SCRIPT_DIR")")"
BUILD_DIR="$PROJECT_ROOT/build/bin"
DIST_DIR="$PROJECT_ROOT/dist"

RED='\033[0;31m'
GREEN='\033[0;32m'
CYAN='\033[0;36m'
YELLOW='\033[1;33m'
NC='\033[0m'

log_info()  { echo -e "${CYAN}[INFO]${NC} $*"; }
log_ok()    { echo -e "${GREEN}[OK]${NC} $*"; }
log_error() { echo -e "${RED}[ERROR]${NC} $*" >&2; }
log_warn()  { echo -e "${YELLOW}[WARN]${NC} $*"; }

USE_DOCKER="${1:-}"

# ── Native binary cross-compile ─────────────────────────────────────────────
build_binary() {
    log_info "Cross-compiling OpenWrt binary (mipsel, softfloat)..."
    mkdir -p "$BUILD_DIR"

    cd "$PROJECT_ROOT"
    GOOS=linux GOARCH=mipsle GOMIPS=softfloat \
        go build -trimpath -ldflags="-s -w" \
        -o "$BUILD_DIR/unbound-openwrt-mipsle" ./...

    log_ok "Binary: $BUILD_DIR/unbound-openwrt-mipsle"
    ls -lh "$BUILD_DIR/unbound-openwrt-mipsle"
}

# ── Docker IPK build ────────────────────────────────────────────────────────
build_docker() {
    if ! command -v docker &>/dev/null; then
        log_error "docker not found. Install Docker first."
        exit 1
    fi

    log_info "Building OpenWrt IPK via Docker SDK..."

    OPENWRT_DIR="$PROJECT_ROOT/openwrt/unbound-wrt"
    if [ ! -d "$OPENWRT_DIR" ]; then
        log_warn "openwrt/unbound-wrt/ not found, building binary only"
        build_binary
        return
    fi

    docker build \
        --build-arg VERSION=1.0.4 \
        --build-arg ARCH=mipsel_24kc \
        -t unbound-openwrt-builder \
        -f "$SCRIPT_DIR/../docker/Dockerfile.openwrt" \
        "$OPENWRT_DIR"

    mkdir -p "$DIST_DIR/openwrt"

    # Extract IPKs
    CONTAINER_ID=$(docker create unbound-openwrt-builder)
    docker cp "$CONTAINER_ID:/bin/packages/" "$DIST_DIR/openwrt/" 2>/dev/null || true
    docker rm "$CONTAINER_ID" >/dev/null

    log_ok "OpenWrt packages: $DIST_DIR/openwrt/"
}

# ── Main ─────────────────────────────────────────────────────────────────────
if [ "$USE_DOCKER" = "docker" ]; then
    build_docker
else
    build_binary
fi
