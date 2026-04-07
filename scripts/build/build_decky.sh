#!/usr/bin/env bash
# ============================================================================
# UNBOUND — Decky Loader Plugin Build Script (Steam Deck)
# ============================================================================
# Usage:
#   ./scripts/build/build_decky.sh [docker]
# ============================================================================

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(dirname "$(dirname "$SCRIPT_DIR")")"
DECKY_DIR="$PROJECT_ROOT/decky-plugin"
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

build_native() {
    if [ ! -d "$DECKY_DIR" ]; then
        log_error "decky-plugin/ directory not found"
        exit 1
    fi

    log_info "Building Decky Loader plugin..."

    if ! command -v npm &>/dev/null; then
        log_error "npm not found. Install Node.js first."
        exit 1
    fi

    cd "$DECKY_DIR"
    npm install
    npm run build 2>/dev/null || log_warn "npm run build not defined or failed"

    mkdir -p "$DIST_DIR/decky"
    cp -r "$DECKY_DIR"/* "$DIST_DIR/decky/" 2>/dev/null || true

    log_ok "Decky plugin: $DIST_DIR/decky/"
}

build_docker() {
    if ! command -v docker &>/dev/null; then
        log_error "docker not found"
        exit 1
    fi

    log_info "Building Decky plugin via Docker..."

    docker build \
        -t unbound-decky-builder \
        -f "$SCRIPT_DIR/../docker/Dockerfile.decky" \
        "$PROJECT_ROOT"

    mkdir -p "$DIST_DIR/decky"

    CONTAINER_ID=$(docker create unbound-decky-builder)
    docker cp "$CONTAINER_ID:/output/" "$DIST_DIR/decky/" 2>/dev/null || true
    docker rm "$CONTAINER_ID" >/dev/null

    log_ok "Decky plugin: $DIST_DIR/decky/"
}

if [ "$USE_DOCKER" = "docker" ]; then
    build_docker
else
    build_native
fi
