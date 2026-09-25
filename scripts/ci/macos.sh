#!/usr/bin/env bash
# Provider-independent macOS CI entrypoint. It never starts tpws measurement.
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
cd "$ROOT"
# shellcheck source=common.sh
source ./scripts/ci/common.sh

TARGET="${1:-all}"
case "$TARGET" in
    frontend)
        exec ./scripts/ci/frontend.sh
        ;;
    go)
        ci_check_checkout
        ci_go_checks
        GOOS=darwin GOARCH=amd64 CGO_ENABLED=0 go build ./engine/...
        ;;
    all)
        ci_check_checkout
        ci_frontend
        ci_go_checks
        GOOS=darwin GOARCH=amd64 CGO_ENABLED=0 go build ./engine/...
        ;;
    *)
        printf 'usage: %s [all|frontend|go]\n' "$0" >&2
        exit 2
        ;;
esac
