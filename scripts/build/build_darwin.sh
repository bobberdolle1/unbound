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

echo "[INFO] Generating high-resolution macOS iconset..."
if [ -f "build/appicon.png" ] && command -v sips >/dev/null 2>&1 && command -v iconutil >/dev/null 2>&1; then
    ICONSET_DIR="$(mktemp -d)/appicon.iconset"
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
DMG_STAGE="$(mktemp -d)"
cp -R "$STAGE_APP" "$DMG_STAGE/Unbound.app"
ln -s /Applications "$DMG_STAGE/Applications" 2>/dev/null || true
cat > "$DMG_STAGE/ОБЯЗАТЕЛЬНО К ПРОЧТЕНИЮ.txt" << 'EOF'
===================================================
  🍎 ИНСТРУКЦИЯ ПО УСТАНОВКЕ UNBOUND НА macOS
===================================================

1. Перетащите "Unbound.app" в папку "Applications" (Программы).

2. ПЕРВЫЙ ЗАПУСК (Apple Gatekeeper):
   Так как приложение является открытым программным обеспечением (Open Source),
   при первом запуске нажмите на Unbound.app ПРАВОЙ КНОПКОЙ МЫШИ (или Ctrl + клик)
   и выберите «Открыть» (Open).

   В появившемся окне macOS нажмите «Открыть» (Open).

3. При первом подключении система запросит пароль администратора РОНО ОДИН РАЗ,
   чтобы настроить правила сетевого файрвола pfctl.
   Все последующие подключения будут происходить мгновенно в фоне!
EOF

TMP_DMG="$(mktemp -d)/Unbound.dmg"
hdiutil create -volname "UNBOUND" -srcfolder "$DMG_STAGE" -ov -format UDZO "$TMP_DMG" >/dev/null
mv "$TMP_DMG" "$DMG_PATH"
rm -rf "$DMG_STAGE" "$STAGE_DIR"
echo "  - Zip Archive: $ZIP_PATH"
echo "  - Disk Image: $DMG_PATH"
