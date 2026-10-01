# YouTube: исследование bounded service graph, сентябрь 2026

Разделы до «PHASE 2» сохраняют результат Phase 1. Актуальный вывод Phase 2 и следующая миссия приведены в конце документа; отрицательные результаты Phase 1 не пересмотрены.

## Решение

`SERVICE_GRAPH_RESEARCH_RESULT=INSUFFICIENT_EVIDENCE`.

Текущий single-host vNext воспроизвёл `SERVICE_SCOPE_VERIFIED_FIXED` для `www.youtube.com`, затем `Apply=APPLIED` и `ManagedHealth=HEALTHY`. Однако во всех 12 активных браузерных сессиях максимум — M5: запрос media наблюдался, первый кадр и устойчивое воспроизведение не подтверждены. Отключение QUIC только в исследовательском браузере этот результат не изменило.

Три повторяющихся CDN-hosts независимо получили `SERVICE_SCOPE_VERIFIED_FIXED` для фиксированной диагностической HTTPS-пробы. Это **не** доказательство получения настоящего video/audio и **не** разрешение объединить их в product Apply. У трёх точных hosts, распознанных по попыткам запросить audio/video в дополнительной серии, свежая DNS-диагностика не дала пригодных endpoints.

Production multi-host scope, QUIC observer, discovery, Apply, ManagedHealth и StrategyIR не изменены. Следующая миссия: `REPEAT_SERVICE_GRAPH_RESEARCH`, с контролируемым продвижением M5 → M6 → M8, а не реализация графа по наблюдаемым именам.

## Источник, среда и разделение ролей

- Проверенный master: `4e645f47048ecf0a57c8080e3624dae3ae9012e3`.
- Выделенная Windows-лаборатория: Windows 10 Pro, build `19045`, x64; Edge `154.0.4258.37`, Node `24.21.0`.
- Control plane использовался только для чтения исходников, компиляции внешнего harness, SSH, переноса и анализа evidence, git и отчёта. Его HAPP, xray и proxy остались в восстановленном рабочем состоянии; итоговая проверка совпала с исходным recovered baseline, включая процессы.
- Перед удалёнными блоками и запуском каждого исследовательского браузера выполнялись `hostname`, `whoami` и проверка ожидаемого имени лаборатории. На control plane браузеры, сетевые пробы, Unbound и WinDivert не запускались.
- Предыдущая попытка после ошибочной сетевой мутации control plane исключена: `INVALID_CONTROL_HOST_MUTATION`, одна предыдущая попытка. Её данные не используются.
- Первичный аудит обнаружил оставшийся от старого лабораторного Unbound драйвер WinDivert без живого runtime. До его остановки preflight отказал всем кандидатам; эти отказы не считаются проверкой эффективности. Убрано только это мешающее лабораторное состояние.
- Две незавершённые попытки настройки браузерного инструмента исключены из session matrix. Ошибочный native-argument запуск и ошибочные результаты первого diagnostic lookup-wrapper также не используются как сетевые доказательства.

## Метод и приватность

Каждый браузер запускался с отдельным временным `--user-data-dir`, без входа, sync, extensions и personal profile. Использовались headless Edge и диагностический autoplay flag; это ограничение переносимости на обычный пользовательский браузер. После каждой сессии браузер закрывался и профиль удалялся. Существовавший посторонний Edge не использовался и не останавливался.

CDP events обрабатывались в памяти. На диск попадали только hostname, категория/роль ресурса, protocol/transport, status class, success/failure, закрытая failure boundary, относительное время и milestone. Пути, query, userinfo, cookies, headers, тела и remote addresses не сохранялись в evidence. Raw CDP dump отсутствует; TLS interception не использовался. Синтетические privacy-проверки request и response с секретами в URL, headers и cookies прошли; проверены также фактически сохранённые reduced records.

Первый reducer распознавал media по CDP resource type и response MIME. Дополнительная серия распознавала также запросы audio/video по разрешённым признакам URL **в памяти**, до удаления пути и query. Старые записи не были переклассифицированы задним числом. Fetch к CDN сам по себе остаётся API/UNKNOWN, если признаки media не установлены. Наличие suffix не служит доказательством роли или capture authority.

Настоящие content identifiers не входят в публичный документ: только `TEST_VIDEO_A` и `TEST_VIDEO_B`. Подробные reduced JSON, canonical product proofs и внешний harness хранятся локально вне git; addresses и Apply tokens в них отсутствуют.

### Milestones

| Код | Условие |
|---|---|
| M0 | Браузер запущен |
| M1 | Начата загрузка document |
| M2 | Успешный document и загрузка страницы |
| M3 | Достигнут watch/player bootstrap |
| M4 | Наблюдается player UI и video element |
| M5 | Наблюдается media request |
| M6 | Продвижение playback time и декодированные кадры, без ad state |
| M7 | Непрерывное playback ≥15 секунд |
| M8 | Непрерывное playback ≥60 секунд |

Активная сессия имела 85 bounded samples; прямой document без ответа ограничивался отдельно. M5 не означает, что получены пригодные video/audio bytes. HTTP 2xx от маленького media-ресурса entry host также не доказывает M6.

## Session matrix

Всего 13 завершённых сессий: A — 9, B — 4. Из них одна direct и 12 WWW-only. В основной серии для A было три независимых default и три QUIC-disabled профиля; дополнительная серия не заменяет эту повторяемость.

| Серия | Content | Число | Максимум | Наблюдение |
|---|---|---:|---|---|
| DIRECT_DEFAULT | A | 1 | M1 | Document response не получен в bounded window |
| WWW_ONLY_DEFAULT, основная | A | 3 | M5 во всех | UI достигнут, M6–M8 не достигнуты |
| WWW_ONLY_QUIC_DISABLED, основная | A | 3 | M5 во всех | Улучшения playback нет |
| WWW_ONLY_DEFAULT, основная | B | 1 | M5 | Другая первичная CDN identity |
| WWW_ONLY_QUIC_DISABLED, основная | B | 1 | M5 | Улучшения playback нет |
| WWW_ONLY_DEFAULT, дополнительная | A | 1 | M5 | `MEDIA_DATA_NOT_AVAILABLE` |
| WWW_ONLY_QUIC_DISABLED, дополнительная | A | 1 | M5 | `MEDIA_DATA_NOT_AVAILABLE` |
| WWW_ONLY_DEFAULT, дополнительная | B | 1 | M5 | `MEDIA_DATA_NOT_AVAILABLE` |
| WWW_ONLY_QUIC_DISABLED, дополнительная | B | 1 | M5 | `MEDIA_DATA_NOT_AVAILABLE` |

В основной серии наблюдалось 14–16 внешних hosts за активную сессию, включая 3–5 CDN-кандидатов. Это распределение **наблюдаемых**, не REQUIRED hosts; по нему нельзя выбирать production MaxHosts.

## WWW scope и независимые HTTPS-пробы

Прямой initial HTTPS на entry host завершился `ECONNRESET` на TLS; свежий scope содержал восемь IPv4 endpoints. Production vNext использовал весь bounded scope, protected controls и per-edge direct-before / active / direct-after. Выбран `prod-tls-hostfakesplit-v1`; low multisplit и overlap multisplit остались `STILL_FAILING`. Run завершился с `state_restored=true`. Apply заново разрешил DNS, ManagedHealth вернул `HEALTHY`. После обеих WWW browser-серий Revert вернул `REVERTED`.

Protected controls: `cloudflare.com` и `store.steampowered.com`. Отдельная direct-диагностика также получила от них HTTP response. Regression, отменяющий выбранные результаты, не был сообщён текущим experiment engine. Это не отдельная оценка всего функционала контрольных сервисов.

| Node | Scope endpoints | Фиксированная HTTPS-проба | Strategy ID | Canonical fingerprint |
|---|---:|---|---|---|
| ENTRY | 8 | `SERVICE_SCOPE_VERIFIED_FIXED` | `prod-tls-hostfakesplit-v1` | `1581f92ae2ef902f91d8dd5c261a283a79c1877f33c4ed00c2819f9e9190b973` |
| H07 | 1 | `SERVICE_SCOPE_VERIFIED_FIXED` | `prod-tls-hostfakesplit-v1` | `9ae7788a6d460535f10aa8143b90f8bb5a8912eb72219dcaa8b6013f938ad912` |
| H10 | 1 | `SERVICE_SCOPE_VERIFIED_FIXED` | `prod-tls-hostfakesplit-v1` | `44518451cd93462b1880346be1e38348bd63cb710d0d4dc7bfc58a03bc53f5ac` |
| H17 | 1 | `SERVICE_SCOPE_VERIFIED_FIXED` | `prod-tls-hostfakesplit-v1` | `887a304211e439c2919d017ebb81c291d3c86901ecc9440fcc942033912cb593` |

Все host experiments выполнялись независимо и восстановили состояние. Несколько single-host Apply не склеивались. Пробы CDN использовали фиксированный diagnostic contract, не signed content URL.

Сохранены limitations production reports: `CONTEXT_IDENTITY_UNAVAILABLE`, `CAPABILITY_IDENTITY_UNAVAILABLE`, baseline attribution confidence `LOW`. TLS failure не различает DPI, remote edge, routing, middlebox, local security software и provider policy. Наблюдаемый product verdict не устраняет эти ограничения и не даёт независимого доказательства механизма блокировки.

**Fingerprint caveat:** hostname входит в StrategyIR selector и canonical identity. Разные fingerprints этих четырёх планов не доказывают разные packet operations. Здесь выбран один Strategy ID с одним audited operation chain; scopes различны. Полные fingerprints нельзя приравнять, удалить selector из identity или превратить совпадение операций в verified service graph.

## Dependency matrix

28 внешних точных hosts; 11 встречались во всех 12 активных сессиях. ENTRY исключён из этого количества.

Для **каждой** внешней строки `REQUIRED_FOR_M6=INCONCLUSIVE`, `REQUIRED_FOR_M8=INCONCLUSIVE`; confidence высокий для самого наблюдения, низкий для необходимости. Общие limitations: нет playback causal differential, нет M6–M8, нет all-edge service-playback proof. Следовательно, proven REQUIRED=0, proven OPTIONAL для полного playback=0, INCONCLUSIVE=28. Ноль здесь означает отсутствие доказанной классификации, а не отсутствие необходимых внешних зависимостей.

Обозначения: S — `STATIC_EXACT` в наблюдаемой серии, D — `DYNAMIC_EXACT_WITH_STABLE_PATTERN`; `?` — не установлено. DNS — число endpoints свежей диагностики, не address authority. `FIXED/probe` — только фиксированный single-host diagnostic contract. Protocol — успешно наблюдавшийся **default-browser** protocol; у failed requests он неизвестен. Transport h2 — TCP, h3 — QUIC/UDP; `?` не позволяет вывести transport.

| Node | Host | Наблюдаемая роль | Identity | Сессии | Direct diagnostic | Default protocol | DNS | Single-host vNext / strategy relation |
|---|---|---|---|---:|---|---|---:|---|
| H01 | accounts.google.com | DOCUMENT | S | 12 | REACHABLE; browser 4xx | h2 | 1 | NOT_RUN / ? |
| H02 | fonts.googleapis.com | STATIC | S | 12 | REACHABLE | h2 | 1 | NOT_RUN / ? |
| H03 | fonts.gstatic.com | STATIC/API | S | 12 | REACHABLE | h3 | 1 | NOT_RUN / ? |
| H04 | googleads.g.doubleclick.net | API; browser aborts | S | 12 | TLS timeout | ? | 1 | NOT_RUN / ? |
| H05 | i.ytimg.com | IMAGE/UNKNOWN | S | 12 | HOST_SCOPE_OVERFLOW | ? | 12 | NOT_RUN / ? |
| H06 | rr1---sn-5hne6nz6.googlevideo.com | API | D | 3 | TLS timeout | ? | 1 | NOT_RUN / ? |
| H07 | rr1---sn-jvhnu5g-n8ve7.googlevideo.com | API/UNKNOWN | D | 8 | TLS reset | ? | 1 | FIXED/probe; same operations, scoped fingerprint |
| H08 | rr1---sn-q4fl6nsy.googlevideo.com | MEDIA_REQUEST | D | 1 | DNS INCONCLUSIVE | ? | ? | INCONCLUSIVE_DNS / ? |
| H09 | rr1---sn-q4flrne7.googlevideo.com | API | D | 1 | TLS reset | ? | 1 | NOT_RUN / ? |
| H10 | rr1---sn-u15hn5-5h.googlevideo.com | API | D | 7 | TLS reset | ? | 1 | FIXED/probe; same operations, scoped fingerprint |
| H11 | rr2---sn-4g5ednky.c.youtube.com | UNKNOWN | D | 1 | TLS reset | ? | 1 | NOT_RUN / ? |
| H12 | rr2---sn-5hne6n6e.googlevideo.com | API | D | 1 | DNS INCONCLUSIVE | ? | ? | NOT_RUN / ? |
| H13 | rr2---sn-5hneknek.googlevideo.com | API/MEDIA_REQUEST | D | 2 | DNS INCONCLUSIVE | ? | ? | INCONCLUSIVE_DNS / ? |
| H14 | rr2---sn-q4fl6nds.googlevideo.com | API | D | 1 | TLS reset | ? | 1 | NOT_RUN / ? |
| H15 | rr3---sn-5hnekn7s.googlevideo.com | API | D | 1 | TLS reset | ? | 1 | NOT_RUN / ? |
| H16 | rr3---sn-5hneknek.googlevideo.com | API | D | 1 | DNS INCONCLUSIVE | ? | ? | NOT_RUN / ? |
| H17 | rr3---sn-jvhnu5g-n8vr.googlevideo.com | API/UNKNOWN | D | 4 | TLS reset | ? | 1 | FIXED/probe; same operations, scoped fingerprint |
| H18 | rr3---sn-t0a7lnee.googlevideo.com | MEDIA_REQUEST | D | 1 | DNS INCONCLUSIVE | ? | ? | INCONCLUSIVE_DNS / ? |
| H19 | rr4---sn-4g5lznl6.c.youtube.com | UNKNOWN | D | 3 | TLS reset | ? | 1 | NOT_RUN / ? |
| H20 | rr5---sn-4g5ednr7.c.youtube.com | UNKNOWN | D | 2 | TLS timeout | ? | 1 | NOT_RUN / ? |
| H21 | rr5---sn-oj5hn5-5f.googlevideo.com | API | D | 1 | TLS reset | ? | 1 | NOT_RUN / ? |
| H22 | rr5---sn-u14g55-5m.c.youtube.com | UNKNOWN | D | 6 | TLS reset | ? | 1 | NOT_RUN / ? |
| H23 | static.doubleclick.net | STATIC | S | 12 | TLS reset | ? | 1 | NOT_RUN / ? |
| H24 | www.google.com | STATIC/IMAGE | S | 12 | REACHABLE | h3 | 8 | NOT_RUN / ? |
| H25 | www.google.ru | IMAGE | S | 12 | REACHABLE | h3 | 1 | NOT_RUN / ? |
| H26 | www.gstatic.com | STATIC/API/IMAGE/UNKNOWN | S | 12 | REACHABLE | h2/h3 | 1 | NOT_RUN / ? |
| H27 | youtube.com | API | S | 12 | TLS reset | ? | 1 | NOT_RUN / ? |
| H28 | yt3.ggpht.com | IMAGE | S | 12 | TLS reset | ? | 1 | NOT_RUN / ? |

Direct diagnostic получал только bounded TLS/HTTP response на одном текущем endpoint; 4xx от корня здорового static host не считался packet failure. Проверка необходимости host или полного scope этим не заменяется. Overflow у H05 не усекался; пробы/capture этого scope не выполнялись.

H07 повторялся во всех восьми активных A-сессиях, H17 — во всех четырёх активных B-сессиях. Fallback identities менялись и внутри одного content. Это подтверждает наблюдаемую структурную динамику, но не список REQUIRED hosts. Семантическое значение каждого API/UNKNOWN запроса без успешного response не установлено.

## Transport и первая блокирующая граница

Наблюдаемая последовательность: ENTRY → document/player UI (M4) → media attempts (M5) → отсутствие пригодных media data → M6 не достигнут. Это timeline, не доказанный полный граф обязательных hosts.

Default браузер успешно использовал h3 для части entry/static/media-init ресурсов. При `--disable-quic` такие успешно завершённые обращения переходили на h2, однако M6 не появился. Для реально распознанных внешних audio/video attempts response отсутствовал и protocol не установился. Нельзя назвать их h2 либо h3 по одному желаемому transport или по suffix.

OS metadata подтвердила TCP connections к 443 и UDP listeners только у process tree временного браузера. `Get-NetUDPEndpoint` не раскрывает remote UDP endpoint; listener не доказывает UDP к 443 и не заменяет CDP protocol evidence.

`QUIC_MATERIALLY_RELEVANT=NO` означает **отсутствие улучшения playback в этой диагностической матрице**, не отсутствие QUIC-рисков вообще. Универсальный UDP failure, QUIC handshake failure, HTTP/3 failure или DPI mechanism не установлен. Системная QUIC policy не менялась; рекомендовать глобальное отключение QUIC оснований нет.

Ошибки image/auth/advertising-class обращений не мешали достичь M4–M5. Это исключает их как обязательные для уже достигнутых milestones в этой серии, но не доказывает OPTIONAL для M8. В будущую capture graph они автоматически не включаются. Player bootstrap наблюдался без добавления других host scopes; media delivery не доказана.

## Что исследование пока не разрешает

- Нет контролируемого browser differential, где исправление конкретного внешнего host продвинуло M5 → M6/M8. Ни один внешний exact host не повышен до REQUIRED capture authority.
- H08/H13/H18 распознаны как попытки получить audio/video, но свежая DNS не образовала текущий exact-edge experiment. Это `INCONCLUSIVE`, не failure кандидата. Подмена DNS, cached addresses или принудительная загрузка другого edge не выполнялись.
- Три CDN probe fixes не являются проверкой signed media content, полученных bytes, decoder или устойчивого playback.
- Поэтому optional external multi-host executor **не запускался**: complete independently validated required host set отсутствует. Ручные WinDivert-команды и несколько Apply вместе не использовались.
- Не доказаны full-service boundedness, production MaxHosts, обязательность runtime discovery, достаточность static preset и необходимость per-host разных packet strategies.
- Нужные delivery identities появлялись при bootstrap/media attempts, а не задавались исследователем заранее. Это аргумент исследовать bounded discovery; не готовая discovery authority.

## Source-level feasibility и неизменные product constraints

Windows renderer может выразить альтернативы точных addresses с typed ports/families и сохранением compiler EngineArgv: `engine/autotunevnext/windows_scope_capture.go:14–90`, `execution.go:48–70`, `runtime_windows.go:142–231`.

Linux nft/NFQUEUE модель выражает точные family-specific destination sets без ranges: `engine/autotunevnext/linux_rules.go:53–155`, `runtime_linux.go:174–245`. Physical Linux service graph в этой миссии не проверялся.

**Ограничение текущего API:** общий `canonicalCaptureEdges` принимает максимум восемь input edges **в сумме**, не восемь для каждого из нескольких hosts. Product request, grants и evidence связывают один normalized hostname с одним fingerprint. Source-level representability не означает существующую multi-host product authorization. WWW с восемью endpoints плюс отдельный endpoint уже нельзя просто передать текущему scope renderer; limit не увеличивался.

StrategyIR содержит один selector и ordered operation chain, без typed host-to-strategy sections: `engine/strategyir/strategy.go:189–201`. Legacy `--new` argv существует, но не предоставляет typed per-host compiler/authority. Различные strategies нельзя молча flatten в один process.

Сохраняются: fresh DNS; ≤8 exact endpoints на текущий single-host scope; no truncation; per-edge D/A/D; protected controls; exact capture; bounded lifecycle и restore; отсутствие persisted edge authority; новый edge требует validation; history не даёт permission; Apply заново разрешает DNS; ManagedHealth сравнивает с actual active capture, не с большим исходным grant.

### Возможные discovery sources — только анализ

| Источник | Приватность / trust / переносимость | Главный риск |
|---|---|---|
| Versioned service metadata | Portable, deterministic logical admission | Pattern не означает capture permission; dynamic identity не доказана |
| DNS observation | Browsing metadata, platform-specific correlation | Посторонние queries и malicious page; нет milestone/role proof |
| Browser/CDP | Сильный resource/protocol context, чувствительные raw events | Не превращать research instrument в обязательное управление браузером обычного пользователя |
| OS connections | Payload-free process/IP/port correlation | Нет encrypted hostname/role authority; слабая UDP peer visibility |
| Application parsing | Может открыть dynamic identity, service-coupled | Чувствительные bodies, untrusted redirects, schema churn и broadening |

Малicious page может вызвать произвольные hostname lookups. Поэтому `DISCOVERED != VALIDATED != ACTIVE`. Нужны closed role/identity admission, отдельное fresh bounded evidence и явное ограничение host/edge/candidate/duration; наблюдение query не даёт права capture. Нормальный product не должен случайно стать generic traffic learning или требовать CDP.

### Будущие Apply / health / restart — не реализованы

Current graph может быть subset validated graph только если весь требуемый текущий milestone graph остаётся полным и каждый active host/edge имеет current authority. Исчезновение optional node может сузить graph; отсутствие required node либо incomplete discovery не является HEALTHY. Новый host/edge или возврат grant-only edge, отсутствовавшего при Apply, требует revalidation.

Будущий ManagedHealth должен сравнивать fresh required graph с **actual active capture graph**: HEALTHY только при полноте и успешной текущей проверке; NEEDS_REVALIDATION при host/edge drift; FAULT при ownership/restore либо failure существующего active path. Сохраняется урок `vnext_managed.go:422–447`, а не сравнение со старым широким grant. На restart допустим logical intent, но не address replay: discovery/DNS/validation заново.

### Observation cost

Пусть H — hosts, N — сумма текущих edges (не более H·E), C — eligible candidates, K — controls. Для полных независимых single-host runs: `N + H·K + C·(3·N + H·K)` observations. Один complete exact graph phase на candidate при совместимом control baseline: `N + K + C·(3·N + K)`, C activations вместо C·H или наивных C·N. Это design estimate, не действующая graph API. Apply добавляет примерно `2·N + K`, health — N и ownership audit.

Оптимизация уменьшает churn, не D/A/D evidence каждого host/edge. Admission → полный fresh DNS либо overflow → direct/controls → Planner → compile/preflight → совместимый exact active phase → controls → direct-after → restore → aggregate. Host-set incompleteness или исчерпанный budget дают inconclusive/not-run, не silent omission.

## Adversarial review и следующая миссия

Чистые profiles уменьшают cache объяснение; повторяемость A не основана на одном запуске. B меняет первичный CDN-host, но content и время не позволяют приписать всю динамику только content. Direct browser имеет один завершённый запуск — его confidence ограничен. QUIC A/B меняет protocol, но не playback milestone. Media-init success и generic HTTPS response не выданы за первое video frame. Signed media retrieval, codec/decoder и полный required graph не установлены.

Добавление host доказывало бы только его current exact-edge candidate improvement и, при отдельном browser differential, tested milestone dependency. При смене host/edge прежний history не является authority. Двадцать DNS endpoints означают overflow, не усечение. Если ломается только QUIC, scope expansion не обоснован. Если потребуются разные packet operations, нынешний typed IR не предоставляет безопасный per-host composite. Pattern admission не защищает от attacker-driven capture без validation и budgets.

Никаких Google ASN, CDN ranges, giant IPSet, Flowseal import, persisted CDN cache, wildcard capture или generic DNS learning это исследование не оправдывает.

Следующая конкретная миссия — повторить guarded service-graph research с разрешимыми текущими delivery identities и privacy-safe media-path measurement: воспроизвести реальный media failure, получить independent same-edge candidate evidence на настоящем compatible media contract и контролируемый M5 → M6 → M8 differential. До полного bounded validated host set multi-host activation не разрешать. Проверить content/playability и decoder limitations без сохранения bodies или content identifiers. Production graph не реализовывать по этой таблице.

## Cleanup и предел утверждения

После последней серии лабораторный audit подтвердил: Unbound/winws2 — 0, owned research processes — 0, temporary browser profiles — 0, активного WinDivert нет, исследовательский workspace удалён. HAPP отсутствовал в лабораторном baseline, proxy не был включён и не менялся. Пять посторонних Edge processes сохранились. Control-plane HAPP/process/proxy baseline совпал с recovered исходным состоянием.

`FULL_YOUTUBE_SERVICE_EFFECTIVENESS=NOT_ESTABLISHED`. Документ фиксирует tested network/time/browser context, не универсальную эффективность YouTube и не authorization для production Apply.

## PHASE 2 — M5→M6 causal media investigation

### Решение и проверяемый предел

`SERVICE_GRAPH_SHAPE=INSUFFICIENT_EVIDENCE`; следующая миссия — `REPEAT_TARGETED_MEDIA_RESEARCH`. Production source не изменён. PR остаётся draft, без merge.

Три чистые сессии с уточнённой stream-классификацией подтвердили запросы настоящего playback stream, но **ни одного response header и ни одного байта этих streams**. Активный video element не получил buffer, playable state или frames. Две точные media identities не разрешались даже непосредственно во время запроса; три другие имели устойчивую live DNS, но запросы завершались timeout либо оставались незавершёнными до границы сессии. Это свидетельство network-path failure до доставки media, не доказательство decoder failure и не установление механизма блокировки.

Один общий enum failure boundary — `UNKNOWN`: в первой сессии доказан `MEDIA_DNS_FAILURE`, в двух остальных есть timeouts до headers, которые CDP не разделяет на connect и TLS. Не подменяем эту неоднородность общим «TLS failure» или «decoder failure».

### Продолжение Phase 1, а не новая broad discovery

- Master повторно проверен: `4e645f47048ecf0a57c8080e3624dae3ae9012e3`. Все browser/network/runtime операции выполнялись только в прежней выделенной Windows-лаборатории, с hostname/whoami guard.
- WWW control: direct probe дошла до TLS и не получила HTTP success; current vNext снова дал `SERVICE_SCOPE_VERIFIED_FIXED`, восемь product endpoints, `prod-tls-hostfakesplit-v1`, fingerprint `1581f92ae2ef902f91d8dd5c261a283a79c1877f33c4ed00c2819f9e9190b973`; Apply — `APPLIED`, ManagedHealth — `HEALTHY`.
- Шесть fresh-profile WWW-only сессий: три calibration и три уточнённые target-stream сессии. Ни одна не достигла M6. После каждой browser profile удалён; после каждой из двух WWW activation-серий — `Revert=REVERTED`, без winws2 и активного WinDivert.
- После calibration потребовалась новая current WWW authority для исправленного инструмента; повторный Run не являлся browser bootstrap research. Protected controls `cloudflare.com` и `store.steampowered.com` оставались healthy до и после всех сессий.
- Ephemeral content value Phase 1 не сохранился. Для всех шести сессий Phase 2 выбран один фиксированный public `TEST_VIDEO_A`; идентичность контента между фазами **не доказана**. Никаких content identifiers в evidence или документе нет. TEST_VIDEO_B не запускался: рабочего causal graph для A нет.

### Reducer и отделение ancillary audio от playback

Network domain включён; requestId хранится в памяти, на диск попадает только salted hash. Для запросов сохраняются hostname, resource/MIME category, relative start/end, status class, protocol, connection-reused flag, aggregate data/encoded bytes, terminal success либо нормализованный `ERR_*`/CORS/BLOCKED/OTHER.

URL разбирается только в памяти и не удерживается reducer. Признаки stream endpoint плюс MIME, range либо format metadata превращаются в безопасную классификацию `PLAYBACK_STREAM`; path, query и format value не сохраняются. Один hostname suffix никогда не устанавливает роль. Browser-selected remote address используется только в памяти для membership comparison.

Calibration обнаружила важную ловушку: четыре успешных WWW audio/MIME requests давали суммарно **26 285 байт в каждой сессии**, но активный video element оставался `readyState=0`, buffer=0, frames=0. Resource type Media или audio MIME сам по себе не связывает asset с целевым video. Первоначальный calibration M5C **не считается доставкой playback stream**. Исходные traces сохранены без ретроспективной переклассификации; уточнённые три traces отделяют ancillary audio.

Для M5C нет произвольного общего размера segment: требуется положительный dataLength настоящего stream, audio/video response MIME, успешный HTTP status и terminal request evidence. Ни один target-stream request этим условиям не соответствовал; «headers/bytes получены» не утверждается по ancillary assets или одному encoded header count.

Media domain поддерживался во всех шести сессиях. Сохранялись только hashed player ID, allowlisted event name, symbolic error code либо UNKNOWN и относительное время. Media properties и свободные messages не сохранялись. В каждой уточнённой сессии — 81 reduced player event; allowlisted decoder/network error не установлен. UNKNOWN не превращён в доказательство отсутствия ошибки.

Polling активного video element сохранял только time, ready/network state, pause/end/ad state, numeric error, buffer length/end, dimensions и frame counters. DOM text, title и currentSrc не считывались в evidence. В каждой уточнённой сессии был один bounded telemetry timeout; остальные samples пригодны. Максимумы currentTime, readyState, buffer length и totalVideoFrames — ноль; numeric media error не наблюдался.

### Уточнённая session matrix

| Сессия A | Stream requests | Headers / data bytes | Terminal evidence | M5A | M5B–M5E | M6–M8 |
|---|---:|---|---|---|---|---|
| Target 1 | 6 | 0 / 0 | 6 × ERR_NAME_NOT_RESOLVED | PASS | NOT_OBSERVED | FAIL |
| Target 2 | 9 | 0 / 0 | 5 × ERR_TIMED_OUT; 4 in flight at bound | PASS | NOT_OBSERVED | FAIL |
| Target 3 | 12 | 0 / 0 | 6 × ERR_NAME_NOT_RESOLVED; 4 × ERR_TIMED_OUT; 2 in flight at bound | PASS | NOT_OBSERVED | FAIL |

Всего: 27 stream requests, 21 terminal failures, шесть незавершённых до границы наблюдения. Незавершённый запрос не назван terminal failure. Ни buffer, ни playable state, ни rendering не подтверждены. M6 требует нескольких последовательных time advances и растущих rendered frame counters вне ad state; M7/M8 — непрерывных 15/60 секунд, не время существования страницы.

`NETWORK_PATH_FAILURE=YES`; `PLAYER_PIPELINE_FAILURE=INCONCLUSIVE`. Отсутствие доставленного stream делает decoder diagnosis недоступным; это не разрешает исключить дополнительные player/content проблемы.

### Live system DNS и точные media nodes

На первом potential-media hostname сразу запускался bounded Windows `DnsQuery_W` для A и AAAA с `DNS_QUERY_BYPASS_CACHE`; Windows DNS, маршруты и browser policies не менялись. Полный address set и browser remote edge существовали только в process memory. Persisted DNS evidence: counts, TTL summary, status, overflow и SAME/CHANGED/UNKNOWN membership stability.

T0 query запускалась из request callback; native results завершались через 2,1–3,9 секунды. T1 планировалась через пять секунд; фактический query-start offset был 6,4–7,4 секунды из-за instrumentation scheduling. T2 — после bounded сессии, через 84–90 секунд относительно T0. Эти задержки отражены в reduced evidence, а не выданы за одновременный DNS/browser snapshot.

Ниже глобальные logical IDs Phase 2; session-local hashes/IDs не являются сквозной host authority.

| Node | Exact hostname | Clean sessions | Stream requests | DNS total min–max T0/T1/T2 | TTL min–max, s | Scope evidence |
|---|---|---:|---:|---|---|---|
| MEDIA_NODE_1 | rr1---sn-q4flrne7.googlevideo.com | 1 | 6 | 2–2 | 30–1556 | A=1, AAAA=1; SAME; structural only |
| MEDIA_NODE_2 | rr2---sn-5hne6n6e.googlevideo.com | 1 | 6 | 0–0 | нет ответа | UNRESOLVABLE |
| MEDIA_NODE_3 | rr3---sn-5hneknek.googlevideo.com | 1 | 6 | 0–0 | нет ответа | UNRESOLVABLE |
| MEDIA_NODE_4 | rr3---sn-q4fl6n6d.googlevideo.com | 1 | 5 | 2–2 | 30–1564 | A=1, AAAA=1; SAME; structural only |
| MEDIA_NODE_5 | rr3---sn-q4flrnss.googlevideo.com | 1 | 4 | 2–2 | 30–1390 | A=1, AAAA=1; SAME; structural only |

Live DNS capture — PASS как работа инструмента, **не** как успешное разрешение всех nodes. Overflow >8 у этих пяти stream nodes не наблюдался. У трёх resolved nodes sets оставались SAME; у двух остальных usable answer отсутствовал на всех трёх timings. Общая temporal class — UNKNOWN; resolved subset нормален в наблюдавшемся интервале, ephemeral lifetime не доказан.

Имена менялись между clean sessions при постоянном Phase 2 A; наблюдался устойчивый hostname pattern. Это не доказательство content-only rotation, конечного static graph или безопасного wildcard admission. Ни один exact stream host не повторился в двух уточнённых clean sessions.

Для actual stream не пришёл response с remoteIPAddress, поэтому `BROWSER_EDGE_IN_SYSTEM_DNS_SET=UNKNOWN`, не YES и не NO. Успешные ancillary WWW responses были h2/TCP с membership YES; это не переносится на failed stream. Native WWW DNS также вернула A=8/AAAA=8, тогда как product scope содержал восемь endpoints; различные resolver/family результаты не названы доказанным browser-authority mismatch.

### Hostname-only transport diagnostic, не media vNext

После target-серии для пяти exact nodes выполнена отдельная fresh bounded TLS-only диагностика без HTTP path/query и без signed media URL:

- MEDIA_NODE_2/3 снова не дали usable DNS; TLS не запускался.
- MEDIA_NODE_1/4/5: IPv4 TCP соединение установилось, TLS handshake истёк по timeout; IPv6 TCP — address unreachable.
- Эти результаты принадлежат отдельному TLS client. Они не устанавливают TCP/TLS stage browser timeout, browser-selected edge, successful media response или verified strategy.

Current production vNext использует HTTPS GET и принимает HTTP 2xx/3xx (`vnext_history.go:27–72`, `engine/observatory/observer.go:131–249`). Root/generate_204 не представляет signed media delivery. Поэтому `MEDIA_HOST_SAFE_SINGLE_HOST_PROBE=NOT_REPRESENTATIVE`, `MEDIA_HOST_SINGLE_VNEXT=NOT_RUN`; TLS-only diagnostic не выдан за vNext selection или Apply grant.

Actual media protocol — UNKNOWN, transport — UNKNOWN: до response negotiated protocol не установлен. Ancillary h2 не отвечает на этот вопрос. Actual media h3 не наблюдался, поэтому условная дополнительная QUIC-disabled сессия не запускалась; результаты Phase 1 не перепроверялись broad QUIC research.

### Почему exact-host A/B/A не запускался

Все пять identities остаются **наблюдавшимися media candidates**, не HIGH_PRIORITY_CAUSAL_CANDIDATE и не REQUIRED. Повторы запросов внутри одной сессии не заменяют повторяемость exact host между clean sessions.

Решающая причина отсутствия A/B/A: ни один exact stream host не повторился между тремя clean sessions, поэтому не выполнен критерий repeatable exact causal candidate. Два unresolvable nodes также не могут образовать current scope; остальные три имеют bounded DNS. Неизвестный browser edge и отсутствие representative production media probe остаются ограничениями evidence, **не дополнительными обязательными prerequisites** разрешённого research experiment. Доверенный существующий StrategyIR template можно проверять в таком эксперименте без предварительного production media selection. Добавлять stable preliminary CDN Fetch либо WWW audio asset вместо actual stream означало бы подменить гипотезу.

`RESEARCH_MULTI_HOST_EXECUTOR_USED=NO`, `CAUSAL_ABA_COMPLETED=NO`, REQUIRED — ноль **доказанных**, не ноль существующих dependencies. H1/H2/combinations не запускались без repeatable exact causal candidate; causal repetitions и TEST_VIDEO_B не запускались без causal success для A. Никакой competing winws2, persisted edge set, wildcard, suffix capture, third-host auto-admission или browser-IP bypass не применялся.

Исследование backend representation всё же уточнило Phase 1:

- Pinned Zapret2 `a1bca5a85e25ab138e9617a560c262fcf53e969a`, `docs/manual.en.md:678,875–877`, документирует inline `--ipset-ip` и `--hostlist-domains=^hostname`; `^` отключает suffix matching. Это file-free exact-section representation, **не physical parser/capture acceptance** данной миссии.
- `productionVNextStrategyCatalog(hostname)` (`vnext_product.go:336–399`) независимо связывает доверенный template с каждым host. Сравнивать надо ID, ordered operations, range/safety/desync semantics отдельно от bound canonical fingerprint. Current media template effectiveness не установлена; DIFFERENT_TEMPLATE не выводится из hostname fingerprint.
- Текущий `RenderWindowsServiceScopeCapture` по-прежнему ограничен восемью edges **в сумме**. Никакой limit, production IR/compiler, grants или runtime API не менялся.
- Future research executor должен иметь отдельный bounded manifest, независимый bind каждой section, exact current guards, один owned process, healthy nonoverlapping controls, global timeout и proven cleanup. Представимость argv не является разрешением активировать graph.

### Adversarial review и следующий targeted эксперимент

Не было B, поэтому нет утверждения «изменён только H», cache-safe differential, возвращения failure в A2 или трёх causal repetitions. Fresh profiles не отменяют DNS/CDN/content drift. Семантика static/audio assets не подменяет playable stream; no-buffer не подменяет decoder diagnosis; отсутствие headers не устанавливает censorship mechanism. Healthy controls не доказывают отсутствия всех побочных эффектов, но их regression не наблюдался.

Следующий эксперимент: получить **повторяющийся exact current playback-stream host** с live bounded DNS и пригодной edge/path evidence либо исследовать явно bounded session-dynamic causal admission. После этого — один exact-host A1/B/A2 с неизменной WWW authority, свежими profiles, проверкой DNS/CDN drift, actual stream bytes/buffer/frames и protected controls. Только воспроизводимый M6/M7 differential может повысить host до REQUIRED; production graph пока не реализовывать.

### Privacy, cleanup и доставка Phase 2

Privacy smoke с synthetic secrets прошёл; фактический audit шести persisted traces проверил 1058 reduced requests, 486 player events и 506 media-element samples. Raw CDP, URLs/paths/queries, signed parameters, content IDs, headers/cookies/bodies и remote/local addresses в evidence не сохранялись. Полные media URLs существовали только в ephemeral browser/CDP memory.

Финальный лабораторный audit после переноса reduced evidence — PASS: Unbound/winws2 — 0, owned research processes — 0, temporary profiles — 0, WinDivert services — 0; owned workspace удалён. Пять посторонних Edge processes сохранены; proxy не включён, PAC отсутствует. Control-plane hostname, HAPP service/process identities и proxy совпали с начальным baseline Phase 2; network mutations на control plane не выполнялись.

`FULL_YOUTUBE_SERVICE_EFFECTIVENESS=NOT_ESTABLISHED`. Итог Phase 2 — ограниченное отрицательное causal исследование с улучшенной media/DNS наблюдаемостью; PR #71 остаётся draft, без production cutover.

## PHASE 3 — session-dynamic exact-host causal intervention

**Итог: `INSUFFICIENT_EVIDENCE`; эксперимент остановлен до browser/capture calibration.** Complete fresh ENTRY DNS дал 16 endpoints при research bound восемь на host. Это отказ admission, не отрицательный результат media intervention. PR #71 остаётся **DRAFT**; production source и `MaxServiceScopeEdges=8` не менялись.

### Новый causal unit и неизменная граница authority

Phase 3 сняла прежнее требование одинакового exact media hostname между clean sessions. Causal unit — exact playback-stream host, выбранный браузером **в этой сессии**. План: CDP Fetch request-stage pause, fresh exact DNS, bounded temporary graph, затем continuation **того же** исходного signed request без reload или diagnostic root substitution.

Discovery eligibility была ограничена actual-stream classification Phase 2, registrable domain `googlevideo.com` и anchored rr/sn hostname shape. Pattern предназначался только для consideration; он не должен попадать в WinDivert authority. Packet authority — только independently admitted exact host и его exact current endpoints; никакого wildcard, suffix/range capture или browser-IP fallback.

До запуска был зафиксирован порядок `C-I-I-C-C-I`, одна external WWW-only calibration и одна pause-only calibration. Target pause — восемь секунд, hard maximum — 12; максимум два media hosts, восемь endpoints на host и 16 distinct endpoints в research union. Эти session/intervention правила **не были физически проверены**: ENTRY admission не прошёл.

### Текущий WWW production control — PASS

Исследовался master `4e645f47048ecf0a57c8080e3624dae3ae9012e3`; все networking/runtime действия — только на guarded `DESKTOP-MNEHCPT`, под `unbound-lab`.

- Direct `www.youtube.com`: TCP дошёл до TLS, HTTP success отсутствовал; current failure воспроизведён.
- Один production vNext Run: `COMPLETED_SELECTED`, `SERVICE_SCOPE_VERIFIED_FIXED`, восемь scope endpoints, `state_restored=true`.
- Selected semantic template: `prod-tls-hostfakesplit-v1`; WWW-bound canonical fingerprint `1581f92ae2ef902f91d8dd5c261a283a79c1877f33c4ed00c2819f9e9190b973`.
- Apply не вызывался. Initial Run использовал isolated temporary configuration; его обычная WWW диагностика не является graph grant. Research graph не создавал Apply capability, managed intent, saved graph или media outcome-ledger success.
- Protected controls до Run и после restoration: `cloudflare.com` — healthy 3xx, `store.steampowered.com` — healthy 2xx.

Первая попытка запуска external test binary завершилась на PowerShell native-argument parsing **до Go test entry**. После исправления аргументов выполнился ровно один production Run; неуспешный launcher не считается network experiment или browser session.

### ENTRY: полный fresh DNS не помещается в host budget

На lab выполнены native Windows `DnsQuery_W` A/AAAA с `DNS_QUERY_BYPASS_CACHE=8`. Адреса существовали только в памяти; в evidence сохранялись counts, TTL range и outcome.

| Наблюдение | A | AAAA | Всего | TTL, секунды | Overflow >8 |
|---|---:|---:|---:|---|---|
| Independent fresh native preflight | 8 | 8 | 16 | 110–206 | YES |
| Native query внутри corrected compiled admission gate | 8 | 8 | 16 | 226–271 | YES |
| Complete production-equivalent system `LookupNetIP` | 8 | 0 | 8 | API не предоставляет TTL | NO |

System result и fresh native result различались по address-family coverage. Eight-edge production scope не был молча выдан за полный fresh A+AAAA scope research ENTRY. Не выбирались первые восемь addresses, IPv4-only subset либо удобный resolver result; native result не усекался.

Применён conservative fail-closed ENTRY contract: полный fresh A+AAAA answer должен удовлетворять тому же eight-endpoint-per-host budget, что и media host. Для sixteen-endpoint ENTRY этот prerequisite отсутствует. Даже research total limit 16 не отменяет per-host limit восемь.

Corrected external harness физически выдал:

- `ENTRY_NATIVE_SCOPE_OVERFLOW`, `entry_edges=16`, `entry_bound=8`;
- `research_capture_started=false`, `browser_started=false`, `causal_sessions_started=0`, `truncation_used=false`;
- после отказа: `product_state_restored=true`, `owned_process_absent=true`, `driver_absent=true`, `ownership_audited=true`.

Lab command завершился с exit 1 через 102.28 секунды на этом единственном admission rejection. Это exercised safety-stop smoke, **не passing CI/test-suite claim**.

Для продолжения нужен complete current ENTRY scope в пределах восьми endpoints либо отдельно разрешённая и validated address-family-specific ENTRY authority. Нельзя молча принять такую policy, изменить production limit или расширить capture, чтобы получить положительный результат. Разница resolver sets сама по себе **не доказывает browser service-discovery-authority mismatch**.

### Какие causal результаты отсутствуют

В Phase 3 — **ноль** browser calibrations, pause-only sessions, CONTROL sessions и INTERVENTION sessions. TEST_VIDEO_A не выбирался; TEST_VIDEO_B, H1/H2, template search и A2 reversal не запускались. Ни один signed playback request не создавался этим экспериментом.

Поэтому:

- Fetch interception и preparation/continuation budget не проверены на живом browser request.
- Media hosts discovered/resolved/admitted — ноль; observed simultaneous host budget и media edge count — ноль, не evidence для production constant.
- M5B/M5C/M5D/M6/M7/M8 differential отсутствует. `SAME_TEMPLATE_MEDIA_INEFFECTIVE` не установлен; ограниченный template search не разрешён его prerequisites.
- SAME_TEMPLATE rebinding остаётся source-level hypothesis, не media effectiveness result. Different bound fingerprint не означает different semantic template.
- Нет causal REQUIRED hosts, class-level causal proof, tested graph или основания реализовать production service graph.

В обязательном final-report формате `M0`, zero counts и `FAIL` media/Fetch fields означают **NOT_RUN / no verification evidence**, а не физически наблюдавшиеся media failures. `ANY_MEDIA_HOST_OVERFLOW_GT8=NO` не относится к ENTRY: media hosts не исследовались, тогда как ENTRY overflow — YES.

### Adversarial review и backend implications

Read-only security/causal review выявил дефекты в предварительном external tooling до его browser/intervention execution: ENTRY authority substitution, possible uncaptured-path attribution, insufficient complete-block differential, negative-request linkage и pause-bound checks, page-controlled telemetry, process/job cancellation и playback provenance. Эти findings не выдаются за наблюдавшиеся browser failures. ENTRY authority исправлена и её rejection path выполнен; неиспользованные browser/graph prototypes удалены, в git они не попали.

Для future design ответ на «может ли arbitrary page content заставить UNBOUND захватывать произвольный attacker-chosen host?» обязан быть **NO**. Service-definition eligibility, playback-role/provenance constraint, host budget, fresh bounded DNS и independent validation должны предшествовать ACTIVE. Pattern и наличие playback-shaped query не являются cryptographic authentication или самостоятельным capture grant. Эта миссия не реализовала и не проверила такой production механизм.

Успешной Windows graph representation нет. Pinned `--ipset-ip`, `--hostlist-domains=^hostname` и AND-semantics `--wf-raw-filter` остаются documented syntax, не Phase-3 multi-section parser/capture acceptance. [Pinned manual](https://github.com/bol-van/zapret2/blob/a1bca5a85e25ab138e9617a560c262fcf53e969a/docs/manual.en.md#L603-L607), exact-host static syntax — lines 875–877.

Linux проверялся только по source: `engine/autotunevnext/linux_rules.go` и `runtime_linux.go` разделяют exact nft/NFQUEUE acquisition и compiled engine argv, владеют одной table/queue/process lifecycle; production contract остаётся single-target с flat eight-edge bound. Successful per-host graph не получен, физического Linux теста нет; architectural cutover не обоснован.

### Privacy, cleanup и следующий шаг

Persisted Phase-3 evidence содержит reduced DNS/product/cleanup outcomes, а не CDP dumps, URLs/paths/queries/signatures/content IDs, cookies/headers/bodies или remote/local addresses. Synthetic reducer smoke выполнен, но live browser privacy и job behavior не объявляются проверенными: browser не запускался.

Final lab audit — PASS: Unbound/winws2, research processes, temporary research profiles, WinDivert services и active packet state — ноль; owned remote workspace удалён. Пять посторонних Edge processes сохранены. Proxy/PAC baseline не изменён. Control-plane HAPP service/process identities и proxy совпали с исходным baseline; local network mutations не выполнялись.

`SERVICE_GRAPH_SHAPE=INSUFFICIENT_EVIDENCE`, `FULL_YOUTUBE_SERVICE_EFFECTIVENESS=NOT_ESTABLISHED`, `NEXT_REQUIRED_MISSION=REPEAT_SESSION_DYNAMIC_INTERVENTION`. Повторять следует после разрешения ENTRY authority/budget prerequisite; не возвращаться к globally stable media hostname gate и не считать этот stop доказательством неэффективности media graph.

## PHASE 3B — resolver authority alignment and dynamic intervention

### Исправление research admission model; Phase 3 сохранена

Исторический Phase-3 stop выше не удалён: native A/AAAA действительно дали 16 addresses, и прежний external gate действительно отказал до browser/capture. Отказ был корректен **под тем research gate**, но gate ошибочно считал diagnostic DNS union production packet authority. Он не воспроизводил текущий resolver contract.

Теперь различаются:

- **PRODUCT_SCOPE** — точный текущий canonical set, возвращённый `DefaultScopeResolver.ResolveServiceScope` → `ResolveServiceScope(..., net.DefaultResolver)` для TCP/ANY. Только этот set после validation может авторизовать research capture; `MaxServiceScopeEdges=8` применяется к нему.
- **DIAGNOSTIC_DNS_SUPERSET** — отдельные native A/AAAA и `ip4`/`ip6` comparisons. Это evidence, не дополнительные packet grants и не admission veto для bounded PRODUCT_SCOPE.

`AddressFamilyAny` не означает обязательный union результатов всех DNS APIs. В текущем `service_scope.go:95–159` canonicalization принимает поддерживаемые семьи из **фактически возвращённого** `LookupNetIP("ip", hostname)` результата, deduplicates и явно отказывает при overflow; silent truncation нет. Production source, resolver, family semantics, Apply, ManagedHealth и capture renderer не изменены.

### Физическое воспроизведение resolver authority — PASS

External temporary Go overlay собран из текущего source master `4e645f47048ecf0a57c8080e3624dae3ae9012e3` и выполнен только на guarded `DESKTOP-MNEHCPT` под `unbound-lab`. Helper прямо вызывает `DefaultScopeResolver`; сразу после него выполняется current WWW product Run. Addresses и exact set comparison существуют только в памяти.

Первый authority-only smoke завершился `PASS` за 82.93 секунды:

| Resolver / authority | IPv4 / A | IPv6 / AAAA | Total | Outcome |
|---|---:|---:|---:|---|
| Helper PRODUCT_SCOPE | 8 | 0 | 8 | PASS |
| Current product Run scope | 8 | 0 | 8 | `SERVICE_SCOPE_VERIFIED_FIXED`; exact equality YES |
| `net.DefaultResolver.LookupNetIP("ip", host)` | 8 | 0 | 8 | OK |
| `net.DefaultResolver.LookupNetIP("ip4", host)` | 8 | 0 | 8 | OK |
| `net.DefaultResolver.LookupNetIP("ip6", host)` | 0 | 0 | 0 | LOOKUP_FAILED; это не native AAAA count |
| Native `DnsQuery_W` A / AAAA | 8 | 8 | 16 | diagnostic only |

Helper scope fingerprint: `f4ff4c8016ff8c44aa5131b02a0652b66a561ee1f7660823cf2a18ffffc7e480`. Это scope fingerprint, не StrategyIR fingerprint. Product выбрал `prod-tls-hostfakesplit-v1`, восстановил state; Apply не вызывался, unused grant был invalidated. Native-only AAAA не добавлялись в ENTRY. Protected controls до/после — healthy; после smoke owned engine count — ноль.

### IPv6 capability и граница объяснения

Read-only Windows inspection и Go inspection согласны: usable non-link-local IPv6 address отсутствует; IPv6 default route отсутствует. Actual outbound TCP6/443 к healthy control не подключился. Settings, routes, DNS и adapter bindings не менялись.

`IPV6_HOST_USABLE=NO`, `RESOLVER_DISCREPANCY_CLASS=NO_USABLE_IPV6`. На этой lab/network product resolver возвращает восемь IPv4 endpoints, тогда как native diagnostic enumeration дополнительно возвращает восемь AAAA. Эти наблюдения не доказывают точный Windows/Go implementation mechanism, не означают production truncation и не делают IPv6 answers универсально нерелевантными.

### Pre-execution review: исправление прототипа, не browser findings

Read-only security/causal review обнаружил до browser execution дефекты prototype authority ordering, original-request provenance, hard-pause failure handling, directory/PID ownership, response-MIME attribution, pause-only prerequisite, H2 delivery/cancellation gates и same-template accounting. Первый successful authority-only smoke проверял healthy path, не adversarial rejection boundary. Эти source findings не выдаются за физические playback failures; browser/intervention evidence приводится отдельно ниже.

### Executor calibration и baseline surface gate — PASS

Owned ENTRY-only executor откалиброван на guarded `DESKTOP-MNEHCPT` под `unbound-lab`. Session 0 (`BASELINE`) признана `VALID`; `baseline_gate=true`; достигнуты `M2` (target document), `M3` (frame), `M4` (player surface) и `M5A` (classified `PLAYBACK_STREAM` request). Admitted scope остаётся точным product set: 8 addresses, 8 IPv4, 0 IPv6, resolver `status: OK`; `authority: admitted=true, diagnostic_authority=false, equality=PASS`. Media edge budget соблюдён (`admission.media_scope.total <= 8`).

Три дефекта прототипа, найденных и исправленных по ходу, относятся к коду исследования, а не к browser findings, и фиксируются отдельно:

1. `Page.navigate` имел 5-секундный timeout и необработанный reject — CDP acknowledge timeout прерывал session как `CDP_TIMEOUT`. Acknowledgement-timeout не является browser failure; навигация теперь имеет 15-секундный timeout и `.catch(() => {})`.
2. `SAMPLE` вызывал `playVideo()`/`play()` на каждом 1-секундном тике. При `readyState === 0` это порождало волну `ERR_ABORTED` и подавляло `M5A` во всех arms. Вызов удалён: baseline gate проходит только без него.
3. `select()` предпочитал `duration === null`, то есть live/premiere/members cards. Live stream возвращает `NO_DIRECT_FORMATS` для guest context, и поскольку `run()` вызывает `select()` только для session 0 (`if (!this.destination)`), один live pick инвалидировал все последующие arms. Выбор теперь требует ограниченный VOD (90–1200 s, затем 90–3600 s) и падает закрыто при его отсутствии; добавлен второй public-www search surface.

### Media boundary: измеренные failure modes

`DYNAMIC_MEDIA_HOSTS_DISCOVERED` подтверждает session-dynamic nature: session 0 открыл 2 media host (`rr3---sn-q4flrne7.googlevideo.com`, `rr5---sn-aigl6nzl.googlevideo.com`), session 1 — другой (`rr4---sn-5hne6nzd.googlevideo.com`). Ни один адрес не совпал между arms. Media requests действительно выдаются браузером и классифицируются: 6 на arm, `mime=VIDEO`, `headers_received=false`.

| Arm | Media host | Requests | Terminal |
|---|---|---:|---|
| 0 BASELINE | `rr3---sn-q4flrne7` | 2 | `PENDING` |
| 0 BASELINE | `rr5---sn-aigl6nzl` | 2 | `FAILED` `ERR_CONNECTION_RESET` |
| 0 BASELINE | `rr5---sn-aigl6nzl` | 4 | `PENDING` |
| 1 PAUSE_ONLY | `rr4---sn-5hne6nzd` | 6 | `FAILED` `ERR_NAME_NOT_RESOLVED` |

Итог по обоим arms: `media_headers_count=0`, `aggregate_media_bytes=0`, `media_denied_count=0`. Наблюдаются **два разных механизма** на media boundary — DNS-level (`ERR_NAME_NOT_RESOLVED`) и transport-level (`ERR_CONNECTION_RESET`) — и оба возникают без вмешательства продукта: `admission_attempt_count=0`. Protected controls (`cloudflare.com`, `store.steampowered.com`) healthy до и после каждой session, поэтому отказ target-scoped, а не общий network outage.

### Блокирующий результат: intervention arm недостижим под текущим player delivery

Trigger pause регистрируется только при `associateStream(url, inventory).ok === true` (`dynamic-browser.mjs:577-579`), а inventory строится из `representation()`, который принимает исключительно напрямую адресуемые signed media URLs. Измеренная форма player response:

```
streaming_data_keys: [expiresInSeconds, adaptiveFormats, serverAbrStreamingUrl]
format_count: 0    adaptive_format_count: 50    playability_status: OK
player_reject_shape:   {no_addressable_url: 50, url_without_sparams: 0}
player_reject_reasons: {UNSUPPORTED_FORMAT: 50}
cipher_unresolved: 0  representations: 0
player_xhr: {seen: 0, post_seen: 0, classified: 0, unclassified: 0,
             inventoried: 0, insecure_rejected: 0, watch_document_inventoried: 1, methods: {}}
```

`player_xhr.seen = 0` и пустой `methods` — отдельное измерение, закрывающее последний inference: **`/youtubei/v1/player` XHR этим player не выдаётся вовсе**. Счётчики велись независимо от classification (`dynamic-browser.mjs`, ветка `requestWillBeSent`), поэтому состояние «XHR не классифицирован из-за path/method/frame mismatch» (`unclassified > 0`) отделилось бы от состояния «XHR не произошёл» (`seen == 0`). Наблюдено именно `seen == 0`, то есть classification дефекта нет: единственный авторитетный источник player data — watch document, и других нет.

Следовательно утверждение о SABR delivery **измерено на обоих концах**, а не выведено из одного наблюдения: player не запрашивает player endpoint, а встроенный ответ содержит только server-described форматы.

Все 50 объявленных форматов **server-described**: ни `url`, ни `signatureCipher`. `cipher_unresolved=0` исключает cipher-путь, `url_without_sparams=0` исключает coverage-путь, `serverAbrStreamingUrl` присутствует. Воспроизведено в трёх последовательных runs и в обоих arms. Фактический media URL, использованный браузером, получен SABR-протоколом; его воспроизведение или дешифровка прямо вне scope (`// No deciphering or URL synthesis.`), а `signatureCipher` несёт opaque server-computed signature, которую нельзя авторить.

Следствие: доверенный inventory неконструируем **по построению**, а не из-за дефекта. Ни один trigger не регистрируется, `PAUSE_ONLY` даёт корректный true-negative `NO_ORIGINAL_TRIGGER`, и intervention недостижим. Ослабление provenance gate недопустимо: фабрикация pass хуже честного stop.

Прогностическая классификация: `TRUSTED_INVENTORY_ASSOCIATION_UNREACHABLE`, `MEDIA_DELIVERY=SABR_SERVER_DESCRIBED`, `NO_SABR_FORBIDDEN=TRUE`. Это ограничение измерятельного контура, а не доказательство неэффективности media graph.

### Cleanup, controls и граница вывода

`OWNED_CLEANUP_UNVERIFIED` и `GRAPH_CLEANUP_UNVERIFIED` не возникали ни в одном arm: `profile_removed=true`, `owned_job_stopped=true`, `child_exit_observed=true`, `owned_descendants_remaining=0`; `finish: ok=true, controls_ok=true, state_restored=true, owned_process_count=0`; `final_cleanup: actual_engine_process_count=0, driver_registration_restored=true`. Транзакционное восстановление состояния соблюдено во всех прогонах. Privacy сохранена: `Fetch.enable` ограничен `https://*.googlevideo.com/videoplayback*` на `Request` stage и только для non-baseline arms; в evidence попадают counts, key names и status enums — без URL, query parameters, headers и page content.

`PHYSICAL_ACCEPTANCE=NOT_RUN`, `INTERVENTION_ARM=UNREACHABLE_UNDER_SABR_DELIVERY`, `SERVICE_GRAPH_SHAPE=INSUFFICIENT_EVIDENCE`, `FULL_YOUTUBE_SERVICE_EFFECTIVENESS=NOT_ESTABLISHED`. Наблюдённое media-edge blocking (DNS-level и transport-level, target-scoped, controls healthy) документировано; product-graph effectiveness **не** опровергнуто и **не** подтверждено. Требуется отдельная миссия, способная получить доверенный inventory на не-SABR delivery path, либо явное решение владельца о допустимости SABR-уровневой provenance.

### Per-host graph bounds — оценка без изменения production

Production не изменялся; оценка приведена как input для следующей миссии. Измеренная ширина на arm: 2 и 1 media host, до 6 classified media requests, `admission.media_scope.total <= 8` — существующий предел не был достигнут ни разу. Наблюдённые хосты полностью session-dynamic и не пересекались между arms, поэтому **глобально стабильный media hostname не существует** и не может быть gate. Практический вывод: per-host enumeration должна выполняться внутри каждой session, а budget (число хостов, media scope, requests) обязан быть per-session и per-arm; любой фиксированный host или IP не является валидным адресатом. Product-side rendering ограничен `MaxServiceScopeEdges=8`, чего достаточно для наблюдённой ширины, но это не доказательство достаточности для произвольного session — расширение потребовало бы отдельного измерения, а не изменения константы.

### Dynamic discovery authority chain — review

Цепочка полномочий и её границы проверены read-only:

1. **Admission authority** — только PRODUCT_SCOPE из `DefaultScopeResolver`; `diagnostic_authority=false` подтверждает, что diagnostic DNS не участвовал в авторизации.
2. **Target provenance** — inventory читается только для target frame: watch document и `POST /youtubei/v1/player` на `www.youtube.com`, с проверками `securityState === 'secure'`, TLS 1.2/1.3 либо QUIC+`h3`, `status_class === 2`, отсутствие service worker, disk cache и prefetch. Наружный источник не может подменить player data.
3. **Media association** — `associateStream` требует совпадения host, path и параметров `id`/`itag`/`expire` с inventory entry; `n` проверяется покрытием `sparams`. Изменение любого из них даёт отказ (покрыто self-test).
4. **Deactivation authority** — решение о candidate принимает только `associateStream(...).ok === true` **и** `player_provenance === true`, то есть независимое подтверждение того, что страница действительно принадлежит выбранному target video и не показывает рекламу.
5. **Fail-closed** — при пустом inventory trigger не регистрируется, arm помечается `NO_ORIGINAL_TRIGGER`, а не «успехом». Provenance gate не ослаблен.

Слабое место цепочки — не security, а достижимость: при SABR delivery шаги 2–3 не могут дать non-empty inventory, поэтому вся цепочка корректно останавливается на шаге 3. Это fail-closed поведение, а не обход.

### Fresh product DNS drift — проверено, drift отсутствует

Drift проверяется независимо от intervention и потому не зависит от SABR blocker. Два независимых fresh `resolver_diagnostics` (по одному на session) дали идентичный результат: `ip: total 8, ipv4 8, ipv6 0, status OK`; `ip4: 8/8/0 OK`; `ip6: LOOKUP_FAILED`. Admitted `entry.total = 8` в обеих sessions — product scope не изменился между session. В session 1 зарегистрировано 10 pause, и `outcome_drift` для всех десяти равен `SAME`. Session 0 (`BASELINE`) pause не имеет by design, поэтому 0 pause там ожидаемо, а не дефект.

Следствие для интерпретации: ENTRY product DNS не дрейфует, тогда как media hostname полностью ротируется per session (нулевое пересечение между arms). Поэтому недостижимость intervention **не может быть объяснена** ENTRY DNS drift, и `SERVICE_GRAPH_SHAPE=INSUFFICIENT_EVIDENCE` не артефакт resolver drift. Это отделяет измеренную нестабильность media edge от стабильного admission path.

## PHASE 4 — non-SABR client, exact media provenance, causal dynamic host test

### Owner decisions applied

Provenance до SABR-level **не ослаблялся**. Media URL получен от авторизованного non-SABR research-клиента, требует `logged-out`, без персональных cookies и без аккаунта. Ни один URL, path, query, signature, itag, PO token, cookie, header, video ID или raw IP не попал ни в один файл evidence. В persistent evidence остались только hostname, агрегатные счётчики, status enum и template ID.

### Phase 0 — tooling и preflight

Guard: `DESKTOP-MNEHCPT`, `desktop-mnehcpt\unbound-lab`, Windows 10 19045. Инвентарь: Node v24.21.0, `curl.exe`, Edge 154; Python и ffmpeg отсутствуют. `yt-dlp` отсутствовал — установлен официальный standalone build: версия **2026.08.19**, 17 840 399 байт, SHA256 `66674953fe251b89f4d08c5f0e35e0728679bd67ab3d7d05c0562af101dd3e7a`. Бинарь не попал в репозиторий и не коммитится.

### Direct HTTPS baseline — блокировка специфична для YouTube

Первая попытка прогнать client matrix дала `EXTRACT_FAILED` у **всех десяти** клиентов одинаково. Это не свойство клиентов: `yt-dlp` выполнялся без активного bypass. Диагностика без extractor'а:

- `cloudflare.com` → HTTP 301; `www.google.com` → HTTP 200 — общий интернет работает.
- `www.youtube.com` → `code=000` (~15 с), `youtubei.googleapis.com` → `code=000` (~15 с).
- `yt-dlp` stderr: `ConnectionResetError(10054)` («connection forcibly closed by remote host») и на webpage, и на API page — характерный DPI RST.

**DNS для googlevideo не заблокирован.** `googlevideo.com`, `redirector.googlevideo.com` и синтетический `rr1---sn-4g5ednsz.googlevideo.com` резолвятся нормально. Более ранний `LOOKUP_FAILED` на `rr3---sn-aigl6nz1.googlevideo.com` объясняется тем, что googlevideo-имена session-scoped и этот конкретный host просто не существует. Media delivery блокируется не на DNS, а на TLS, и поэтому per-host packet strategy в принципе применима.

Следствие: client matrix должен выполняться **при активном extraction graph**, иначе он измеряет не клиент, а отсутствие bypass.

### Phase 1–4 — non-SABR client matrix (при активном graph)

Прогон при `extraction_graph` (`OWNED_PROCESS_COUNT=1`, `CAPTURE_READY=true`), TEST_VIDEO_A, logged out:

| Client | FORMAT_COUNT | DIRECT_HTTPS | AUDIO_ONLY | ERROR_CLASS |
|---|---|---|---|---|
| `tv` | 0 | 0 | 0 | `NO_DIRECT_HTTPS_FORMATS` |
| `ios` | 0 | 0 | 0 | `NO_DIRECT_HTTPS_FORMATS` |
| `android` | 5 | 5 | 4 | `NONE` |

**Выбран `android`**: 5 direct HTTPS representations, 4 audio-only, 1 muxed, `LOGIN_REQUIRED=false`, PO token **не требуется**. Это самый простой воспроизводимый вариант без ослабления гарантий; `tv`/`ios` direct delivery не дали вовсе.

Важный побочный факт: `yt-dlp` требует доступности и `www.youtube.com`, и `youtubei.googleapis.com`. Текущий ENTRY scope покрывает только `www.youtube.com` (8 endpoint). Извлечение provenance возможно только при добавлении InnerTube API host в scope — это отдельное архитектурное наблюдение, а не следствие SABR.

### Phase 5–7 — извлечение и допуск scope

- TEST_VIDEO_A → `rr4---sn-jvhnu5g-n8ve7.googlevideo.com`
- TEST_VIDEO_B → `rr6---sn-jvhnu5g-n8vk.googlevideo.com`
- Product-equivalent scope каждого media host: **1 IPv4, 0 IPv6, status OK, overflow false** — допустимо (`<=8`).

Тот же самый `p3Scope` = `autotunevnext.DefaultScopeResolver.ResolveServiceScope`, что и в Phase 3B. `/admit` endpoint намеренно **не** использовался: он требует browser discovery token, то есть ровно тот gate, который в Phase 3B не проходил. Owner decision 2 авторизует media host от независимого non-SABR клиента, поэтому переиспользован только шаг разрешения, а не цепочка browser provenance.

### Phase 8–13 — media A/B/A на фактической доставке байтов

Один и тот же подписанный media request, одна и та же logical format, четыре arms. Ни один payload не сохранён — байты считались и сразу сбрасывались в `io.Discard`:

| Arm | Граф | BYTES | STATUS | ELAPSED_MS |
|---|---|---|---|---|
| `A1` | нет стратегии | **0** | `NETWORK_ERROR` / `TIMEOUT` | 10013 |
| `B_ENTRY_ONLY` | только ENTRY host | **0** | `NETWORK_ERROR` / `TIMEOUT` | 10017 |
| `B_MEDIA_BOUND` | + точный media host | **65536** | `HTTP2XX`, HTTP **206**, `ACCEPTS_RANGES=true`, `DELIVERED` | **57** |
| `A2` | стратегия снята | **0** | `NETWORK_ERROR` / `TIMEOUT` | 10013 |

Это `DIRECT FAIL / ACTIVE PASS / DIRECT FAIL` на **реальной доставке медиа**, а не на TLS/TCP-probe: HTTP 206, ненулевой byte range, чистое завершение. Критерий Phase 13 выполнен — media delivery, а не connect-only.

### Phase 14–16 — ENTRY против MEDIA

Ключевой отрицательный результат: **`B_ENTRY_ONLY` дал 0 байт**. Та же самая стратегия `prod-tls-hostfakesplit-v1`, связанная только с `www.youtube.com`, **не переносит** доставку медиа. Помогает только связывание с точным media host.

`STRATEGY_RELATION = PER_HOST_STRATEGY`, `VERIFIED_FIXED = true`, `MEDIA_TEMPLATE = prod-tls-hostfakesplit-v1`.

Template ID тот же, но семантика привязки разная, поэтому сравнение идёт по поведению, а не по canonical fingerprint: hostname selector входит в fingerprint и сделал бы сравнение бессмысленным. `render()` из Phase 3B дополнительно проверяет, что перепривязанный к media host шаблон сохраняет идентичные `Operations`, `Safety` и `Range` — то есть это буквально тот же стратег, а не перенабранный argv.

### Phase 17–18 — повторы и класс host

Пять успешных A/B/A прогонов: три на video A host, два на video B host. Во всех `B_MEDIA_BYTES=65536`, `B_ENTRY_BYTES=0`, `A1_BYTES=0`, `A2_BYTES=0`. Оба media host принадлежат одному структурному классу (`rr{n}---sn-jvhnu5g-n8v*.googlevideo.com`), но являются разными edges с разными product scope.

`DYNAMIC_MEDIA_HOST_CLASS_CAUSAL = YES`: причинность воспроизводится на структурном классе dynamic media host, а не на одном конкретном имени.

Один прогон из шести **не дошёл до media и fail-closed на ENTRY authority**: product vNext вернул `COMPLETED_NO_VERIFIED_CANDIDATE` (0 verified edges), поэтому `equality=FAIL`, `admitted=false`, `state_restored=true`. Это отказ guard'а, а не media-провал; он показывает, что ENTRY authority admission не является 100% воспроизводимым и сам по себе требует внимания.

### Privacy

Независимый аудит Phase 3B (`audit-evidence.mjs`) по всем 10 файлам Phase 4 evidence: **PASS**, ни одного URL, query, адреса или запрещённого поля. Дополнительно сам emitter имеет allowlist и второй независимый контроль по сериализованным байтам (`://`, `?`, `googlevideo.com/`); отклонение любой записи — `panic`, то есть молчаливая потеря evidence невозможна. Allowlist Phase 3B не расширялся и не ослаблялся — для Phase 4 написан отдельный emitter с не меньшей строгостью.

### Что НЕ выполнялось

Browser graph causal run (Phase 20–25) **не выполнялся**. Причина установлена и не является допуском: Phase 3B измерил, что web/SABR player не отдаёт `/youtubei/v1/player` XHR и рекламирует только server-described форматы без `url`/`signatureCipher`, поэтому browser-side addressable media provenance по-прежнему недоступен. Media-стратегия доказана независимо, но связать её с host'ом конкретной browser-сессии через SABR-путь нельзя, не ослабляя authority. Это ровно случай `MEDIA_STRATEGY_PROVEN_BUT_WEB_DISCOVERY_AUTHORITY_UNRESOLVED` из Phase 24, и он остаётся открытым следующей миссией.

### Архитектурный вывод

`ARCHITECTURE_RESULT = PER_HOST_STRATEGY_GRAPH_REQUIRED`, при этом `DYNAMIC_MEDIA_HOST_CLASS_CAUSAL = YES`.

Продуктовая стратегия, доказанная для ENTRY, **не является** стратегией для сервиса: медиа требует per-host привязки. Это меняет требование к графу — сервисный граф обязан нести отдельный media node на каждый текущий session media host, а не один ENTRY node на `www.youtube.com`. Второй практический вывод: текущий ENTRY scope недостаточен даже для non-SABR извлечения, поскольку не покрывает `youtubei.googleapis.com`.

`FULL_YOUTUBE_SERVICE_EFFECTIVENESS` остаётся `NOT_ESTABLISHED`: доказана доставка отдельного media request по causal A/B/A, но не воспроизведён полный browser playback graph.

