#!/bin/bash
# UNBOUND Telegram Strategy for macOS
DIR="$(cd "$(dirname "$0")" && pwd)"
UNBOUND="$DIR/unbound"
[ ! -f "$UNBOUND" ] && [ -f "$DIR/../unbound" ] && UNBOUND="$DIR/../unbound"
[ ! -f "$UNBOUND" ] && [ -d "$DIR/unbound.app" ] && UNBOUND="$DIR/unbound.app/Contents/MacOS/unbound"
[ ! -f "$UNBOUND" ] && [ -d "$DIR/Unbound.app" ] && UNBOUND="$DIR/Unbound.app/Contents/MacOS/unbound"

# Remove macOS Gatekeeper quarantine attribute if present
xattr -dr com.apple.quarantine "$DIR" 2>/dev/null || true
if [ -d "$DIR/unbound.app" ]; then
    xattr -dr com.apple.quarantine "$DIR/unbound.app" 2>/dev/null || true
fi
if [ "$(id -u)" -ne 0 ]; then
    echo "Запрос прав администратора (sudo)..."
    exec sudo "$0" "$@"
fi

if [ -f "$UNBOUND" ]; then
    exec "$UNBOUND" --cli --profile telegram
else
    echo "[!] Ошибка: unbound не найден!"
    read -p "Нажмите Enter для выхода..."
fi
