#!/usr/bin/env bash
# ============================================================================
# Unbound Core — Magisk Module Packaging Script
# ============================================================================
# Packages the magisk-module/ directory into UnboundCore-v2.5.0.zip
# ============================================================================

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT_DIR="$(cd "${SCRIPT_DIR}/.." && pwd)"
MODULE_DIR="${ROOT_DIR}/magisk-module"
OUTPUT_ZIP="${ROOT_DIR}/UnboundCore-v2.5.0.zip"

echo "=================================================="
echo "  Unbound Core — Magisk Module Packager"
echo "=================================================="

if [ ! -d "${MODULE_DIR}" ]; then
    echo "❌ Error: Directory ${MODULE_DIR} does not exist."
    exit 1
fi

# Verify module.prop
if [ ! -f "${MODULE_DIR}/module.prop" ]; then
    echo "❌ Error: module.prop not found."
    exit 1
fi

# Check for binaries
ARCHS=("arm64" "arm" "x86_64" "x86")
MISSING_BINARIES=0

for arch in "${ARCHS[@]}"; do
    bin_path="${MODULE_DIR}/binaries/${arch}/nfqws"
    if [ ! -f "${bin_path}" ]; then
        echo "⚠️ Warning: Missing binary for ${arch} at ${bin_path}"
        MISSING_BINARIES=$((MISSING_BINARIES + 1))
    else
        echo "✓ Found binary for ${arch} ($(file -b "${bin_path}" 2>/dev/null || echo "binary"))"
    fi
done

if [ ${MISSING_BINARIES} -gt 0 ]; then
    echo "⚠️ Note: Packaging with ${MISSING_BINARIES} missing binaries. Run scripts/build-magisk-binaries.sh first for production builds."
fi

# Create ZIP archive
echo "📦 Packaging module into ${OUTPUT_ZIP}..."
cd "${MODULE_DIR}"

rm -f "${OUTPUT_ZIP}"
zip -r "${OUTPUT_ZIP}" \
    module.prop \
    customize.sh \
    service.sh \
    uninstall.sh \
    post-fs-data.sh \
    config/ \
    scripts/ \
    binaries/ \
    webui/ \
    -x "*.gitkeep" \
    -x "*.DS_Store"

if [ -f "${OUTPUT_ZIP}" ]; then
    SIZE=$(du -h "${OUTPUT_ZIP}" | cut -f1)
    SHA256=$(shasum -a 256 "${OUTPUT_ZIP}" | cut -d' ' -f1)
    echo "✅ Magisk module packaged successfully!"
    echo "   File:   ${OUTPUT_ZIP}"
    echo "   Size:   ${SIZE}"
    echo "   SHA256: ${SHA256}"
else
    echo "❌ Error: Failed to create ZIP package."
    exit 1
fi
