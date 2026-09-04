#!/usr/bin/env bash
# ============================================================================
# Generate or verify checksums for the vendored bypass-engine assets.
# ============================================================================
# engine/core_bin holds third-party binaries - winws2.exe from Zapret 2, the
# WinDivert driver, GoodbyeDPI - that are committed rather than fetched. Until
# now nothing recorded which upstream version they came from (a line in the
# CHANGELOG was the only trace) or whether the committed bytes were still the
# bytes someone vetted.
#
# That matters more here than in most projects: this is a censorship-bypass
# tool whose users run these binaries with administrator rights, and the
# WinDivert component is a kernel driver.
#
# Usage:
#   ./scripts/engine-assets.sh verify     # check the committed assets (default)
#   ./scripts/engine-assets.sh generate   # rewrite the manifest after an update
# ============================================================================
set -uo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$REPO_ROOT"

MANIFEST="engine/ENGINE_ASSETS.sha256"

# Trees whose contents are vendored third-party artefacts. engine/lists is
# deliberately absent: those hostlists are data the app updates at runtime and
# the user edits, so pinning them would fail on every ordinary run.
TRACKED_DIRS=(engine/core_bin engine/lua_scripts engine/windivert.filter)

generate() {
    {
        cat <<'HEADER'
# ============================================================================
# Checksums for the vendored bypass-engine assets
# ============================================================================
# Regenerate with:  ./scripts/engine-assets.sh generate
# Verify with:      ./scripts/engine-assets.sh verify   (run by scripts/check.sh)
#
# Provenance
# ----------
#   engine/core_bin/windows/*                 Zapret 2 v1.0.5 Windows x86_64 bundle
#   engine/core_bin/linux/{amd64,arm64}/*     Zapret 2 v1.0.5 Linux release bundle
#   engine/core_bin/windows/goodbyedpi.exe    https://github.com/ValdikSS/GoodbyeDPI
#   engine/core_bin/*.bin                     fake-packet payloads from Zapret releases
#   engine/lua_scripts/zapret-{lib,antidpi,auto,obfs,pcap,tests}.lua
#                                               Zapret 2 v1.0.5; other Lua files are local
#   engine/windivert.filter/*                 local WinDivert filter presets
#
# Zapret 2 snapshot
# -----------------
#   Tag:          v1.0.5
#   Commit:       0b8182d24a887059a628d7266577c4ba8e9b8f2d
#   Release URL:  https://github.com/bol-van/zapret2/releases/tag/v1.0.5
#   Source asset: zapret2-v1.0.5.zip
#   Asset SHA256: d73a4c57dad0f20f473aa62ed950505f0737154c3d9ab8fca717e75f1a21fa69
#
# Windows binaries, Linux binaries and all six standard Lua files are copied
# from that one verified release archive. Never update them independently.
# ============================================================================
HEADER
        for dir in "${TRACKED_DIRS[@]}"; do
            [ -d "$dir" ] || continue
            # git ls-files keeps the manifest to tracked files, so a developer's
            # local scratch files never end up in it.
            git ls-files "$dir" | sort | while IFS= read -r f; do
                # Git-Bash/BusyBox sha256sum defaults to binary mode and
                # writes "hash *path", which verifyEmbeddedAssets would read
                # as a path starting with "*". Normalize to the two-space
                # text-mode separator everywhere.
                [ -f "$f" ] && sha256sum "$f" | sed -e 's/ \*/  /'
            done
        done
    } > "$MANIFEST"

    local n
    n=$(grep -cv '^#' "$MANIFEST")
    echo "Wrote $MANIFEST ($n files)"
}

verify() {
    if [ ! -f "$MANIFEST" ]; then
        echo "error: $MANIFEST is missing - run './scripts/engine-assets.sh generate'" >&2
        return 1
    fi

    local out
    if ! out="$(sha256sum -c --quiet "$MANIFEST" 2>&1)"; then
        echo "error: vendored engine assets do not match the manifest:" >&2
        printf '%s\n' "$out" | sed 's/^/  /' >&2
        echo >&2
        echo "If this was an intentional engine update, rerun:" >&2
        echo "  ./scripts/engine-assets.sh generate" >&2
        return 1
    fi

    # A checksum file only proves the listed files are unchanged; it says
    # nothing about files added since. Catch those too.
    local listed actual
    listed=$(grep -cv '^#' "$MANIFEST")
    actual=0
    for dir in "${TRACKED_DIRS[@]}"; do
        [ -d "$dir" ] || continue
        actual=$((actual + $(git ls-files "$dir" | wc -l)))
    done
    if [ "$listed" -ne "$actual" ]; then
        echo "error: manifest lists $listed files but $actual are tracked" >&2
        echo "Rerun './scripts/engine-assets.sh generate' after adding or removing assets." >&2
        return 1
    fi

    echo "$listed engine assets verified"
}

case "${1:-verify}" in
    generate) generate ;;
    verify)   verify ;;
    *) echo "usage: $0 [generate|verify]" >&2; exit 2 ;;
esac
