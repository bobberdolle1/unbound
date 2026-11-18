#!/usr/bin/env bash
# Build script for .rpm package (Fedora/RHEL)
# Run from project root: bash packaging/build-rpm.sh [version]
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
cd "$PROJECT_ROOT"

for command in go node npm tar rpmbuild; do
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
RPM_VERSION="${VERSION//-/_}"
PKG_NAME="unbound-cli"
RPMBUILD_DIR="$PROJECT_ROOT/packaging/rpm-build"

echo "Building .rpm package: ${PKG_NAME}-${RPM_VERSION}"

rm -rf "${RPMBUILD_DIR}"
mkdir -p "${RPMBUILD_DIR}"/{SOURCES,SPECS,BUILD,RPMS,SRPMS}

echo "[1/5] Building embedded frontend assets (frontend/dist)..."
(cd frontend && npm ci --no-audit --no-fund && npm run build)

echo "[2/5] Creating source tarball (${PKG_NAME}-${RPM_VERSION}.tar.gz)..."
tar czf "${RPMBUILD_DIR}/SOURCES/${PKG_NAME}-${RPM_VERSION}.tar.gz" \
    --transform="s,^\.,${PKG_NAME}-${RPM_VERSION}," \
    --exclude='./.git' \
    --exclude='./.worktrees' \
    --exclude='./packaging/rpm-build' \
    --exclude='./packaging/deb-staging' \
    --exclude='./packaging/output' \
    --exclude='./frontend/node_modules' \
    --exclude='./website' \
    --exclude='./build/bin' \
    --exclude='./dist' \
    --exclude='./release' \
    --exclude='./tmp' \
    .

cp "packaging/unbound-cli.spec" "${RPMBUILD_DIR}/SPECS/"

echo "[3/5] Running rpmbuild..."
rpmbuild --define "_topdir ${RPMBUILD_DIR}" \
    --define "version ${RPM_VERSION}" \
    -ba "${RPMBUILD_DIR}/SPECS/unbound-cli.spec"

echo "[4/5] Collecting output..."
mkdir -p packaging/output
find "${RPMBUILD_DIR}/RPMS" -name "*.rpm" -exec cp {} packaging/output/ \;

echo "[5/5] Package built in packaging/output/"
