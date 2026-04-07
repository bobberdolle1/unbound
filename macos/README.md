# UNBOUND — Порт для macOS

## Обзор

Реализация двигателя обхода DPI UNBOUND для macOS. Заменяет Windows-специфичный конвейер WinDivert + winws2 на **SpoofDPI**, работающий как локальный SOCKS5-прокси, с общесистемной маршрутизацией трафика через macOS `networksetup`.

## Архитектура

```
┌──────────────────┐     ┌──────────────────┐     ┌───────────────┐
│  React Фронтенд  │────▶│  Wails Go бэкенд  │────▶│ SpoofDPI      │
│  (без изменений)  │◀────│  (app_darwin.go)  │◀────│ (SOCKS :8080) │
└──────────────────┘     └────────┬─────────┘     └───────────────┘
                                  │
                         osascript (админ)
                                  │
                         ┌────────▼────────┐
                         │   networksetup   │
                         │  SOCKS-прокси на │
                         │   Wi-Fi/Ethernet  │
                         └──────────────────┘
```

### Ключевые компоненты

| Файл | Назначение |
|---|---|
| `../engine/providers/zapret_macos.go` | Основной `ZapretMacOSProvider` — запускает SpoofDPI, управляет жизненным циклом SOCKS-прокси |
| `../app_darwin.go` | Проверка привилегий (всегда true; эскалация во время выполнения через osascript), регистрация провайдеров |
| `../engine/startup_darwin.go` | Автозапуск launchd через `~/Library/LaunchAgents/com.bobberdolle1.unbound.plist` |
| `../engine/diagnostics_darwin.go` | Диагностика для macOS (доступность SpoofDPI, сетевые сервисы, конфликты) |
| `../engine/config_darwin.go` | Настройки автозапуска для платформы |
| `../engine/platform_paths.go` | Кроссплатформенное определение кэш-директории Discord |
| `../main_darwin.go` | Регистрация провайдера CLI-режима headless для macOS |
| `../conflicts_darwin.go` | Обнаружение конфликтов для macOS (`pgrep`/`pkill`) |

### Поток работы двигателя

1. **Запуск**: Пользователь нажимает «Подключить» → `StartEngine()` → `ZapretMacOSProvider.Start()`
2. **Запуск SpoofDPI**: `spoofdpi --port 8080 --mode <профиль>` запускается как подпроцесс
3. **Настройка прокси**: `osascript` вызывает запрос Touch ID / пароля → `networksetup -setsocksfirewallproxy` включает общесистемный SOCKS-прокси
4. **Маршрутизация трафика**: Весь TCP/UDP-трафик на активном сетевом сервисе проходит через SpoofDPI
5. **Остановка**: `StopEngine()` → завершает процесс SpoofDPI → `networksetup -setsocksfirewallproxystate off` восстанавливает прямой интернет

### Модель привилегий

- Приложение **не требует** запуска от root
- Изменения системного прокси используют `osascript -e 'do shell script "..." with administrator privileges'`
- macOS запрашивает Touch ID или пароль нативно
- При первом запуске может появиться запрос аутентификации; последующие вызовы в рамках одной сессии могут кэшироваться

## Зависимости

### Обязательные

| Зависимость | Версия | Назначение |
|---|---|---|
| Go | 1.23+ | Компилятор |
| Wails CLI | v2.11+ | Фреймворк приложения + сборка |
| SpoofDPI | последняя | Двигатель обхода DPI через SOCKS5 |
| Xcode CLT | — | Инструментарий CGO (для systray) |

### Установка SpoofDPI

```bash
# Вариант 1: Homebrew (рекомендуется)
brew install spoofdpi

# Вариант 2: Сборка из исходников
git clone https://github.com/xvzc/SpoofDPI.git
cd SpoofDPI
go build -o /usr/local/bin/spoofdpi ./cmd/spoofdpi
```

### Остальные зависимости

Все Go-зависимости управляются через `go.mod`. Дополнительные пакеты не требуются сверх того, что уже используется в Windows-сборке, за исключением того, что `systray` требует нативной компиляции для macOS (CGO).

## Сборка

### Требования

```bash
# Установить Xcode Command Line Tools
xcode-select --install

# Установить Wails CLI
go install github.com/wailsapp/wails/v2/cmd/wails@latest

# Убедиться, что SpoofDPI доступен
brew install spoofdpi   # ИЛИ поместить бинарник в core_bin/darwin/
```

### Команды сборки

```bash
# Универсальный бинарник (Intel + Apple Silicon)
wails build -platform darwin/universal

# Только Intel Mac
wails build -platform darwin/amd64

# Только Apple Silicon
wails build -platform darwin/arm64

# Отладочная сборка (с DevTools)
wails build -platform darwin/universal -debug
```

### Ручная сборка (без Wails)

```bash
# Фронтенд
cd frontend && npm install && npm run build && cd ..

# Бэкенд-бинарник
GOOS=darwin GOARCH=arm64 go build -o build/bin/Unbound .
```

### Результат

Сборка создаёт `build/bin/Unbound.app` — стандартный пакет приложения macOS.

## Запуск

```bash
# Режим GUI (по умолчанию)
open build/bin/Unbound.app

# Режим headless CLI
./build/bin/Unbound --cli --profile "Unbound Ultimate (God Mode)"

# Запуск свёрнутым в строку меню
./build/bin/Unbound --tray

# Отладочное логирование
./build/bin/Unbound --debug
```

## Расположение файлов (macOS)

| Путь | Назначение |
|---|---|
| `~/Library/Application Support/Unbound/` | Конфигурация, настройки, списки |
| `~/Library/Application Support/Unbound/settings.json` | Пользовательские настройки |
| `~/Library/Application Support/Unbound/unbound.log` | Файл лога |
| `~/Library/Application Support/Unbound/lists/` | Загруженные списки хостов |
| `~/Library/LaunchAgents/com.bobberdolle1.unbound.plist` | Определение автозапуска |
| `/tmp/clearflow/` | Извлечённые ассеты времени выполнения (бинарники, Lua-скрипты) |
| `~/Library/Application Support/discord/Cache/` | Кэш Discord (очищается при запуске, если включено) |

## Профили

| Профиль | Режим SpoofDPI | Описание |
|---|---|---|
| Unbound Ultimate (God Mode) | `aggressive` | Максимальный обход, все цели |
| Unbound Standard | `default` | Сбалансированный обход |
| Unbound Lite | `lite` | Минимальный обход, низкая задержка |
| Unbound Aggressive | `aggressive` | Агрессивный режим обхода |

## Автозапуск (launchd)

При включении «Запуск при входе» в настройках:

1. Файл `.plist` записывается в `~/Library/LaunchAgents/com.bobberdolle1.unbound.plist`
2. Plist загружается через `launchctl load`
3. При следующем входе Unbound запускается с флагом `--tray` (свёрнут в строку меню)

Для ручного отключения:
```bash
launchctl unload ~/Library/LaunchAgents/com.bobberdolle1.unbound.plist
rm ~/Library/LaunchAgents/com.bobberdolle1.unbound.plist
```

## Диагностика

### «Бинарник SpoofDPI не найден»
- Установите через `brew install spoofdpi`
- Или поместите бинарник `spoofdpi` в `core_bin/darwin/`

### Прокси не отключается после сбоя
```bash
networksetup -setsocksfirewallproxystate "Wi-Fi" off
networksetup -setsocksfirewallproxy "Wi-Fi" '' 0
```

### Приложение не запускается (застревает на запросе аутентификации)
- Убедитесь, что ваша учётная запись имеет права администратора
- Попробуйте запустить из терминала: `./Unbound --cli`

### Нет интернета после закрытия Unbound
- SOCKS-прокси может остаться включённым. Отключите его:
  ```bash
  networksetup -setsocksfirewallproxystate "Wi-Fi" off
  ```

## Изоляция кода

Все файлы для macOS используют теги сборки `//go:build darwin` и компилируются только при `GOOS=darwin`. Сборки Windows и Linux не затрагиваются.
