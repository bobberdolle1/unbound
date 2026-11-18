#!/bin/bash
# UNBOUND Ultimate Bypass Strategy for macOS
DIR="$(cd "$(dirname "$0")" && pwd)"
UNBOUND=""
for candidate in \
    "$DIR/Unbound.app/Contents/MacOS/Unbound" \
    "$DIR/Unbound.app/Contents/MacOS/unbound" \
    "$DIR/unbound.app/Contents/MacOS/Unbound" \
    "$DIR/unbound.app/Contents/MacOS/unbound" \
    "$DIR/unbound" \
    "$DIR/Unbound" \
    "$DIR/../unbound" \
    "$DIR/../Unbound" \
    "/Applications/Unbound.app/Contents/MacOS/Unbound" \
    "/Applications/Unbound.app/Contents/MacOS/unbound" \
    "/Applications/unbound.app/Contents/MacOS/Unbound" \
    "/Applications/unbound.app/Contents/MacOS/unbound"
do
    if [ -x "$candidate" ] || [ -f "$candidate" ]; then
        UNBOUND="$candidate"
        break
    fi
done

# Remove macOS Gatekeeper quarantine attribute strictly on Unbound.app / binary
if [ -d "$DIR/Unbound.app" ]; then
    xattr -dr com.apple.quarantine "$DIR/Unbound.app" 2>/dev/null || true
elif [ -d "$DIR/unbound.app" ]; then
    xattr -dr com.apple.quarantine "$DIR/unbound.app" 2>/dev/null || true
elif [ -n "$UNBOUND" ] && [ -f "$UNBOUND" ]; then
    xattr -d com.apple.quarantine "$UNBOUND" 2>/dev/null || true
fi

if [ "$(id -u)" -ne 0 ]; then
    echo "Запрос прав администратора (sudo)..."
    exec sudo "$0" "$@"
fi

if [ -n "$UNBOUND" ] && [ -f "$UNBOUND" ]; then
    exec "$UNBOUND" --cli --profile ultimate
else
    echo "[!] Ошибка: исполняемый файл Unbound не найден!"
    read -p "Нажмите Enter для выхода..."
fi
