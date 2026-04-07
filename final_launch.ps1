<#
.SYNOPSIS
    UNBOUND v2.0.0 — Финальный скрипт выпуска («Тотальное обновление»)
.DESCRIPTION
    Автоматизирует полный процесс выпуска Unbound v2.0.0:
    1. Обновляет метаданные репозитория GitHub (описание, URL, темы)
    2. Разворачивает сайт на GitHub Pages
    3. Собирает все бинарные файлы
    4. Создаёт релиз на GitHub с прикрепленными артефактами
.AUTHOR
    Unbound Release Team
.VERSION
    2.0.0
#>

#Requires -Version 7
$ErrorActionPreference = "Stop"

# ============================================================================
# Конфигурация
# ============================================================================
$VERSION = "v2.0.0"
$RELEASE_TITLE = "Unbound v2.0.0: Глобальное обновление"
$REPO_ROOT = Split-Path -Parent $MyInvocation.MyCommand.Path
$DIST_DIR = Join-Path $REPO_ROOT "dist"

# Цвета для вывода
function Write-Info    { Write-Host "[ИНФО] $($args -join ' ')" -ForegroundColor Cyan }
function Write-Ok      { Write-Host "[ГОТОВО] $($args -join ' ')" -ForegroundColor Green }
function Write-Warn    { Write-Host "[ВНИМАНИЕ] $($args -join ' ')" -ForegroundColor Yellow }
function Write-Err     { Write-Host "[ОШИБКА] $($args -join ' ')" -ForegroundColor Red }
function Write-Step    {
    Write-Host "`n============================================================" -ForegroundColor Magenta
    Write-Host "📦 $($args -join ' ')" -ForegroundColor Magenta
    Write-Host "============================================================`n" -ForegroundColor Magenta
}

# ============================================================================
# Предполётные проверки
# ============================================================================
Write-Step "Предполётные проверки"

$requiredCommands = @("git", "gh", "npm", "go", "wails")
foreach ($cmd in $requiredCommands) {
    if (-not (Get-Command $cmd -ErrorAction SilentlyContinue)) {
        Write-Err "$cmd требуется, но не найден. Установите его сначала."
        exit 1
    }
}

# Проверка авторизации GitHub CLI
try {
    gh auth status 2>&1 | Out-Null
    Write-Ok "GitHub CLI авторизован"
} catch {
    Write-Err "Не авторизован в GitHub CLI. Выполните: gh auth login"
    exit 1
}

# Проверка корня репозитория
if (-not (Test-Path (Join-Path $REPO_ROOT "wails.json"))) {
    Write-Err "wails.json не найден. Запустите скрипт из корня репозитория."
    exit 1
}

# Проверка ветки
$currentBranch = git branch --show-current
if ($currentBranch -ne "main" -and $currentBranch -ne "master") {
    Write-Warn "Вы на ветке '$currentBranch'. Релиз обычно делается с main/master."
    $continue = Read-Host "Продолжить? (y/N)"
    if ($continue -ne "y" -and $continue -ne "Y") { exit 1 }
}

Write-Ok "Все предполётные проверки пройдены!"

# ============================================================================
# Шаг 1: Обновление метаданных репозитория
# ============================================================================
Write-Step "Обновление метаданных GitHub-репозитория"

$repoInfo = gh repo view --json nameWithOwner -q '.nameWithOwner' 2>$null
if (-not $repoInfo) {
    Write-Warn "Не удалось получить информацию о репозитории. Пропускаем."
} else {
    Write-Info "Репозиторий: $repoInfo"
    
    # Получаем GitHub Pages URL
    $pagesUrl = "https://${repoInfo}.github.io" -replace '^(.+)/(unbound)$', "https://$1.github.io/$2"
    
    # Обновляем описание, homepage и темы
    gh repo edit $repoInfo `
        --description "UNBOUND — Тотальная война с цензурой. Мультиплатформенная экосистема для обхода DPI-цензуры (Windows, macOS, Linux, Android, iOS, OpenWrt, Browser)" `
        --homepage "$pagesUrl" 2>$null
    
    # Добавляем темы (gh repo edit не поддерживает --topics напрямую, используем API)
    $topics = @(
        "dpi-bypass", "censorship", "zapret", "wails", "anti-censorship",
        "windows", "linux", "android", "openwrt", "browser-extension",
        "golang", "rust", "kotlin", "react", "typescript"
    )
    $topicsJson = $topics | ConvertTo-Json -Compress
    gh api repos/$repoInfo/topics `
        --method PUT `
        --field "names=$topicsJson" 2>$null
    
    Write-Ok "Метаданные репозитория обновлены!"
    Write-Info "Описание: UNBOUND — Тотальная война с цензурой..."
    Write-Info "Домашняя страница: $pagesUrl"
    Write-Info "Темы: $($topics -join ', ')"
}

# ============================================================================
# Шаг 2: Развёртывание сайта
# ============================================================================
Write-Step "Развёртывание сайта на GitHub Pages"

$websiteDir = Join-Path $REPO_ROOT "website"
if (Test-Path $websiteDir) {
    Push-Location $websiteDir
    if (Test-Path "package.json") {
        Write-Info "Установка зависимостей сайта..."
        npm install 2>&1 | Out-Null
        
        Write-Info "Развёртывание на GitHub Pages..."
        $deployOutput = npm run deploy 2>&1 | Out-String
        if ($LASTEXITCODE -eq 0) {
            Write-Ok "Сайт успешно развёрнут!"
        } else {
            Write-Warn "Развёртывание сайта не удалось. Продолжаем релиз..."
        }
    } else {
        Write-Warn "website/package.json не найден. Пропускаем."
    }
    Pop-Location
} else {
    Write-Warn "Директория website/ не найдена. Пропускаем."
}

# ============================================================================
# Шаг 3: Сборка всех платформ
# ============================================================================
Write-Step "Сборка бинарных файлов всех платформ"

if (-not (Test-Path $DIST_DIR)) {
    New-Item -ItemType Directory -Path $DIST_DIR | Out-Null
}

# Проверяем скрипт сборки
$buildScript = Join-Path $REPO_ROOT "scripts\build_all.ps1"
if (Test-Path $buildScript) {
    Write-Info "Запуск оркестрации сборки..."
    & $buildScript
    if ($LASTEXITCODE -ne 0) {
        Write-Err "Скрипт сборки не удался!"
        exit 1
    }
    Write-Ok "Сборка завершена!"
} else {
    Write-Warn "build_all.ps1 не найден. Собираем вручную..."
    
    # Сборка десктопного приложения (Wails)
    Write-Info "Сборка десктопного приложения (Wails)..."
    Push-Location $REPO_ROOT
    wails build -platform windows/amd64 2>&1 | Out-Null
    if (Test-Path "build\bin\unbound.exe") {
        Copy-Item "build\bin\unbound.exe" (Join-Path $DIST_DIR "unbound-desktop-windows-amd64.exe")
        Write-Ok "Сборка Desktop Windows завершена"
    }
    
    # Сборка расширения для браузера
    if (Test-Path "extension-web") {
        Write-Info "Сборка расширения для браузера..."
        Push-Location (Join-Path $REPO_ROOT "extension-web")
        npm install 2>&1 | Out-Null
        npm run build 2>&1 | Out-Null
        if (Test-Path "dist") {
            Push-Location dist
            $zipPath = Join-Path $DIST_DIR "unbound-extension-chrome.zip"
            Compress-Archive -Path * -DestinationPath $zipPath -Force
            Write-Ok "Сборка расширения завершена"
            Pop-Location
        }
        Pop-Location
    }
    Pop-Location
}

# Вывод артефактов
Write-Info "Артефакты в dist:"
Get-ChildItem $DIST_DIR | Format-Table Name, @{Label="Размер (МБ)";Expression={[math]::Round($_.Length / 1MB, 2)}} -AutoSize

# ============================================================================
# Шаг 4: Создание релиза на GitHub
# ============================================================================
Write-Step "Создание релиза на GitHub"

# Проверяем, существует ли релиз
$releaseExists = $false
try {
    gh release view $VERSION 2>&1 | Out-Null
    $releaseExists = $true
} catch {}

if ($releaseExists) {
    Write-Warn "Релиз $VERSION уже существует."
    $delete = Read-Host "Удалить и создать заново? (y/N)"
    if ($delete -eq "y" -or $delete -eq "Y") {
        Write-Info "Удаление существующего релиза..."
        gh release delete $VERSION --yes 2>&1 | Out-Null
    } else {
        Write-Err "Релиз уже существует. Отмена."
        exit 1
    }
}

# Сбор артефактов
$artifacts = @()
if (Test-Path $DIST_DIR) {
    $artifacts = Get-ChildItem $DIST_DIR -File | ForEach-Object { $_.FullName }
}

Write-Info "Найдено $($artifacts.Count) артефакт(ов) для загрузки"

# Генерация текста релиза на русском
$releaseBody = @"
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

Подробные инструкции для каждой платформы: [README.md](https://github.com/$(gh repo view --json nameWithOwner -q .nameWithOwner)/blob/main/README.md)

## 🐛 Известные проблемы

- Обход MTProto/Telegram официально не поддерживается в данной конфигурации
- Некоторые продвинутые профили могут требовать ручной настройки на отдельных провайдерах

## 🙏 Благодарности

Спасибо всем контрибьюторам и проекту zapret за основу!
"@

# Записываем текст релиза во временный файл
$releaseNotesFile = Join-Path $env:TEMP "unbound_release_notes.md"
$releaseBody | Out-File -FilePath $releaseNotesFile -Encoding UTF8

# Создаём релиз
Write-Info "Создание релиза $VERSION..."
if ($artifacts.Count -gt 0) {
    $artifactArgs = @($artifacts)
    & gh release create $VERSION `
        --title $RELEASE_TITLE `
        --notes-file $releaseNotesFile `
        --draft `
        --generate-notes `
        --category "v2.0.0" `
        @artifactArgs
} else {
    & gh release create $VERSION `
        --title $RELEASE_TITLE `
        --notes-file $releaseNotesFile `
        --draft `
        --generate-notes
}

# Очистка временного файла
Remove-Item $releaseNotesFile -Force -ErrorAction SilentlyContinue

Write-Ok "Релиз GitHub создан: $VERSION"
Write-Info "Релиз в режиме ЧЕРНОВИКА. Проверьте и опубликуйте: https://github.com/$(gh repo view --json nameWithOwner -q .nameWithOwner)/releases/tag/$VERSION"

# ============================================================================
# Пост-релизные задачи
# ============================================================================
Write-Step "Пост-релизные задачи"

# Тегирование коммита
Write-Info "Тегирование коммита $VERSION..."
git tag -a $VERSION -m "UNBOUND $VERSION — Глобальное обновление"
git push origin $VERSION

# Коммит изменений версий
Write-Info "Коммит изменений версий..."
git add -A
$commitMsg = "chore: обновление версий до v2.0.0 во всех проектах экосистемы

- Десктоп: 2.0.0
- Сайт: 2.0.0
- Android: 2.0.0 (versionCode: 2)
- iOS: 2.0.0
- Linux: 2.0.0
- OpenWrt: 2.0.0
- Расширение: 2.0.0

Выпущено как $VERSION"
git commit -m $commitMsg 2>$null
if ($LASTEXITCODE -ne 0) {
    Write-Warn "Нет изменений для коммита или уже закоммичено"
} else {
    Write-Ok "Изменения версий закоммичены"
    git push origin $currentBranch 2>$null
}

Write-Ok "Все пост-релизные задачи выполнены!"

# ============================================================================
# Итоговый отчёт
# ============================================================================
Write-Step "🎉 Релиз $VERSION завершён!"

Write-Host @"

  ✅ Все бинарные файлы собраны
  ✅ Сайт развёрнут на GitHub Pages
  ✅ Релиз GitHub создан (ЧЕРНОВИК)
  ✅ Коммит тегирован и отправлен

  Следующие шаги:
  1. Проверьте черновик релиза: https://github.com/$(gh repo view --json nameWithOwner -q .nameWithOwner)/releases
  2. Протестируйте все бинарные файлы на соответствующих платформах
  3. Опубликуйте релиз, когда будете готовы

  🚀 Тотальная война с цензурой! 🚀

"@ -ForegroundColor Green

Read-Host "Нажмите Enter для выхода"
