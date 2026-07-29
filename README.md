<div align="center">

<img src="./build/logo.svg" alt="UNBOUND Logo" width="140" />

# UNBOUND Refresh `v0.2.0`
**Локальный настольный оркестратор обхода DPI-блокировок для Windows, Linux и macOS.**

[![Version](https://img.shields.io/badge/Status-v0.2.0-6366f1?style=for-the-badge&logo=rocket)](#)
[![Security](https://img.shields.io/badge/Security-Verified_100%25-10b981?style=for-the-badge&logo=shield)](SECURITY.md)
[![License](https://img.shields.io/badge/License-GPL--3.0-blue?style=for-the-badge)](LICENSE)
[![Language](https://img.shields.io/badge/Language-Russian_🇷🇺-10b981?style=for-the-badge)](#)

[**🛡 Безопасность и Аудит**](#-безопасность-и-гарантии-доверия) &nbsp; | &nbsp; [**⚡ Архитектура**](#-как-работает-движок) &nbsp; | &nbsp; [**🖥 Платформы**](#-поддерживаемые-платформы) &nbsp; | &nbsp; [**🛠 Сборка**](#-сборка-из-исходников)

</div>

---

## 🛡 Безопасность и Гарантии Доверия

В отличие от подозрительных форков, внедряющих поддельные SSL-сертификаты (Root CA) и перехватывающих пользовательский трафик:

- 🔒 **0% Телеметрии и 0% Аналитики**: Приложение работает 100% автономно на вашем компьютере.
- 🛑 **Без MITM и без подмены Root CA**: Ваше TLS-соединение устанавливается строго напрямую с сайтом.
- 🔑 **Контроль целостности бинарников (SHA256)**: Все бинарные файлы покрыты манифестом `engine/ENGINE_ASSETS.sha256` и проверяются на лету кнопкой в UI.
- 📄 Полный технический отчёт: [**`SECURITY.md`**](SECURITY.md) и [**`docs/SECURITY_AUDIT.md`**](docs/SECURITY_AUDIT.md).

---

## 🚀 О перезагрузке UNBOUND Refresh

Проект **UNBOUND** переведён в режим перезагрузки (**Refresh**). Старые версии (v1.x - v2.5.0) содержали множество экспериментальных, сырых и нестабильных мобильных/дополнительных целей (Android VpnService, Magisk, Steam Deck, Web Extension, OpenWrt, LG WebOS, tvOS, gh-pages). 

В рамках **Unbound Refresh**:
1. **Фокус на 3 настольных платформах**: Поддерживаются и полируются исключительно **Windows 10/11**, **Linux** и **macOS**.
2. **Очистка от мусора**: Все неработающие, сырые платформенные модули и автоматические релизы удалены из основной ветки.
3. **Архивация легаси**: Старый код версии 2.5.0 со всеми сторонними модулями сохранён в отдельной ветке `legacy/v2.x`.
4. **Сброс версионирования**: Проект переведён на чистую стартовую версию **`0.1.0-refresh`**. Готовые релизные бинарники не публикуются до завершения глубокой полировки desktop-версии.

---

## 💡 О проекте

**UNBOUND** — это инструмент десинхронизации сетевого трафика на уровне L3/L4. Он **не является VPN** или прокси-сервисом и не перенаправляет ваш трафик через внешние серверы.

Приложение перехватывает TCP/UDP-пакеты (TLS ClientHello, HTTP Host, QUIC) на лету до отправки в сеть и применяет стратегии сегментации, расщепления и подделки пакетов, заставляя системы Deep Packet Inspection (DPI) терять контекст сессии с **нулевой задержкой**.

---

## 🧩 Поддерживаемые платформы

| Платформа | Драйвер / Механизм | Статус |
| :--- | :--- | :---: |
| **Windows 10 / 11** | `WinDivert` (драйвер ядра WinWS2) | ✅ Поддерживается |
| **Linux (amd64 / arm64)** | `NFQUEUE` + `nftables` / `iptables` | ✅ Поддерживается |
| **macOS (Apple Silicon / Intel)** | `pf` divert socket (`nfqws`) | ✅ Поддерживается |

---

## ⚙️ Как работает движок

```mermaid
sequenceDiagram
    participant B as Браузер / Приложение
    participant U as UNBOUND Engine
    participant D as DPI Провайдера
    participant S as Целевой Сервер

    B->>U: Отправка ClientHello [domain.com]
    Note over U: Перехват TCP/UDP пакета (WinDivert / NFQUEUE / pf)
    U->>D: 1. Fake Packet (малый TTL) с мусорным SNI
    Note over D: DPI обрабатывает фейк и сбрасывает контекст
    U->>D: 2. Сегментация / Фрагментация реального пакета
    Note over D: DPI не может сопоставить сигнатуру
    D->>S: Реальные сегменты доходят до сервера
    S-->>B: Успешный TLS-Handshake 🚀
```

---

## 🖥 Использование без GUI (Консольный режим и Запускные Скрипты)

Помимо основного десктопного GUI, UNBOUND поддерживает полноценную работу в консоли и готовые скрипты запуска для **Windows**, **macOS** и **Linux**.

### 1. Интерактивный Control Center (`unbound --control`)
Встроенный консольный менеджер (работает 100% одинаково на всех трех ОС):
```bash
# Запуск главного меню управления
sudo ./unbound --control
```
**Возможности меню:**
- ⚡ **AutoTune**: Автоматический подбор и запуск лучшей стратегии.
- 🎯 **Выбор профиля**: Запуск любой поддерживаемой ОС стратегии.
- 🔍 **Диагностика (`--test`)**: Проверка доступности YouTube, Discord, Instagram, Cloudflare, Ozon.
- ⚙️ **Автозапуск**: Включение/выключение автозапуска службы (Task Scheduler / LaunchAgent / systemd).
- 📝 **Списки обхода**: Редактирование `youtube.txt`, `discord.txt`, `ipset-exclude.txt` в системном редакторе.
- 🌐 **Secure DNS**: Переключение Cloudflare DoH (1.1.1.1) с отображением текущего статуса.
- 🛡 **Конфликты**: Проверка конфликтующих процессов (GoodbyeDPI, ByeDPI, sing-box) и правил файрвола.
- 🧹 **Очистка Discord**: Исправление голосовых каналов Discord в 1 клик.
- 🛑 **Экстренный сброс**: Остановка процессов и сброс драйверов WinDivert / pf / nftables.

### 2. Кликабельные запускные скрипты в релизных архивах
В каждый релизный архив включен комплект готовых запускных скриптов:
- **🪟 Windows (`.cmd` в `scripts/control_windows/`)**:
  - `service_control.cmd` — открыть меню управления.
  - `general_autotune.cmd`, `general_recommended.cmd`, `general_universal.cmd`, `general_alt1_multisplit.cmd`, `general_alt2_fake_tls.cmd` — запуск профилей с авто-запросом прав Администратора.
- **🍎 macOS (`.command` в `scripts/control_macOS/`)**:
  - `service_control.command`, `general_autotune.command`, `general_ultimate.command`, `general_discord.command`, `general_youtube.command`, `general_telegram.command` — запуск из Finder с подтягиванием `sudo`.
- **🐧 Linux (`.sh` в `scripts/control_linux/`)**:
  - `service_control.sh`, `general_autotune.sh`, `general_ultimate.sh`, `general_discord.sh`, `general_youtube.sh`, `general_telegram.sh`.

### 3. Быстрые алиасы для CLI (`--profile`)
Вы можете запускать профили по коротким именам:
```bash
sudo ./unbound --cli --profile rec        # Recommended (hostfakesplit)
sudo ./unbound --cli --profile universal  # Universal 2026 (All-in-One)
sudo ./unbound --cli --profile alt1       # Alternative 1 (multisplit)
sudo ./unbound --cli --profile ultimate   # Ultimate Bypass (macOS/Linux)
sudo ./unbound --cli --profile discord    # Discord Voice Optimized
sudo ./unbound --cli --profile youtube    # YouTube QUIC Aggressive
```

---

## 🛠 Сборка из исходников

### Требования:
- **Go** 1.25+
- **Node.js** 20+
- **Wails v2.13+**: `go install github.com/wailsapp/wails/v2/cmd/wails@v2.13.0`

### Команды сборки:

```bash
# 1. Клонирование репозитория
git clone https://github.com/bobberdolle1/unbound.git
cd unbound

# 2. Сборка фронтенда (React + Tailwind CSS)
cd frontend && npm ci && npm run build && cd ..

# 3. Сборка CLI-версии
go build -trimpath -ldflags="-s -w -X unbound/engine.Version=0.1.0-refresh" -o build/bin/unbound .

# 4. Сборка десктопного GUI (Wails)
wails build -clean -ldflags "-X unbound/engine.Version=0.1.0-refresh"
```

### Локальные проверки:
```bash
make check     # Полная локальная проверка (gofmt, vet, tests, desktop cross-compile)
make quick     # Быстрая проверка без кросс-компиляции
```

---

## 🗂 Структура репозитория и релизы

- **`master` / `main`**: Ветка Unbound Refresh (`v0.1.0-refresh`). Чистая структура, сфокусированная на Windows, Linux и macOS.
- **`legacy/v2.x`**: Заархивированная ветка предыдущего репозитория (версии v2.x) со всеми экспериментальными модулями (Android, Magisk, OpenWrt, Decky, Web Extension, LG WebOS, tvOS).

---

## 📜 Лицензия

Проект распространяется под лицензией **GPL-3.0 License**.
