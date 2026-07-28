#!/bin/bash
# UNBOUND YouTube Strategy for macOS
DIR="$(cd "$(dirname "$0")" && pwd)"
UNBOUND="$DIR/unbound"
[ ! -f "$UNBOUND" ] && [ -f "$DIR/../unbound" ] && UNBOUND="$DIR/../unbound"
[ ! -f "$UNBOUND" ] && [ -d "$DIR/unbound.app" ] && UNBOUND="$DIR/unbound.app/Contents/MacOS/unbound"
[ ! -f "$UNBOUND" ] && [ -d "$DIR/Unbound.app" ] && UNBOUND="$DIR/Unbound.app/Contents/MacOS/unbound"

if [ "$(id -u)" -ne 0 ]; then
    echo "Запрос прав администратора (sudo)..."
    exec sudo "$0" "$@"
fi

if [ -f "$UNBOUND" ]; then
    exec "$UNBOUND" --cli --profile youtube
else
    echo "[!] Ошибка: unbound не найден!"
    read -p "Нажмите Enter для выхода..."
fi
