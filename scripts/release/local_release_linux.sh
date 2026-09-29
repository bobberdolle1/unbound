#!/usr/bin/env bash
# Produce and attest one future Linux release candidate locally. Never publishes.
set -euo pipefail

if [[ $# -ne 2 ]]; then
    printf 'usage: %s VERSION EXPECTED_COMMIT\n' "$0" >&2
    exit 2
fi

VERSION="$1"
EXPECTED_COMMIT="$2"
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
cd "$ROOT"
[[ "$(uname -s)" == Linux && "$(uname -m)" == x86_64 ]] || { printf 'local Linux release coordination requires Linux x86_64\n' >&2; exit 1; }
[[ "$(git rev-parse HEAD)" == "$EXPECTED_COMMIT" ]] || { printf 'checkout commit does not match expected commit\n' >&2; exit 1; }
[[ -z "$(git status --porcelain --untracked-files=normal)" ]] || { printf 'release checkout is dirty\n' >&2; exit 1; }
[[ "$(node -p "require('./wails.json').info.productVersion")" == "$VERSION" ]] || { printf 'wails.json version does not match requested release version\n' >&2; exit 1; }
[[ "$(node -p "require('./frontend/package.json').version")" == "$VERSION" ]] || { printf 'frontend version does not match requested release version\n' >&2; exit 1; }

bash ./scripts/ci/linux.sh all
./scripts/build/package_linux_release.sh --version "$VERSION" --expected-commit "$EXPECTED_COMMIT" --mode release
archive="$ROOT/release/unbound-v${VERSION}-linux-amd64.tar.gz"
[[ -f "$archive" ]] || { printf 'Linux release archive is missing after packaging\n' >&2; exit 1; }
export UNBOUND_LINUX_ARCHIVE="$archive"
go test ./tests -run '^TestLinuxPackagingArchive$' -count=1

smoke="$(mktemp -d)"
trap 'rm -rf "$smoke"' EXIT
tar -xzf "$archive" -C "$smoke"
package="$(find "$smoke" -mindepth 1 -maxdepth 1 -type d -name 'unbound-v*-linux-amd64' -print -quit)"
[[ -n "$package" ]] || { printf 'release package root is missing\n' >&2; exit 1; }
identity="$("$package/unbound" --version --json)"
ARCHIVE="$archive" VERSION="$VERSION" COMMIT="$EXPECTED_COMMIT" IDENTITY="$identity" node - <<'NODE'
const crypto = require('crypto');
const fs = require('fs');
const cp = require('child_process');
const identity = JSON.parse(process.env.IDENTITY);
const evidence = {
  schema_version: 1,
  artifact: require('path').basename(process.env.ARCHIVE),
  sha256: crypto.createHash('sha256').update(fs.readFileSync(process.env.ARCHIVE)).digest('hex'),
  version: process.env.VERSION,
  source_commit: process.env.COMMIT,
  dirty: false,
  channel: 'release',
  os: 'linux',
  arch: 'amd64',
  built_at: new Date().toISOString(),
  go_version: cp.execFileSync('go', ['version'], {encoding: 'utf8'}).trim(),
  package_smoke: 'PASS',
  engine_asset_verification: 'PASS',
  identity
};
fs.writeFileSync('release/linux-release-evidence.json', JSON.stringify(evidence, null, 2) + '\n');
NODE
printf 'LINUX_LOCAL_RELEASE=PASS artifact=%s\n' "$archive"
