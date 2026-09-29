#!/usr/bin/env bash
# Build and verify the supported macOS release artifacts from one exact clean checkout.
set -euo pipefail

if [[ $# -ne 2 ]]; then
    printf 'usage: %s VERSION EXPECTED_COMMIT\n' "$0" >&2
    exit 2
fi

VERSION="$1"
EXPECTED_COMMIT="$2"
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
cd "$ROOT"


ACTUAL_COMMIT="$(git rev-parse HEAD)"
[[ "$ACTUAL_COMMIT" == "$EXPECTED_COMMIT" ]] || { printf 'checkout commit %s does not match %s\n' "$ACTUAL_COMMIT" "$EXPECTED_COMMIT" >&2; exit 1; }
[[ -z "$(git status --porcelain --untracked-files=normal)" ]] || { printf 'release checkout is dirty\n' >&2; exit 1; }
[[ "$(node -p "require('./wails.json').info.productVersion")" == "$VERSION" ]] || { printf 'wails.json version mismatch\n' >&2; exit 1; }
[[ "$(node -p "require('./frontend/package.json').version")" == "$VERSION" ]] || { printf 'frontend/package.json version mismatch\n' >&2; exit 1; }

EXPECTED_SHA="$EXPECTED_COMMIT" bash ./scripts/ci/macos.sh all
UNBOUND_BUILD_COMMIT="$EXPECTED_COMMIT" \
UNBOUND_BUILD_DIRTY=false \
UNBOUND_BUILD_CHANNEL=release \
  bash ./scripts/build/build_macos_release.sh "$VERSION"

NAME="unbound-v${VERSION}-macos-universal"
ZIP="release/${NAME}.zip"
DMG="release/${NAME}.dmg"
PKG="release/unbound-v${VERSION}-macOS-Installer.pkg"
for artifact in "$ZIP" "$DMG" "$PKG"; do
    [[ -s "$artifact" ]] || { printf 'missing artifact: %s\n' "$artifact" >&2; exit 1; }
done

SMOKE="$(mktemp -d)"
trap 'rm -rf "$SMOKE"' EXIT
ditto -x -k "$ZIP" "$SMOKE"
APP="$SMOKE/$NAME/Unbound.app"
codesign --verify --deep --strict "$APP"
IDENTITY="$($APP/Contents/MacOS/Unbound --version --json)"
node -e '
const identity = JSON.parse(process.argv[1]);
const [version, commit] = process.argv.slice(2);
if (identity.version !== version || identity.commit !== commit || identity.dirty !== false || identity.channel !== "release" || identity.os !== "darwin") process.exit(1);
' "$IDENTITY" "$VERSION" "$EXPECTED_COMMIT" || { printf 'macOS release binary identity mismatch\n' >&2; exit 1; }

IDENTITY="$IDENTITY" node -e '
const fs = require("fs"); const crypto = require("crypto");
const [version, commit, ...artifacts] = process.argv.slice(1);
const result = {version, source_commit: commit, platform: "macos", build_channel: "release", build_host_role: "macos-release-host", built_at: new Date().toISOString(), artifacts: artifacts.map(path => ({artifact: path.split("/").pop(), sha256: crypto.createHash("sha256").update(fs.readFileSync(path)).digest("hex")})), identity: JSON.parse(process.env.IDENTITY)};
fs.writeFileSync("release/macos-release-evidence.json", JSON.stringify(result, null, 2) + "\n");
' "$VERSION" "$EXPECTED_COMMIT" "$ZIP" "$DMG" "$PKG"
printf 'MACOS_LOCAL_RELEASE=PASS artifacts=%s,%s,%s\n' "$ZIP" "$DMG" "$PKG"
