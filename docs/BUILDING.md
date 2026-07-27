# Building UNBOUND `v0.1.0-refresh`

Руководство по локальной сборке и проверке UNBOUND Refresh на поддерживаемых настольных платформах.

## Quick Start

```bash
# Unix / macOS / Linux — проверить и собрать desktop-цели
make check

# Быстрая локальная сборка GUI
wails build -clean -ldflags "-X unbound/engine.Version=0.1.0-refresh"

# CLI бинарник для хост-платформы
go build -trimpath -ldflags="-s -w -X unbound/engine.Version=0.1.0-refresh" -o build/bin/unbound .
```

---

## Поддерживаемые цели

| Платформа | Формат сборки | Инструменты |
|-----------|---------------|-------------|
| **Windows 10/11** | `.exe` (GUI Wails / CLI) | Go 1.25+, Wails v2.13+, MSVC / WebView2 |
| **Linux (amd64 / arm64)** | CLI / GUI бинарник | Go 1.25+, NFQUEUE (`libnetfilter_queue`), gcc |
| **macOS (Universal / arm64 / x86_64)** | `.app` / CLI бинарник | Xcode Command Line Tools, Go 1.25+, Wails v2.13+ |

---

## Требования к окружению

- **Go 1.25+** — основной язык ядра обхода DPI.
- **Node.js 20+** — сборка фронтенда (React + Tailwind CSS).
- **Wails CLI v2.13+**: `go install github.com/wailsapp/wails/v2/cmd/wails@v2.13.0`

---

## Скрипты проверки и сборки

### 1. `scripts/check.sh`
Повторяет проверки CI локально (gofmt, `go vet`, `go test -race`, кросс-компиляция под Windows/Linux/macOS):
```bash
./scripts/check.sh          # Полный цикл проверок
./scripts/check.sh --quick  # Быстрая проверка без кросс-компиляции
```

### 2. `build_all.sh`
Единая точка входа для сборки бинарников desktop-платформ:
```bash
./build_all.sh linux       # Сборка Linux бинарника
./build_all.sh windows     # Сборка Windows GUI
./build_all.sh darwin      # Сборка macOS GUI
./build_all.sh all         # Сборка всех доступных настольных целей
```

### 3. `Makefile`
Утилита повседневных задач:
```bash
make check      # Запуск всего комплекса проверок
make quick      # Быстрая проверка
make build      # Сборка CLI бинарника
make gui        # Сборка GUI через Wails
make clean      # Очистка артефактов
```
