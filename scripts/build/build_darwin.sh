#!/usr/bin/env bash
# Build the native macOS Wails application.
# Usage: ./scripts/build/build_darwin.sh [amd64|arm64|universal] [debug]
# Environment: UNBOUND_VERSION=<override>

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"
PLATFORM="${1:-universal}"
MODE="${2:-}"

case "$PLATFORM" in
    amd64|arm64|universal) ;;
    *) echo "[ERROR] Unsupported macOS platform: $PLATFORM" >&2; exit 2 ;;
esac
if [ -n "$MODE" ] && [ "$MODE" != "debug" ]; then
    echo "[ERROR] Unknown build mode: $MODE" >&2
    exit 2
fi
if [ "$(uname -s)" != "Darwin" ]; then
    echo "[ERROR] macOS Wails bundles must be built natively on macOS" >&2
    exit 1
fi
for command in go node npm wails codesign; do
    command -v "$command" >/dev/null 2>&1 || {
        echo "[ERROR] $command is required" >&2
        exit 1
    }
done

cd "$PROJECT_ROOT"
test -f engine/core_bin/darwin/tpws || {
    echo "[ERROR] Bundled Universal tpws is missing" >&2
    exit 1
}
VERSION="${UNBOUND_VERSION:-$(node -p "require('./wails.json').info.productVersion")}"

echo "[INFO] Building frontend..."
(cd frontend && npm ci --no-audit --no-fund && npm run build)

WAILS_ARGS=(build -platform "darwin/$PLATFORM" -clean -ldflags "-X unbound/engine.Version=$VERSION")
if [ "$MODE" = "debug" ]; then
    WAILS_ARGS+=(-debug)
fi

export CGO_LDFLAGS="-framework UniformTypeIdentifiers ${CGO_LDFLAGS:-}"
echo "[INFO] Building macOS $PLATFORM Wails app v$VERSION..."
wails "${WAILS_ARGS[@]}"

APP_PATH=""
for candidate in build/bin/unbound.app build/bin/Unbound.app; do
    if [ -d "$candidate" ]; then
        APP_PATH="$candidate"
        break
    fi
done
if [ -z "$APP_PATH" ]; then
    echo "[ERROR] Wails reported success but no app bundle was created" >&2
    exit 1
fi

STAGE_DIR="$(mktemp -d)"
STAGE_APP="$STAGE_DIR/Unbound.app"
cp -R "$APP_PATH" "$STAGE_APP"

xattr -cr "$STAGE_APP" 2>/dev/null || true
codesign --force -s - "$STAGE_APP/Contents/MacOS/"* 2>/dev/null || true
codesign --force -s - "$STAGE_APP"
codesign --verify --strict "$STAGE_APP"

EXECUTABLE="$(find "$STAGE_APP/Contents/MacOS" -type f -perm -111 | head -1)"
if [ -z "$EXECUTABLE" ]; then
    echo "[ERROR] App bundle has no executable" >&2
    exit 1
fi
"$EXECUTABLE" --version | grep -F "$VERSION" >/dev/null
"$EXECUTABLE" --list-profiles --json >/dev/null

RELEASE_DIR="$PROJECT_ROOT/release"
mkdir -p "$RELEASE_DIR"

ZIP_NAME="unbound-v${VERSION}-macos-${PLATFORM}.zip"
DMG_NAME="unbound-v${VERSION}-macos-${PLATFORM}.dmg"

ZIP_PATH="$RELEASE_DIR/$ZIP_NAME"
DMG_PATH="$RELEASE_DIR/$DMG_NAME"

rm -f "$ZIP_PATH" "$DMG_PATH"

echo "[INFO] Packaging $ZIP_NAME..."
(cd "$STAGE_DIR" && ditto -c -k --sequesterRsrc --keepParent "Unbound.app" "$ZIP_PATH")

echo "[INFO] Building macOS Disk Image $DMG_NAME..."
TMP_DMG="$(mktemp -d)/Unbound.dmg"
hdiutil create -volname "UNBOUND" -srcfolder "$STAGE_APP" -ov -format UDZO "$TMP_DMG" >/dev/null
mv "$TMP_DMG" "$DMG_PATH"

rm -rf "$STAGE_DIR"

echo "[OK] macOS app built, smoke-tested, and packaged:"
echo "  - App Bundle: $APP_PATH"
echo "  - Zip Archive: $ZIP_PATH"
echo "  - Disk Image: $DMG_PATH"
