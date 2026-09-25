#!/usr/bin/env bash
# Provider-independent frontend CI entrypoint.
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
cd "$ROOT"
# shellcheck source=common.sh
source ./scripts/ci/common.sh

ci_check_checkout
ci_frontend
