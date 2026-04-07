#!/usr/bin/env bash
# Build script for .deb package (Debian/Ubuntu)
# Run from project root: bash packaging/build-deb.sh
set -euo pipefail

VERSION="${1:-0.1.0}"
PKG_NAME="unbound-cli"
PKG_FULL="${PKG_NAME}_${VERSION}_amd64"
DEB_DIR="packaging/deb-staging/${PKG_FULL}"

echo "Building .deb package: ${PKG_FULL}"

rm -rf "packaging/deb-staging"
mkdir -p "${DEB_DIR}"

# Build Rust binary
echo "[1/4] Building Rust binary..."
cd linux
cargo build --release --target-dir ../target
cd ..

# Stage files
echo "[2/4] Staging files..."
mkdir -p "${DEB_DIR}"/{usr/bin,usr/lib/systemd/system,usr/share/doc/${PKG_NAME},DEBIAN}

cp "target/release/unbound-cli" "${DEB_DIR}/usr/bin/"
chmod 755 "${DEB_DIR}/usr/bin/unbound-cli"
cp "packaging/unbound.service" "${DEB_DIR}/usr/lib/systemd/system/"
cp "README.md" "${DEB_DIR}/usr/share/doc/${PKG_NAME}/" 2>/dev/null || true

cat > "${DEB_DIR}/DEBIAN/control" <<EOF
Package: ${PKG_NAME}
Version: ${VERSION}
Section: net
Priority: optional
Architecture: amd64
Depends: nftables, libnetfilter-queue1
Maintainer: Unbound Contributors <unbound@example.com>
Description: DPI/censorship bypass daemon
 Wraps the zapret nfqws binary with nftables rule management
 for transparent DPI bypass on Linux systems.
EOF

cat > "${DEB_DIR}/DEBIAN/postinst" <<'POSTINST'
#!/bin/bash
set -e
if command -v systemctl &>/dev/null; then
    systemctl daemon-reload
    systemctl enable unbound.service 2>/dev/null || true
fi
POSTINST
chmod 755 "${DEB_DIR}/DEBIAN/postinst"

cat > "${DEB_DIR}/DEBIAN/postrm" <<'POSTRM'
#!/bin/bash
set -e
if [ "$1" = "remove" ] || [ "$1" = "purge" ]; then
    systemctl stop unbound.service 2>/dev/null || true
    systemctl disable unbound.service 2>/dev/null || true
    systemctl daemon-reload
fi
POSTRM
chmod 755 "${DEB_DIR}/DEBIAN/postrm"

# Build
echo "[3/4] Running dpkg-deb..."
cd "packaging/deb-staging"
dpkg-deb --build --root-owner-group "${PKG_FULL}"
cd ../..

mkdir -p packaging/output
cp "packaging/deb-staging/${PKG_FULL}.deb" "packaging/output/${PKG_FULL}.deb"

echo "[4/4] Package built: packaging/output/${PKG_FULL}.deb"
