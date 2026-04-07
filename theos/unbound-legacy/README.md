# Unbound Legacy

> Обход DPI/цензуры для джейлбрейкнутых iOS-устройств.
> Поддержка **iOS 6.1.3** (iPhone 4s, скевоморфный UI) и **iOS 15+** (ARM64, современный плоский UI) в одном DEB-пакете.

## Архитектура

```
┌──────────────────────────────────────────────────┐
│              Unbound Legacy                       │
├────────────────┬─────────────────────────────────┤
│  Приложение    │ Скевоморфный (iOS 6) / Современный│
│                │ Objective-C, CoreGraphics, UIKit│
├────────────────┼─────────────────────────────────┤
│  Менеджер прокси│ API SCPreferences — внедрение   │
│                │ SOCKS-прокси, без перезагрузки  │
├────────────────┼─────────────────────────────────┤
│  Твик          │ Хуки Cydia Substrate / ElleKit  │
│                │ SpringBoard + Настройки         │
├────────────────┼─────────────────────────────────┤
│  C Двигатель   │ Портировано из bol-van/zapret   │
│  (tpws)        │ epoll→kqueue shim, ARMv7+ARM64  │
├────────────────┼─────────────────────────────────┤
│  Launch Daemon │ com.unbound.tpws.plist           │
│                │ Автозапуск, KeepAlive            │
└────────────────┴─────────────────────────────────┘
```

## Структура проекта

```
theos/unbound-legacy/
├── Makefile                          # Основная сборка Theos (двойная архитектура)
├── control                           # Метаданные DEB
├── README.md                         # Этот файл
│
├── engine/
│   ├── Makefile.tpws                 # Кросс-компиляция tpws
│   └── tpws/
│       ├── darwin_compat.h           # epoll→kqueue, signalfd, timerfd shim
│       ├── tpws.h                    # Публичный API tpws
│       ├── ios_main.c                # Точка входа демона iOS
│       ├── Entitlements.xml          # ldid entitlements для бинарника
│       ├── epoll-shim/
│       │   ├── include/sys/epoll.h   # Объявления API epoll
│       │   └── src/epoll_shim.c      # Полная реализация на базе kqueue
│       └── macos/
│           ├── net/pfvar.h           # Структуры PF NAT
│           └── sys/socket.h          # Опции сокетов Darwin
│
├── unboundApp/                       # Приложение
│   ├── UnboundAppDelegate.h/m        # Маршрутизация UI по версии
│   ├── UnboundProxyManager.h/m       # Общий менеджер прокси/демона
│   ├── iOS6/                         # Скевоморфный UI
│   │   ├── UnboundSkeuomorphicViewController.h/m
│   │   ├── UnboundLinenBackgroundView.h/m
│   │   ├── UnboundLeatherPanelView.h/m
│   │   ├── UnboundGlossyButton.h/m
│   │   └── UnboundSkeuomorphicSwitch.h/m
│   └── Modern/                       # Современный UI iOS 15+
│       └── UnboundModernViewController.h/m
│
├── unboundTweak/
│   ├── Tweak.xm                      # Logos-твик (SpringBoard + Настройки)
│   └── UnboundProxyManager.m         # Менеджер прокси твика (non-ARC)
│
├── layout/                           # Файлы, устанавливаемые на устройство
│   ├── DEBIAN/
│   │   ├── postinst                  # Пост-установка (правила PF, права)
│   │   └── prerm                     # Предварительная очистка
│   ├── Library/LaunchDaemons/
│   │   └── com.unbound.tpws.plist
│   └── Applications/Unbound.app/
│       ├── Info.plist
│       └── Entitlements.plist
│
└── scripts/
    ├── build.sh                      # Bash-сборка (Linux/macOS/WSL)
    └── build.ps1                     # PowerShell-сборка (Windows+WSL)
```

## Сборка

### Требования

- **Theos** установлен (https://theos.dev)
- **Xcode** или **Clang** (для кросс-компиляции)
- **ldid** (для подписи entitlements)
- **dpkg-deb** (для упаковки DEB)

### Сборка через скрипт

```bash
# Linux/macOS/WSL
cd theos/unbound-legacy
./scripts/build.sh

# Windows (через WSL)
.\scripts\build.ps1
```

### Ручная сборка

```bash
cd theos/unbound-legacy

# Собрать tpws
make -C engine -f Makefile.tpws

# Собрать DEB-пакет
make package
```

Результат: `packages/com.unbound.legacy_2.0.0_iphoneos-arm.deb`

## Установка

```bash
# Передать DEB на устройство
scp packages/com.unbound.legacy_2.0.0_iphoneos-arm.deb root@<IP-УСТРОЙСТВА>:/var/root/

# Установить через SSH
ssh root@<IP-УСТРОЙСТВА>
dpkg -i /var/root/com.unbound.legacy_2.0.0_iphoneos-arm.deb
uicache
```

Или установите через Cydia/Sileo как локальный DEB-пакет.

## Использование

### iOS 6.1.3 (iPhone 4s)

После установки откроется приложение Unbound с классическим скевоморфным интерфейсом:

1. Нажмите **«Подключить»** для запуска tpws
2. Выберите профиль (По умолчанию / Агрессивный / Лёгкий)
3. Приложение внедряет SOCKS-прокси через `SCPreferences`
4. Весь HTTP/HTTPS-трафик проходит через tpws

### iOS 15+

На современных устройствах открывается минималистичный плоский интерфейс:

1. Переключатель в шапке — подключить/отключить
2. Кнопка «Настройки» — выбор профиля и информации
3. Твик интегрируется в системные настройки (Настройки → Unbound)

### Твик

Твик добавляет:
- Индикатор статуса в строке состояния SpringBoard
- Быстрый переключатель в Центре управления
- Страницу настроек в приложении «Настройки»

## Архитектура двигателя

### tpws (Transparent Proxy Web Server)

Портированный двигатель из проекта zapret:

- **Перехват трафика**: Захватывает исходящие соединения на указанных портах
- **Манипуляция пакетами**: Применяет техники обхода DPI (split, fake, disorder)
- **kqueue**: Эмуляция epoll через kqueue для Darwin-систем
- **Демон**: Работает как launchd-демон с автозапуском

### Внедрение прокси

Приложение использует `SCPreferences` API для установки системного SOCKS-прокси:

1. Приложение создаёт SOCKS-прокси на `127.0.0.1:1080`
2. `SCPreferences` записывает настройки в системную конфигурацию
3. Все приложения, поддерживающие системный прокси, используют его
4. При отключении настройки восстанавливаются

## Профили

| Профиль | Аргументы tpws | Применение |
|---------|---------------|------------|
| **По умолчанию** | `--split-pos=2 --split-repeats=6` | Большинство провайдеров |
| **Агрессивный** | `--split-pos=1 --split-repeats=11 --fake-ttl=1` | Упрямые DPI |
| **Лёгкий** | `--split-pos=2 --split-repeats=3` | Лёгкая цензура |

## Диагностика

### tpws не запускается

```bash
# Проверить статус демона
launchctl list | grep unbound

# Проверить логи
cat /var/log/syslog | grep tpws

# Перезапустить вручную
launchctl stop com.unbound.tpws
launchctl start com.unbound.tpws
```

### Прокси не отключается после удаления

```bash
# Сбросить настройки прокси вручную
/usr/libexec/PlistBuddy -c "Delete :HTTPProxy" /Library/Preferences/SystemConfiguration/preferences.plist
```

## Лицензия

MIT
