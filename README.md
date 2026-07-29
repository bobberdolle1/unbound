<div align="center">

<img src="./build/logo.svg" alt="UNBOUND Logo" width="140" />

# UNBOUND Refresh `v0.2.1`
**Локальный настольный оркестратор обхода DPI-блокировок для Windows, Linux и macOS.**

[![Version](https://img.shields.io/badge/Status-v0.2.1-6366f1?style=for-the-badge&logo=rocket)](#)
[![Security](https://img.shields.io/badge/Security-SHA256_pinned-10b981?style=for-the-badge&logo=shield)](SECURITY.md)
[![License](https://img.shields.io/badge/License-GPL--3.0-blue?style=for-the-badge)](LICENSE)
[![Language](https://img.shields.io/badge/Language-Russian_🇷🇺-10b981?style=for-the-badge)](#)

[**🛡 Безопасность и Аудит**](#-безопасность-и-гарантии-доверия) &nbsp; | &nbsp; [**⚡ Архитектура**](#-как-работает-движок) &nbsp; | &nbsp; [**🖥 Платформы**](#-поддерживаемые-платформы) &nbsp; | &nbsp; [**🛠 Сборка**](#-сборка-из-исходников)

</div>

---

## 🛡 Безопасность и Гарантии Доверия

В отличие от подозрительных форков, внедряющих поддельные SSL-сертификаты (Root CA) и перехватывающих пользовательский трафик:

- 🔒 **Без телеметрии и аналитики**: приложение не отправляет метрики или историю посещений; сетевые обращения нужны только функциям обновления, синхронизации списков, диагностики и AutoTune.
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
4. **Чистое версионирование**: актуальные стабильные сборки публикуются в [GitHub Releases](https://github.com/bobberdolle1/unbound/releases); версия приложения едина для GUI, CLI и архивов.

---

## 💡 О проекте

**UNBOUND** — это инструмент десинхронизации сетевого трафика на уровне L3/L4. Он **не является VPN** или прокси-сервисом и не перенаправляет ваш трафик через внешние серверы.

На Windows и Linux приложение перехватывает TCP/UDP-пакеты и применяет стратегии Zapret 2. На macOS используется локальный прозрачный TCP-прокси `tpws` через изолированный `pf`-якорь. Дополнительная обработка может увеличить задержку; AutoTune выбирает профиль по измеренным результатам, а не гарантирует обход в любой сети.

---

## 🧩 Поддерживаемые платформы

| Платформа | Драйвер / Механизм | Статус |
| :--- | :--- | :---: |
| **Windows 10 / 11** | `WinDivert` + Zapret 2 `winws2.exe` | ✅ GUI + CLI |
| **Linux (amd64 / arm64)** | `NFQUEUE` + `nftables` / `iptables` + `nfqws2` | ✅ CLI-релиз |
| **macOS (Apple Silicon / Intel)** | `pf` redirect + Zapret `tpws` | ✅ Universal GUI + CLI |

Детальные требования и ограничения: [**`docs/PLATFORMS.md`**](docs/PLATFORMS.md).

---

## ⚙️ Как работает движок

```mermaid
sequenceDiagram
    participant B as Браузер / Приложение
    participant U as UNBOUND Engine
    participant D as DPI Провайдера
    participant S as Целевой Сервер

    B->>U: TCP-соединение; UDP там, где платформа поддерживает
    Note over U: WinDivert / NFQUEUE либо pf redirect в локальный tpws
    U->>D: Пакетная десинхронизация или TCP split по выбранному профилю
    Note over D: DPI получает изменённое представление начального трафика
    D->>S: Сервер получает допустимый TCP/UDP-трафик
    S-->>B: Ответ сервера
```

---

## 🖥 Использование без GUI (Консольный режим и Запускные Скрипты)

Помимо основного десктопного GUI, UNBOUND поддерживает полноценную работу в консоли и готовые скрипты запуска для **Windows**, **macOS** и **Linux**.

### 1. Интерактивный Control Center (`unbound --control`)
Встроенный консольный менеджер использует единое меню на всех трёх ОС; доступные профили, системные команды и запрос прав зависят от платформы:
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
Скрипты лежат рядом с исполняемым файлом, поэтому архив можно распаковать и запустить без ручной правки путей:
- **🪟 Windows (`.cmd`)**: `service_control.cmd`, `general_autotune.cmd`, `general_recommended.cmd`, `general_universal.cmd`, `general_alt1_multisplit.cmd`, `general_alt2_fake_tls.cmd`.
- **🍎 macOS (`.command`)**: `service_control.command`, `general_autotune.command`, `general_ultimate.command`, `general_discord.command`, `general_youtube.command`, `general_telegram.command`.
- **🐧 Linux (`.sh`)**: `run-unbound.sh`, `service_control.sh`, `general_autotune.sh`, `general_ultimate.sh`, `general_discord.sh`, `general_youtube.sh`, `general_telegram.sh`.

### 3. Быстрые алиасы для CLI (`--profile`)
Короткое имя сопоставляется только с профилями, доступными на текущей ОС:
```bash
sudo ./unbound --cli --profile rec        # Windows: Recommended
sudo ./unbound --cli --profile universal  # Windows: Universal
sudo ./unbound --cli --profile ultimate   # macOS/Linux: Ultimate
sudo ./unbound --cli --profile discord    # macOS/Linux: Discord
sudo ./unbound --cli --profile youtube    # macOS/Linux: YouTube
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
go build -trimpath -ldflags="-s -w -X unbound/engine.Version=0.2.1" -o build/bin/unbound .

# 4. Сборка десктопного GUI (Wails)
wails build -clean -ldflags "-X unbound/engine.Version=0.2.1"
```

### Локальные проверки:
```bash
make check     # Полная локальная проверка (gofmt, vet, tests, desktop cross-compile)
make quick     # Быстрая проверка без кросс-компиляции
```

---

## 🗂 Структура репозитория и релизы

- **`master`**: актуальная ветка Unbound Refresh (`v0.2.1`), сфокусированная на Windows, Linux и macOS.
- **`legacy/v2.x`**: Заархивированная ветка предыдущего репозитория (версии v2.x) со всеми экспериментальными модулями (Android, Magisk, OpenWrt, Decky, Web Extension, LG WebOS, tvOS).

---

## 📜 Лицензии и происхождение движков

Код UNBOUND распространяется под **GPL-3.0**. Вендоренные Zapret и Zapret 2 распространяются под MIT; тексты лицензий включаются в релизные архивы. Точные теги, коммиты, SHA256 исходных архивов и пути компонентов записаны в [`engine/ENGINE_PROVENANCE.json`](engine/ENGINE_PROVENANCE.json). Контрольные суммы файлов каждого готового архива находятся в `BUNDLE_SHA256SUMS.txt`, общие суммы архивов — в `SHA256SUMS.txt`.

