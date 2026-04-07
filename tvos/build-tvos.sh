#!/bin/bash
# ============================================================================
# Unbound tvOS — Build script for tpws engine
# Cross-compiles the tpws DPI bypass engine for tvOS ARM64
# ============================================================================

set -e

# Configuration
TVOS_SDK_VERSION="17.0"
ARCH="arm64"
BUILD_TYPE="Release"

# Detect host OS
HOST_OS=$(uname -s)
echo "Host OS: $HOST_OS"

# Find Xcode toolchain
if [ "$HOST_OS" = "Darwin" ]; then
    XCODE_PATH=$(xcode-select -p)
    TOOLCHAIN_PATH="$XCODE_PATH/Toolchains/XcodeDefault.xctoolchain/usr/bin"
    SDK_PATH="$XCODE_PATH/Platforms/AppleTVOS.platform/Developer/SDKs/AppleTVOS${TVOS_SDK_VERSION}.sdk"
    CC="$TOOLCHAIN_PATH/clang"
else
    echo "ERROR: tvOS build requires macOS with Xcode installed"
    exit 1
fi

# Verify SDK exists
if [ ! -d "$SDK_PATH" ]; then
    echo "ERROR: tvOS SDK not found at $SDK_PATH"
    echo "Available SDKs:"
    ls -1 "$XCODE_PATH/Platforms/AppleTVOS.platform/Developer/SDKs/" 2>/dev/null || echo "  None found"
    exit 1
fi

echo "Using tvOS SDK: $SDK_PATH"
echo "Compiler: $CC"

# Source paths
TPWS_SRC="../../theos/unbound-legacy/engine/tpws"
EPOLL_SHIM="$TPWS_SRC/epoll-shim"

# Build directory
BUILD_DIR="./build/tvos-$ARCH"
mkdir -p "$BUILD_DIR"

# Compiler flags for tvOS
CFLAGS="-target arm64-apple-tvos${TVOS_SDK_VERSION} \
  -isysroot $SDK_PATH \
  -miphoneos-version-min=${TVOS_SDK_VERSION} \
  -std=gnu99 \
  -Os \
  -flto=auto \
  -ffunction-sections \
  -fdata-sections \
  -DtvOS \
  -DDARWIN \
  -Wno-address-of-packed-member"

LDFLAGS="-flto=auto -Wl,-dead_strip -L$SDK_PATH/usr/lib"
LIBS="-lz -lpthread"

# Include paths
INCLUDES="-I$TPWS_SRC \
  -I$TPWS_SRC/macos \
  -I$EPOLL_SHIM/include \
  -I./UnboundTV/UnboundEngine/include"

# Source files
SRC_CORE="$TPWS_SRC/tpws.c \
  $TPWS_SRC/tpws_conn.c \
  $TPWS_SRC/helpers.c \
  $TPWS_SRC/hostlist.c \
  $TPWS_SRC/protocol.c \
  $TPWS_SRC/tamper.c \
  $TPWS_SRC/resolver.c \
  $TPWS_SRC/redirect.c \
  $TPWS_SRC/params.c \
  $TPWS_SRC/pools.c \
  $TPWS_SRC/sec.c \
  $TPWS_SRC/gzip.c \
  $TPWS_SRC/ipset.c"

SRC_EPOLL="$EPOLL_SHIM/src/epoll_shim.c"

echo ""
echo "==> Building tpws for tvOS ($ARCH)..."
echo ""

# Build the engine wrapper
$CC $CFLAGS $INCLUDES \
  -c ./UnboundTV/UnboundEngine/src/UnboundTunnelEngine.c \
  -o "$BUILD_DIR/tunnel_engine.o"

# Build core tpws
$CC $CFLAGS $INCLUDES \
  -c $SRC_CORE \
  -o "$BUILD_DIR/tpws_core.o"

# Build epoll-shim
$CC $CFLAGS $INCLUDES \
  -c $SRC_EPOLL \
  -o "$BUILD_DIR/epoll_shim.o"

# Link everything into a static library
ar rcs "$BUILD_DIR/libunboundengine.a" \
  "$BUILD_DIR/tunnel_engine.o" \
  "$BUILD_DIR/tpws_core.o" \
  "$BUILD_DIR/epoll_shim.o"

# Strip the library
strip -S "$BUILD_DIR/libunboundengine.a"

echo ""
echo "==> Built libunboundengine.a for tvOS ($ARCH)"
echo "    Location: $BUILD_DIR/libunboundengine.a"
echo ""

# Copy to the engine target directory
cp "$BUILD_DIR/libunboundengine.a" ./UnboundTV/UnboundEngine/

echo "==> Installed library to UnboundEngine/"
echo ""
echo "Build complete! You can now build the Swift app with:"
echo "  cd tvos/UnboundTV"
echo "  swift build --arch arm64"
