#!/bin/bash
# macOS build script for UNBOUND
# Usage: ./scripts/build/build_darwin.sh [amd64|arm64|universal] [debug]
# Example: ./scripts/build/build_darwin.sh arm64
#          ./scripts/build/build_darwin.sh universal debug

set -e

PLATFORM="${1:-universal}"
DEBUG_FLAG=""

if [ "$2" = "debug" ]; then
    DEBUG_FLAG="-debug"
fi

echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo "🍎 UNBOUND — macOS Build"
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo "Platform: $PLATFORM"
[ -n "$DEBUG_FLAG" ] && echo "Mode: DEBUG" || echo "Mode: RELEASE"
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"

# Check prerequisites
if ! command -v go &> /dev/null; then
    echo "❌ Go not found. Install: brew install go"
    exit 1
fi

if ! command -v wails &> /dev/null; then
    echo "❌ Wails CLI not found. Install: go install github.com/wailsapp/wails/v2/cmd/wails@latest"
    exit 1
fi

# Check engine binary availability
if [ -f "engine/core_bin/darwin/tpws" ]; then
    echo "✓ Bundled macOS tpws binary detected in engine/core_bin/darwin/tpws"
elif command -v tpws &> /dev/null || [ -f "/opt/homebrew/bin/tpws" ] || [ -f "/usr/local/bin/tpws" ]; then
    echo "✓ System tpws binary detected"
else
    echo "⚠️  tpws binary not found in PATH or standard locations"
    echo "   Install zapret: brew install zapret (or build manually)"
fi

# Build frontend
echo ""
echo "📦 Building frontend..."
if [ -d "frontend" ]; then
    cd frontend
    npm install
    npm run build
    cd ..
else
    echo "⚠️  frontend/ directory not found, skipping"
fi

# Build macOS app
echo ""
echo "🔨 Building macOS app..."

xattr -cr . 2>/dev/null || true
if [ "$PLATFORM" = "universal" ]; then
    wails build -platform darwin/universal $DEBUG_FLAG -ldflags "-X unbound/engine.Version=0.1.0-refresh" || true
else
    wails build -platform darwin/$PLATFORM $DEBUG_FLAG -ldflags "-X unbound/engine.Version=0.1.0-refresh" || true
fi

APP_PATH=""
if [ -d "build/bin/Unbound.app" ]; then
    APP_PATH="build/bin/Unbound.app"
elif [ -d "build/bin/unbound.app" ]; then
    APP_PATH="build/bin/unbound.app"
fi

if [ -n "$APP_PATH" ]; then
    xattr -cr "$APP_PATH" 2>/dev/null || true
    codesign --force --deep -s - "$APP_PATH" 2>/dev/null || true
    echo "✓ App bundle signed: $APP_PATH"
fi

echo ""
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo "✅ Build complete!"
echo "📁 Output: build/bin/Unbound.app"
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"

# Show file info
if [ -d "build/bin/Unbound.app" ]; then
    du -sh build/bin/Unbound.app
    echo ""
    echo "To run:"
    echo "  open build/bin/Unbound.app"
    echo ""
    echo "CLI mode:"
    echo "  ./build/bin/Unbound.app/Contents/MacOS/Unbound --cli"
fi
