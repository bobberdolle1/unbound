# YouTube: исследование bounded service graph, сентябрь 2026

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
