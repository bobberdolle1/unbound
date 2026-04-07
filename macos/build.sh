#!/bin/bash
# macOS build script for UNBOUND
# Usage: ./build.sh [amd64|arm64|universal] [debug]
# Example: ./build.sh arm64
#          ./build.sh universal debug

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

# Check SpoofDPI availability
if ! command -v spoofdpi &> /dev/null; then
    echo "⚠️  SpoofDPI not found in PATH"
    echo "   Install: brew install spoofdpi"
    echo "   Or place binary in core_bin/darwin/"
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

if [ "$PLATFORM" = "universal" ]; then
    wails build -platform darwin/universal $DEBUG_FLAG
else
    wails build -platform darwin/$PLATFORM $DEBUG_FLAG
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
