#!/usr/bin/env bash
# Shared, provider-independent CI checks. Source from platform entrypoints.
set -euo pipefail

ci_root() {
    cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd
}

ci_fail() {
    printf 'CI failure: %s\n' "$*" >&2
    exit 1
}

ci_check_checkout() {
    local actual expected
    actual="$(git rev-parse HEAD)"
    expected="${EXPECTED_SHA:-${BUILDKITE_COMMIT:-${GITHUB_SHA:-$actual}}}"
    printf 'EXPECTED_SHA=%s\nACTUAL_SHA=%s\n' "$expected" "$actual"
    [[ "$expected" = "$actual" ]] || ci_fail "checkout does not match the requested commit"
}

ci_require_go() {
    local directive required_minor installed
    directive="$(sed -nE 's/^go ([0-9]+\.[0-9]+)(\.[0-9]+)?$/\1\2/p' go.mod)"
    [[ -n "$directive" ]] || ci_fail "cannot read the Go directive from go.mod"
    required_minor="${directive%.*}"
    installed="$(go env GOVERSION)"
    printf 'GO_REQUIRED=%s\nGO_ACTUAL=%s\n' "$directive" "$installed"
    [[ "$installed" = "go${required_minor}."* ]] || ci_fail "Go $directive-compatible toolchain is required"
}

ci_require_node() {
    local required actual required_major actual_major
    required="${NODE_VERSION:-22.13.0}"
    actual="$(node --version | sed 's/^v//')"
    required_major="${required%%.*}"
    actual_major="${actual%%.*}"
    printf 'NODE_REQUIRED=%s\nNODE_ACTUAL=%s\n' "$required" "$actual"
    command -v npm >/dev/null 2>&1 || ci_fail "npm is required"
    (( actual_major >= required_major )) || ci_fail "Node $required or newer is required"
}

ci_frontend() {
    ci_require_node
    (
        cd frontend
        npm ci
        npm run typecheck
        npm test
        npm run build
    )
}

ci_gofmt() {
    local unformatted
    unformatted="$(gofmt -l $(git ls-files -- '*.go'))"
    [[ -z "$unformatted" ]] || ci_fail "gofmt required for:\n$unformatted"
}

ci_go_checks() {
    ci_require_go
    ci_gofmt
    go vet ./...
    go test ./... -count=1
    if [[ "$(go env CGO_ENABLED)" = "1" ]]; then
        go test -race ./... -count=1
    else
        printf 'RACE_TEST=SKIPPED_CGO_DISABLED\n'
    fi
}

ci_cross_builds() {
    local pair goos goarch
    for pair in linux/amd64 linux/arm64 windows/amd64 darwin/amd64 darwin/arm64; do
        goos="${pair%%/*}"
        goarch="${pair##*/}"
        GOOS="$goos" GOARCH="$goarch" CGO_ENABLED=0 GOMIPS=softfloat go build ./engine/...
    done
}

ci_assets() {
    bash ./scripts/engine-assets.sh verify
}

ci_shell() {
    local script interpreter checked=0
    while IFS= read -r script; do
        [[ -f "$script" ]] || continue
        case "$(head -1 "$script")" in
            *bash*) interpreter=bash ;;
            *) interpreter=sh ;;
        esac
        "$interpreter" -n "$script"
        checked=$((checked + 1))
    done < <(git ls-files '*.sh')
    printf 'SHELL_SCRIPTS_CHECKED=%s\n' "$checked"
}

ci_actionlint() {
    go run github.com/rhysd/actionlint/cmd/actionlint@v1.7.12 .github/workflows/ci.yml .github/workflows/release.yml
}
