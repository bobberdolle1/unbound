#!/usr/bin/env bash
# UNBOUND Discord Strategy for Linux
DIR="$(cd "$(dirname "$0")" && pwd)"
UNBOUND="$DIR/unbound-linux"
[ ! -f "$UNBOUND" ] && [ -f "$DIR/unbound" ] && UNBOUND="$DIR/unbound"
[ ! -f "$UNBOUND" ] && [ -f "$DIR/../unbound-linux" ] && UNBOUND="$DIR/../unbound-linux"

if [ "$(id -u)" -ne 0 ]; then
    echo "Запрос root прав (sudo)..."
    exec sudo "$0" "$@"
fi

if [ -f "$UNBOUND" ]; then
    exec "$UNBOUND" --cli --profile discord
else
    echo "[!] Ошибка: unbound-linux не найден!"
    exit 1
fi
