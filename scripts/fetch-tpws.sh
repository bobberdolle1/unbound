#!/usr/bin/env bash
# ============================================================================
# Fetch the upstream tpws sources the iOS and tvOS builds compile against.
# ============================================================================
# theos/unbound-legacy/engine/Makefile.tpws and tvos/build-tvos.sh both compile
# 13 upstream C files - tpws.c, tpws_conn.c, helpers.c, hostlist.c, protocol.c,
# tamper.c, resolver.c, redirect.c, params.c, pools.c, sec.c, gzip.c, ipset.c -
# none of which are in this repository. Only the iOS entry point (ios_main.c)
# and the epoll shim are local.
#
# They are not vendored because they are third-party GPL-3.0 code from
# https://github.com/bol-van/zapret; the OpenWrt package fetches them the same
# way, from the same pinned tag.
#
# Usage:
#   ./scripts/fetch-tpws.sh            # fetch the pinned version
#   ./scripts/fetch-tpws.sh --clean    # remove the fetched sources
# ============================================================================
set -euo pipefail

# Keep in step with PKG_SOURCE_VERSION in openwrt/unbound-wrt/Makefile so every
# platform builds the same engine revision.
ZAPRET_REPO="https://github.com/bol-van/zapret.git"
ZAPRET_TAG="v72.13"
ZAPRET_COMMIT="87e058624c72863db53bdaf7fb6f16576dddb6ab"

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
DEST="$REPO_ROOT/theos/unbound-legacy/engine/tpws"

# Local files that must never be overwritten or removed: ios_main.c is the
# platform entry point, tpws.h is *our* wrapper header declaring
# tpws_init()/tpws_run_loop(), and darwin_compat.h is a local shim. Note that
# tpws.c pairs with tpws.h by name, which is exactly how an earlier version of
# this script clobbered it.
LOCAL_FILES="ios_main.c tpws.h darwin_compat.h"

is_local() {
    case " $LOCAL_FILES " in *" $1 "*) return 0 ;; *) return 1 ;; esac
}

# The files Makefile.tpws and build-tvos.sh expect to find next to ios_main.c.
SOURCES=(
    tpws.c tpws_conn.c helpers.c hostlist.c protocol.c tamper.c
    resolver.c redirect.c params.c pools.c sec.c gzip.c ipset.c
)

if [ "${1:-}" = "--clean" ]; then
    for f in "${SOURCES[@]}"; do
        is_local "$f" || rm -f "$DEST/$f"
        header="${f%.c}.h"
        is_local "$header" || rm -f "$DEST/$header"
    done
    echo "Removed fetched upstream sources from $DEST"
    exit 0
fi

command -v git >/dev/null || { echo "git is required" >&2; exit 1; }

TMP="$(mktemp -d)"
trap 'rm -rf "$TMP"' EXIT

echo "==> Cloning $ZAPRET_REPO at $ZAPRET_TAG"
git clone --quiet --depth 1 --branch "$ZAPRET_TAG" "$ZAPRET_REPO" "$TMP/zapret"

# Verify we got the revision we pinned, not whatever the tag points at today:
# a tag can be moved, a commit cannot.
actual="$(git -C "$TMP/zapret" rev-parse HEAD)"
if [ "$actual" != "$ZAPRET_COMMIT" ]; then
    echo "::error::$ZAPRET_TAG resolves to $actual, expected $ZAPRET_COMMIT" >&2
    echo "The upstream tag has moved. Review the change before bumping the pin." >&2
    exit 1
fi
echo "==> Verified commit $actual"

SRC="$TMP/zapret/tpws"
[ -d "$SRC" ] || { echo "upstream layout changed: no tpws/ directory" >&2; exit 1; }

mkdir -p "$DEST"
copied=0
for f in "${SOURCES[@]}"; do
    if [ ! -f "$SRC/$f" ]; then
        echo "::error::upstream is missing $f - layout changed?" >&2
        exit 1
    fi
    cp "$SRC/$f" "$DEST/$f"
    # Headers sit beside their .c files upstream; copy any that exist - except
    # the local shims. tpws.c pairs with tpws.h, which here is *our* wrapper
    # header declaring tpws_init()/tpws_run_loop(), not upstream's.
    header="${f%.c}.h"
    if ! is_local "$header" && [ -f "$SRC/$header" ]; then
        cp "$SRC/$header" "$DEST/$header"
    fi
    copied=$((copied + 1))
done

# Shared headers the sources include but which have no matching .c file.
for h in "$SRC"/*.h; do
    [ -f "$h" ] || continue
    base="$(basename "$h")"
    is_local "$base" && continue
    cp -n "$h" "$DEST/$base" 2>/dev/null || true
done

echo "==> Fetched $copied sources into $DEST"
echo "    These are GPL-3.0 from bol-van/zapret and are gitignored."
