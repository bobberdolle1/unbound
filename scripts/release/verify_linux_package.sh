#!/usr/bin/env bash
# Validate a Linux CLI archive from outside the source tree.
set -euo pipefail

if [[ $# -lt 5 || $# -gt 6 ]]; then
    printf 'usage: %s ARCHIVE VERSION COMMIT CHANNEL DIRTY [EXTRACT_DIRECTORY]\n' "$0" >&2
    exit 2
fi

ARCHIVE="$1"
VERSION="$2"
COMMIT="$3"
CHANNEL="$4"
DIRTY="$5"
EXTRACT_DIRECTORY="${6:-$(mktemp -d)}"
OWN_EXTRACT=false
[[ $# -eq 5 ]] && OWN_EXTRACT=true
trap '[[ "$OWN_EXTRACT" == true ]] && rm -rf "$EXTRACT_DIRECTORY"' EXIT

[[ -f "$ARCHIVE" ]] || { printf 'archive is missing: %s\n' "$ARCHIVE" >&2; exit 1; }
[[ "$(uname -s)" == Linux && "$(uname -m)" == x86_64 ]] || { printf 'Linux x86_64 host is required for package verification\n' >&2; exit 1; }
for command in tar sha256sum find sort node; do
    command -v "$command" >/dev/null || { printf 'required command is missing: %s\n' "$command" >&2; exit 1; }
done

mapfile -t entries < <(tar -tzf "$ARCHIVE")
[[ ${#entries[@]} -gt 0 ]] || { printf 'archive is empty\n' >&2; exit 1; }
root="${entries[0]%%/*}"
[[ "$root" =~ ^unbound-(v[^/]+|local-[0-9a-f]+)-linux-amd64$ ]] || { printf 'unexpected archive root: %s\n' "$root" >&2; exit 1; }
for entry in "${entries[@]}"; do
    [[ "$entry" == "$root" || "$entry" == "$root/"* ]] || { printf 'archive has more than one root: %s\n' "$entry" >&2; exit 1; }
    [[ "$entry" != /* && "$entry" != *'../'* && "$entry" != '..' ]] || { printf 'unsafe archive path: %s\n' "$entry" >&2; exit 1; }
done

rm -rf "$EXTRACT_DIRECTORY"
mkdir -p "$EXTRACT_DIRECTORY"
tar -xzf "$ARCHIVE" -C "$EXTRACT_DIRECTORY"
package="$EXTRACT_DIRECTORY/$root"
for required in unbound README.md CHANGELOG.md LICENSE ZAPRET2_LICENSE.txt ZAPRET_LICENSE.txt ENGINE_PROVENANCE.json ENGINE_ASSETS.sha256 BUNDLE_SHA256SUMS.txt runtime/core_bin/linux/amd64/nfqws2 runtime/lua_scripts/zapret-lib.lua runtime/lists scripts/general_autotune.sh; do
    [[ -e "$package/$required" ]] || { printf 'required package content is missing: %s\n' "$required" >&2; exit 1; }
done
for executable in unbound runtime/core_bin/linux/amd64/nfqws2 runtime/core_bin/linux/amd64/ip2net runtime/core_bin/linux/amd64/mdig scripts/general_autotune.sh; do
    [[ -x "$package/$executable" ]] || { printf 'required package executable is not executable: %s\n' "$executable" >&2; exit 1; }
done
if find "$package" -type l -print -quit | grep -q .; then
    printf 'package contains symlinks\n' >&2
    exit 1
fi
if find "$package" -type f -perm /6000 -print -quit | grep -q .; then
    printf 'package contains setuid or setgid payload\n' >&2
    exit 1
fi
if find "$package" -type f -perm -0002 -perm /0111 -print -quit | grep -q .; then
    printf 'package contains world-writable executable\n' >&2
    exit 1
fi
(
    cd "$package"
    sha256sum -c BUNDLE_SHA256SUMS.txt
    expected="$(awk '$2 == "runtime/core_bin/linux/amd64/nfqws2" { print $1 }' BUNDLE_SHA256SUMS.txt)"
    source_expected="$(awk '$2 == "engine/core_bin/linux/amd64/nfqws2" { print $1 }' ENGINE_ASSETS.sha256)"
    [[ "$expected" == "$source_expected" && -n "$expected" ]] || { printf 'nfqws2 package hash does not match ENGINE_ASSETS.sha256\n' >&2; exit 1; }
)
identity="$($package/unbound --version --json)"
IDENTITY="$identity" EXPECTED_VERSION="$VERSION" EXPECTED_COMMIT="$COMMIT" EXPECTED_CHANNEL="$CHANNEL" EXPECTED_DIRTY="$DIRTY" node - <<'NODE'
const identity = JSON.parse(process.env.IDENTITY);
const dirty = process.env.EXPECTED_DIRTY === 'true';
if (identity.version !== process.env.EXPECTED_VERSION || identity.commit !== process.env.EXPECTED_COMMIT || identity.channel !== process.env.EXPECTED_CHANNEL || identity.dirty !== dirty || identity.os !== 'linux' || identity.arch !== 'amd64') process.exit(1);
NODE
"$package/unbound" --help >/dev/null
profiles="$($package/unbound --list-profiles --json)"
PROFILES="$profiles" node -e 'const p = JSON.parse(process.env.PROFILES); if (!Object.values(p).some(Array.isArray) || !Object.values(p).flat().length) process.exit(1)'
test_output="$($package/unbound --test)"
printf '%s\n' "$test_output" | grep -F 'UNBOUND - Connectivity Diagnostic Probe' >/dev/null
printf 'LINUX_PACKAGE_VERIFICATION=PASS archive=%s extract=%s\n' "$ARCHIVE" "$package"
