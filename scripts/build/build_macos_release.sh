#!/usr/bin/env bash
# Build, sanitize, sign, package, and smoke-test one universal macOS release.
# Usage: scripts/build/build_macos_release.sh [version]
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
VERSION="${1:-$(node -p "require('$ROOT/wails.json').info.productVersion")}" 
DIST="$ROOT/release"
APP=""
WORK=""
SMOKE=""
PKG_SMOKE=""

require_version() {
  local expected="$1"
  test "$(node -p "require('$ROOT/wails.json').info.productVersion")" = "$expected"
  test "$(sed -n 's/^var Version = "\(.*\)"$/\1/p' "$ROOT/engine/version.go")" = "$expected"
}

unsafe_xattrs() {
  local bundle="$1"
  # xattr -r visits descendants but does not clear the bundle root itself.
  # Clear root and descendants explicitly.
  xattr -d com.apple.FinderInfo "$bundle" 2>/dev/null || true
  xattr -rd com.apple.FinderInfo "$bundle" 2>/dev/null || true
  xattr -d com.apple.ResourceFork "$bundle" 2>/dev/null || true
  xattr -rd com.apple.ResourceFork "$bundle" 2>/dev/null || true
}

find_bundle() {
  if test -d "$ROOT/build/bin/unbound.app"; then
    printf '%s\n' "$ROOT/build/bin/unbound.app"
  elif test -d "$ROOT/build/bin/Unbound.app"; then
    printf '%s\n' "$ROOT/build/bin/Unbound.app"
  else
    return 1
  fi
}

require_version "$VERSION"
rm -rf "$DIST" "$ROOT/build/bin"
mkdir -p "$DIST"

# Wails currently signs the generated app before callers can sanitize
# FileProvider-created FinderInfo. Keep the generated bundle only when its
# build reached packaging; then replace that failed ad-hoc signing step below.
set +e
(
  cd "$ROOT"
  wails build -platform darwin/universal -clean \
    -ldflags "-X unbound/engine.Version=$VERSION"
)
WAILS_STATUS=$?
set -e
RAW_APP="$(find_bundle)" || {
  echo "Wails produced no app bundle" >&2
  exit "$WAILS_STATUS"
}
# The project tree is under a macOS File Provider. Its metadata daemon can
# reattach FinderInfo under build/bin after removal, so sign and package a
# copied bundle in a non-FileProvider temporary directory.
WORK="$(mktemp -d)"
trap 'rm -rf "$WORK" "$DMG_STAGE" "$SMOKE" "$PKG_SMOKE"' EXIT
APP="$WORK/Unbound.app"
ditto "$RAW_APP" "$APP"
unsafe_xattrs "$APP"
codesign --force --deep --options runtime \
  --entitlements "$ROOT/build/darwin/entitlements.plist" --sign - "$APP"
codesign --verify --deep --strict --verbose=2 "$APP"

NAME="unbound-v${VERSION}-macos-universal"
BUNDLE="$WORK/$NAME"
mkdir -p "$BUNDLE"
ditto "$APP" "$BUNDLE/Unbound.app"
unsafe_xattrs "$BUNDLE/Unbound.app"
cp "$ROOT/README.md" "$ROOT/LICENSE" "$BUNDLE/"
cp "$ROOT/engine/third_party/ZAPRET_LICENSE.txt" "$BUNDLE/"
cp "$ROOT/engine/ENGINE_PROVENANCE.json" "$BUNDLE/"
cp "$ROOT/scripts/control_macOS/"*.command "$BUNDLE/"
chmod +x "$BUNDLE/"*.command
(
  cd "$BUNDLE"
  shasum -a 256 README.md LICENSE ZAPRET_LICENSE.txt ENGINE_PROVENANCE.json \
    *.command > BUNDLE_SHA256SUMS.txt
)
ditto -c -k --keepParent "$BUNDLE" "$DIST/$NAME.zip"

DMG_STAGE="$(mktemp -d)"
ditto "$APP" "$DMG_STAGE/Unbound.app"
trap 'rm -rf "$WORK" "$DMG_STAGE" "$SMOKE" "$PKG_SMOKE"' EXIT
unsafe_xattrs "$DMG_STAGE/Unbound.app"
ln -s /Applications "$DMG_STAGE/Applications"
hdiutil create -volname UNBOUND -srcfolder "$DMG_STAGE" -ov -format UDZO "$DIST/$NAME.dmg"

"$ROOT/scripts/build/build_pkg.sh" "$APP" "$DIST/unbound-v${VERSION}-macOS-Installer.pkg" "$VERSION"

SMOKE="$(mktemp -d)"
ditto -x -k "$DIST/$NAME.zip" "$SMOKE"
SMOKE_APP="$SMOKE/$NAME/Unbound.app"
test -x "$SMOKE_APP/Contents/MacOS/Unbound"
test -f "$SMOKE/$NAME/ZAPRET_LICENSE.txt"
codesign --verify --deep --strict --verbose=2 "$SMOKE_APP"
"$SMOKE_APP/Contents/MacOS/Unbound" --version | grep -F "$VERSION"

PKG_SMOKE="$(mktemp -d)"
rmdir "$PKG_SMOKE"
pkgutil --expand-full "$DIST/unbound-v${VERSION}-macOS-Installer.pkg" "$PKG_SMOKE"
(
  cd "$DIST"
  shasum -a 256 *.zip *.dmg *.pkg > SHA256SUMS.txt
)
printf 'macOS release artifacts verified in %s\n' "$DIST"
