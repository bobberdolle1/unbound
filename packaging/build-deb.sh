#!/usr/bin/env bash
# Build script for .deb package (Debian/Ubuntu)
# Run from project root: bash packaging/build-deb.sh [version]
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
cd "$PROJECT_ROOT"

for command in go node npm dpkg-deb; do
    if ! command -v "$command" >/dev/null 2>&1; then
        echo "[ERROR] Required tool '$command' is not installed." >&2
        exit 1
    fi
done

DEFAULT_VERSION="0.9.0"
if [ -f "wails.json" ]; then
    DEFAULT_VERSION="$(node -p "require('./wails.json').info.productVersion")"
fi
VERSION="${1:-$DEFAULT_VERSION}"
BUILD_COMMIT="$(git rev-parse --verify HEAD 2>/dev/null || printf 'unknown')"

GOARCH="${GOARCH:-$(go env GOARCH)}"
case "$GOARCH" in
    amd64) DEB_ARCH="amd64" ;;
    arm64) DEB_ARCH="arm64" ;;
    *)
        echo "[ERROR] Unsupported architecture for .deb build: $GOARCH" >&2
        exit 2
        ;;
esac

PKG_NAME="unbound-cli"
PKG_FULL="${PKG_NAME}_${VERSION}_${DEB_ARCH}"
DEB_DIR="packaging/deb-staging/${PKG_FULL}"

echo "Building .deb package: ${PKG_FULL}"

rm -rf "packaging/deb-staging"
mkdir -p "${DEB_DIR}"

echo "[1/5] Building embedded frontend assets (frontend/dist)..."
(cd frontend && npm ci --no-audit --no-fund && npm run build)

echo "[2/5] Building static Go CLI binary (CGO_ENABLED=0)..."
CGO_ENABLED=0 GOOS=linux GOARCH="${GOARCH}" go build -trimpath \
    -ldflags="-s -w -X unbound/engine.Version=${VERSION} -X unbound/engine.BuildCommit=${BUILD_COMMIT} -X unbound/engine.BuildChannel=release" \
    -o packaging/deb-staging/unbound .

echo "[3/5] Staging package payload..."
mkdir -p "${DEB_DIR}"/{usr/bin,usr/lib/systemd/system,usr/share/doc/${PKG_NAME},DEBIAN}

mv "packaging/deb-staging/unbound" "${DEB_DIR}/usr/bin/unbound"
chmod 0755 "${DEB_DIR}/usr/bin/unbound"
ln -s unbound "${DEB_DIR}/usr/bin/unbound-cli"

cp "packaging/unbound.service" "${DEB_DIR}/usr/lib/systemd/system/"
chmod 0644 "${DEB_DIR}/usr/lib/systemd/system/unbound.service"
cp "README.md" "${DEB_DIR}/usr/share/doc/${PKG_NAME}/"
cp "CHANGELOG.md" "${DEB_DIR}/usr/share/doc/${PKG_NAME}/"
cp "LICENSE" "${DEB_DIR}/usr/share/doc/${PKG_NAME}/copyright"
cp "engine/third_party/ZAPRET2_LICENSE.txt" "${DEB_DIR}/usr/share/doc/${PKG_NAME}/"

cat > "${DEB_DIR}/DEBIAN/control" <<EOF
Package: ${PKG_NAME}
Version: ${VERSION}
Section: net
Priority: optional
Architecture: ${DEB_ARCH}
Depends: nftables, libnetfilter-queue1
Homepage: https://github.com/bobberdolle1/unbound
Maintainer: UNBOUND Contributors <https://github.com/bobberdolle1/unbound>
Description: Local L3/L4 DPI desynchronization CLI and systemd service
 Wraps SHA-256-verified Zapret 2 nfqws2 with isolated nftables/iptables
 NFQUEUE rule management for local DPI packet desynchronization on Linux
 (unbound --cli, unbound --control, unbound --autotune).
EOF

cat > "${DEB_DIR}/DEBIAN/postinst" <<'POSTINST'
#!/bin/bash
set -e
if [ "$1" = "configure" ] && command -v systemctl >/dev/null 2>&1; then
    systemctl daemon-reload 2>/dev/null || true
fi
POSTINST
chmod 0755 "${DEB_DIR}/DEBIAN/postinst"

cat > "${DEB_DIR}/DEBIAN/prerm" <<'PRERM'
#!/bin/bash
set -e
if { [ "$1" = "remove" ] || [ "$1" = "deconfigure" ]; } && command -v systemctl >/dev/null 2>&1; then
    systemctl stop unbound.service 2>/dev/null || true
    systemctl disable unbound.service 2>/dev/null || true
fi
PRERM
chmod 0755 "${DEB_DIR}/DEBIAN/prerm"

cat > "${DEB_DIR}/DEBIAN/postrm" <<'POSTRM'
#!/bin/bash
set -e
if { [ "$1" = "remove" ] || [ "$1" = "purge" ]; } && command -v systemctl >/dev/null 2>&1; then
    systemctl daemon-reload 2>/dev/null || true
fi
POSTRM
chmod 0755 "${DEB_DIR}/DEBIAN/postrm"

echo "[4/5] Running dpkg-deb..."
(
    cd "packaging/deb-staging"
    dpkg-deb --build --root-owner-group "${PKG_FULL}"
)

mkdir -p packaging/output
cp "packaging/deb-staging/${PKG_FULL}.deb" "packaging/output/${PKG_FULL}.deb"

echo "[5/5] Package built: packaging/output/${PKG_FULL}.deb"
