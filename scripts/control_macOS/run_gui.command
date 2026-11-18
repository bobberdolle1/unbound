#!/bin/bash
# UNBOUND GUI Launcher for macOS
set -euo pipefail

DIR="$(cd "$(dirname "$0")" && pwd)"

APP_PATH=""
for candidate in \
    "$DIR/Unbound.app" \
    "$DIR/unbound.app" \
    "/Applications/Unbound.app" \
    "/Applications/unbound.app" \
    "$HOME/Applications/Unbound.app"
do
    if [ -d "$candidate" ]; then
        APP_PATH="$candidate"
        break
    fi
done

if [ -n "$APP_PATH" ]; then
    xattr -dr com.apple.quarantine "$APP_PATH" 2>/dev/null || true
    echo "✓ Атрибут карантина снят с $APP_PATH. Запускаем UNBOUND GUI..."
    open "$APP_PATH"
else
    echo "[!] Ошибка: Unbound.app не найден!"
    read -r -p "Нажмите Enter для выхода..."
fi
