#!/usr/bin/env bash
# Build script for .rpm package (Fedora/RHEL)
# Run from project root: bash packaging/build-rpm.sh
set -euo pipefail

VERSION="${1:-0.1.0}"
PKG_NAME="unbound-cli"
RPMBUILD_DIR="packaging/rpm-build"

echo "Building .rpm package: ${PKG_NAME}-${VERSION}"

mkdir -p "${RPMBUILD_DIR}"/{SOURCES,SPECS,BUILD,RPMS,SRPMS}

# Create source tarball
echo "[1/4] Creating source tarball..."
tar czf "${RPMBUILD_DIR}/SOURCES/${PKG_NAME}-${VERSION}.tar.gz" \
    --exclude='.git' \
    --exclude='packaging' \
    --exclude='decky-plugin' \
    --exclude='target' \
    --exclude='frontend' \
    --exclude='build' \
    --exclude='dist' \
    --exclude='node_modules' \
    -C "$(dirname "$PWD")" "$(basename "$PWD")"

cp "packaging/unbound-cli.spec" "${RPMBUILD_DIR}/SPECS/"

echo "[2/4] Running rpmbuild..."
rpmbuild --define "_topdir $(pwd)/${RPMBUILD_DIR}" \
    --define "version ${VERSION}" \
    -ba "${RPMBUILD_DIR}/SPECS/unbound-cli.spec"

echo "[3/4] Collecting output..."
mkdir -p packaging/output
find "${RPMBUILD_DIR}/RPMS" -name "*.rpm" -exec cp {} packaging/output/ \;

echo "[4/4] Package built: packaging/output/"
