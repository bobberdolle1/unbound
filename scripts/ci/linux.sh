#!/usr/bin/env bash
# Provider-independent Linux CI entrypoint. No privileged or network-mutating checks.
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
cd "$ROOT"
# shellcheck source=common.sh
source ./scripts/ci/common.sh

TARGET="${1:-all}"
case "$TARGET" in
    static)
        ci_check_checkout
        ci_require_go
        ci_actionlint
        ci_assets
        ci_shell
        ;;
    frontend)
        exec ./scripts/ci/frontend.sh
        ;;
    go)
        ci_check_checkout
        ci_go_checks
        ci_cross_builds
        ;;
    all)
        ci_check_checkout
        ci_require_go
        ci_actionlint
        ci_assets
        ci_shell
        ci_frontend
        ci_go_checks
        ci_cross_builds
        GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build ./...
        ;;
    *)
        printf 'usage: %s [all|static|frontend|go]\n' "$0" >&2
        exit 2
        ;;
esac
