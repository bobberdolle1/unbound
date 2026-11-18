#!/usr/bin/env bash
# UNBOUND AutoTune for Linux
set -euo pipefail

DIR="$(cd "$(dirname "$0")" && pwd)"
ARCH_SUFFIX="amd64"
case "$(uname -m)" in
    aarch64|arm64) ARCH_SUFFIX="arm64" ;;
esac

UNBOUND=""
for candidate in \
    "$DIR/unbound-linux-$ARCH_SUFFIX" \
    "$DIR/unbound-linux-amd64" \
    "$DIR/unbound-linux-arm64" \
    "$DIR/unbound-linux" \
    "$DIR/unbound" \
    "$DIR/../unbound-linux-$ARCH_SUFFIX" \
    "$DIR/../unbound-linux-amd64" \
    "$DIR/../unbound-linux-arm64" \
    "$DIR/../unbound-linux" \
    "$DIR/../unbound" \
    "$DIR/../../build/bin/unbound-linux-$ARCH_SUFFIX"
do
    if [ -f "$candidate" ]; then
        UNBOUND="$candidate"
        break
    fi
done

if [ "$(id -u)" -ne 0 ]; then
    echo "Запрос root прав (sudo)..."
    exec sudo "$0" "$@"
fi

if [ -n "$UNBOUND" ] && [ -f "$UNBOUND" ]; then
    exec "$UNBOUND" --autotune
else
    echo "[!] Ошибка: исполняемый файл unbound (unbound-linux-amd64 / unbound-linux-arm64 / unbound) не найден!" >&2
    exit 1
fi
