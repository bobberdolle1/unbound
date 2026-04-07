#!/usr/bin/env bash
# ============================================================================
# UNBOUND — Android Standalone Build Script
# ============================================================================
# Usage:
#   ./scripts/build/build_android.sh [debug]
# ============================================================================

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(dirname "$(dirname "$SCRIPT_DIR")")"
DIST_DIR="$PROJECT_ROOT/dist"

RED='\033[0;31m'
GREEN='\033[0;32m'
CYAN='\033[0;36m'
NC='\033[0m'

log_info()  { echo -e "${CYAN}[INFO]${NC} $*"; }
log_ok()    { echo -e "${GREEN}[OK]${NC} $*"; }
log_error() { echo -e "${RED}[ERROR]${NC} $*" >&2; }
log_warn()  { echo -e "${YELLOW}[WARN]${NC} $*"; }

ANDROID_DIR="$PROJECT_ROOT/android"

if [ ! -d "$ANDROID_DIR" ]; then
    log_error "android/ directory not found"
    exit 1
fi

VARIANT="assembleRelease"
if [ "${1:-}" = "debug" ]; then
    VARIANT="assembleDebug"
    log_info "Building DEBUG APK"
fi

log_info "Building Android APK..."

cd "$ANDROID_DIR"

if [ -f "./gradlew" ]; then
    chmod +x ./gradlew
    ./gradlew $VARIANT
elif command -v gradle &>/dev/null; then
    gradle $VARIANT
else
    log_error "Gradle wrapper or gradle CLI not found"
    exit 1
fi

mkdir -p "$DIST_DIR"
find "$ANDROID_DIR" -name "*.apk" -exec cp -v {} "$DIST_DIR/" \;

log_ok "Android APK(s) copied to: $DIST_DIR/"
ls -lh "$DIST_DIR"/*.apk 2>/dev/null || true
