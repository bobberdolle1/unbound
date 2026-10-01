# 🧪 Руководство по тестированию и верификации UNBOUND Refresh `v0.2.1`

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

### Экспериментальный Linux `amd64` пакет

Пакетирование выполняется только на Linux `x86_64`; оно не публикует артефакт:

```bash
commit="$(git rev-parse HEAD)"
version="$(node -p "require('./wails.json').info.productVersion")"
./scripts/build/package_linux_release.sh \
  --version "$version" --expected-commit "$commit" --mode local
```

Скрипт создаёт локальный archive, извлекает его в временную директорию, проверяет `BUNDLE_SHA256SUMS.txt`, identity, права и команды `--version`, `--version --json`, `--help`, `--list-profiles --json`, `--test`. Для будущего release candidate используйте только чистый checkout и `scripts/release/local_release_linux.sh <VERSION> <COMMIT>`.

### Bounded service graph (герметичные тесты)

Тесты графа в `engine/autotunevnext/service_graph_test.go` полностью герметичны: ни один из них не выполняет DNS, не запускает процесс и не обращается к сети. Все scope собираются литерально, поэтому набор детерминирован и ничего не может «доказать» о живой сети.

```bash
go test ./engine/autotunevnext/... -run 'ServiceGraph|Revalidation|ActiveGraph|StrategyTemplateIdentity|RenderWindowsGraph' -v
```

Покрытие: одноузловой граф, эквивалентный существующему single-host scope; два node с одним template (доказанный случай Phase 4); два node с разными bindings; per-node лимит в 8 адресов; переполнение графа сверх 4 hosts; запрет двух node на один target; обязательное совпадение scope с own target; обязательный strategy binding для validated node; `DISCOVERED` node не даёт active capture; ревалидация при `NEW_EDGE`, `NEW_NODE`, исчезновении required node, безвредном исчезновении optional node и возврате непровалидированного node; active capture graph уже experiment graph; отсутствие raw edges в сериализации; независимость логического identity графа от текущих DNS-ответов; независимость `StrategyTemplateIdentity` от host binding при сохранении различия разных стратегий; multi-section рендеринг двух host с одним template; отказ section, связанной с чужим host; отказ wildcard/list/capture-level authority; исключение не-ACTIVE node.

Физическая приёмка bounded ServiceGraph **пройдена** на выделенных лабораториях из исходного дерева релиза: на Windows — реальный `engine.ExtractAssets`, реальный asset pipeline, один процесс `winws2` и реальный WinDivert-захват с точным union по нескольким хостам; на Linux — реальный `nfqws2`, одна executor-owned таблица `nft` с per-family правилами по точным адресам, NFQUEUE, mark exclusion и bypass, штатная очистка и сохранность сторонних правил. Инструментарий приёмки находится в репозитории (`physical_accept_windows_test.go`, `physical_accept_linux_test.go`) и запускается только по явному признаку лабораторного хоста. Герметичные тесты не заменяют физическую приёмку; приёмка **упакованного артефакта** `v0.8.0` ещё не выполнялась, поскольку сборка macOS заблокирована недоступностью лаборатории.

Исполнение графа, Apply-гейт и health покрыты отдельно:

```bash
go test ./engine/autotunevnext/... -run 'ServiceGraphExperiment|PrepareServiceGraph|ApplyServiceGraph|ServiceGraphHealth' -v
```

Покрытие: одна активация на двухузловой граф и точный порядок вызовов executor'а; restore при сбое активации; приоритет провала restore; отмена во время наблюдения с входом в restore; остановка на регрессии protected control с последующим restore; запрет ложного `VERIFIED_FIXED`; Apply без изменений допустим; отказ при новом адресе, новом node и исчезновении required node; использование только ACTIVE nodes; отказ section с чужим host и с wildcard/list authority; отказ пустого validated graph; гейт останавливает Apply до касания executor'а; успешный Apply восстанавливает состояние и сохраняет обязанность отката; `HEALTHY`; мёртвый процесс как `FAULT`; сбой резолвера fail-closed; новый адрес и новый node как `NEEDS_REVALIDATION`; урок PR59 — node вне активного capture не авторизуется; отсутствие активного capture не здоровье; health snapshot без адресов.

---

## 3. Пошаговый Чек-лист Ручного Тестирования (Manual QA)

### 🪟 Windows (WinDivert + winws2.exe)
- [x] GUI автоматически запрашивает UAC, а `--version` и `--list-profiles --json` запускаются и отдают stdout без повышения.
- [x] Запуск и остановка профиля `Recommended (hostfakesplit)` без ошибок.
- [x] Кнопка **«Остановить winws2.exe»** корректно убивает процесс и отгружает драйвер WinDivert.
- [x] Сворачивание в трей при клике на крестик (открытие через иконку в трее).
- [x] Включение/выключение Secure DNS (Cloudflare, Google, Quad9, AdGuard).
- [x] Использование виртуального конструктора LUA-стратегий и редактирование списков (`youtube.txt`).

### 🐧 Linux (NFQUEUE + nftables / iptables)
- [x] Проверка root-привилегий при старте.
- [x] Создание изолированной таблицы `inet unbound` в `nftables` (или правил в `iptables`).
- [x] Запуск и остановка `nfqws2` с `--qnum` и `--lua-init` (очередь использует fail-open bypass при аварии).
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
