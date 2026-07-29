# Building UNBOUND `v0.2.1`

Руководство по локальной сборке и проверке UNBOUND Refresh на поддерживаемых настольных платформах.

## Quick Start

```bash
# Полный локальный gate
make check

# Linux CLI (amd64 по умолчанию; GOARCH=arm64 для arm64)
./scripts/build/build_linux.sh

# macOS Universal GUI — только на macOS
./scripts/build/build_darwin.sh universal
```

```powershell
# Windows GUI — только в Windows PowerShell
.\scripts\build\build_windows.ps1
```

---

## Поддерживаемые цели

| Платформа | Формат сборки | Инструменты |
|-----------|---------------|-------------|
| **Windows 10/11 x64** | Wails GUI `.exe` с CLI-флагами | Go 1.25+, Node.js 22.13+, Wails v2.13+, WebView2 |
| **Linux amd64 / arm64** | Самодостаточный CLI-бинарник | Go 1.25+, Node.js 22.13+ |
| **macOS Universal / arm64 / x86_64** | Wails `.app` с CLI-флагами | macOS, Xcode Command Line Tools, Go 1.25+, Node.js 22.13+, Wails v2.13+ |

---

## Требования к окружению

- **Go 1.25+** — основной язык ядра обхода DPI.
- **Node.js 22.13+** — сборка фронтенда (React + Tailwind CSS).
- **Wails CLI v2.13+**: `go install github.com/wailsapp/wails/v2/cmd/wails@v2.13.0`

---

## Скрипты проверки и сборки

### 1. `scripts/check.sh`
Повторяет проверки CI локально (gofmt, `go vet`, `go test -race`, кросс-компиляция под Windows/Linux/macOS):
```bash
./scripts/check.sh          # Полный цикл проверок
./scripts/check.sh --quick  # Быстрая проверка без кросс-компиляции
```

### 2. `scripts/build_all.sh`
Единый Unix/Git-Bash диспетчер. Цель `all` собирает Linux CLI и нативный GUI текущего хоста; macOS `.app` никогда не кросс-собирается:
```bash
./scripts/build_all.sh linux
./scripts/build_all.sh windows     # только Windows Git Bash
./scripts/build_all.sh darwin      # только macOS
./scripts/build_all.sh all
```

### 3. Платформенные скрипты
```bash
GOARCH=amd64 ./scripts/build/build_linux.sh
GOARCH=arm64 ./scripts/build/build_linux.sh
./scripts/build/build_darwin.sh universal
```
```powershell
.\scripts\build\build_windows.ps1
```

Каждый скрипт получает версию из `wails.json`; для контролируемого override используйте переменную `UNBOUND_VERSION`. Linux и macOS дополнительно выполняют CLI smoke-test на нативной архитектуре. Любая ошибка компиляции, отсутствие выходного файла или провал smoke-test завершает скрипт ненулевым кодом.

### 4. `Makefile`
Утилита повседневных задач:
```bash
make check      # Запуск всего комплекса проверок
make quick      # Быстрая проверка
make build      # Нативный desktop-бинарник через Go (для релизного Windows/macOS GUI используйте Wails-скрипты выше)
make gui        # Нативная GUI-сборка через Wails
make clean      # Очистка артефактов
```

---

## GitHub Actions локально через `act`

`act` запускает только Linux-контейнеры. Он воспроизводит Ubuntu jobs, зависимости, matrix-сборки Linux и обмен artifacts, но не эмулирует `windows-latest` или `macos-latest`.

```bash
# CI jobs; локальный artifact server обязателен для upload-artifact/download-artifact
mkdir -p .act-artifacts
act workflow_dispatch -W .github/workflows/ci.yml \
  -j frontend -j engine-assets -j shell \
  -P ubuntu-latest=catthehacker/ubuntu:act-latest \
  --artifact-server-path .act-artifacts

# Release verify + Linux amd64/arm64 package/smoke/upload.
# --privileged нужен реальному CLI E2E для NFQUEUE/firewall.
act workflow_dispatch -W .github/workflows/release.yml \
  -j build-linux \
  -P ubuntu-latest=catthehacker/ubuntu:act-latest \
  --artifact-server-path .act-artifacts \
  --container-options="--privileged"
```

Windows job проверяется нативно через `scripts/build/build_windows.ps1`; macOS job — на настоящем Mac:

```bash
./scripts/check.sh
./scripts/build/build_darwin.sh universal
```

`act` на Mac всё равно использует Linux Docker и не может собрать/подписать Wails `.app`. Для полного локального аналога `build-macos` нужен нативный macOS runner с Xcode и `codesign`; публикация GitHub Release дополнительно требует `GITHUB_TOKEN`.

---

## Linux firewall integration в Docker

Обычный unit-test не проверяет, принимает ли ядро реальные `nftables`/`iptables` правила. На Linux Docker host или Docker Desktop с поддержкой NFQUEUE запустите привилегированный контейнер:

```bash
docker run --rm --privileged \
  -v "$PWD:/src" -w /src \
  golang:1.25-bookworm \
  bash -c 'apt-get update && apt-get install -y --no-install-recommends iptables nftables && UNBOUND_FIREWALL_TEST=1 go test -v ./engine/providers -run Live -count=1'
```

Оба теста `Live` должны завершиться как `PASS`, не `SKIP`. Контейнер изменяет только собственный network namespace.
