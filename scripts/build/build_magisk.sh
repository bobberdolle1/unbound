#!/usr/bin/env bash
# ============================================================================
# UNBOUND — Magisk Module Build Script
# ============================================================================
# Usage:
#   ./scripts/build/build_magisk.sh
# ============================================================================

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(dirname "$(dirname "$SCRIPT_DIR")")"
MAGISK_DIR="$PROJECT_ROOT/magisk-module"
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

if [ ! -d "$MAGISK_DIR" ]; then
    log_error "magisk-module/ directory not found"
    exit 1
fi

# Resolve version
VERSION="0.0.0"
if command -v jq &>/dev/null && [ -f "$PROJECT_ROOT/wails.json" ]; then
    VERSION=$(jq -r '.info.productVersion // "0.0.0"' "$PROJECT_ROOT/wails.json" 2>/dev/null || echo "0.0.0")
fi

log_info "Building Magisk module v$VERSION..."

mkdir -p "$DIST_DIR"

ZIP_NAME="unbound-magisk-v${VERSION}.zip"
ZIP_PATH="$DIST_DIR/$ZIP_NAME"

cd "$MAGISK_DIR"

if command -v zip &>/dev/null; then
    zip -r "$ZIP_PATH" . -x "*.git*" -x "*.DS_Store"
    log_ok "Magisk module: $ZIP_PATH"
    ls -lh "$ZIP_PATH"
elif command -v tar &>/dev/null; then
    TAR_PATH="$DIST_DIR/unbound-magisk-v${VERSION}.tar.gz"
    tar -czf "$TAR_PATH" .
    log_ok "Magisk module (tar.gz): $TAR_PATH"
    ls -lh "$TAR_PATH"
else
    log_error "zip or tar required to package the module"
    exit 1
fi
