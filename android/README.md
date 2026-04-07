# Unbound Android — DPI Bypass для Android

Дуальная экосистема для обхода DPI и цензуры на Android. Состоит из двух компонентов:

1. **Unbound Mobile (APK)** — Без root-прав, использует Android `VpnService` + локальный SOCKS5-прокси.
2. **Unbound Core (Magisk/KernelSU Module)** — Системный модуль на root-устройствах через `nfqws` + `iptables`/`nftables`.

---

## 📋 Оглавление

- [Архитектура](#архитектура)
- [Возможности](#возможности)
- [Структура проекта](#структура-проекта)
- [Требования](#требования)
- [Сборка APK](#сборка-apk)
- [Сборка Magisk-модуля](#сборка-magisk-модуля)
- [Установка](#установка)
- [Конфигурация](#конфигурация)
- [Использование](#использование)
- [Система тем](#система-тем)
- [Диагностика](#диагностика)
- [Лицензия](#лицензия)

---

## Архитектура

### Unbound Mobile (без root)

```
┌─────────────────────────────────────────┐
│          Jetpack Compose UI              │
│  ┌────────┐ ┌────────┐ ┌──────────────┐ │
│  │  Home  │ │Settings│ │Split Tunnel  │ │
│  └────────┘ └────────┘ └──────────────┘ │
└─────────────────┬───────────────────────┘
                  ▼
     ┌──────────────────────────┐
     │   UnboundVpnService      │
     │   (Android VpnService)   │
     │                          │
     │  ┌────────────────────┐  │
     │  │  TUN Interface     │  │
     │  │  10.0.0.2/24       │  │
     │  └─────────┬──────────┘  │
     │            ▼             │
     │  ┌────────────────────┐  │
     │  │  Packet Forwarding │  │
     │  │  (TUN ↔ SOCKS5)    │  │
     │  └─────────┬──────────┘  │
     │            ▼             │
     │  ┌────────────────────┐  │
     │  │  SOCKS5 Proxy      │  │
     │  │  127.0.0.1:1080    │  │
     │  │  (ByeDPI / Go)     │  │
     │  └────────────────────┘  │
     └──────────────────────────┘
                  │
                  ▼
        Сетевой стек → Интернет
```

### Unbound Core (Magisk/KernelSU)

```
┌─────────────────────────────────────────────────┐
│            Linux Kernel / Netfilter              │
│                                                  │
│  ┌──────────┐   ┌───────────┐   ┌────────────┐  │
│  │ OUTPUT   │──▶│ NFQUEUE   │──▶│ nfqws      │  │
│  │ (mangle) │   │ (queue 200│   │ (DPI bypass│  │
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

---

## Возможности

### Unbound Mobile
- ✅ Без root-прав — стандартный Android VpnService
- ✅ Раздельное туннелирование — включение/исключение приложений
- ✅ Умный автозапуск:
  - При загрузке устройства (`BOOT_COMPLETED`)
  - При подключении к указанным Wi-Fi (`NetworkCallback`)
  - При запуске определённых приложений (`UsageStatsManager`)
- ✅ 3 темы: Doodle Jump, Modern Dark, Modern Light
- ✅ Полный русский язык
- ✅ Управление Magisk-модулем через broadcast

### Unbound Core
- ✅ Системный обход — весь трафик без определения VPN
- ✅ Невидим для банковских приложений и игр
- ✅ Hotspot — устройства через точку доступа тоже защищены
- ✅ Фильтрация по UID — исключение приложений через iptables `owner`
- ✅ IPv6 + IPv4
- ✅ iptables и nftables

---

## Структура проекта

```
android/
├── app/
│   ├── src/main/
│   │   ├── java/ru/unbound/app/
│   │   │   ├── UnboundApplication.kt
│   │   │   ├── MainActivity.kt
│   │   │   ├── data/
│   │   │   │   ├── SettingsManager.kt       # DataStore preferences
│   │   │   │   └── AppDataManager.kt         # Списки приложений/SSID
│   │   │   ├── ui/
│   │   │   │   ├── screens/
│   │   │   │   │   ├── MainScreen.kt         # Навигация
│   │   │   │   │   ├── HomeScreen.kt         # Подключение VPN
│   │   │   │   │   ├── SettingsScreen.kt
│   │   │   │   │   ├── SplitTunnelingScreen.kt
│   │   │   │   │   └── AutostartScreen.kt
│   │   │   │   └── theme/
│   │   │   │       ├── Color.kt              # 3 палитры
│   │   │   │       ├── Typography.kt
│   │   │   │       └── Theme.kt
│   │   │   ├── vpn/
│   │   │   │   └── UnboundVpnService.kt      # VpnService
│   │   │   ├── autostart/
│   │   │   │   ├── BootReceiver.kt
│   │   │   │   ├── WifiStateReceiver.kt
│   │   │   │   └── ForegroundAppMonitor.kt
│   │   │   └── root/
│   │   │       └── MagiskModuleReceiver.kt
│   │   ├── res/values/strings.xml
│   │   ├── res/xml/
│   │   └── AndroidManifest.xml
│   └── build.gradle.kts
├── build.gradle.kts
├── settings.gradle.kts
└── gradle.properties

../magisk-module/
├── module.prop
├── customize.sh
├── service.sh
├── uninstall.sh
├── post-fs-data.sh
├── config/unbound.conf.default
├── scripts/
│   ├── iptables_setup.sh
│   └── iptables_cleanup.sh
└── binaries/
    ├── arm64/nfqws
    ├── arm/nfqws
    ├── x86_64/nfqws
    └── x86/nfqws
```

---

## Требования

### Для сборки APK
- **JDK 17+** (OpenJDK, Adoptium)
- **Android SDK** (API 35, compile SDK 35)
- **Android Studio** (Iguana 2024+)
- **Android NDK** (если собираете нативный прокси)
- **Gradle 8.7+**

### Для сборки Magisk-модуля
- **Linux** (WSL2 на Windows подходит)
- **Android NDK** (кросс-компиляция nfqws)
- **Исходники Zapret**: https://github.com/bol-van/zapret
- Утилита **zip**

### Устройство
- **Android 10+** (API 29)
- **Root** (Magisk 24+ / KernelSU) — только для Unbound Core
- **Разрешение на местоположение** — для чтения Wi-Fi SSID (Android 8.1+)

---

## Сборка APK

### Способ 1: Android Studio

1. **Откройте проект:**
   ```
   File → Open → выберите папку `android/`
   ```

2. **Синхронизируйте Gradle** — нажмите "Sync Now".

3. **Соберите APK:**
   ```
   Build → Build Bundle(s) / APK(s) → Build APK(s)
   ```
   - Debug: `app/build/outputs/apk/debug/app-debug.apk`
   - Release: `./gradlew assembleRelease` (нужна подпись)

### Способ 2: Командная строка

```bash
cd android/

# Windows
gradlew.bat assembleDebug

# Linux/macOS
./gradlew assembleDebug
```

Результат: `app/build/outputs/apk/debug/app-debug.apk`

### Нативный прокси (опционально)

Если хотите встроить бинарный DPI-прокси:

1. **Кросс-компиляция:**
   ```bash
   GOOS=android GOARCH=arm64 go build -o libunbound-proxy.so -buildmode=c-shared proxy.go
   ```

2. **Разместите в:**
   ```
   app/src/main/jniLibs/arm64-v8a/libunbound-proxy.so
   app/src/main/jniLibs/armeabi-v7a/libunbound-proxy.so
   app/src/main/jniLibs/x86_64/libunbound-proxy.so
   ```

3. **Раскомментируйте** код `ProcessBuilder` в `UnboundVpnService.kt` → `startLocalProxy()`.

---

## Сборка Magisk-модуля

### Шаг 1: Компиляция nfqws из Zapret

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
zip -r ../UnboundCore-v1.0.0.zip \
    module.prop customize.sh service.sh uninstall.sh post-fs-data.sh \
    config/ scripts/ binaries/ -x "*.gitkeep"
```

Результат: `UnboundCore-v1.0.0.zip`

---

## Установка

### APK

```bash
adb install app/build/outputs/apk/debug/app-debug.apk
```

Необходимые разрешения:
- VPN (запрос при первом подключении)
- Доступ к статистике использования (Настройки → Приложения → Специальный доступ)
- Местоположение (для Wi-Fi SSID)
- Уведомления (для foreground-сервиса)

### Magisk-модуль

```bash
adb push UnboundCore-v1.0.0.zip /sdcard/Download/
```

1. Откройте Magisk → Модули → Установка из хранилища
2. Выберите ZIP → Перезагрузите устройство

Проверка:
```bash
adb shell su -c "/data/adb/modules/unbound-core/service.sh status"
# Ожидается: ACTIVE (PID: XXXX)
```

---

## Конфигурация

### Настройки APK

| Параметр | Описание | По умолчанию |
|----------|----------|-------------|
| Тема | Doodle Jump / Modern Dark / Modern Light | Modern Dark |
| Хост прокси | Адрес локального прокси | 127.0.0.1 |
| Порт прокси | Порт локального прокси | 1080 |
| DNS | Пользовательский DNS | Авто |
| Root-модуль | Интеграция с Magisk | Отключено |

### Конфиг Magisk-модуля

`/data/adb/modules/unbound-core/etc/unbound.conf`:

```bash
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

**Поиск UID приложения:**
```bash
adb shell su -c "ls -la /data/data/ | grep com.google.chrome"
# u0_a85 → UID = 10085
```

Перезапуск после изменения:
```bash
adb shell su -c "/data/adb/modules/unbound-core/service.sh restart"
```

---

## Использование

### Без root (APK)

1. Откройте приложение → **Подключить**
2. Разрешите VPN
3. Весь трафик идёт через локальный DPI-bypass прокси

**Раздельное туннелирование:** Выберите режим — все приложения / исключить выбранные / только выбранные.

**Автозапуск:** Включите триггеры — загрузка, Wi-Fi SSID, запуск приложений.

### С root (Magisk-модуль)

Модуль работает независимо от APK:

```bash
# Запуск
adb shell su -c "/data/adb/modules/unbound-core/service.sh start"

# Остановка
adb shell su -c "/data/adb/modules/unbound-core/service.sh stop"

# Статус
adb shell su -c "/data/adb/modules/unbound-core/service.sh status"
```

APK отображает статус модуля и может отправлять команды через broadcast.

---

## Система тем

### Встроенные темы

| Тема | Стиль | Цвета |
|------|-------|-------|
| **Doodle Jump** | Игровой, ностальгический | Кремовый фон, зелёный, жёлтый |
| **Modern Dark** | AMOLED, ночной | Чёрный #000000, синий, фиолетовый |
| **Modern Light** | Корпоративный, дневной | Белый/серый, Google Blue |

### Добавление своей темы

1. Определите палитру в `Color.kt`:
   ```kotlin
   object MyCustomPalette {
       val Background = Color(0xFF...)
       val Primary = Color(0xFF...)
       // ...
   }
   ```

2. Добавьте фабрику в `UnboundColors`:
   ```kotlin
   fun myCustom() = UnboundColors(
       background = MyCustomPalette.Background,
       primary = MyCustomPalette.Primary,
       // ...
   )
   ```

3. Добавьте значение в `enum class AppTheme`.

4. Обновите `when (theme)` в `Theme.kt`.

---

## Диагностика

### APK

```bash
# Логи VPN-сервиса
adb logcat | grep UnboundVpnService

# Проверка работы сервиса
adb shell dumpsys activity services ru.unbound.app
```

### Magisk-модуль

```bash
# Проверка nfqws
adb shell su -c "ps -A | grep nfqws"

# Правила iptables
adb shell su -c "iptables -t mangle -L -n -v"

# Логи nfqws
adb shell su -c "cat /data/adb/modules/unbound-core/log/nfqws.log"

# Проверка NFQUEUE
adb shell su -c "lsmod | grep nfnetlink_queue"

# Загрузка модуля
adb shell su -c "modprobe nfnetlink_queue"
```

### Тест обхода DPI

```bash
adb shell curl -I https://blocked-site.example.com
```

---

## Лицензия

**GNU General Public License v3.0 (GPL-3.0)**

---

## Благодарности

- **GoodbyeDPI / ByeDPI** — Вдохновение для техник обхода DPI
- **Zapret (bol-van)** — nfqws и интеграция с iptables
- **hev-socks5-tunnel** — Мост TUN ↔ SOCKS5
- **Jetpack Compose** — Современный UI-тулкит Android
- **Magisk & KernelSU** — Решения для root-управления
