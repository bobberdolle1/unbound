#!/usr/bin/env bash
# Assemble one publishable local release manifest without consulting GitHub Actions.
set -euo pipefail

if [[ $# -lt 4 ]]; then
    printf 'usage: %s VERSION SOURCE_COMMIT OUTPUT_DIRECTORY ARTIFACT...\n' "$0" >&2
    exit 2
fi

VERSION="$1"
COMMIT="$2"
OUTPUT="$3"
expected=(
    "unbound-v${VERSION}-windows-amd64.zip"
    "unbound-v${VERSION}-macos-universal.zip"
    "unbound-v${VERSION}-macos-universal.dmg"
    "unbound-v${VERSION}-macOS-Installer.pkg"
    "unbound-v${VERSION}-linux-amd64.tar.gz"
)
[[ $(($# - 3)) -eq ${#expected[@]} ]] || { printf 'exactly the five supported release artifacts are required\n' >&2; exit 1; }
[[ -n "${UNBOUND_WINDOWS_BUILT_AT:-}" && -n "${UNBOUND_MACOS_BUILT_AT:-}" && -n "${UNBOUND_LINUX_BUILT_AT:-}" ]] || { printf 'UNBOUND_WINDOWS_BUILT_AT, UNBOUND_MACOS_BUILT_AT, and UNBOUND_LINUX_BUILT_AT are required from verified platform evidence\n' >&2; exit 1; }

shift 3
mkdir -p "$OUTPUT"

artifacts=()
for source in "$@"; do
    [[ -f "$source" ]] || { printf 'missing artifact: %s\n' "$source" >&2; exit 1; }
    name="$(basename "$source")"
    destination="$OUTPUT/$name"
    allowed=false
    for expected_name in "${expected[@]}"; do
        if [[ "$name" == "$expected_name" ]]; then
            allowed=true
            break
        fi
    done
    [[ "$allowed" == true ]] || { printf 'unsupported release artifact: %s\n' "$name" >&2; exit 1; }
    [[ ! -e "$destination" ]] || { printf 'duplicate release artifact: %s\n' "$name" >&2; exit 1; }
    cp "$source" "$destination"
    artifacts+=("$name")
done

(
    cd "$OUTPUT"
    if command -v shasum >/dev/null 2>&1; then
        shasum -a 256 "${artifacts[@]}"
    else
        sha256sum "${artifacts[@]}" | sed 's/ \*/  /'
    fi | LC_ALL=C sort > SHA256SUMS.txt
)
VERSION="$VERSION" COMMIT="$COMMIT" OUTPUT="$OUTPUT" node - <<'NODE'
const fs = require('fs');
const path = require('path');
const output = process.env.OUTPUT;
const artifacts = fs.readFileSync(path.join(output, 'SHA256SUMS.txt'), 'utf8').trim().split('\n').filter(Boolean).map(line => {
  const [sha256, name] = line.split(/\s{2,}/);
  const platform = name.includes('-windows-') ? 'windows' : name.includes('-macos-') || name.includes('-macOS-') ? 'macos' : name.includes('-linux-') ? 'linux' : 'unknown';
  const built_at = platform === 'windows' ? process.env.UNBOUND_WINDOWS_BUILT_AT : platform === 'macos' ? process.env.UNBOUND_MACOS_BUILT_AT : process.env.UNBOUND_LINUX_BUILT_AT;
  const build_host_role = platform === 'windows' ? 'windows-release-host' : platform === 'macos' ? 'macos-release-host' : 'linux-release-host';
  const architecture = name.includes('windows-amd64') || name.includes('linux-amd64') ? 'amd64' : name.includes('macos-universal') || name.includes('macOS-Installer') ? 'universal' : 'unknown';
  return {name, sha256, platform, architecture, experimental: platform === 'linux', build_host_role, built_at};
});
const manifest = {version: process.env.VERSION, source_commit: process.env.COMMIT, tag: `v${process.env.VERSION}`, build_channel: 'release', artifacts};
fs.writeFileSync(path.join(output, 'RELEASE_MANIFEST.json'), JSON.stringify(manifest, null, 2) + '\n');
NODE
printf 'LOCAL_RELEASE_ASSEMBLY=PASS output=%s\n' "$OUTPUT"
