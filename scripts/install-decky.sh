#!/usr/bin/env bash
# Install Unbound on Steam Deck
# Run this script ON the Steam Deck (via SSH or Konsole).
#
# Usage (on Steam Deck):
#   bash install-decky.sh /path/to/unbound-plugin-dir
set -euo pipefail

PLUGIN_NAME="unbound"
PLUGIN_DIR="/home/deck/homebrew/plugins/${PLUGIN_NAME}"
SOURCE_DIR="${1:-}"

echo "Unbound -- Steam Deck Installer"

if [ ! -d "/home/deck/homebrew/plugins" ]; then
    echo "ERROR: Decky Loader plugins directory not found."
    echo "  Install Decky Loader: https://github.com/SteamDeckHomebrew/decky-loader"
    exit 1
fi

if [ -n "${SOURCE_DIR}" ]; then
    echo "Installing from: ${SOURCE_DIR}"
    mkdir -p "${PLUGIN_DIR}"
    cp -r "${SOURCE_DIR}/." "${PLUGIN_DIR}/"
else
    echo "Installing from local package..."
    if [ ! -d "./${PLUGIN_NAME}" ]; then
        echo "ERROR: No '${PLUGIN_NAME}' directory found."
        echo "Usage: bash install-decky.sh /path/to/unbound"
        exit 1
    fi
    mkdir -p "${PLUGIN_DIR}"
    cp -r "./${PLUGIN_NAME}/." "${PLUGIN_DIR}/"
fi

chmod +x "${PLUGIN_DIR}/bin/unbound-cli" 2>/dev/null || true
chmod +x "${PLUGIN_DIR}/bin/nfqws" 2>/dev/null || true
chmod 755 "${PLUGIN_DIR}/bin" 2>/dev/null || true

echo ""
echo "Files installed to: ${PLUGIN_DIR}"
ls -la "${PLUGIN_DIR}/"
ls -la "${PLUGIN_DIR}/bin/" 2>/dev/null || true

echo ""
echo "Restarting Decky Loader..."
if systemctl --user is-active plugin-loader.service &>/dev/null; then
    systemctl --user restart plugin-loader.service
    echo "Decky Loader restarted"
else
    echo "WARNING: Could not restart Decky Loader automatically."
    echo "  Run: systemctl --user restart plugin-loader.service"
fi

echo ""
echo "Installation complete!"
echo ""
echo "1. Open Steam Deck Quick Access Menu (button)"
echo "2. Find the Unbound plugin icon"
echo "3. Toggle DPI bypass on/off"
echo ""
echo "CLI usage:"
echo "  sudo ${PLUGIN_DIR}/bin/unbound-cli start"
echo "  sudo ${PLUGIN_DIR}/bin/unbound-cli status"
echo "  sudo ${PLUGIN_DIR}/bin/unbound-cli stop"
