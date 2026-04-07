# Модуль Magisk — Unbound Core

Системный модуль обхода DPI/цензуры для Android-устройств с root-доступом через Magisk или KernelSU.

## Обзор

Модуль `Unbound Core` устанавливает системный двигатель обхода DPI на основе `nfqws` из проекта zapret. В отличие от APK-версии, которая использует Android VpnService, этот модуль работает на уровне системы и:

- ✅ Невидим для банковских приложений и игр
- ✅ Обрабатывает весь трафик устройства
- ✅ Поддерживает раздачу Wi-Fi (hotspot) — подключённые устройства тоже защищены
- ✅ Фильтрация по UID — исключение отдельных приложений через iptables `owner`
- ✅ Поддержка IPv4 и IPv6
- ✅ iptables и nftables

## Архитектура

```
┌─────────────────────────────────────────────────┐
│            Linux Kernel / Netfilter              │
│                                                  │
│  ┌──────────┐   ┌───────────┐   ┌────────────┐  │
│  │ OUTPUT   │──▶│ NFQUEUE   │──▶│ nfqws      │  │
│  │ (mangle) │   │ (queue 200│   │ (обход DPI) │  │
│  └──────────┘   └───────────┘   └─────┬──────┘  │
│                                       │         │
│  ┌──────────┐   ┌───────────┐         │         │
│  │ FORWARD  │──▶│ NFQUEUE   │─────────┘         │
│  │(hotspot) │   │ (queue 200│                   │
│  └──────────┘   └───────────┘                   │
└─────────────────────────────────────────────────┘
       │                              │
       ▼                              ▼
 Локальные приложения          Устройства через
 (фильтр по UID)               Wi-Fi Hotspot
```

## Структура модуля

```
magisk-module/
├── module.prop              # Метаданные модуля
├── customize.sh             # Скрипт настройки при установке
├── service.sh               # Главный сервис
├── uninstall.sh             # Очистка при удалении
├── post-fs-data.sh          # Выполняется после монтирования данных
├── config/
│   └── unbound.conf.default # Конфигурация по умолчанию
├── scripts/
│   ├── iptables_setup.sh    # Настройка правил iptables/nftables
│   └── iptables_cleanup.sh  # Очистка правил
└── binaries/
    ├── arm64/nfqws          # Бинарник для ARM64
    ├── arm/nfqws            # Бинарник для ARM
    ├── x86_64/nfqws         # Бинарник для x86_64
    └── x86/nfqws            # Бинарник для x86
```

## Установка

### Через Magisk Manager

1. Скачайте ZIP-файл модуля со страницы [Releases](https://github.com/your-org/unbound/releases/latest)
2. Откройте Magisk Manager → Модули
3. Нажмите «Установить из хранилища»
4. Выберите ZIP-файл модуля
5. Перезагрузите устройство

### Через ADB

```bash
# Передать ZIP на устройство
adb push UnboundCore-v2.0.0.zip /sdcard/Download/

# Установить через Magisk
adb shell su -c "magisk --install-module /sdcard/Download/UnboundCore-v2.0.0.zip"

# Перезагрузить
adb reboot
```

## Настройка

### Конфигурационный файл

`/data/adb/modules/unbound-core/etc/unbound.conf`:

```bash
# Открыть для редактирования
adb shell su -c "nano /data/adb/modules/unbound-core/etc/unbound.conf"
```

| Параметр | Описание | По умолчанию |
|----------|----------|-------------|
| `nfqueue_num` | Номер NFQUEUE | 200 |
| `iptables_mode` | `iptables` или `nftables` | iptables |
| `filter_ports` | Порты для обработки | 80,443 |
| `enable_hotspot` | Проброс трафика хотспота | true |
| `excluded_uids` | UID приложений для исключения | (пусто) |
| `debug_mode` | Подробный лог | false |

### Поиск UID приложения

```bash
adb shell su -c "ls -la /data/data/ | grep com.google.chrome"
# u0_a85 → UID = 10085
```

### Перезапуск после изменений

```bash
adb shell su -c "/data/adb/modules/unbound-core/service.sh restart"
```

## Управление

```bash
# Запуск
adb shell su -c "/data/adb/modules/unbound-core/service.sh start"

# Остановка
adb shell su -c "/data/adb/modules/unbound-core/service.sh stop"

# Статус
adb shell su -c "/data/adb/modules/unbound-core/service.sh status"
# Ожидается: ACTIVE (PID: XXXX)
```

## Сборка nfqws

### Шаг 1: Компиляция из Zapret

```bash
git clone https://github.com/bol-van/zapret.git
cd zapret

export NDK=/path/to/android-ndk-r26
export TOOLCHAIN=$NDK/toolchains/llvm/prebuilt/linux-x86_64

# arm64-v8a
export CC=$TOOLCHAIN/bin/aarch64-linux-android29-clang
make nfqws

# armeabi-v7a
export CC=$TOOLCHAIN/bin/armv7a-linux-androideabi29-clang
make nfqws
```

### Шаг 2: Размещение бинарников

```bash
mkdir -p ../magisk-module/binaries/{arm64,arm,x86_64,x86}
cp zapret/nfqws ../magisk-module/binaries/arm64/nfqws
# ... повторите для каждой архитектуры
```

### Шаг 3: Упаковка ZIP

```bash
cd ../magisk-module/
zip -r ../UnboundCore-v2.0.0.zip \
    module.prop customize.sh service.sh uninstall.sh post-fs-data.sh \
    config/ scripts/ binaries/ -x "*.gitkeep"
```

## Диагностика

```bash
# Проверка nfqws
adb shell su -c "ps -A | grep nfqws"

# Правила iptables
adb shell su -c "iptables -t mangle -L -n -v"

# Логи nfqws
adb shell su -c "cat /data/adb/modules/unbound-core/log/nfqws.log"

# Проверка NFQUEUE
adb shell su -c "lsmod | grep nfnetlink_queue"

# Загрузка модуля ядра
adb shell su -c "modprobe nfnetlink_queue"
```

## Интеграция с приложением Unbound Android

Модуль работает независимо от APK. Приложение Unbound может:

- Отображать статус модуля
- Отправлять команды через broadcast
- Управлять настройками модуля

## Требования

- **Magisk 24+** или **KernelSU**
- **Android 10+** (API 29)
- **Root-доступ**
- **NDK r26+** (для сборки nfqws)

## Лицензия

GPL-3.0
