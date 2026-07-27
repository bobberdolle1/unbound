# 🧪 Руководство по тестированию и верификации UNBOUND Refresh `v0.1.0-refresh`

Настоящее руководство подробно описывает процедуры локального тестирования, проверки кода и контроля качества (QA) для **UNBOUND Refresh**.

---

## 1. Автоматический тестовый комбайн (`scripts/check.sh`)

Скрипт `scripts/check.sh` повторяет проверки GitHub Actions CI в полном объеме локально на вашей машине.

```bash
# Полный запуск всех проверок (Frontend, Go fmt/vet/test, cross-compile, shell, assets)
./scripts/check.sh

# Быстрая локальная проверка (без кросс-компиляции)
./scripts/check.sh --quick

# Отдельные этапы
./scripts/check.sh frontend   # Проверка типов tsc и сборка Vite
./scripts/check.sh go         # gofmt, go vet, go test -race и cross-vet
./scripts/check.sh assets     # Проверка SHA256 хешей 130 встроенных бинарников
./scripts/check.sh shell      # Синтаксический анализ shell-скриптов
```

---

## 2. Модульное и интеграционное тестирование Go (`go test`)

```bash
# Быстрый запуск модульных тестов в режиме race detector
go test -race ./...

# Подробный запуск тестов для конкретного пакета
go test -v ./engine/providers/...

# Запуск тестов фаервола на живом ядре Linux (требуются права root)
sudo UNBOUND_FIREWALL_TEST=1 go test -v ./engine/providers/ -run Live
```

---

## 3. Пошаговый Чек-лист Ручного Тестирования (Manual QA)

### 🪟 Windows (WinDivert + winws2.exe)
- [x] Автоматический запрос UAC-прав при старте приложения.
- [x] Запуск и остановка профиля `Recommended (hostfakesplit)` без ошибок.
- [x] Кнопка **«Остановить winws2.exe»** корректно убивает процесс и отгружает драйвер WinDivert.
- [x] Сворачивание в трей при клике на крестик (открытие через иконку в трее).
- [x] Включение/выключение Secure DNS (Cloudflare, Google, Quad9, AdGuard).
- [x] Использование виртуального конструктора LUA-стратегий и редактирование списков (`youtube.txt`).

### 🐧 Linux (NFQUEUE + nftables / iptables)
- [x] Проверка root-привилегий при старте.
- [x] Создание изолированной таблицы `inet unbound` в `nftables` (или правил в `iptables`).
- [x] Запуск и остановка `nfqws` с флагом `--queue-bypass` (сети не блокируются при аварии).
- [x] Гарантированный `Flush` правил фаервола при завершении процесса (`SIGTERM`/`SIGINT`).
- [x] Сборка и запуск CLI-версии (`./build/bin/unbound-linux --version`).

### 🍎 macOS (pf divert socket + universal tpws)
- [x] Проверка admin/wheel прав пользователя.
- [x] Настройка изолированного `pf`-якоря `com.unbound.zapret` без затирания пользовательских правил.
- [x] Автоматическая развертка нативного **Universal (arm64 + x86_64) `tpws`** из `engine/core_bin/darwin/tpws`.
- [x] Кнопка **«Остановить tpws»** мгновенно завершает процессы `tpws` и очищает якорь `pfctl`.
- [x] Выход из приложения (`QuitApp`) полностью отключает обход и очищает фаервол.

---

## 4. Контроль безопасности и провенанс ассетов

Для проверки целостности всех бинарных ассетов выполните:
```bash
./scripts/engine-assets.sh verify
```

Или нажмите кнопку **«Проверить целостность файлов (SHA256)»** в настройках графического интерфейса.
