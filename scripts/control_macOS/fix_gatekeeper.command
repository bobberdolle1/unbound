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
    echo "[1/4] Снятие атрибутов карантина с $TARGET_APP..."
    xattr -dr com.apple.quarantine "$TARGET_APP" 2>/dev/null || true
    xattr -cr "$TARGET_APP" 2>/dev/null || true

    echo "[2/4] Восстановление ad-hoc подписи..."
    codesign --force --deep -s - "$TARGET_APP" 2>/dev/null || true

    echo "[3/4] Добавление в Gatekeeper (spctl)..."
    spctl --add "$TARGET_APP" 2>/dev/null || true

    echo "[4/4] Запуск приложения..."
    echo "✓ Приложение успешно подготовлено!"
    open "$TARGET_APP"
else
    echo "[!] Ошибка: Unbound.app не найден ни в текущей папке, ни в /Applications!"
    echo "Пожалуйста, скопируйте Unbound.app в папку «Программы» (Applications) и повторите запуск."
    read -p "Нажмите Enter для выхода..."
fi
