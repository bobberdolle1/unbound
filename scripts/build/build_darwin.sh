#!/usr/bin/env bash
# Build native macOS release artifacts for UNBOUND.
# Usage: ./scripts/build/build_darwin.sh [amd64|arm64|universal] [debug]
# Environment: UNBOUND_VERSION=<override>

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"
PLATFORM="${1:-universal}"
MODE="${2:-}"

case "$PLATFORM" in
    amd64|arm64|universal) ;;
    *)
        echo "[ERROR] Unsupported platform: $PLATFORM (use amd64, arm64, or universal)" >&2
        exit 1
        ;;
esac

if [ -n "$MODE" ] && [ "$MODE" != "debug" ]; then
    echo "[ERROR] Unsupported mode: $MODE (leave empty for release, or pass debug)" >&2
    exit 1
fi

if [ "$(uname -s)" != "Darwin" ]; then
    echo "[ERROR] This build script must be run on macOS." >&2
    exit 1
fi

export PATH="$HOME/go/bin:/opt/homebrew/bin:/usr/local/bin:$PATH"

for command in go node npm wails codesign pkgbuild productbuild hdiutil; do
    if ! command -v "$command" >/dev/null 2>&1; then
        echo "[ERROR] Required tool '$command' is not installed or not in PATH." >&2
        exit 1
    fi
done

cd "$PROJECT_ROOT"
test -f engine/core_bin/darwin/tpws || {
    echo "[ERROR] macOS engine binary engine/core_bin/darwin/tpws is missing" >&2
    exit 1
}

VERSION="${UNBOUND_VERSION:-$(node -e "try { console.log(require('./wails.json').info.productVersion); } catch(e) { console.log('0.6.7'); }")}"

echo "==================================================="
echo "  🚀 BUILDING UNBOUND v$VERSION for macOS ($PLATFORM)"
echo "==================================================="

echo "[INFO] Generating high-resolution macOS iconset..."
if [ -f "build/appicon.png" ] && command -v sips >/dev/null 2>&1 && command -v iconutil >/dev/null 2>&1; then
    ICONSET_DIR="$(mktemp -d)/AppIcon.iconset"
    mkdir -p "$ICONSET_DIR"
    sips -z 16 16     build/appicon.png --out "$ICONSET_DIR/icon_16x16.png" >/dev/null 2>&1 || true
    sips -z 32 32     build/appicon.png --out "$ICONSET_DIR/icon_16x16@2x.png" >/dev/null 2>&1 || true
    sips -z 32 32     build/appicon.png --out "$ICONSET_DIR/icon_32x32.png" >/dev/null 2>&1 || true
    sips -z 64 64     build/appicon.png --out "$ICONSET_DIR/icon_32x32@2x.png" >/dev/null 2>&1 || true
    sips -z 128 128   build/appicon.png --out "$ICONSET_DIR/icon_128x128.png" >/dev/null 2>&1 || true
    sips -z 256 256   build/appicon.png --out "$ICONSET_DIR/icon_128x128@2x.png" >/dev/null 2>&1 || true
    sips -z 256 256   build/appicon.png --out "$ICONSET_DIR/icon_256x256.png" >/dev/null 2>&1 || true
    sips -z 512 512   build/appicon.png --out "$ICONSET_DIR/icon_256x256@2x.png" >/dev/null 2>&1 || true
    sips -z 512 512   build/appicon.png --out "$ICONSET_DIR/icon_512x512.png" >/dev/null 2>&1 || true
    iconutil -c icns "$ICONSET_DIR" -o build/darwin/iconfile.icns >/dev/null 2>&1 || true
    rm -rf "$ICONSET_DIR"
fi

echo "[INFO] Building frontend..."
(cd frontend && npm ci --no-audit --no-fund && npm run build)

LDFLAGS="-X unbound/engine.Version=$VERSION"
if [ "$MODE" != "debug" ]; then
    LDFLAGS="-s -w $LDFLAGS"
fi

WAILS_ARGS=(build -platform "darwin/$PLATFORM" -clean -ldflags "$LDFLAGS")
if [ "$MODE" = "debug" ]; then
    WAILS_ARGS+=(-debug)
fi

export CGO_LDFLAGS="-framework UniformTypeIdentifiers ${CGO_LDFLAGS:-}"
rm -rf build/bin
xattr -cr . 2>/dev/null || true
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

# Bundle tpws helper inside Contents/MacOS/ if not present
if [ -f "engine/core_bin/darwin/tpws" ] && [ ! -f "$STAGE_APP/Contents/MacOS/tpws" ]; then
    cp "engine/core_bin/darwin/tpws" "$STAGE_APP/Contents/MacOS/tpws"
    chmod 0755 "$STAGE_APP/Contents/MacOS/tpws"
    codesign --force -s - "$STAGE_APP/Contents/MacOS/tpws" 2>/dev/null || true
fi

echo "[INFO] Code-signing app bundle..."
xattr -cr "$STAGE_APP" 2>/dev/null || true

ENTITLEMENTS="build/darwin/entitlements.plist"
if [ -f "$ENTITLEMENTS" ]; then
    echo "[INFO] Deep codesigning with hardened runtime entitlements..."
    codesign --force --deep --options runtime --entitlements "$ENTITLEMENTS" --sign - "$STAGE_APP"
else
    codesign --force --deep --sign - "$STAGE_APP"
fi
codesign --verify --deep --strict "$STAGE_APP"

EXECUTABLE="$STAGE_APP/Contents/MacOS/Unbound"
if [ ! -f "$EXECUTABLE" ]; then
    EXECUTABLE="$(find "$STAGE_APP/Contents/MacOS" -type f -perm -111 ! -name "tpws" | head -1)"
fi
if [ -z "$EXECUTABLE" ] || [ ! -f "$EXECUTABLE" ]; then
    echo "[ERROR] App bundle has no executable" >&2
    exit 1
fi
"$EXECUTABLE" --version | grep -F "$VERSION" >/dev/null
"$EXECUTABLE" --list-profiles --json >/dev/null
DIST_DIR="$PROJECT_ROOT/dist"
RELEASE_DIR="$PROJECT_ROOT/release"
mkdir -p "$DIST_DIR" "$RELEASE_DIR"

PKG_NAME="unbound-v${VERSION}-macOS-Installer.pkg"
DMG_NAME="unbound-v${VERSION}-macos-${PLATFORM}.dmg"
ZIP_NAME="unbound-v${VERSION}-macos-${PLATFORM}.zip"

PKG_PATH="$DIST_DIR/$PKG_NAME"
DMG_PATH="$DIST_DIR/$DMG_NAME"
ZIP_PATH="$DIST_DIR/$ZIP_NAME"

rm -f "$PKG_PATH" "$DMG_PATH" "$ZIP_PATH"

echo "[INFO] 1/3: Building native macOS installer package: $PKG_NAME..."
"$SCRIPT_DIR/build_pkg.sh" "$STAGE_APP" "$PKG_PATH" "$VERSION"

echo "[INFO] 2/3: Packaging universal DMG: $DMG_NAME..."
DMG_STAGE="$(mktemp -d)"
cp -R "$STAGE_APP" "$DMG_STAGE/Unbound.app"
ln -s /Applications "$DMG_STAGE/Applications" 2>/dev/null || true
cp "$PKG_PATH" "$DMG_STAGE/Установить UNBOUND.pkg"

TMP_DMG="$(mktemp -d)/Unbound.dmg"
hdiutil create -volname "UNBOUND" -srcfolder "$DMG_STAGE" -ov -format UDZO "$TMP_DMG" >/dev/null
mv "$TMP_DMG" "$DMG_PATH"
rm -rf "$DMG_STAGE"

echo "[INFO] 3/3: Packaging universal ZIP archive: $ZIP_NAME..."
(cd "$STAGE_DIR" && ditto -c -k --sequesterRsrc --keepParent "Unbound.app" "$ZIP_PATH")
rm -rf "$STAGE_DIR"

# Sync artifacts to release/ as well for backward compatibility
cp "$PKG_PATH" "$RELEASE_DIR/$PKG_NAME"
cp "$DMG_PATH" "$RELEASE_DIR/$DMG_NAME"
cp "$ZIP_PATH" "$RELEASE_DIR/$ZIP_NAME"

echo ""
echo "==================================================="
echo "  ✅ ALL ARTIFACTS BUILT SUCCESSFULLY"
echo "==================================================="
ls -lh "$PKG_PATH" "$DMG_PATH" "$ZIP_PATH"

echo ""
echo "[INFO] Calculating SHA256 checksums..."
(cd "$DIST_DIR" && shasum -a 256 "$PKG_NAME" "$DMG_NAME" "$ZIP_NAME" | tee SHA256SUMS.txt)

# Update Homebrew Cask formula if present
CASK_FILE="$PROJECT_ROOT/Casks/unbound.rb"
if [ -f "$CASK_FILE" ]; then
    PKG_SHA256="$(shasum -a 256 "$PKG_PATH" | cut -d' ' -f1)"
    python3 -c "
cask_path = '$CASK_FILE'
with open(cask_path, 'r') as f:
    text = f.read()

import re
text = re.sub(r'version \".*?\"', 'version \"$VERSION\"', text)
text = re.sub(r'sha256 \".*?\"|sha256 :no_check', 'sha256 \"$PKG_SHA256\"', text)

with open(cask_path, 'w') as f:
    f.write(text)
print('[INFO] Updated Casks/unbound.rb with sha256: $PKG_SHA256')
"
fi
