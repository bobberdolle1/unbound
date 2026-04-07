#!/usr/bin/env bash
# ============================================================================
# UNBOUND — Linux Standalone Build Script
# ============================================================================
# Usage:
#   ./scripts/build/build_linux.sh [debug]
# ============================================================================

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(dirname "$(dirname "$SCRIPT_DIR")")"
BUILD_DIR="$PROJECT_ROOT/build/bin"

RED='\033[0;31m'
GREEN='\033[0;32m'
CYAN='\033[0;36m'
NC='\033[0m'

log_info()  { echo -e "${CYAN}[INFO]${NC} $*"; }
log_ok()    { echo -e "${GREEN}[OK]${NC} $*"; }
log_error() { echo -e "${RED}[ERROR]${NC} $*" >&2; }

DEBUG_FLAG=""
if [ "${1:-}" = "debug" ]; then
    DEBUG_FLAG="-gcflags='all=-N -l'"
    log_info "Building in DEBUG mode"
fi

log_info "Building Linux binary..."

mkdir -p "$BUILD_DIR"

# Build CLI binary
cd "$PROJECT_ROOT"
go build -trimpath $DEBUG_FLAG -o "$BUILD_DIR/unbound-linux" ./...

log_ok "Linux binary built: $BUILD_DIR/unbound-linux"
ls -lh "$BUILD_DIR/unbound-linux"
