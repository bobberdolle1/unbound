#!/bin/bash
# UNBOUND Gatekeeper Fix & GUI Launcher for macOS
set -euo pipefail

DIR="$(cd "$(dirname "$0")" && pwd)"

echo "==================================================="
echo "  🍎 UNBOUND macOS Gatekeeper Repair & Launcher"
echo "==================================================="

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
    echo "[1/3] Снятие атрибута карантина com.apple.quarantine с $APP_PATH..."
    xattr -dr com.apple.quarantine "$APP_PATH" 2>/dev/null || true

    echo "[2/3] Проверка целостности подписи пакета (сохранение entitlements)..."
    if codesign --verify --deep --strict "$APP_PATH" >/dev/null 2>&1; then
        echo "      ✓ Существующая подпись и hardened runtime валидны."
    else
        echo "      ! Подпись не подтверждена; запуск будет выполнен после снятия карантина без перезаписи entitlements."
    fi

    echo "[3/3] Запуск приложения..."
    echo "✓ Приложение подготовлено!"
    open "$APP_PATH"
else
    echo "[!] Ошибка: Unbound.app не найден ни в текущей папке, ни в /Applications!"
    echo "Пожалуйста, скопируйте Unbound.app в папку «Программы» (Applications) и повторите запуск."
    read -r -p "Нажмите Enter для выхода..."
fi
