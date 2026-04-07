# Unbound для WebOS (LG Smart TV)

Двигатель обхода DPI/цензуры для рутированных LG WebOS TV через платформу webosbrew.

## Архитектура

```
Приложение Unbound WebOS (Enact/React)
    │
    ├── UnboundPanel.js            # Главный UI с навигацией D-pad
    └── services/UnboundService.js # Клиент сервиса Luna
         │
         ▼
Сервис выполнения root webosbrew
    (org.webosbrew.hbchannel.service)
         │
         ▼
Скрипты сервисного сервиса
    ├── unbound-service.sh         # Демон управления двигателем
    └── unbound-init.sh            # Инициализация при загрузке
         │
         ▼
Двигатель nfqws (бинарник C)
    (Кросс-компилирован из bol-van/zapret)
         │
         ▼
Правила iptables NFQUEUE
    (Направляет трафик YouTube через обход DPI)
```

## Как это работает

1. **Пользователь нажимает ПОДКЛЮЧИТЬ** в UI Enact (навигация через D-pad пульта)
2. **UnboundService.js** вызывает сервис выполнения root webosbrew
3. **Root-сервис** запускает бинарник `nfqws` с аргументами профиля
4. **Правила iptables** перенаправляют трафик порта 443 в NFQUEUE 200
5. **nfqws** перехватывает пакеты и применяет техники обхода DPI
6. **Изменённые пакеты** обходят DPI-инспекцию, разблокируя YouTube и другие сервисы

## Требования

- **Рутированный LG WebOS TV** (через RootMyTV или аналогичный эксплойт)
- **webosbrew Homebrew Channel** установлен
- **Доступ по SSH** к ТВ (для развёртывания и отладки)
- **WebOS NDK** установлен в `/opt/webos-sdk-x86_64` (для сборки nfqws)
- **Node.js 18+** и npm (для сборки фронтенда Enact)

## Сборка

### Шаг 1: Сборка двигателя nfqws (только Linux/macOS)

Бинарник nfqws необходимо кросс-компилировать для WebOS ARM через WebOS NDK:

```bash
cd webos/native/nfqws

# Вариант А: Через Make (требует WebOS NDK)
make WEBOS_SDK_PATH=/opt/webos-sdk-x86_64 package

# Вариант Б: Через CMake
mkdir build && cd build
cmake -DCMAKE_TOOLCHAIN_FILE=/opt/webos-sdk-x86_64/1.0.g/sysroots/x86_64-webossdk-linux/usr/share/cmake/OEToolchainConfig.cmake ..
make
```

**Зависимости** (должны быть кросс-компилированы заранее):
- `libnetfilter_queue`
- `libnfnetlink`
- `libmnl`

Makefile автоматически скачает и соберёт эти зависимости.

### Шаг 2: Сборка фронтенда Enact

```bash
cd webos
npm install
npm run build
```

Результат — упакованное приложение в директории `dist/`.

### Шаг 3: Упаковка для webosbrew

```bash
# Установить CLI-инструменты webOS
npm install -g @webosose/ares-cli

# Упаковать приложение
ares-package ./dist com.unbound.app

# Результат: com.unbound.app_2.0.0_all.ipk
```

## Установка на ТВ

### Через SSH/SCP

```bash
# Передать IPK на ТВ
scp com.unbound.app_2.0.0_all.ipk root@<IP_ТВ>:/media/developer/

# Подключиться по SSH
ssh root@<IP_ТВ>

# Установить приложение
luna-send-pub -n 1 luna://com.webos.appInstallService/dev/install '{"id":"com.unbound.app","ipkUrl":"/media/developer/com.unbound.app_2.0.0_all.ipk"}'

# Проверить установку
ls -la /media/developer/apps/usr/palm/applications/com.unbound.app/
```

### Через webosbrew Homebrew Channel

Если установлен Homebrew Channel, можно устанавливать приложения через его интерфейс.

## Настройка автозагрузки

Для запуска Unbound при включении ТВ:

```bash
# Подключиться по SSH
ssh root@<IP_ТВ>

# Скопировать init-скрипт в init.d webosbrew
cp services/unbound-init.sh /var/lib/webosbrew/init.d/unbound
chmod +x /var/lib/webosbrew/init.d/unbound

# Скопировать скрипт сервиса
cp services/unbound-service.sh /media/developer/apps/usr/palm/applications/com.unbound.app/services/
chmod +x /media/developer/apps/usr/palm/applications/com.unbound.app/services/unbound-service.sh
```

**Важно**: Отключите «Быстрый запуск» в настройках ТВ для надёжного выполнения init.d-скриптов.

## Использование

### Навигация по UI

Интерфейс полностью управляется через D-pad пульта ТВ:

1. Кнопка **ПОДКЛЮЧИТЬ/ОТКЛЮЧИТЬ** — главный переключатель (автофокус)
2. **Кнопки профилей** — выбор стратегии обхода
3. Кнопка **Настройки** — информация о двигателе

### Профили

| Профиль | Аргументы nfqws | Применение |
|---------|----------------|------------|
| **По умолчанию** | `--dpi-desync=split --dpi-desync-pos=2 --dpi-desync-repeats=6` | Большинство провайдеров |
| **Агрессивный** | `--dpi-desync=fake,split --dpi-desync-pos=1,midsld --dpi-desync-repeats=11 --fake-ttl=1` | Упрямые DPI |
| **Лёгкий** | `--dpi-desync=split --dpi-desync-pos=2 --dpi-desync-repeats=3` | Лёгкая цензура |

### Списки доменов

Отредактируйте список YouTube для добавления/удаления доменов:
```
webos/native/nfqws/lists/youtube.txt
```

После редактирования пересоберите и переустановите приложение.

## Диагностика

### Приложение не запускается

- Убедитесь, что режим разработчика webOS включён
- Проверьте корректность установки приложения:
  ```bash
  luna-send-pub -n 1 luna://com.webos.applicationManager/dev/listApps
  ```

### nfqws не запускается

- Проверьте работу root-доступа:
  ```bash
  luna-send-pub -n 1 luna://org.webosbrew.hbchannel.service/exec '{"command":"whoami"}'
  ```
  Должно вернуть «root»

- Проверьте наличие и исполняемость бинарника nfqws:
  ```bash
  ls -la /media/developer/apps/usr/palm/applications/com.unbound.app/bin/nfqws
  ```

### Правила iptables не применены

- Проверьте статус iptables:
  ```bash
  iptables -L -n -v
  ```
- Ищите UNBOUND_CHAIN в выводе

### YouTube всё ещё заблокирован

- Попробуйте профиль «Агрессивный»
- Проверьте логи nfqws:
  ```bash
  tail -f /var/log/messages | grep nfqws
  ```
- Убедитесь, что путь к файлу hostlist корректен

## Ограничения платформы

### Особенности WebOS

1. **Требуется root**: В отличие от tvOS, WebOS требует рутированного ТВ для манипуляции iptables
2. **Выполнение на ранней загрузке**: init.d-скрипты запускаются до готовности сети; скрипт ждёт до 30 сек
3. **Помехи быстрого запуска**: Функция «Быстрый запуск» ТВ может пропускать выполнение init.d
4. **Обновления системы**: Могут сломать root-доступ; перерутируйте после крупных обновлений

### Выбор двигателя: nfqws или tpws

WebOS использует **nfqws** (очередь netfilter) вместо tpws, потому что:
- WebOS работает на Linux с полной поддержкой iptables
- Root-доступ позволяет манипуляцию NFQUEUE
- Эффективнее режима SOCKS-прокси (прозрачный перехват)
- Меньший расход памяти (важно для железа ТВ)

## Лицензия

GPL-3.0
