#!/usr/bin/env bash
# Build Unbound for SteamOS Decky
# Cross-compiles the Rust binary for x86_64-unknown-linux-gnu
# and packages the Decky plugin for installation on Steam Deck.
#
# Usage:
#   bash scripts/build-decky.sh              # build locally
#   bash scripts/build-decky.sh --deploy     # build + deploy via SSH
#   bash scripts/build-decky.sh --deploy --host deck@192.168.1.100
set -euo pipefail

DECKY_PLUGIN_NAME="unbound"
DECKY_PLUGIN_DIR="/home/deck/homebrew/plugins/${DECKY_PLUGIN_NAME}"
SSH_HOST="${SSH_HOST:-deck@localhost}"
DEPLOY=false
RUST_TARGET="x86_64-unknown-linux-gnu"
NFQWS_SOURCE="${NFQWS_SOURCE:-}"

for arg in "$@"; do
    case "$arg" in
        --deploy) DEPLOY=true ;;
        --host=*) SSH_HOST="${arg#*=}" ;;
        --host) shift; SSH_HOST="$1" ;;
        --nfqws-source=*) NFQWS_SOURCE="${arg#*=}" ;;
    esac
done

echo "Unbound -- Decky Plugin Builder"
echo "  Target:    ${RUST_TARGET}"
echo "  Deploy:    ${DEPLOY}"

# Step 1: Build Rust binary
echo ""
echo "[1/5] Building unbound-cli (x86_64 Linux)..."

cd linux
cargo build --release --target "${RUST_TARGET}" --target-dir ../target
cd ..

RUST_BIN="target/${RUST_TARGET}/release/unbound-cli"
if [ ! -f "${RUST_BIN}" ]; then
    echo "ERROR: Rust binary not found at ${RUST_BIN}"
    exit 1
fi
echo "  Binary: ${RUST_BIN} ($(du -h "${RUST_BIN}" | cut -f1))"

# Step 2: Locate nfqws binary
echo ""
echo "[2/5] Locating nfqws binary..."

NFQWS_BIN=""
if [ -n "${NFQWS_SOURCE}" ] && [ -f "${NFQWS_SOURCE}" ]; then
    NFQWS_BIN="${NFQWS_SOURCE}"
elif [ -f "linux/nfqws" ]; then
    NFQWS_BIN="linux/nfqws"
elif command -v nfqws &>/dev/null; then
    NFQWS_BIN="$(which nfqws)"
fi

if [ -z "${NFQWS_BIN}" ]; then
    echo "  WARNING: nfqws not found. Provide it via NFQWS_SOURCE env var."
fi

# Step 3: Build Decky frontend
echo ""
echo "[3/5] Building Decky plugin frontend..."

cd decky-plugin
if command -v pnpm &>/dev/null; then
    pnpm install 2>/dev/null || true
    pnpm run build 2>/dev/null || true
elif command -v npm &>/dev/null; then
    npm install 2>/dev/null || true
    npm run build 2>/dev/null || true
else
    echo "  WARNING: No pnpm/npm found, skipping frontend build"
fi
cd ..

# Step 4: Package for distribution
echo ""
echo "[4/5] Packaging plugin..."

DIST_DIR="packaging/decky-dist"
rm -rf "${DIST_DIR}"
mkdir -p "${DIST_DIR}/${DECKY_PLUGIN_NAME}/bin"
mkdir -p "${DIST_DIR}/${DECKY_PLUGIN_NAME}/dist"

cp decky-plugin/main.py "${DIST_DIR}/${DECKY_PLUGIN_NAME}/"
cp decky-plugin/plugin.json "${DIST_DIR}/${DECKY_PLUGIN_NAME}/"
cp decky-plugin/package.json "${DIST_DIR}/${DECKY_PLUGIN_NAME}/"

if [ -f "decky-plugin/dist/index.js" ]; then
    cp decky-plugin/dist/index.js "${DIST_DIR}/${DECKY_PLUGIN_NAME}/dist/"
fi

cp "${RUST_BIN}" "${DIST_DIR}/${DECKY_PLUGIN_NAME}/bin/unbound-cli"
chmod 755 "${DIST_DIR}/${DECKY_PLUGIN_NAME}/bin/unbound-cli"

if [ -n "${NFQWS_BIN}" ] && [ -f "${NFQWS_BIN}" ]; then
    cp "${NFQWS_BIN}" "${DIST_DIR}/${DECKY_PLUGIN_NAME}/bin/nfqws"
    chmod 755 "${DIST_DIR}/${DECKY_PLUGIN_NAME}/bin/nfqws"
fi

echo "  Package: ${DIST_DIR}/${DECKY_PLUGIN_NAME}/"

# Step 5: Deploy (optional)
if [ "$DEPLOY" = true ]; then
    echo ""
    echo "[5/5] Deploying to Steam Deck (${SSH_HOST})..."

    ssh "${SSH_HOST}" "mkdir -p ${DECKY_PLUGIN_DIR}"

    rsync -avz --delete \
        "${DIST_DIR}/${DECKY_PLUGIN_NAME}/" \
        "${SSH_HOST}:${DECKY_PLUGIN_DIR}/"

    ssh "${SSH_HOST}" "systemctl --user restart plugin-loader.service" 2>/dev/null || true

    echo "  Deployed to ${DECKY_PLUGIN_DIR}"
else
    echo ""
    echo "[5/5] Skipping deployment (use --deploy to enable)"
fi

echo ""
echo "Build complete!"
echo ""
echo "Package: ${DIST_DIR}/${DECKY_PLUGIN_NAME}/"
if [ "$DEPLOY" = false ]; then
    echo ""
    echo "To deploy manually:"
    echo "  scp -r ${DIST_DIR}/${DECKY_PLUGIN_NAME} deck@<steam-deck-ip>:/home/deck/homebrew/plugins/"
    echo "  ssh deck@<steam-deck-ip> 'systemctl --user restart plugin-loader.service'"
fi
