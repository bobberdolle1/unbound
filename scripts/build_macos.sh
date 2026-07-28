#!/usr/bin/env bash
# ============================================================================
# Build and publish macOS GUI .app bundle
# Run this on a Mac with Xcode + Wails installed
# ============================================================================
set -euo pipefail

VERSION="0.1.0-refresh"
REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$REPO_ROOT"

echo "━━━ macOS Universal Build ━━━"

# 1. Pull latest
echo "[1/4] Pulling latest from origin/master..."
git pull origin master

# 2. Build universal .app
echo "[2/4] Building Wails universal binary..."
wails build -platform darwin/universal -clean \
  -ldflags "-X unbound/engine.Version=${VERSION}"

# 3. Package
APP_PATH=$(find build/bin -name "Unbound.app" -type d | head -1)
if [ -z "$APP_PATH" ]; then
  echo "ERROR: Unbound.app not found"
  exit 1
fi

echo "[3/4] Packaging ${APP_PATH}..."
xattr -cr "$APP_PATH" 2>/dev/null || true
codesign --force --deep -s - "$APP_PATH" 2>/dev/null || true

mkdir -p dist
ZIP_PATH="dist/Unbound-macOS-universal.zip"
rm -f "$ZIP_PATH"
ditto -c -k --sequesterRsrc --keepParent "$APP_PATH" "$ZIP_PATH"
echo "  ✓ ${ZIP_PATH} ($(du -h "$ZIP_PATH" | cut -f1))"

# 4. Upload to GitHub release
echo "[4/4] Uploading to GitHub release v${VERSION}..."
gh release upload "v${VERSION}" "$ZIP_PATH" --clobber
echo "  ✓ Done! Release updated."
