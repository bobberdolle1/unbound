#!/bin/bash
# UNBOUND Gatekeeper Fix & GUI Launcher for macOS
DIR="$(cd "$(dirname "$0")" && pwd)"

echo "==================================================="
echo "  🍎 UNBOUND macOS Gatekeeper Repair & Launcher"
echo "==================================================="
echo "Снятие атрибута карантина com.apple.quarantine..."

xattr -cr "$DIR" 2>/dev/null || true

if [ -d "$DIR/unbound.app" ]; then
    xattr -cr "$DIR/unbound.app" 2>/dev/null || true
    echo "✓ Атрибуты защиты успешно сняты!"
    echo "Запускаем UNBOUND GUI..."
    open "$DIR/unbound.app"
elif [ -d "$DIR/Unbound.app" ]; then
    xattr -cr "$DIR/Unbound.app" 2>/dev/null || true
    echo "✓ Атрибуты защиты успешно сняты!"
    echo "Запускаем UNBOUND GUI..."
    open "$DIR/Unbound.app"
else
    echo "[!] Ошибка: unbound.app не найден в той же папке!"
    read -p "Нажмите Enter для выхода..."
fi
