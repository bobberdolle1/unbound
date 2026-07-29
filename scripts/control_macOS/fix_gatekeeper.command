#!/bin/bash
# UNBOUND Gatekeeper Fix & GUI Launcher for macOS
DIR="$(cd "$(dirname "$0")" && pwd)"

echo "==================================================="
echo "  🍎 UNBOUND macOS Gatekeeper Repair & Launcher"
echo "==================================================="
echo "Снятие атрибута карантина com.apple.quarantine..."

xattr -d com.apple.quarantine "$DIR" 2>/dev/null || true
xattr -cr "$DIR" 2>/dev/null || true

TARGET_APP=""
if [ -d "$DIR/unbound.app" ]; then
    TARGET_APP="$DIR/unbound.app"
elif [ -d "$DIR/Unbound.app" ]; then
    TARGET_APP="$DIR/Unbound.app"
fi

if [ -n "$TARGET_APP" ]; then
    xattr -d com.apple.quarantine "$TARGET_APP" 2>/dev/null || true
    xattr -d com.apple.quarantine "$TARGET_APP"/Contents/MacOS/* 2>/dev/null || true
    xattr -cr "$TARGET_APP" 2>/dev/null || true
    spctl --add "$TARGET_APP" 2>/dev/null || true
    echo "✓ Атрибуты защиты успешно сняты с $(basename "$TARGET_APP")!"
    echo "Запускаем UNBOUND GUI..."
    open "$TARGET_APP"
else
    echo "[!] Ошибка: unbound.app не найден в той же папке!"
    read -p "Нажмите Enter для выхода..."
fi
