#!/bin/bash
# UNBOUND GUI Launcher for macOS
DIR="$(cd "$(dirname "$0")" && pwd)"

xattr -d com.apple.quarantine "$DIR" 2>/dev/null || true
xattr -cr "$DIR" 2>/dev/null || true

TARGET_APP=""
for candidate in \
    "$DIR/Unbound.app" \
    "$DIR/unbound.app" \
    "/Applications/Unbound.app" \
    "/Applications/unbound.app" \
    "$HOME/Applications/Unbound.app"
do
    if [ -d "$candidate" ]; then
        TARGET_APP="$candidate"
        break
    fi
done

if [ -n "$TARGET_APP" ]; then
    xattr -dr com.apple.quarantine "$TARGET_APP" 2>/dev/null || true
    xattr -cr "$TARGET_APP" 2>/dev/null || true
    codesign --force --deep -s - "$TARGET_APP" 2>/dev/null || true
    echo "✓ Атрибуты карантина сняты. Запускаем UNBOUND GUI..."
    open "$TARGET_APP"
else
    echo "[!] Ошибка: Unbound.app не найден!"
    read -p "Нажмите Enter для выхода..."
fi
