#!/usr/bin/env bash
# ============================================================================
# Unbound Core — Magisk Binaries Cross-Compiler Script
# ============================================================================
# Fetches and cross-compiles nfqws from bol-van/zapret for Android target ABIs:
# - arm64 (aarch64-linux-android)
# - arm (armv7a-linux-androideabi)
# - x86_64 (x86_64-linux-android)
# - x86 (i686-linux-android)
#
# Outputs binaries to magisk-module/binaries/<arch>/nfqws
# ============================================================================

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT_DIR="$(cd "${SCRIPT_DIR}/.." && pwd)"
MODULE_DIR="${ROOT_DIR}/magisk-module"
BINARIES_DIR="${MODULE_DIR}/binaries"
BUILD_TEMP="${ROOT_DIR}/build/zapret_src"
ZAPRET_TAG="v72.13"

echo "=================================================="
echo "  Unbound Core — Magisk Binaries Builder"
echo "=================================================="

# Check for NDK
NDK_PATH="${ANDROID_NDK_HOME:-${ANDROID_NDK:-${NDK:-}}}"

if [ -z "${NDK_PATH}" ] || [ ! -d "${NDK_PATH}" ]; then
    # Look in common macOS/Linux locations
    POSSIBLE_PATHS=(
        "$HOME/Library/Android/sdk/ndk-bundle"
        "$HOME/Library/Android/sdk/ndk/"*
        "/usr/local/share/android-ndk"
        "/opt/android-ndk"
    )
    for p in "${POSSIBLE_PATHS[@]}"; do
        if [ -d "$p" ]; then
            NDK_PATH="$p"
            break
        fi
    done
fi

if [ -z "${NDK_PATH}" ] || [ ! -d "${NDK_PATH}" ]; then
    echo "⚠️ Warning: Android NDK not found in environment."
    echo "   Set ANDROID_NDK_HOME=/path/to/ndk to cross-compile nfqws binaries."
    echo "   Creating ABI output structure in ${BINARIES_DIR}..."
    mkdir -p "${BINARIES_DIR}"/{arm64,arm,x86_64,x86}
    echo "   Done. Place pre-built nfqws binaries in binaries/<arch>/nfqws."
    exit 0
fi

echo "✓ Found Android NDK: ${NDK_PATH}"

# Prepare directories
mkdir -p "${BINARIES_DIR}"/{arm64,arm,x86_64,x86}
mkdir -p "${BUILD_TEMP}"

# Clone or update zapret source
if [ ! -d "${BUILD_TEMP}/.git" ]; then
    echo "📥 Fetching zapret source (${ZAPRET_TAG})..."
    git clone --depth 1 --branch "${ZAPRET_TAG}" https://github.com/bol-van/zapret.git "${BUILD_TEMP}"
else
    echo "✓ Using existing zapret source at ${BUILD_TEMP}"
fi

# Locate toolchain
HOST_OS="$(uname -s | tr '[:upper:]' '[:lower:]')"
HOST_ARCH="$(uname -m)"
TOOLCHAIN="${NDK_PATH}/toolchains/llvm/prebuilt/${HOST_OS}-${HOST_ARCH}"

if [ ! -d "${TOOLCHAIN}" ]; then
    # Fallback to x86_64 toolchain directory name
    TOOLCHAIN="${NDK_PATH}/toolchains/llvm/prebuilt/${HOST_OS}-x86_64"
fi

if [ ! -d "${TOOLCHAIN}" ]; then
    echo "❌ Error: Toolchain directory not found at ${TOOLCHAIN}"
    exit 1
fi

echo "✓ Using toolchain: ${TOOLCHAIN}"

# Compile for each target ABI
# ABI => Compiler prefix + Android API level
declare -A TARGETS=(
    ["arm64"]="aarch64-linux-android29-clang"
    ["arm"]="armv7a-linux-androideabi29-clang"
    ["x86_64"]="x86_64-linux-android29-clang"
    ["x86"]="i686-linux-android29-clang"
)

cd "${BUILD_TEMP}/nfqws"

for arch in "${!TARGETS[@]}"; do
    compiler_bin="${TOOLCHAIN}/bin/${TARGETS[$arch]}"
    output_path="${BINARIES_DIR}/${arch}/nfqws"

    if [ -x "${compiler_bin}" ]; then
        echo "🔨 Compiling nfqws for ${arch}..."
        make clean 2>/dev/null || true
        CC="${compiler_bin}" make nfqws
        cp -f nfqws "${output_path}"
        chmod 755 "${output_path}"
        echo "  └─▶ Output: ${output_path} ($(du -h "${output_path}" | cut -f1))"
    else
        echo "⚠️ Compiler ${compiler_bin} not found. Skipping ${arch}."
    fi
done

echo "=================================================="
echo "✅ Magisk binaries build process complete!"
echo "=================================================="
