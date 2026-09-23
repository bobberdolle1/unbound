#!/usr/bin/env bash
# Build the self-contained Linux CLI for the selected architecture.
# Usage: ./scripts/build/build_linux.sh [debug]
# Environment: GOARCH=amd64|arm64, UNBOUND_VERSION=<override>,
# UNBOUND_BUILD_COMMIT=<sha>, UNBOUND_BUILD_DIRTY=true|false|unknown,
# UNBOUND_BUILD_CHANNEL=development|release
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"
BUILD_DIR="$PROJECT_ROOT/build/bin"
ARCH="${GOARCH:-$(go env GOARCH)}"

case "$ARCH" in
    amd64|arm64) ;;
    *) echo "[ERROR] Unsupported Linux architecture: $ARCH" >&2; exit 2 ;;
esac

for command in go npm node; do
    command -v "$command" >/dev/null 2>&1 || {
        echo "[ERROR] $command is required" >&2
        exit 1
    }
done

cd "$PROJECT_ROOT"
VERSION="${UNBOUND_VERSION:-$(node -p "require('./wails.json').info.productVersion")}"
BUILD_COMMIT="${UNBOUND_BUILD_COMMIT:-$(git rev-parse --verify HEAD 2>/dev/null || printf 'unknown')}"
if [ -n "${UNBOUND_BUILD_DIRTY:-}" ]; then
    BUILD_DIRTY="$UNBOUND_BUILD_DIRTY"
elif [ "$BUILD_COMMIT" = "unknown" ]; then
    BUILD_DIRTY="unknown"
elif [ -n "$(git status --porcelain --untracked-files=normal)" ]; then
    BUILD_DIRTY="true"
else
    BUILD_DIRTY="false"
fi
BUILD_CHANNEL="${UNBOUND_BUILD_CHANNEL:-development}"
GO_ARGS=(build -trimpath)
LDFLAGS="-s -w -X unbound/engine.Version=$VERSION -X unbound/engine.BuildCommit=$BUILD_COMMIT -X unbound/engine.BuildDirty=$BUILD_DIRTY -X unbound/engine.BuildChannel=$BUILD_CHANNEL"
if [ "${1:-}" = "debug" ]; then
    GO_ARGS+=("-gcflags=all=-N -l")
    LDFLAGS="-X unbound/engine.Version=$VERSION -X unbound/engine.BuildCommit=$BUILD_COMMIT -X unbound/engine.BuildDirty=$BUILD_DIRTY -X unbound/engine.BuildChannel=$BUILD_CHANNEL"
elif [ "$#" -gt 0 ]; then
    echo "[ERROR] Unknown argument: $1" >&2
    exit 2
fi

echo "[INFO] Building frontend..."
(cd frontend && npm ci --no-audit --no-fund && npm run build)

mkdir -p "$BUILD_DIR"
OUTPUT="$BUILD_DIR/unbound-linux-$ARCH"
echo "[INFO] Building Linux $ARCH CLI v$VERSION..."
CGO_ENABLED=0 GOOS=linux GOARCH="$ARCH" \
    go "${GO_ARGS[@]}" -ldflags="$LDFLAGS" -o "$OUTPUT" .
chmod +x "$OUTPUT"

if [ "$ARCH" = "$(go env GOARCH)" ] && [ "$(go env GOOS)" = "linux" ]; then
    "$OUTPUT" --version | grep -F "$VERSION" >/dev/null
    "$OUTPUT" --list-profiles --json >/dev/null
fi

echo "[OK] Linux binary built: $OUTPUT"
