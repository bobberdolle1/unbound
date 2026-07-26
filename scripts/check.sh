#!/usr/bin/env bash
# ============================================================================
# UNBOUND — local verification, equivalent to .github/workflows/ci.yml
# ============================================================================
# GitHub Actions is currently unavailable on this repository (account billing),
# so nothing verifies a change before it reaches master. This script runs the
# same checks ci.yml would, on your own machine, and is the thing to run before
# pushing until Actions is restored.
#
# Usage:
#   ./scripts/check.sh            # everything
#   ./scripts/check.sh go         # gofmt + vet + tests + cross-compile
#   ./scripts/check.sh frontend   # type-check + build the desktop UI
#   ./scripts/check.sh website    # build the Astro site
#   ./scripts/check.sh extension  # type-check + build the browser extension
#   ./scripts/check.sh decky      # build the Steam Deck plugin
#   ./scripts/check.sh --quick    # skip cross-compilation (the slow part)
#
# Exit code is non-zero if any check fails, so it is usable as a pre-push hook:
#   ln -s ../../scripts/check.sh .git/hooks/pre-push
# ============================================================================
set -uo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$REPO_ROOT"

# ── Output ───────────────────────────────────────────────────────────────────
if [ -t 1 ]; then
    RED=$'\033[0;31m'; GREEN=$'\033[0;32m'; YELLOW=$'\033[1;33m'
    CYAN=$'\033[0;36m'; BOLD=$'\033[1m'; NC=$'\033[0m'
else
    RED=''; GREEN=''; YELLOW=''; CYAN=''; BOLD=''; NC=''
fi

FAILURES=()
step()  { printf '\n%s==>%s %s%s%s\n' "$CYAN" "$NC" "$BOLD" "$1" "$NC"; }
ok()    { printf '  %s✓%s %s\n' "$GREEN" "$NC" "$1"; }
warn()  { printf '  %s!%s %s\n' "$YELLOW" "$NC" "$1"; }
fail()  { printf '  %s✗%s %s\n' "$RED" "$NC" "$1"; FAILURES+=("$1"); }

have()  { command -v "$1" >/dev/null 2>&1; }

# run <label> <command...> — report pass/fail, capture output only on failure.
run() {
    local label="$1"; shift
    local output
    if output="$("$@" 2>&1)"; then
        ok "$label"
        return 0
    fi
    fail "$label"
    printf '%s\n' "$output" | sed 's/^/      /'
    return 1
}

# ── Arguments ────────────────────────────────────────────────────────────────
TARGET="all"
QUICK=0
for arg in "$@"; do
    case "$arg" in
        --quick) QUICK=1 ;;
        go|frontend|website|extension|decky|all) TARGET="$arg" ;;
        -h|--help) sed -n '2,22p' "$0" | sed 's/^# \?//'; exit 0 ;;
        *) echo "Unknown argument: $arg (try --help)" >&2; exit 2 ;;
    esac
done

# ── Frontend ─────────────────────────────────────────────────────────────────
# Runs first: main.go embeds frontend/dist with //go:embed, so the Go build
# needs those assets to exist.
check_frontend() {
    step "Frontend (desktop UI)"
    if ! have npm; then
        warn "npm not installed - skipping"
        return
    fi

    if [ ! -d frontend/node_modules ]; then
        run "npm ci" bash -c 'cd frontend && npm ci --no-audit --no-fund'
    fi
    run "tsc --noEmit"  bash -c 'cd frontend && npm run typecheck'
    run "vite build"    bash -c 'cd frontend && npm run build'
}

# ── Browser extension ────────────────────────────────────────────────────────
# Nothing covered this before, which is how four type errors and a dead module
# reached the tree. `npm run build` now type-checks first, so one call is enough.
check_extension() {
    step "Browser extension (Chrome + Firefox)"
    if ! have npm; then
        warn "npm not installed - skipping"
        return
    fi

    if [ ! -d extension-web/node_modules ]; then
        run "npm ci" bash -c 'cd extension-web && npm ci --no-audit --no-fund'
    fi
    if ! run "type-check + build" bash -c 'cd extension-web && npm run build'; then
        return
    fi

    for target in chrome firefox; do
        if [ -f "extension-web/dist/$target/manifest.json" ]; then
            ok "$target manifest produced"
        else
            fail "$target build produced no manifest.json"
        fi
    done
}

# ── Steam Deck plugin ────────────────────────────────────────────────────────
check_decky() {
    step "Decky plugin (Steam Deck)"
    if ! have npm; then
        warn "npm not installed - skipping"
        return
    fi
    if [ ! -f decky-plugin/package.json ]; then
        warn "decky-plugin/package.json not found - skipping"
        return
    fi

    if [ ! -d decky-plugin/node_modules ]; then
        run "npm ci" bash -c 'cd decky-plugin && npm ci --no-audit --no-fund'
    fi
    run "rollup build" bash -c 'cd decky-plugin && npm run build'
}

# ── Website ──────────────────────────────────────────────────────────────────
check_website() {
    step "Website (Astro)"
    if ! have npm; then
        warn "npm not installed - skipping"
        return
    fi

    if [ ! -d website/node_modules ]; then
        run "npm ci" bash -c 'cd website && npm ci --no-audit --no-fund'
    fi
    if ! run "astro build" bash -c 'cd website && npm run build'; then
        return
    fi

    # Same guards pages.yml applies before publishing.
    if [ ! -f website/dist/index.html ]; then
        fail "website/dist/index.html missing after build"
    elif ! grep -q '/unbound/' website/dist/index.html; then
        fail "built HTML has no /unbound/ base path - check astro.config.mjs"
    else
        ok "base path /unbound/ present"
    fi
}

# ── Go ───────────────────────────────────────────────────────────────────────
check_go() {
    step "Go"
    if ! have go; then
        warn "go not installed - skipping"
        return
    fi

    # //go:embed all:frontend/dist fails the entire build if the directory is
    # absent, which is a confusing way to learn the frontend was never built.
    if [ ! -d frontend/dist ]; then
        fail "frontend/dist missing - run './scripts/check.sh frontend' first"
        return
    fi

    local unformatted
    unformatted="$(gofmt -l . 2>/dev/null | grep -v '^frontend/' || true)"
    if [ -n "$unformatted" ]; then
        fail "gofmt: these files need formatting"
        printf '%s\n' "$unformatted" | sed 's/^/      /'
    else
        ok "gofmt"
    fi

    run "go vet (host)"  go vet ./...
    run "go test -race"  go test -race ./...

    # The firewall rules are otherwise only compared as strings, which cannot
    # catch a spec the kernel rejects. These tests hand the generated rules to
    # the real iptables/nft, in a chain nothing jumps to, so no packet is ever
    # matched. They need root, so they are skipped for an ordinary run.
    if [ "$(id -u)" -eq 0 ] && [ "$(uname -s)" = "Linux" ]; then
        run "live firewall rules" env UNBOUND_FIREWALL_TEST=1 \
            go test ./engine/providers/ -run Live -count=1
    else
        warn "live firewall rule test skipped (needs root on Linux)"
    fi

    # Build tags are the easiest thing to get wrong in this codebase, and a
    # host-only vet will not catch it: the whole reason CI grew a cross-compile
    # matrix is that five files once referenced a Windows-only symbol with no
    # build constraint and broke every non-Windows build.
    step "Cross-platform vet"
    for goos in linux darwin windows; do
        run "GOOS=$goos go vet ./..." env GOOS="$goos" go vet ./...
    done

    if [ "$QUICK" -eq 1 ]; then
        warn "cross-compilation skipped (--quick)"
        return
    fi

    step "Cross-compilation"
    # The GUI needs cgo (webkit2gtk / Cocoa / WebView2) which is unavailable
    # when cross-compiling, so check the packages that must build everywhere.
    for pair in linux/amd64 linux/arm64 linux/mipsle windows/amd64 darwin/amd64 darwin/arm64; do
        local goos="${pair%%/*}" goarch="${pair##*/}"
        run "$pair" env GOOS="$goos" GOARCH="$goarch" CGO_ENABLED=0 GOMIPS=softfloat \
            go build -o /dev/null ./engine/...
    done
}

# ── Run ──────────────────────────────────────────────────────────────────────
printf '%sUnbound — local checks%s  (mirrors .github/workflows/ci.yml)\n' "$BOLD" "$NC"

case "$TARGET" in
    frontend)  check_frontend ;;
    website)   check_website ;;
    extension) check_extension ;;
    decky)     check_decky ;;
    go)        check_go ;;
    all)       check_frontend; check_go; check_website; check_extension; check_decky ;;
esac

printf '\n'
if [ ${#FAILURES[@]} -eq 0 ]; then
    printf '%s✓ all checks passed%s\n' "$GREEN$BOLD" "$NC"
    exit 0
fi

printf '%s✗ %d check(s) failed:%s\n' "$RED$BOLD" "${#FAILURES[@]}" "$NC"
printf '  - %s\n' "${FAILURES[@]}"
exit 1
