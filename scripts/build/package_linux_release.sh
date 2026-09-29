#!/usr/bin/env bash
# Build a deterministic, self-contained linux/amd64 CLI archive. It never publishes.
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
MODE="local"
VERSION=""
EXPECTED_COMMIT=""
OUTPUT_DIR=""

usage() {
    printf 'usage: %s --version VERSION --expected-commit COMMIT [--mode local|release] [--output-dir DIRECTORY]\n' "$0" >&2
}

while [[ $# -gt 0 ]]; do
    case "$1" in
        --version) VERSION="${2:-}"; shift 2 ;;
        --expected-commit) EXPECTED_COMMIT="${2:-}"; shift 2 ;;
        --mode) MODE="${2:-}"; shift 2 ;;
        --output-dir) OUTPUT_DIR="${2:-}"; shift 2 ;;
        *) usage; exit 2 ;;
    esac
done

[[ -n "$VERSION" && -n "$EXPECTED_COMMIT" ]] || { usage; exit 2; }
[[ "$MODE" == local || "$MODE" == release ]] || { printf 'unsupported package mode: %s\n' "$MODE" >&2; exit 2; }
[[ "$(uname -s)" == Linux && "$(uname -m)" == x86_64 ]] || { printf 'Linux x86_64 host is required for package smoke tests\n' >&2; exit 1; }
for command in git go node tar gzip sha256sum find sort mktemp; do
    command -v "$command" >/dev/null || { printf 'required command is missing: %s\n' "$command" >&2; exit 1; }
done

test -f "$ROOT/wails.json"
METADATA_VERSION="$(node -p "require('$ROOT/wails.json').info.productVersion")"
[[ "$VERSION" == "$METADATA_VERSION" ]] || { printf 'requested version %s does not match wails.json %s\n' "$VERSION" "$METADATA_VERSION" >&2; exit 1; }
ACTUAL_COMMIT="$(git -C "$ROOT" rev-parse --verify HEAD)"
[[ "$ACTUAL_COMMIT" == "$EXPECTED_COMMIT" ]] || { printf 'HEAD %s does not match expected commit %s\n' "$ACTUAL_COMMIT" "$EXPECTED_COMMIT" >&2; exit 1; }

if [[ "$MODE" == release ]]; then
    [[ -z "$(git -C "$ROOT" status --porcelain --untracked-files=normal)" ]] || { printf 'release packaging requires a clean checkout\n' >&2; exit 1; }
    BUILD_DIRTY=false
    BUILD_CHANNEL=release
    NAME="unbound-v${VERSION}-linux-amd64"
    OUTPUT_DIR="${OUTPUT_DIR:-$ROOT/release}"
else
    BUILD_DIRTY=true
    [[ -z "$(git -C "$ROOT" status --porcelain --untracked-files=normal)" ]] && BUILD_DIRTY=false
    BUILD_CHANNEL=development
    NAME="unbound-local-${ACTUAL_COMMIT:0:12}-linux-amd64"
    OUTPUT_DIR="${OUTPUT_DIR:-$ROOT/release/local}"
fi

SOURCE_EPOCH="$(git -C "$ROOT" show -s --format=%ct "$ACTUAL_COMMIT")"
WORK="$(mktemp -d)"
STAGE="$WORK/$NAME"
SMOKE="$(mktemp -d)"
trap 'rm -rf "$WORK" "$SMOKE"' EXIT
mkdir -p "$STAGE/runtime/core_bin" "$STAGE/runtime/lua_scripts" "$STAGE/runtime/lists" "$STAGE/scripts" "$OUTPUT_DIR"

"$ROOT/scripts/engine-assets.sh" verify
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -trimpath -buildvcs=false \
    -ldflags "-s -w -X unbound/engine.Version=$VERSION -X unbound/engine.BuildCommit=$ACTUAL_COMMIT -X unbound/engine.BuildDirty=$BUILD_DIRTY -X unbound/engine.BuildChannel=$BUILD_CHANNEL" \
    -o "$STAGE/unbound" "$ROOT"
chmod 0755 "$STAGE/unbound"

cp "$ROOT/README.md" "$ROOT/CHANGELOG.md" "$ROOT/LICENSE" "$STAGE/"
cp "$ROOT/engine/third_party/ZAPRET2_LICENSE.txt" "$ROOT/engine/third_party/ZAPRET_LICENSE.txt" "$STAGE/"
cp "$ROOT/engine/ENGINE_PROVENANCE.json" "$ROOT/engine/ENGINE_ASSETS.sha256" "$STAGE/"
cp -a "$ROOT/engine/core_bin/." "$STAGE/runtime/core_bin/"
# Generic payloads are shared with Linux strategies; platform drivers are not.
rm -rf "$STAGE/runtime/core_bin/windows" "$STAGE/runtime/core_bin/darwin" "$STAGE/runtime/core_bin/linux/arm64"
rm -f "$STAGE/runtime/core_bin/WinDivert32.sys" "$STAGE/runtime/core_bin/WinDivert64.sys"
cp -a "$ROOT/engine/lua_scripts/." "$STAGE/runtime/lua_scripts/"
cp -a "$ROOT/engine/lists/." "$STAGE/runtime/lists/"
cp "$ROOT/scripts/control_linux/"*.sh "$STAGE/scripts/"
chmod 0755 "$STAGE/runtime/core_bin/linux/amd64/nfqws2" "$STAGE/runtime/core_bin/linux/amd64/ip2net" "$STAGE/runtime/core_bin/linux/amd64/mdig" "$STAGE/scripts/"*.sh

(
    cd "$STAGE"
    LC_ALL=C find . -type f ! -name BUNDLE_SHA256SUMS.txt -printf '%P\n' | LC_ALL=C sort | while IFS= read -r file; do
        sha256sum "$file"
    done | sed 's#  \./#  #' > BUNDLE_SHA256SUMS.txt
)

ARCHIVE="$OUTPUT_DIR/$NAME.tar.gz"
rm -f "$ARCHIVE"
(
    cd "$WORK"
    tar --sort=name --format=posix --mtime="@$SOURCE_EPOCH" --owner=0 --group=0 --numeric-owner -cf - "$NAME" | gzip -n > "$ARCHIVE"
)
"$ROOT/scripts/release/verify_linux_package.sh" "$ARCHIVE" "$VERSION" "$ACTUAL_COMMIT" "$BUILD_CHANNEL" "$BUILD_DIRTY" "$SMOKE"
printf '%s\n' "$ARCHIVE"
