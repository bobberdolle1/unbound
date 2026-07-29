#!/bin/bash
# UNBOUND GUI Launcher for macOS
DIR="$(cd "$(dirname "$0")" && pwd)"

# Strip Gatekeeper quarantine and metadata attributes
xattr -cr "$DIR" 2>/dev/null || true
if [ -d "$DIR/unbound.app" ]; then
    xattr -cr "$DIR/unbound.app" 2>/dev/null || true
    echo "✓ Атрибуты карантина сняты. Запускаем UNBOUND GUI..."
    open "$DIR/unbound.app"
elif [ -d "$DIR/Unbound.app" ]; then
    xattr -cr "$DIR/Unbound.app" 2>/dev/null || true
    echo "✓ Атрибуты карантина сняты. Запускаем UNBOUND GUI..."
    open "$DIR/Unbound.app"
else
    echo "[!] Ошибка: unbound.app не найден!"
    read -p "Нажмите Enter для выхода..."
fi
