# 📱 Поддерживаемые платформы UNBOUND v2.0.0

## Обзор экосистемы

Unbound v2.0 — это **мультиплатформенная экосистема** для обхода DPI-блокировок. Каждая платформа реализована как отдельный модуль с нативными инструментами, но использует единый движок обхода.

| Платформа | Статус | Формат | Документация |
|-----------|--------|--------|--------------|
| 🪟 **Windows** | ✅ Продукт | `.exe` (Wails GUI + установщик) | [Ниже](#windows) |
| 🍎 **macOS** | ✅ Продукт | `.app` (Universal: Intel + Apple Silicon) | [Ниже](#macos) |
| 🐧 **Linux** | ✅ Продукт | Бинарник amd64 | [Ниже](#linux) |
| 🤖 **Android** | ✅ Продукт | `.apk` (VpnService) | [android/README.md](../android/README.md) |
| 📡 **OpenWrt** | ✅ Продукт | `.ipk` пакет + LuCI GUI | [openwrt/README.md](../openwrt/README.md) |
| 🧩 **Браузер** | ✅ Продукт | Расширение Chrome/Firefox | [extension-web/README.md](../extension-web/README.md) |
| 🎮 **Steam Deck** | ✅ Продукт | Decky Loader плагин | [decky-plugin/README.md](../decky-plugin/README.md) |
| 📦 **Magisk** | ✅ Продукт | ZIP-модуль | [magisk-module/README.md](../magisk-module/README.md) |
| 📺 **LG WebOS** | ⚠️ Бета | `.ipk` домашнее приложение | [webos/README.md](../webos/README.md) |
| 🍎 **iOS (JB)** | ⚠️ Бета | `.deb` твик | [theos/README.md](../theos/README.md) |
| 📺 **tvOS** | ⚠️ Бета | `.ipa` | [tvos/README.md](../tvos/README.md) |

---

## 🪟 Windows

**Технология:** Wails (Go + WebView2) + WinDivert + zapret2 (winws2)

### Установка
1. Скачайте `.exe` со страницы [Releases](https://github.com/bobberdolle1/unbound/releases/latest)
2. Запустите **от имени администратора**
3. Следуйте установщику

### Особенности
- WinDivert драйвер требует прав администратора
- Автозапуск через Windows Task Scheduler (без UAC)
- Системный трей: сворачивание, управление из трея
- Очистка кэша Discord
- TCP Timestamps
- Автоподбор профилей (70+ пресетов)

### Сборка
```powershell
wails build -clean -o unbound.exe
```

---

## 🍎 macOS

**Технология:** Wails (Go + WebView2) + SpoofDPI (SOCKS5 прокси)

### Установка
1. Скачайте `.app` (Universal) со страницы Releases
2. Перетащите в Applications
3. При первом запуске: Системные настройки → Безопасность → Открыть

### Особенности
- SOCKS5 прокси через SpoofDPI
- Автонастройка прокси через `networksetup` (osascript / Touch ID)
- Автозапуск через `launchd` (`~/Library/LaunchAgents/`)
- Graceful Shutdown: автоотключение прокси при выходе
- Пути: `~/Library/Application Support/Unbound`

### Сборка
```bash
wails build -platform darwin/universal -clean
```

---

## 🐧 Linux

**Технология:** Go CLI + nftables/nfqws (zapret)

### Установка
```bash
curl -LO https://github.com/bobberdolle1/unbound/releases/latest/download/unbound-cli-linux-amd64
chmod +x unbound-cli-linux-amd64
sudo ./unbound-cli-linux-amd64 start
```

### Особенности
- Требует root (nftables/nfqws)
- systemd сервис для автозапуска
- Профили: multidisorder, split-tls, fake-ping, disorder+fake
- Автоподбор через CLI

### Сборка
```bash
go build -trimpath -ldflags="-s -w" -o unbound-cli-linux ./...
```

---

## 🤖 Android

**Технология:** Kotlin + VpnService (без root) / Magisk-модуль (с root)

### Установка (APK)
1. Скачайте APK со страницы Releases
2. Разрешите установку из неизвестных источников
3. Установите и предоставьте разрешение VPN

### Особенности (APK)
- Без root: работает через Android VpnService
- Раздельное туннелирование (выбор приложений)
- Нет необходимости в root

### Magisk-модуль (с root)
- Системная установка на уровне роутера
- nfqws на уровне системы
- [magisk-module/README.md](../magisk-module/README.md)

---

## 📡 OpenWrt (Роутер)

**Технология:** nfqws (zapret) + fw4/nftables + LuCI CBI

### Установка
```bash
opkg install nfqws-unbound_*.ipk
opkg install luci-app-unbound_*.ipk
/etc/init.d/unbound enable
/etc/init.d/unbound start
```

### Особенности
- Защита **всей LAN** без настройки клиентов
- LuCI GUI: **Сервисы → Unbound**
- Стратегии: multidisorder, split-tls, fake-ping
- Исключения доменов/IP через UCI
- fw4/nftables правила: перехват TCP 80/443 с br-lan в NFQUEUE 200

---

## 🧩 Браузер (Chrome / Firefox)

**Технология:** Manifest V3 + Native Messaging (Companion) / PAC-скрипты (Standalone)

### Установка (Chrome)
1. Скачайте ZIP со страницы Releases
2. Распакуйте
3. `chrome://extensions/` → Режим разработчика → Загрузить распакованное

### Режимы
- **Companion:** Взаимодействует с локальным демоном Unbound Desktop
- **Standalone Proxy:** PAC-скрипты для маршрутизации через внешний прокси

### Особенности
- Двойная тема: "Doodle Jump Minimalism" / "Modern Dark"
- Управление доменами обхода
- Фоновый Service Worker (Manifest V3)

---

## 🎮 Steam Deck

**Технология:** Decky Loader плагин + nfqws бинарник

### Установка
1. Установите [Decky Loader](https://github.com/SteamDeckHomebrew/decky-loader)
2. Скопируйте плагин в `~/.local/share/Steam/steamui/decky/plugins/`
3. Перезапустите Steam

### Особенности
- Управление из игрового режима
- Встроенный nfqws для SteamOS
- Настройки через оверлей

---

## 📦 Magisk Module (Android с root)

**Технология:** nfqws + init скрипты

### Установка
1. Скачайте ZIP
2. Magisk Manager → Модули → Установить из хранилища
3. Перезагрузите устройство

### Особенности
- Системная установка (не требует VpnService)
- nfqws на уровне системы
- Конфигурация через `/data/adb/modules/unbound/`

---

## 📺 LG WebOS (Телевизор)

**Технология:** nfqws (cross-compiled для WebOS ARM) + Enact фронтенд + webosbrew сервис

### Установка
1. Нужен rooted WebOS TV
2. Установите через ares или SSH
3. Сервис автозапуска: `/var/lib/webosbrew/init.d/`

### Особенности
- Прозрачный перехват через iptables NFQUEUE
- Управление через D-pad пульта (Spotlight)
- Профили: Default / Aggressive / Lite

---

## 🍎 iOS (Jailbreak)

**Технология:** Theos tweak + tpws движок

### Установка
1. Нужен джейлбрейк (checkra1n, palera1n)
2. Установите `.deb` через Cydia/Sileo
3. Настройки: Приложение «Настройки» → Unbound

### Особенности
- Системный уровень обхода
- Без ограничения песочницы
- Только для джейлбрейк устройств

---

## 📺 tvOS (Apple TV)

**Технология:** NEPacketTunnelProvider (NetworkExtension) + SOCKS прокси

### Установка
1. Нужен Xcode + Developer аккаунт
2. Сбилдите и подпишите `.ipa`
3. Установите через Xcode / sideload

### Особенности
- Без джейлбрейка (официальный NetworkExtension)
- Локальный SOCKS-прокси (песочница tvOS)
- SwiftUI интерфейс с Siri Remote навигацией

---

## Архитектура кроссплатформенности

```
┌─────────────────────────────────────────────┐
│              Движок обхода DPI               │
│  zapret2 (winws2)  │  SpoofDPI  │  nfqws    │
└─────────────────────────────────────────────┘
                      │
    ┌─────────────────┼─────────────────┐
    │                 │                 │
┌───▼───┐      ┌─────▼─────┐    ┌──────▼──────┐
│Windows│      │  macOS    │    │ Linux/Other │
│WinDivert│     │SpoofDPI   │    │   nfqws     │
│  GUI  │      │  SOCKS5   │    │    CLI      │
└───────┘      └───────────┘    └─────────────┘
```

Каждая платформа использует **оптимальный метод** обхода DPI:
- **Windows:** WinDivert (перехват пакетов на уровне ядра)
- **macOS:** SpoofDPI (SOCKS5 прокси)
- **Linux/OpenWrt/Android-root:** nfqws (NFQUEUE)
- **Android без root:** VpnService
- **Браузер:** PAC-скрипты / Native Messaging
- **iOS/tvOS:** NEPacketTunnelProvider

---

## Совместимость

| ОС | Мин. версия | Архитектуры |
|----|-------------|-------------|
| Windows | 10 (64-bit) | amd64 |
| macOS | 12 (Monterey) | amd64, arm64, universal |
| Linux | Ядро 4.14+ | amd64, arm64 |
| Android | 8.0 (API 26) | arm64-v8a, armeabi-v7a, x86_64 |
| OpenWrt | 21.02+ | mipsel, arm, aarch64 |
| WebOS | 3.0+ (rooted) | armv7, aarch64 |
| iOS | 14+ (JB) | arm64 |
| tvOS | 17+ | arm64 |
