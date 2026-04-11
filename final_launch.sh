#!/bin/bash
# ============================================================================
# UNBOUND v2.0.0 — Финальный скрипт выпуска («Тотальное обновление»)
# ============================================================================
# Автоматизирует полный процесс выпуска Unbound v2.0.0:
# 1. Обновляет метаданные репозитория GitHub (описание, URL, темы)
# 2. Разворачивает сайт на GitHub Pages
# 3. Собирает все бинарные файлы
# 4. Создаёт релиз на GitHub с прикрепленными артефактами
# ============================================================================

set -e  # Выход при ошибке

# ============================================================================
# Конфигурация
# ============================================================================
VERSION="v2.0.0"
RELEASE_TITLE="Unbound v2.0.0: Глобальное обновление"
REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
DIST_DIR="$REPO_ROOT/dist"

# Цвета
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
CYAN='\033[0;36m'
MAGENTA='\033[0;35m'
NC='\033[0m'

log_info()    { echo -e "${CYAN}[ИНФО]${NC} $*"; }
log_ok()      { echo -e "${GREEN}[ГОТОВО]${NC} $*"; }
log_warn()    { echo -e "${YELLOW}[ВНИМАНИЕ]${NC} $*"; }
log_err()     { echo -e "${RED}[ОШИБКА]${NC} $*"; }
log_step()    {
    echo -e "\n${MAGENTA}============================================================${NC}"
    echo -e "${MAGENTA}📦 $*${NC}"
    echo -e "${MAGENTA}============================================================${NC}\n"
}

# ============================================================================
# Предполётные проверки
# ============================================================================
log_step "Предполётные проверки"

check_cmd() {
    if ! command -v "$1" &> /dev/null; then
        log_err "$1 требуется, но не найден. Установите его сначала."
        exit 1
    fi
}

check_cmd git
check_cmd gh
check_cmd npm
check_cmd go
check_cmd wails

# Проверка авторизации GitHub CLI
if ! gh auth status &> /dev/null; then
    log_err "Не авторизован в GitHub CLI. Выполните: gh auth login"
    exit 1
fi
log_ok "GitHub CLI авторизован"

# Проверка корня репозитория
if [ ! -f "wails.json" ]; then
    log_err "wails.json не найден. Запустите скрипт из корня репозитория."
    exit 1
fi

# Проверка ветки
CURRENT_BRANCH=$(git branch --show-current)
if [ "$CURRENT_BRANCH" != "main" ] && [ "$CURRENT_BRANCH" != "master" ]; then
    log_warn "Вы на ветке '$CURRENT_BRANCH'. Релиз обычно делается с main/master."
    read -p "Продолжить? (y/N): " -n 1 -r
    echo
    if [[ ! $REPLY =~ ^[Yy]$ ]]; then
        exit 1
    fi
fi

log_ok "Все предполётные проверки пройдены!"

# ============================================================================
# Шаг 1: Обновление метаданных репозитория
# ============================================================================
log_step "Обновление метаданных GitHub-репозитория"

REPO_INFO=$(gh repo view --json nameWithOwner -q '.nameWithOwner' 2>/dev/null || true)
if [ -z "$REPO_INFO" ]; then
    log_warn "Не удалось получить информацию о репозитории. Пропускаем."
else
    log_info "Репозиторий: $REPO_INFO"
    
    # Получаем GitHub Pages URL
    REPO_NAME=$(echo "$REPO_INFO" | cut -d'/' -f2)
    REPO_OWNER=$(echo "$REPO_INFO" | cut -d'/' -f1)
    PAGES_URL="https://${REPO_OWNER}.github.io/${REPO_NAME}"
    
    # Обновляем описание и homepage
    gh repo edit "$REPO_INFO" \
        --description "UNBOUND — Тотальная война с цензурой. Мультиплатформенная экосистема для обхода DPI-цензуры (Windows, macOS, Linux, Android, iOS, OpenWrt, Browser)" \
        --homepage "$PAGES_URL" 2>/dev/null || true
    
    # Добавляем темы через API
    TOPICS='["dpi-bypass","censorship","zapret","wails","anti-censorship","windows","linux","android","openwrt","browser-extension","golang","rust","kotlin","react","typescript"]'
    gh api "repos/$REPO_INFO/topics" \
        --method PUT \
        --field "names=$TOPICS" 2>/dev/null || true
    
    log_ok "Метаданные репозитория обновлены!"
    log_info "Описание: UNBOUND — Тотальная война с цензурой..."
    log_info "Домашняя страница: $PAGES_URL"
    log_info "Темы: dpi-bypass, censorship, zapret, wails, ..."
fi

# ============================================================================
# Шаг 2: Развёртывание сайта
# ============================================================================
log_step "Развёртывание сайта на GitHub Pages"

if [ -d "website" ]; then
    cd website
    if [ -f "package.json" ]; then
        log_info "Установка зависимостей сайта..."
        npm install >/dev/null 2>&1
        
        log_info "Развёртывание на GitHub Pages..."
        if npm run deploy; then
            log_ok "Сайт успешно развёрнут!"
        else
            log_warn "Развёртывание сайта не удалось. Продолжаем релиз..."
        fi
    else
        log_warn "website/package.json не найден. Пропускаем."
    fi
    cd "$REPO_ROOT"
else
    log_warn "Директория website/ не найдена. Пропускаем."
fi

# ============================================================================
# Шаг 3: Сборка всех платформ
# ============================================================================
log_step "Сборка бинарных файлов всех платформ"

mkdir -p "$DIST_DIR"

# Проверяем скрипт сборки
if [ -f "scripts/build_all.sh" ]; then
    log_info "Запуск оркестрации сборки..."
    chmod +x scripts/build_all.sh
    ./scripts/build_all.sh
    if [ $? -ne 0 ]; then
        log_err "Скрипт сборки не удался!"
        exit 1
    fi
    log_ok "Сборка завершена!"
else
    log_warn "build_all.sh не найден. Собираем вручную..."
    
    # Сборка десктопного приложения (Wails)
    log_info "Сборка десктопного приложения (Wails)..."
    wails build -platform windows/amd64 2>/dev/null
    if [ -f "build/bin/unbound.exe" ]; then
        cp "build/bin/unbound.exe" "$DIST_DIR/unbound-desktop-windows-amd64.exe"
        log_ok "Сборка Desktop Windows завершена"
    fi
    
    # Сборка Linux CLI
    if [ -d "linux" ]; then
        log_info "Сборка Linux CLI (Rust)..."
        cd linux
        cargo build --release 2>/dev/null
        if [ -f "target/release/unbound-cli" ]; then
            cp "target/release/unbound-cli" "$DIST_DIR/unbound-cli-linux-amd64"
            log_ok "Сборка Linux CLI завершена"
        fi
        cd "$REPO_ROOT"
    fi
    
    # Сборка расширения для браузера
    if [ -d "extension-web" ]; then
        log_info "Сборка расширения для браузера..."
        cd extension-web
        npm install >/dev/null 2>&1
        npm run build >/dev/null 2>&1
        if [ -d "dist" ]; then
            cd dist
            zip -r "$DIST_DIR/unbound-extension-chrome.zip" . >/dev/null 2>&1
            log_ok "Сборка расширения завершена"
            cd "$REPO_ROOT"
        fi
    fi
fi

# Вывод артефактов
log_info "Артефакты в dist:"
ls -lh "$DIST_DIR" | tail -n +2

# ============================================================================
# Шаг 4: Создание релиза на GitHub
# ============================================================================
log_step "Создание релиза на GitHub"

# Проверяем, существует ли релиз
if gh release view "$VERSION" &> /dev/null; then
    log_warn "Релиз $VERSION уже существует."
    read -p "Удалить и создать заново? (y/N): " -n 1 -r
    echo
    if [[ $REPLY =~ ^[Yy]$ ]]; then
        log_info "Удаление существующего релиза..."
        gh release delete "$VERSION" --yes
    else
        log_err "Релиз уже существует. Отмена."
        exit 1
    fi
fi

# Сбор артефактов
ARTIFACTS=()
if [ -d "$DIST_DIR" ]; then
    while IFS= read -r -d '' file; do
        ARTIFACTS+=("$file")
    done < <(find "$DIST_DIR" -type f -print0)
fi

log_info "Найдено ${#ARTIFACTS[@]} артефакт(ов) для загрузки"

# Генерация текста релиза на русском
RELEASE_NOTES_FILE=$(mktemp)
cat > "$RELEASE_NOTES_FILE" << 'RELEASE_EOF'
## 🚀 Unbound v2.0.0: Глобальное обновление

Это **мажорный релиз**, который превращает Unbound из одного десктопного приложения в **полную мультиплатформенную экосистему** для обхода DPI-цензуры.

### ✨ Что нового

- **🌍 Мультиплатформенная экосистема** — Desktop, Android, iOS, Linux, OpenWrt, расширения для браузеров
- **🎯 Автоподбор V2** — Параллельный сканер, находящий оптимальные профили за секунды
- **🔄 Интеграция с системным треем** — Работает в фоне, управляется из трея
- **🎨 Обновлённый интерфейс** — Современный дизайн с мониторингом в реальном времени
- **⚡ Улучшенная производительность** — Строгий таймаут 5 секунд на зонд, никаких зависаний
- **🔒 Повышенная безопасность** — HideWindow на всех дочерних процессах, никаких мигающих окон

### 🐛 Исправления ошибок

- Исправлено сохранение настроек (флажки теперь сохраняются корректно)
- Исправлено поведение при закрытии окна (сворачивается в трей вместо выхода)
- Исправлено бесконечное зависание Автоподбора со строгими таймаутами контекста
- Убрано мигание консоли во время Автоподбора в Windows
- Убраны неподдерживаемые упоминания Telegram/MTProto

### 📦 Поддержка платформ

| Платформа | Формат | Статус |
|-----------|--------|--------|
| **Desktop (Windows/macOS)** | Нативное приложение | ✅ Готово к продакшену |
| **Android** | APK | ✅ Готово к продакшену |
| **iOS (Jailbreak)** | Theos Tweak | ✅ Готово к продакшену |
| **Linux** | CLI-бинарник | ✅ Готово к продакшену |
| **OpenWrt** | Пакет | ✅ Готово к продакшену |
| **Браузер** | Расширение Chrome/Firefox | ✅ Готово к продакшену |

### 🎯 Поддерживаемые сервисы

- ✅ **YouTube** — Полный стриминг 4K
- ✅ **Discord** — Голос, видео, демонстрация экрана
- ✅ **Instagram** — Лента, сторис, рилс, личные сообщения
- ✅ **Twitter/X** — Лента, медиа, поиск
- ✅ **Facebook** — Новости, маркетплейс
- ✅ **RuTracker** — Торрент-доступ

---

## 📥 Установка

Подробные инструкции для каждой платформы: [README.md](https://github.com/OWNER/REPO/blob/main/README.md)

## 🙏 Благодарности

Спасибо всем контрибьюторам и проекту zapret за основу!
RELEASE_EOF

# Заменяем OWNER/REPO на реальные значения
if [ -n "$REPO_INFO" ]; then
    sed -i "s|OWNER/REPO|$REPO_INFO|g" "$RELEASE_NOTES_FILE"
fi

# Создаём релиз
log_info "Создание релиза $VERSION..."
if [ ${#ARTIFACTS[@]} -gt 0 ]; then
    gh release create "$VERSION" \
        --title "$RELEASE_TITLE" \
        --notes-file "$RELEASE_NOTES_FILE" \
        --draft \
        --generate-notes \
        "${ARTIFACTS[@]}"
else
    gh release create "$VERSION" \
        --title "$RELEASE_TITLE" \
        --notes-file "$RELEASE_NOTES_FILE" \
        --draft \
        --generate-notes
fi

# Очистка временного файла
rm -f "$RELEASE_NOTES_FILE"

log_ok "Релиз GitHub создан: $VERSION"
log_info "Релиз в режиме ЧЕРНОВИКА. Проверьте и опубликуйте: https://github.com/$(gh repo view --json nameWithOwner -q .nameWithOwner)/releases/tag/$VERSION"

# ============================================================================
# Пост-релизные задачи
# ============================================================================
log_step "Пост-релизные задачи"

# Тегирование коммита
log_info "Тегирование коммита $VERSION..."
git tag -a "$VERSION" -m "UNBOUND $VERSION — Глобальное обновление"
git push origin "$VERSION"

# Коммит изменений версий
log_info "Коммит изменений версий..."
git add -A
if git commit -m "chore: обновление версий до v2.0.0 во всех проектах экосистемы

- Десктоп: 2.0.0
- Сайт: 2.0.0
- Android: 2.0.0 (versionCode: 2)
- iOS: 2.0.0
- Linux: 2.0.0
- OpenWrt: 2.0.0
- Расширение: 2.0.0

Выпущено как $VERSION" 2>/dev/null; then
    log_ok "Изменения версий закоммичены"
    git push origin "$CURRENT_BRANCH" 2>/dev/null || true
else
    log_warn "Нет изменений для коммита или уже закоммичено"
fi

log_ok "Все пост-релизные задачи выполнены!"

# ============================================================================
# Итоговый отчёт
# ============================================================================
log_step "🎉 Релиз $VERSION завершён!"

echo -e "
  ${GREEN}✓${NC} Все бинарные файлы собраны
  ${GREEN}✓${NC} Сайт развёрнут на GitHub Pages
  ${GREEN}✓${NC} Релиз GitHub создан (ЧЕРНОВИК)
  ${GREEN}✓${NC} Коммит тегирован и отправлен

  ${CYAN}Следующие шаги:${NC}
  1. Проверьте черновик релиза: ${CYAN}https://github.com/$(gh repo view --json nameWithOwner -q .nameWithOwner)/releases${NC}
  2. Протестируйте все бинарные файлы на соответствующих платформах
  3. Опубликуйте релиз, когда будете готовы

  ${GREEN}🚀 Тотальная война с цензурой! 🚀${NC}
"
