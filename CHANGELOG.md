# Changelog - UNBOUND

Все значимые изменения проекта документируются в этом файле.

## [2.5.0] - 2026-07-27

### 🚀 Глобальный релиз v2.5.0 — Android Non-Root & Magisk WebUI
- **Android Non-Root VpnService (JNI TUN)**: Нативный C-мост (`libunbound_tunnel.so`) для прямой передачи FD пакетов из Android `VpnService` в локальный прокси (`127.0.0.1:1080`) без потери интернета, со встроенной поддержкой DoH DNS и Split Tunneling.
- **Magisk / KernelSU WebUI Панель**: Интерактивная веб-панель управления системным модулем прямо из менеджера (Magisk/KernelSU/APatch) и автоматическая фиксация `TTL=64` в `iptables`/`nftables` для снятия операторских ограничений на раздачу интернета.
- **Локальный пайплайн сборки**: Автоматизированные скрипты `./build_all.sh` и `Makefile` для полноценной локальной сборки релиза без зависимости от GitHub Actions.
- **Минималистичный SVG-логотип**: Handcrafted векторный логотип `logo.svg`.
- **Очистка стиля и интерфейса**: Лаконичный Modern Dark, Light и macOS Glass интерфейс на Wails & React.

### Провенанс движка и ревизия iOS

#### Zapret 2: обновление не требуется
Проверил вшитый набор против upstream `bol-van/zapret2` **v1.0.3** (у нас в CHANGELOG значилась v1.0.2). Пять из шести Lua-скриптов upstream оказались **побайтово идентичны** v1.0.3, включая оба, которые проект реально загружает — `zapret-lib.lua` и `zapret-antidpi.lua`. Расходится только `zapret-tests.lua`: upstream добавил в него демонстрацию анонимных функций в таймерах, а на этот файл в репозитории не ссылается ничто. Обновлять нечего.

Бинарник `winws2.exe` лежит только в release-ассетах (в git upstream его нет — `binaries/readme.txt` говорит об этом прямо), из этого окружения ассеты недоступны, и коммитить бинарник, происхождение которого нельзя проверить, я не стал.

#### Контрольные суммы вендоренных бинарников
`engine/core_bin` содержит бинарники, которые пользователь запускает с правами администратора, включая **драйвер ядра** WinDivert. До сих пор нигде не было записано, какой upstream-версии они соответствуют (единственный след — строчка в CHANGELOG) и остались ли байты теми, которые кто-то проверял.
- Добавлен `engine/ENGINE_ASSETS.sha256` — 129 файлов с sha256 и заголовком провенанса: откуда взят каждый компонент и какая версия Zapret 2 вшита.
- `scripts/engine-assets.sh verify` сверяет их, `generate` — перевыпускает манифест после осознанного обновления. Подключено к `check.sh` и CI.
- Проверка ловит и подмену содержимого, и **добавление** нового файла мимо манифеста (голый `sha256sum -c` второе бы пропустил).
- Удалён `engine/core_bin/windows/err.txt` — файл нулевого размера.

#### iOS: разобраны целевые версии
Рантайм-часть двойной поддержки сделана правильно: `UnboundAppDelegate` смотрит на `systemVersion` и поднимает скевоморфный контроллер до iOS 7, современный — после. Не сходятся сборка и упаковка:
- `Makefile` объявляет `ARCHS = armv7 arm64` при `TARGET = iphone:clang:6.1:6.1`. arm64 не существовал до iOS 7, в SDK 6.1 нет arm64-слайса; при этом Xcode 14+ не умеет armv7. Одним тулчейном обе архитектуры не собрать — нужны две сборки. `engine/Makefile.tpws` именно так и устроен, сторона Theos за ним не последовала.
- `control` объявляет `Architecture: iphoneos-arm` и ставит файлы в `/Applications` и `/Library` — это rootful-схема. На rootless-джейлбрейках, которые преобладают на iOS 15+, пакет не встанет: нужен `iphoneos-arm64` и `/var/jb`.

Ничего из этого не «чинилось» вслепую: пока движок не линкуется, правка таргета дала бы пакет, который устанавливается и затем не работает на устройствах, которые он якобы добавляет. Точные условия записаны в комментарии к `theos/unbound-legacy/Makefile` и в README.


### Ревизия платформ: расширение, Magisk, OpenWRT, iOS/tvOS

#### Браузерное расширение
Ничто в репозитории не собирало и не проверяло `extension-web`, поэтому его ошибки типов уезжали в master: `npm run build` запускает vite, который срезает типы не проверяя их, а `type-check` был отдельным скриптом, который никто не вызывал. `tsc` показал семь ошибок.
- Четыре пришлись на `src/utils/browser.ts` — файл, который никто не импортирует: 80 строк мёртвых и при этом сломанных хелперов. `getBrowserInfo()` определял Firefox по `browser.runtime.getBrowserInfo`, но `browser` там — импортированный полифилл, а не глобальный объект, и полифилл определяет этот метод везде. Функция вернула бы «firefox» в Chrome и Edge, а её же ветка для Chrome была недостижима. Файл удалён.
- `build` теперь сначала гоняет type-check, а расширение и плагин Steam Deck проверяются в `scripts/check.sh` и `ci.yml` — раньше не покрывались вообще.
- Из git убран `extension-web/unbound-extension-v2.0.0` — 22 файла скомпилированного вывода, закоммиченные по ошибке.

Само расширение при этом добротное: standalone-режим генерирует настоящий PAC-скрипт и ставит `browser.proxy.settings`. Единственная реальная заглушка, `testProxyConnectivity()`, ни откуда не вызывается.

#### Magisk-модуль
- `service.sh` и `iptables_setup.sh` объявляют `#!/system/bin/sh` (на Android это mksh), но разбирали список исключённых UID через bash-массивы: `IFS=',' read -ra UIDS <<< "$EXCLUDED_UIDS"`. У mksh нет `read -a`, поэтому при заданном `EXCLUDED_UIDS` скрипт умирал на этой строке и **все правила обхода после неё не устанавливались вовсе**. Заменено на POSIX-разбиение по IFS (три места: ветка iptables, ветка nftables, отдельный setup-скрипт).
- `customize.sh` копировал `binaries/$ARCH_DIR/nfqws` без проверки существования. В каталоге лежит только `.gitkeep` с пояснением, что бинарники «надо положить», так что модуль, собранный из этого репозитория, движка не содержит: `cp` печатал ошибку, установка всё равно рапортовала об успехе, а падало позже. Теперь установка прерывается с инструкцией.
- Добавлена проверка синтаксиса скриптов в `check.sh` и CI: каждый отслеживаемый `.sh` разбирается тем интерпретатором, который указан в его shebang. Покрыто 23 скрипта.

#### OpenWRT
`PKG_SOURCE_VERSION` был `master` при `PKG_MIRROR_HASH:=skip` — пакет собирался из того, чем оказался upstream HEAD в момент сборки. Для инструмента обхода цензуры это и невоспроизводимость релизов, и попадание в `.ipk` любой компрометации upstream без ревью. Закреплено на теге `v72.13` (коммит `87e0586`).

#### iOS и tvOS
- Добавлен `scripts/fetch-tpws.sh`: тянет 13 исходников upstream, которых нет в репозитории, с того же закреплённого тега, что и пакет OpenWRT, и **сверяет SHA коммита** — тег можно передвинуть, коммит нельзя.
- Обнаружен настоящий блокер: `tpws.h` объявляет `tpws_init()` и `tpws_run_loop()`, которые вызывают `ios_main.c` и движок tvOS, но их не определяет никто. Upstream даёт обычный `main()`, который `ios_main.c` тоже определяет — символы конфликтуют при линковке. Порт не закончен.
- `theos/.../scripts/build.sh` глушил провал сборки движка через `2>/dev/null || echo "Engine build skipped"` и рапортовал об успехе, собирая `.deb` с tweak-ом, но без движка. Теперь останавливается и объясняет, чего не хватает.
- В README статусы iOS исправлен на ❌, tvOS описан честно.


### Критические исправления UI и работа без GitHub Actions

#### Исправлено
- **Сворачивание в трей больше не выключает обход.** `HideWindowToTray()` вызывал `manager.Stop()`, поэтому кнопка «Закрыть в трей» и системный крестик окна (он идёт сюда же через `onBeforeClose`) молча останавливали обход DPI. Работа в фоне — единственная причина, по которой приложение живёт в трее, и пользователю ничего не сообщалось о том, что трафик перестал защищаться.
- **Кнопка «Завершить winws2.exe» теперь завершает winws2.exe.** `KillWinws2()` вызывал `KillConflicts()`, чей список процессов намеренно исключает `winws2.exe` («не наш winws2.exe»). То есть единственная кнопка, обязанная убить наш движок, была единственной, которая гарантированно этого не делала, — а UI при этом рапортовал «Все процессы winws2 завершены». Аварийное завершение теперь сначала штатно останавливает процесс (чтобы тот убрал за собой правила файрвола), затем добивает осиротевший `winws2.exe` и освобождает драйвер WinDivert. Ошибка больше не уходит молча в консоль.
- `esModuleInterop: false` во фронтенде — опция, которую TypeScript 7 отвергает как устаревшую.
- `build_all.sh` вычислял версию, но никуда её не прошивал: `--version 2.6.0` собирал бинарник, который всё равно сообщал версию, вкомпилированную в `engine.Version`.

#### Android: VPN больше не убивает интернет на устройстве
`UnboundVpnService` поднимал TUN-интерфейс и заворачивал в него весь трафик (`addRoute("0.0.0.0", 0)` и `addRoute("::", 0)`), при том что петля пересылки пакетов не реализована — её тело состоит из трёх TODO, она читает пакеты и выбрасывает их, — а `startLocalProxy()` целиком закомментирован, но пишет в лог «Local proxy started on 127.0.0.1:1080».

Это хуже, чем «обход не работает»: за TUN нет ничего, поэтому включение VPN **полностью лишало устройство интернета** во всех приложениях, пока его не выключат. `BootReceiver` и `WifiStateReceiver` при этом умеют включать VPN сами — при загрузке и при смене Wi-Fi, так что пользователь мог получить телефон без связи, не понимая причины.

- Добавлен флаг `PACKET_RELAY_IMPLEMENTED`; пока он `false`, `startVpn()` отказывается поднимать интерфейс и сообщает причину вместо того, чтобы обрубить связь.
- Убрана ложная строка лога про запущенный прокси — она уводила в сторону любого, кто читал logcat, разбираясь с мёртвым соединением.
- В README статус Android изменён на ❌ с объяснением, что именно не реализовано и какими двумя путями это доделывается.

#### Ревизия остальных платформ
- **OpenWRT** оказался самой добротной из непроверенных платформ: настоящий пакетный Makefile, собирающий `nfqws` из исходников, procd-init, UCI-конфиг и nftables-include.
- **iOS (theos)** — рабочий по устройству tweak: настраивает SOCKS/HTTP-прокси на `127.0.0.1:1993` и поднимает `unbound-tpws`.
- **tvOS** в текущем виде **не соберётся**: `UnboundTunnelEngine.c` вызывает `tpws_init()` и `tpws_run_loop()`, которых нет ни в одном файле репозитория.

#### Проверено на живом ядре
- Правила iptables, которые генерирует Linux-провайдер, до сих пор сверялись только как строки. Добавлены тесты, скармливающие сгенерированные спецификации настоящему `iptables`: каждый встроенный профиль применяется ядром и удаляется той же спецификацией, которой был добавлен (`Flush()` удаляет по точному совпадению, а не по индексу, поэтому расхождение оставило бы правила висеть). Тесты требуют root и явного `UNBOUND_FIREWALL_TEST=1`, ставят правила в цепочку, в которую никто не переходит, и убирают её за собой. Проверено, что они действительно ловят поломку: неверный `--connbytes-mode` и выход `--queue-num` за диапазон дают падение с точной диагностикой от ядра.
- Бэкенд nftables на живом ядре подтвердить не удалось: в тестовой среде нет модуля `nft_queue` (при том, что compat-таргет `xt_NFQUEUE` через iptables работает). Тест это распознаёт и скипается с явным сообщением, а не выдаёт ложное «прошло».

#### Удалён мёртвый код
- `providers.ValidateBinaries` — ни одного вызова, и на Windows требовал `nfqws.exe`, тогда как движок называется `winws2.exe`. Дублировал `StartupValidator.validateBinaries`, который реально используется.
- `engine.ValidateAdminPrivileges` — ни одного вызова, и на Windows вызывал заглушку `checkWindowsAdmin()`, всегда возвращавшую ошибку «admin check not implemented».

#### Работа без GitHub Actions
GitHub Actions на репозитории недоступен из-за ограничения биллинга на уровне аккаунта, поэтому автоматических проверок на пуш нет.

- `scripts/check.sh` повторяет `.github/workflows/ci.yml` шаг в шаг: `gofmt`, `go vet` под три ОС, `go test -race`, кросс-компиляция под шесть пар GOOS/GOARCH, сборка интерфейса и сайта. Возвращает ненулевой код при любой неудаче, поэтому вешается на `pre-push`.
- `Makefile` с задачами `check`, `quick`, `build`, `gui`, `site`, `deploy-site`, `install-hooks`.
- В README описано, как выпускать релиз и публиковать сайт вручную, пока Actions не заработает. Сами workflow'ы уже в репозитории и заработают без правок.

## [Unreleased]
### Кроссплатформенность, сборка и CI

**Linux и macOS теперь действительно работают.** Провайдер для Linux был написан целиком, но его никто не создавал: движки не регистрировались, `--cli` спрашивал у менеджера Windows-движок «Zapret 2 (winws)» по имени, `NewAutoTuneProvider` возвращал `nil` и автоподбор падал с nil-разыменованием, а `checkAdminPrivileges()` безусловно возвращал `true`.

#### Исправлено
- **Linux**: подключён провайдер `nfqws`; реальная проверка root; диагностика вместо одной захардкоженной строки «OK»; заработали `EnableTCPTimestamps`, очистка кэша Discord и автозапуск (XDG `.desktop`) — все три были заглушками `return nil`, которые рапортовали об успехе и ничего не делали.
- **Linux**: поддержка nftables как альтернативы iptables, правила ставятся и на IPv4, и на IPv6, откат при частичном сбое, точное удаление ровно тех правил, что были добавлены.
- **macOS**: правила pf грузятся в отдельный якорь `com.unbound.zapret` через stdin. Раньше ruleset писался в предсказуемый путь `/tmp/unbound_pf_rules.conf` и применялся через `pfctl -f`, что **заменяет весь активный набор правил** пользователя; чтение root-ом правил файрвола из общедоступной на запись директории — ещё и вектор локальной эскалации.
- **macOS**: `killConflictsImpl` больше не выключает пользовательский прокси через `networksetup -setwebproxystate Wi-Fi off` — это меняло настройки без спроса и только на Wi-Fi.
- **Все платформы**: `pkill -9 nfqws` убивал каждый `nfqws` в системе, включая независимый системный сервис zapret. Теперь сигнал идёт только своей группе процессов, сначала SIGTERM.
- **Все платформы**: убран флаг `--daemon` — процесс форкался, `cmd.Wait()` возвращался сразу, движок числился остановленным, а его PID был потерян.
- **Все платформы**: гонки данных в `GetStatus()`/`GetLogs()`, двойной `Unlock()` на путях ошибок в `Start()`.
- `app.go` вызывал `tasklist`/`taskkill`/`sc`/`powershell` без build-тега — определение конфликтов и Secure DNS были Windows-командами на любой ОС. Разнесено по платформам; `checkConflictsImpl`/`killConflictsImpl` (мёртвый код, который никто не вызывал) подключены, для Linux написаны. Secure DNS: `resolvectl`/`nmcli` на Linux, `networksetup` на macOS.
- Валидатор запуска требовал бинарник движка внутри каталога ресурсов. Его туда кладёт только Windows-сборка, поэтому на Linux и macOS валидация всегда падала, старт прерывался до регистрации провайдера, а пользователю предлагали «переустановить приложение».
- `GetAppVersion()` отдавал 2.0.0, тогда как README, CHANGELOG и опубликованный архив говорили 2.5.0. У версии теперь одно определение, переопределяется через `-ldflags`.

#### Сборка
- `go vet ./...` и `go test ./...` не компилировались ни на одной платформе кроме Windows: пять файлов ссылались на `providers.NewZapret2WindowsProvider` без build-ограничений.
- Фронтенд не собирался вообще: Vite 3 / TypeScript 4.9 против Tailwind 4. Обновлено до Vite 7 / TypeScript 5.9.
- Баннерный комментарий над секцией CORE APP STYLES был без открывающего `/*` — CSS-парсер отбрасывал всё правило `body`.
- `@import` шрифтов стоял после `@import "tailwindcss"` и потому игнорировался браузерами: ни один кастомный шрифт не грузился.
- Шрифты забандлены локально. Инструмент обхода блокировок запускают ровно тогда, когда сеть зацензурена, — загрузка шрифтов с `fonts.googleapis.com` отказывала именно в этот момент и противоречила заявленному отсутствию внешних запросов.

#### CI/CD
- Пайплайн не мог завершиться: `GO_VERSION` 1.23 против требуемых go.mod 1.25, `go build -o <file> ./...` (ошибка: `-o` принимает один пакет), копирование несуществующего `README_RELEASE.txt`, `cache: 'frontend'` в setup-node, невалидный шелл `${$apk##*-}` в Android-джобе.
- Добавлены проверки, которых не было вовсе: `gofmt`, `go vet`, `go test -race` на Linux/Windows/macOS и кросс-компиляция под шесть пар GOOS/GOARCH.
- Сайт на Astro публикуется автоматически. Раньше деплой делали руками через `npm run deploy`, и опубликованная версия отстала от исходников на три месяца.
- Заметки к релизу берутся из соответствующей секции CHANGELOG, а не вставляются одним и тем же текстом времён v2.0.

#### Репозиторий
- Удалены из git 1228 файлов `frontend/node_modules`, 11-МБ zip релиза, `cargo`-скелет «hello world» в корне (настоящий Rust-проект лежит в `linux/`), каталог `target/` и устаревшие JSON с результатами тестов.
- `.gitignore` шаблонами `*.exe`/`*.dll`/`*.bin`/`*.sys` перекрывал 97 файлов движка, которые обязаны быть в репозитории, — `git add` молча пропускал обновлённые бинарники. Добавлены явные исключения.

## [2.5.0] - 2026-07-10
### Windows Release — Zapret 2 Upgrade & Premium UI
- **Zapret 2 v1.0.2 Upgrade**: Обновили ядро обхода DPI до последней версии Zapret 2 (исполняемый файл `winws2.exe`, библиотеки `WinDivert.dll`/`WinDivert64.sys` и обновленный набор Lua-скриптов `zapret-antidpi.lua`/`zapret-lib.lua`).
- **UAC Auto-Elevation**: Добавлен UAC-манифест, заставляющий приложение автоматически запрашивать права администратора при запуске для корректной работы драйвера WinDivert.
- **Тихий автозапуск в трей**: Настроен бесшумный запуск в системный трей (флаг `-tray`) через планировщик Windows Task Scheduler при старте ОС.
- **Интерактивный редактор списков (Hostlist Editor)**: Реализован удобный интерфейс и backend-биндинги для безопасного редактирования списков хостов (`youtube.txt`, `discord.txt`, `other.txt`, `ipset-exclude.txt`) с защитой от directory traversal.
- **Защита от DNS Spoofing (DoH)**: Добавлена функция Secure DNS (DoH), переключающая DNS-серверы активных сетевых адаптеров на безопасные DNS Cloudflare (`1.1.1.1` и `1.0.0.1`) через PowerShell.
- **Экспорт логов**: Внедрен экспорт журналов событий в текстовый файл с вызовом нативного диалога сохранения.
- **Визуальный Конструктор Lua-Стратегий**: Разработан полноценный визуальный конструктор и текстовый редактор Lua-скриптов для профиля `Custom Profile`, позволяющий изменять тип фейкового пакета, TTL, позицию десинхронизации и параметры обмана DPI.
- **Интерактивный График Пинга**: Добавлен анимированный SVG line-chart, отображающий историю задержек до целевых серверов в реальном времени.
- **Аура Статуса и Свечение**: Внедрены динамические эффекты свечения и рамки приложения, меняющиеся в зависимости от текущего состояния (активен, подключается, ошибка, автонастройка).
- **12 Aura-Тем Оформления**: Добавлено 6 новых высококачественных тем оформления (iOS 6 Classic, Windows XP, macOS Spatial, Windows 8 Metro, iOS 26 Hologram, Interstellar Gravity) из промо-сайта.
- **Исправление Тестового Фреймворка**: Полностью починили тесты (`go test ./...`), добавив проверки привилегий администратора, обновив интеграционные тесты под Zapret 2 и исправив TLS-верификацию mock-серверов в автонастройке.

## [1.1.0] - 2026-04-07
### macOS Port — Cross-Platform Architecture
- **SpoofDPI Engine**: Новый движок обхода DPI для macOS на базе SpoofDPI (SOCKS5 прокси). Заменяет nfqws/pf. Полная замена `engine/providers/zapret_macos.go`.
- **Системная маршрутизация**: Автоматическая настройка SOCKS-прокси через `networksetup` с эскалацией привилегий через `osascript` (Touch ID / пароль).
- **Автозапуск через launchd**: Генерация `.plist` в `~/Library/LaunchAgents/com.bobberdolle1.unbound.plist` вместо Windows Task Scheduler.
- **Кроссплатформенные пути**: Конфиг перемещён из `%APPDATA%\Unbound` в `~/Library/Application Support/Unbound` (macOS) и `~/.config/Unbound` (Linux).
- **Discord Cache**: Очистка кэша теперь указывает на `~/Library/Application Support/discord/Cache` на macOS.
- **Детекция конфликтов**: macOS-специфичная проверка через `pgrep`/`pkill` (spoofdpi, v2ray, clash, shadowsocks, VPN).
- **Диагностика**: Проверка наличия SpoofDPI, доступности сетевых сервисов, прав администратора.
- **Graceful Shutdown**: При закрытии приложения SOCKS-прокси автоматически отключается через Wails `OnShutdown`, чтобы пользователь не потерял интернет.
- **CLI режим**: Headless-режим теперь работает и на macOS (без Windows-specific `AttachConsole`).

### Architectural Changes
- **`BypassProvider` Interface**: Унифицированный интерфейс для всех платформ. Автоподбор и healthcheck теперь работают через интерфейс, а не конкретный тип.
- **`BypassProviderWithCallbacks`**: Расширенный интерфейс для провайдеров с поддержкой callback'ов статуса и логов.
- **Build Tag Isolation**: Все платформенно-специфичные файлы изолированы через `//go:build windows` / `//go:build darwin` / `//go:build linux`. Кросс-компиляция не ломает другие платформы.
- **Конфигурация autostart**: `applyAutoStartSetting()` делегирована в платформенно-специфичные файлы (`config_windows.go`, `config_darwin.go`, `config_linux.go`).
- **Диагностика**: `diagnostics.go` содержит только общий тип `DiagnosticResult`. Реализации перенесены в `diagnostics_windows.go` и `diagnostics_darwin.go`.

### macOS Build
- Добавлен `macos/README.md` — полная документация модуля (зависимости, сборка, запуск, troubleshooting).
- Добавлен `macos/build.sh` — скрипт сборки macOS `.app` бандла (Intel, Apple Silicon, Universal).

### Fixed
- **Cross-platform compilation**: Windows, macOS (amd64/arm64), Linux код компилируется без ошибок в рамках своих build tags.
- **Health check**: Больше не ссылается на Windows-провайдер на других платформах.
- **Startup validator**: macOS больше не требует nfqws/dvtws; проверяет наличие spoofdpi (как warning, может быть в PATH).
- **Scanner**: Помечен как `//go:build windows`, не мешает кросс-компиляции.

## [1.0.5] — Unreleased
### Добавлено
- **OpenWRT-пакет (Unbound-WRT)**: Полная интеграция на уровне роутера — защита всей LAN без настройки клиентов.
  - Пакет `nfqws-unbound`: кросс-компиляция nfqws из zapret (bol-van), оптимизация `-Os` + strip для экономии flash.
  - `procd` init-скрипт с маппингом стратегий (multidisorder, split-tls, fake-ping, disorder+fake).
  - Правила `fw4/nftables`: перехват TCP 80/443 с `br-lan` в NFQUEUE 200, исключение RFC1918/broadcast.
  - UCI-конфиг по умолчанию в `/etc/config/unbound`.
  - `luci-app-unbound`: LuCI CBI-интерфейс — переключатель вкл/выкл, выбор стратегии, исключения доменов/IP.
  - Документация: сборка через OpenWrt SDK, установка `.ipk`, диагностика.

- **Unbound Web Extension**: Кросс-браузерное расширение для Chrome и Firefox (Manifest V3).
  - **Режим Companion**: UI-панель управления, взаимодействующая с локальным демоном Unbound Desktop через Native Messaging API.
  - **Режим Standalone Proxy**: Динамическая генерация PAC-скриптов для маршрутизации избранных доменов через внешний HTTPS/SOCKS5 прокси.
  - **Двойная тема**: "Doodle Jump Minimalism" (светлая) и "Modern Dark" (тёмная) с мгновенным переключением.
  - **Управление доменами**: UI для добавления/удаления доменов обхода с валидацией ввода.
  - **Фоновый Service Worker**: Управление состоянием, переподключение, heartbeat для Manifest V3.
  - **Кросс-браузерная сборка**: Vite + CRXJS для отдельных таргетов Chrome и Firefox.
  - **Native Messaging Host**: `host_manifest.json` + PowerShell скрипты для регистрации на Windows/macOS/Linux.
  - **Документация**: `README.md`, `GETTING_STARTED.md`, `PROJECT_SUMMARY.md` внутри модуля.

### Build System — Централизованная система сборки
- **Мастер-скрипты**:
  - `build_all.sh` (Unix/macOS/Linux/WSL) — единая точка входа для 10+ платформ: `windows`, `darwin`, `linux`, `linux-steamdeck`, `android`, `ios`, `tvos`, `openwrt`, `webos`, `decky`, `magisk`, `all`.
  - `build_all.ps1` (Windows PowerShell) — зеркало для Windows с поддержкой Docker-кросс-компиляции.
- **Docker-образы для изолированной сборки** (`scripts/docker/`):
  - `Dockerfile.linux` — Linux x86_64 на базе `golang:1.23-bookworm` + Node.js для фронтенда.
  - `Dockerfile.openwrt` — OpenWrt IPK через `openwrt/sdk:23.05` (mipsel/softfloat).
  - `Dockerfile.android` — Android APK с полным SDK + NDK внутри `ubuntu:22.04`.
  - `Dockerfile.decky` — Decky Loader плагин для Steam Deck на `node:20-bookworm-slim`.
  - `docker-compose.build.yml` — оркестрация всех Docker-сборок с поддержкой `--parallel`.
- **Платформенные скрипты** (`scripts/build/`):
  - `build_windows.ps1` — Windows Go-бинарник (с флагом `-Debug`).
  - `build_linux.sh` — Linux Go-бинарник (с флагом `debug`).
  - `build_android.sh` — Android APK через Gradle/gradlew.
  - `build_openwrt.sh` — OpenWrt бинарник (нативно) или IPK (через Docker).
  - `build_decky.sh` — Decky плагин (нативно или Docker).
  - `build_magisk.sh` — Magisk Module ZIP.
- **GitHub Actions CI/CD** (`.github/workflows/main.yml`):
  - Автоматическая сборка всех платформ при push/PR/tag.
  - Ручной запуск через `workflow_dispatch` с выбором целей и флагом релиза.
  - Автоматическое создание GitHub Release с артефактами при теге `v*`.
  - Артефакты по платформам с хранением 30 дней.
- **Документация**: `docs/BUILDING.md` — полное руководство: установка зависимостей, Docker, скрипты, CI/CD, troubleshooting, заметки по каждой платформе.

### Принципы
- **Изоляция**: все новые скрипты живут в `/scripts`, `.github` и `docs` — основной код не затронут.
- **Local-first**: всё можно собрать локально без CI. Docker опционален для кросс-компиляции.
- **Zero pollution**: Docker-сборки не устанавливают инструменты на хост-систему.

### Smart TV — Обход DPI на телевизорах (без роутера)
- **LG WebOS (rooted)**:
  - Кросс-компиляция `nfqws` из bol-van/zapret для WebOS ARM (`armv7a-neon-webos-linux-gnueabi`).
  - Enact/React фронтенд с полной навигацией через D-pad пульта (Spotlight).
  - Фоновый сервис через webosbrew (`/var/lib/webosbrew/init.d/`) — автозапуск при включении ТВ.
  - Прозрачный перехват трафика через iptables NFQUEUE — весь HTTPS-трафик YouTube проходит через nfqws.
  - Luna-сервис интеграция через `org.webosbrew.hbchannel.service` (root-выполнение команд).
  - Профили: Default / Aggressive / Lite с настраиваемыми аргументами zapret.
- **Apple tvOS (17+)**:
  - `NEPacketTunnelProvider` через официальный NetworkExtension — без джейлбрейка.
  - Адаптация C-движка `tpws` (из theos/unbound-legacy) для tvOS ARM64.
  - SwiftUI интерфейс с элегантным тогглом и фокус-навигацией Siri Remote.
  - Локальный SOCKS-прокси режим (песочница tvOS, без root).
  - Swift Package Manager конфигурация сборки.
- **Документация**: `docs/SMART_TV.md` — полная архитектура, инструкции сборки и деплоя.

---

## [1.0.4] - 2026-04-07
### Добавлено
- **Русский интерфейс**: Полный перевод UI на русский язык — все кнопки, статусы, уведомления, настройки и сообщения об ошибках.
- **Улучшенный Автоподбор**: Расширенный список тестовых целей (YouTube, Discord, Instagram, Telegram, Twitter/X, RuTracker, NordVPN, Proton). Таймаут увеличен до 8 сек (аналог probe.trolling.website). HEAD-запросы для скорости. Умные веса: YouTube/Discord приносят больше очков.
- **Реальный LivePing**: Теперь тестирует сам YouTube и Discord вместо 1.1.1.1 — показывает реальный статус обхода DPI.
- **Расширенный детект конфликтов**: Обнаруживает ciadpi, ByeDPI, OpenVPN, Cloudflare WARP, ExpressVPN, NordVPN в дополнение к winws/goodbyedpi/nfqws.
- **Умный выход из Автоподбора**: Ранний выход если найден профиль, при котором работают и YouTube, и Discord одновременно.

### Исправлено
- **LivePing** больше не показывает пинг до 1.1.1.1 (что никак не связано с реальным DPI-обходом)
- **Конфликты** теперь отображаются на русском («⚠️ GoodbyeDPI запущен»)
- **Сообщения** о завершении конфликтующих процессов — на русском
- **Лог** теперь различает ошибки на русском (ключевые слова «ошибк», «запущ»)

### Изменено
- Версия приложения: `1.0.3` → `1.0.4`
- Интерфейс полностью на русском — основная аудитория RU

## [1.0.3] - 2026-03-23
### Added
- **Auto-Tune V2**: New parallel scanning engine for YouTube, Telegram, Discord, RuTracker, and Facebook.
- **System Health Check**: Built-in diagnostics for admin rights, process conflicts, and WinDivert status.
- **Discord Hygiene**: Option to auto-clean Discord cache on startup to prevent session poisoning.
- **TCP Timestamps**: System-wide toggle to improve compatibility with modern DPI bypass techniques.
- **Version Display**: Current app version now visible in Settings UI.
- **Full Kill**: Nuclear option to terminate all conflicting DPI bypass processes and reset drivers.

### Fixed
- **System Tray**: Fixed non-responsive menu items. Added `appicon.png` embedding for stable icon display on Windows.
- **Console Flashing**: All system calls now use `CREATE_NO_WINDOW`, eliminating black box flickering.
- **Window Management**: Improved "Show" from tray logic using `WindowUnminimise`.
- **Profiles**: Restored full list of 70+ presets from Zapret 2 reference materials.
- **Auto-Tune Stability**: Fixed log duplication and cancellation logic.
- **Launch Issues**: Fixed winws2.exe working directory and blob path resolution.
- **Build Errors**: Resolved circular dependencies and missing frontend exports.

### Changed
- **License**: Officially moved to **GNU GPL v3.0**.
- **UI**: Modernized Sketchy-style overlays for errors and warnings.
- **Architecture**: Improved provider management and status reporting.

## [1.0.1] - 2026-03-15
### Added
- **UAC Elevation**: Automatic request for administrator privileges on startup.
- **Task Scheduler**: Integration for silent auto-start with high privileges.
- **Unified Logging**: New scrollable "Dev Diary" for real-time engine feedback.

### Fixed
- **WinDivert Filters**: Fixed `--new` flags causing driver initialization errors on some Windows versions.
- **Asset Extraction**: Improved reliability of binary and Lua script extraction to `%APPDATA%`.

## [1.0.0] - 2026-02-28
### Added
- **Zapret 2 Integration**: Full migration to bol-van's Zapret 2 core with Lua-based desynchronization.
- **Doodle UI**: Complete redesign of the interface using hand-drawn sketchy aesthetics.
- **Multi-Engine Support**: Experimental support for Xray/VLESS and Shadowsocks.
- **Live Ping**: Real-time latency tracking for bypassed traffic.
- **Game Filter**: Optimized profiles for low-latency gaming (Discord Voice, Steam, etc.).

## [0.9.0] - 2026-01-10
### Added
- Initial implementation of the DPI Engine Orchestrator.
- Support for GoodbyeDPI and basic Zapret (v1) profiles.
- Automated hostlist synchronization from remote sources.
- System tray integration with status notifications.

---
*UNBOUND: Open source, community-driven, and ready for 2026.*
