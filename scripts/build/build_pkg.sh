#!/usr/bin/env bash
# Build native macOS installer package (.pkg) for UNBOUND.
# Usage: ./scripts/build/build_pkg.sh [app_path] [output_pkg] [version]

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"

VERSION="${3:-0.6.7}"
if [ -f "$PROJECT_ROOT/wails.json" ] && [ -z "${3:-}" ]; then
    VERSION="$(node -e "try { console.log(require('$PROJECT_ROOT/wails.json').info.productVersion); } catch(e) { console.log('0.6.7'); }")"
fi

APP_PATH="${1:-}"
if [ -z "$APP_PATH" ]; then
    for candidate in "$PROJECT_ROOT/build/bin/Unbound.app" "$PROJECT_ROOT/build/bin/unbound.app"; do
        if [ -d "$candidate" ]; then
            APP_PATH="$candidate"
            break
        fi
    done
fi

if [ -z "$APP_PATH" ] || [ ! -d "$APP_PATH" ]; then
    echo "[ERROR] Unbound.app bundle not found at: ${APP_PATH:-<default paths>}" >&2
    echo "        Run wails build or ./scripts/build/build_darwin.sh first." >&2
    exit 1
fi

OUTPUT_PKG="${2:-$PROJECT_ROOT/dist/unbound-v${VERSION}-macOS-Installer.pkg}"
OUTPUT_DIR="$(dirname "$OUTPUT_PKG")"
mkdir -p "$OUTPUT_DIR"

echo "[INFO] Packaging native macOS installer for UNBOUND v$VERSION..."
echo "       Application source: $APP_PATH"
echo "       Target installer:   $OUTPUT_PKG"

# Prepare staging directories
TMP_WORK="$(mktemp -d)"
trap 'rm -rf "$TMP_WORK"' EXIT

ROOT_DIR="$TMP_WORK/root"
SCRIPTS_DIR="$TMP_WORK/scripts"
mkdir -p "$ROOT_DIR" "$SCRIPTS_DIR"

# Copy Unbound.app to the staging root
cp -R "$APP_PATH" "$ROOT_DIR/Unbound.app"

# Remove any existing quarantine flags or metadata from the staged bundle
xattr -cr "$ROOT_DIR/Unbound.app" 2>/dev/null || true

# Deep ad-hoc codesign with entitlements if available
ENTITLEMENTS="$PROJECT_ROOT/build/darwin/entitlements.plist"
if [ -f "$ENTITLEMENTS" ]; then
    echo "[INFO] Signing bundle with hardened runtime entitlements..."
    codesign --force --deep --options runtime --entitlements "$ENTITLEMENTS" --sign - "$ROOT_DIR/Unbound.app"
else
    echo "[INFO] Signing bundle ad-hoc..."
    codesign --force --deep --sign - "$ROOT_DIR/Unbound.app"
fi

# Create postinstall script
cat > "$SCRIPTS_DIR/postinstall" << 'EOF'
#!/bin/bash
set -e

APP_DEST="/Applications/Unbound.app"

# 1. Eliminate Gatekeeper quarantine attribute recursively from the installed app
if [ -d "$APP_DEST" ]; then
    xattr -cr "$APP_DEST" 2>/dev/null || true
    chmod -R u+rwX,go+rX "$APP_DEST" 2>/dev/null || true
fi

# 2. Configure passwordless sudo for pfctl in /etc/sudoers.d/unbound_zapret
mkdir -p /etc/sudoers.d
SUDOERS_FILE="/etc/sudoers.d/unbound_zapret"
cat > "$SUDOERS_FILE" << 'SUDO_EOF'
ALL ALL=(ALL) NOPASSWD: /sbin/pfctl, ALL
SUDO_EOF
chmod 0440 "$SUDOERS_FILE"

# 3. Ensure pf.conf anchor com.unbound.zapret is configured
PF_CONF="/etc/pf.conf"
if [ -f "$PF_CONF" ]; then
    NEEDS_RDR=0
    NEEDS_ANCHOR=0
    grep -q 'rdr-anchor "com.unbound.zapret"' "$PF_CONF" || NEEDS_RDR=1
    grep -q 'anchor "com.unbound.zapret"' "$PF_CONF" || NEEDS_ANCHOR=1

    if [ $NEEDS_RDR -eq 1 ] || [ $NEEDS_ANCHOR -eq 1 ]; then
        TMP_PF="$(mktemp /tmp/pf.conf.XXXXXX)"
        RDR_INSERTED=0
        ANCHOR_INSERTED=0
        [ $NEEDS_RDR -eq 0 ] && RDR_INSERTED=1
        [ $NEEDS_ANCHOR -eq 0 ] && ANCHOR_INSERTED=1

        while IFS= read -r line; do
            echo "$line" >> "$TMP_PF"
            if [ $RDR_INSERTED -eq 0 ] && [[ "$line" == *'rdr-anchor "com.apple'* ]]; then
                echo 'rdr-anchor "com.unbound.zapret"' >> "$TMP_PF"
                RDR_INSERTED=1
            fi
            if [ $ANCHOR_INSERTED -eq 0 ] && [[ "$line" == *'anchor "com.apple'* ]] && [[ "$line" != *"load"* ]]; then
                echo 'anchor "com.unbound.zapret"' >> "$TMP_PF"
                ANCHOR_INSERTED=1
            fi
        done < "$PF_CONF"

        if /sbin/pfctl -n -f "$TMP_PF" >/dev/null 2>&1; then
            cp "$TMP_PF" "$PF_CONF"
            chmod 0644 "$PF_CONF"
            /sbin/pfctl -f /etc/pf.conf >/dev/null 2>&1 || true
        fi
        rm -f "$TMP_PF"
    fi
fi

exit 0
EOF
chmod 0755 "$SCRIPTS_DIR/postinstall"

# Build component package
COMPONENT_PKG="$TMP_WORK/unbound-component.pkg"
COMPONENT_PLIST="$TMP_WORK/component.plist"

# Generate and configure component plist to disable relocation (force install to /Applications)
pkgbuild --analyze --root "$ROOT_DIR" "$COMPONENT_PLIST"
python3 -c "
import plistlib
with open('$COMPONENT_PLIST', 'rb') as f:
    plist = plistlib.load(f)
for item in plist:
    item['BundleIsRelocatable'] = False
    item['BundleOverwriteAction'] = 'upgrade'
with open('$COMPONENT_PLIST', 'wb') as f:
    plistlib.dump(plist, f)
"

echo "[INFO] Running pkgbuild for component (BundleIsRelocatable=false)..."
pkgbuild \
    --root "$ROOT_DIR" \
    --component-plist "$COMPONENT_PLIST" \
    --install-location "/Applications" \
    --scripts "$SCRIPTS_DIR" \
    --identifier "com.unbound.app" \
    --version "$VERSION" \
    "$COMPONENT_PKG"

# Synthesize distribution XML
DIST_XML="$TMP_WORK/distribution.xml"
productbuild --synthesize --package "$COMPONENT_PKG" "$DIST_XML"

# Insert title and options into distribution XML
python3 -c "
import xml.etree.ElementTree as ET
tree = ET.parse('$DIST_XML')
root = tree.getroot()

# Set title
title_elem = ET.Element('title')
title_elem.text = 'UNBOUND'
root.insert(0, title_elem)

# Configure installation options
options_elem = ET.Element('options', {
    'customize': 'never',
    'require-scripts': 'true',
    'hostArchitectures': 'arm64,x86_64'
})
root.insert(1, options_elem)

tree.write('$DIST_XML', encoding='utf-8', xml_declaration=True)
"

# Build final product distribution package
echo "[INFO] Running productbuild for final installer..."
productbuild \
    --distribution "$DIST_XML" \
    --package-path "$TMP_WORK" \
    "$OUTPUT_PKG"

PKG_SIZE="$(du -h "$OUTPUT_PKG" | cut -f1)"
echo "[SUCCESS] macOS Installer Package built successfully:"
echo "          Path: $OUTPUT_PKG"
echo "          Size: $PKG_SIZE"
