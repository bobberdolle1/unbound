<p align="center">
  <img src="https://img.shields.io/badge/Версия-2.0.0-ff6b6b?style=for-the-badge&logo=rocket" alt="Версия 2.0.0" />
  <img src="https://img.shields.io/badge/Платформа-Windows%20%7C%20macOS%20%7C%20Linux%20%7C%20Android%20%7C%20iOS%20%7C%20OpenWrt-3b82f6?style=for-the-badge&logo=windows" alt="Мультиплатформа" />
  <img src="https://img.shields.io/badge/Лицензия-GPL--3.0-orange?style=for-the-badge" alt="Лицензия GPL-3.0" />
  <img src="https://img.shields.io/badge/OpenSource-Свободный%20код-brightgreen?style=for-the-badge" alt="Open Source" />
</p>

<p align="center">
  <img src="https://img.shields.io/badge/%F0%9F%8E%AF%20YouTube-ff0000?style=flat-square" alt="YouTube" />
  <img src="https://img.shields.io/badge/%F0%9F%8E%AE%20Discord-5865f2?style=flat-square" alt="Discord" />
  <img src="https://img.shields.io/badge/%F0%9F%93%B7%20Instagram-e4405f?style=flat-square" alt="Instagram" />
  <img src="https://img.shields.io/badge/%F0%9F%90%99%20Twitter/X-000000?style=flat-square" alt="Twitter/X" />
</p>

<br />

<p align="center">
  <h1 align="center">🚀 UNBOUND</h1>
  <p align="center"><strong>Обход DPI-блокировок для всех платформ</strong></p>
  <p align="center"><em>Одна программа — YouTube, Discord, Instagram, Twitter и другие заблокированные сайты</em></p>
</p>

<p align="center">
  <a href="#-скачать"><strong>⬇️ Скачать</strong></a> •
  <a href="#-установка"><strong>📦 Установка</strong></a> •
  <a href="#-как-пользоваться"><strong>📖 Как пользоваться</strong></a> •
  <a href="#-платформы"><strong>📱 Платформы</strong></a> •
  <a href="#-как-это-работает"><strong>🔬 Как это работает</strong></a> •
  <a href="#-сборка"><strong>🏗️ Сборка</strong></a> •
  <a href="#-благодарности"><strong>🙏 Благодарности</strong></a>
</p>

<br />

---

## ⬇️ Скачать

| Платформа | Файл | Ссылка |
|-----------|------|--------|
| **Windows** | `.exe` установщик | [Releases](https://github.com/bobberdolle1/unbound/releases/latest) |
| **Android** | `.apk` | [Releases](https://github.com/bobberdolle1/unbound/releases/latest) |
| **Linux** | Бинарник | [Releases](https://github.com/bobberdolle1/unbound/releases/latest) |
| **OpenWrt** | `.ipk` пакет | [Releases](https://github.com/bobberdolle1/unbound/releases/latest) |
| **Браузер** | Расширение Chrome/Firefox | [Releases](https://github.com/bobberdolle1/unbound/releases/latest) |
| **iOS (JB)** | `.deb` твик | [Releases](https://github.com/bobberdolle1/unbound/releases/latest) |

> Все файлы доступны на странице [GitHub Releases](https://github.com/bobberdolle1/unbound/releases/latest).

---

## 📦 Установка

### Windows

1. Скачайте установщик со страницы [Releases](https://github.com/bobberdolle1/unbound/releases/latest)
2. Запустите **от имени администратора** (правая кнопка → «Запуск от имени администратора»)
3. Следуйте инструкциям установщика
4. Запустите Unbound из меню Пуск или с рабочего стола

> **Важно:** Без прав администратора программа не сможет перехватывать трафик.

### Android

1. Скачайте APK со страницы [Releases](https://github.com/bobberdolle1/unbound/releases/latest)
2. Разрешите установку из неизвестных источников (Настройки → Безопасность)
3. Установите APK
4. Откройте приложение и предоставьте разрешение VPN
5. Нажмите **«Подключить»**

### Linux

```bash
# Скачать
curl -LO https://github.com/bobberdolle1/unbound/releases/latest/download/unbound-cli-linux-amd64

# Сделать исполняемым
chmod +x unbound-cli-linux-amd64

# Запустить
sudo ./unbound-cli-linux-amd64 start
```

### OpenWrt роутер

```bash
# Скачать и установить пакет
opkg install nfqws-unbound_*.ipk
opkg install luci-app-unbound_*.ipk

# Включить
/etc/init.d/unbound enable
/etc/init.d/unbound start
```

После установки откроется страница в LuCI: **Сервисы → Unbound**.

### Расширение для браузера

**Chrome:**
1. Скачайте ZIP со страницы [Releases](https://github.com/bobberdolle1/unbound/releases/latest)
2. Распакуйте в любую папку
3. Откройте `chrome://extensions/`
4. Включите «Режим разработчика»
5. Нажмите «Загрузить распакованное» → выберите папку

**Firefox:**
1. Откройте `about:debugging#/runtime/this-firefox`
2. «Загрузить временное дополнение» → выберите любой файл из распакованной папки

---

## 📖 Как пользоваться

### Десктоп (Windows/macOS)

1. **Откройте приложение** — появится главное окно с двумя кнопками
2. **Выберите профиль** — из выпадающего списка (рекомендуется «Unbound Ultimate»)
3. **Нажмите «ПОДКЛЮЧИТЬ!»** — готово, трафик идёт через обход DPI
4. **Проверьте** — откройте YouTube, Discord — должно работать

**Автоподбор:** Нажмите кнопку ⭐ для автоматического поиска лучшего профиля под вашего провайдера.

**Системный трей:** При закрытии окна приложение сворачивается в трей (возле часов). Правый клик по иконке → «Выход» для полного закрытия.

**Настройки (шестерёнка ⚙️):**
- Автозапуск — запускать при старте Windows
- Тихий старт — сразу в трей
- TCP Timestamps — улучшить совместимость
- Очистка Discord — автоочистка кэша Discord

### Android

1. Откройте приложение
2. Нажмите **«Подключить»**
3. Разрешите VPN-подключение
4. Всё работает — сворачивайте и пользуйтесь

**Раздельное туннелирование:** Настройки → Раздельное туннелирование — выберите какие приложения идут через обход.

### iOS (Jailbreak)

1. Установите `.deb` через Cydia/Sileo
2. Откройте приложение Unbound из SpringBoard
3. Нажмите **«Подключить»**
4. Настройки доступны в приложении «Настройки» → Unbound

---

## 📱 Платформы

| Платформа | README | Формат |
|-----------|--------|--------|
| 🖥️ **Десктоп** (Windows/macOS) | — | Встроен в установщик |
| 📱 **Android** | [android/README.md](android/README.md) | APK |
| 🍎 **iOS** (Jailbreak) | [theos/README.md](theos/unbound-legacy/README.md) | DEB |
| 🐧 **Linux** | [linux/README.md](linux/README.md) | Бинарник |
| 🌐 **OpenWrt** | [openwrt/README.md](openwrt/README.md) | IPK + LuCI |
| 🧩 **Браузер** | [extension-web/README.md](extension-web/README.md) | Расширение |
| 📺 **WebOS** (LG TV) | [webos/README.md](webos/README.md) | IPK |
| 📺 **tvOS** (Apple TV) | [tvos/README.md](tvos/README.md) | IPA |
| 🎮 **Steam Deck** | [decky-plugin/README.md](decky-plugin/README.md) | Плагин |
| 📦 **Magisk** | [magisk-module/README.md](magisk-module/README.md) | ZIP-модуль |

---

## 🔬 Как это работает

Unbound модифицирует исходящие пакеты так, чтобы системы DPI (Deep Packet Inspection) не могли распознать заблокированные сервисы:

```
Ваш запрос → Unbound меняет пакеты → DPI не распознаёт → Сайт открывается
```

### Техники обхода

| Техника | Что делает |
|---------|-----------|
| **Фрагментация TLS** | Разбивает TLS-рукопожатие на части — DPI не видит SNI |
| **Поддельные пакеты** | Отправляет фейковые пакеты с маленьким TTL — путает DPI |
| **Нарушение порядка** | Меняет порядок сегментов — DPI не собирает правильно |
| **Управление TTL** | Подбирает количество хопов — пакеты «исчезают» до DPI |
| **Морфинг отпечатков** | Изменяет TLS-отпечаток — не похоже на стандартный браузер |

### Автоподбор

Автоподбор сканирует все профили параллельно, проверяя доступность YouTube, Discord, Instagram и других сервисов. Лучший профиль выбирается автоматически по метрике успеха и задержки.

---

## 🏗️ Сборка

Если хотите собрать из исходников:

### Требования

- **Go 1.21+** (десктоп)
- **Node.js 18+** (фронтенд)
- **Wails CLI** (`go install github.com/wailsapp/wails/v2/cmd/wails@latest`)
- **Rust 1.70+** (Linux CLI)

### Быстрая сборка (десктоп)

```bash
git clone https://github.com/bobberdolle1/unbound.git
cd unbound
npm install --prefix frontend
wails build
```

Результат: `build/bin/unbound.exe`

### Все платформы

```bash
./scripts/build_all.sh
```

Подробные инструкции по сборке: [docs/BUILDING.md](docs/BUILDING.md)

---

## 📂 Структура проекта

```
unbound/
├── frontend/          # React UI (Wails фронтенд)
├── engine/            # Ядро на Go (движок, автоподбор, провайдеры)
├── android/           # Android-приложение (Kotlin)
├── linux/             # Linux CLI (Rust)
├── openwrt/           # Пакет для OpenWrt + LuCI
├── extension-web/     # Расширение Chrome/Firefox
├── theos/             # iOS-твик (Theos)
├── webos/             # LG WebOS
├── tvos/              # Apple tvOS
├── magisk-module/     # Magisk-модуль для Android
├── decky-plugin/      # Плагин Steam Deck
├── website/           # Сайт (Astro)
├── docs/              # Документация
└── scripts/           # Скрипты сборки
```

---

## 🙏 Благодарности

Без этих людей и проектов Unbound бы не существовал:

- **[bol-van](https://github.com/bol-van/zapret)** — Автор zapret, основы всего движка. Без него ничего бы не было
- **[WinDivert](https://reqrypt.org/windivert.html)** — Библиотека перехвата пакетов для Windows от basil00
- **[Wails](https://wails.io/)** — Фреймворк для десктопного приложения на Go + веб-технологиях
- **[Astro](https://astro.build/)** — Фреймворк для сайта
- **[GoodbyeDPI](https://github.com/ValdikSS/GoodbyeDPI)** — Вдохновитель техник обхода DPI для Windows
- **[ByeDPI](https://github.com/hufrea/byedpi)** — Вдохновитель техник обхода DPI для Linux
- **[SpoofDPI](https://github.com/xvzc/SpoofDPI)** — Альтернативный движок для macOS
- **[Magisk](https://github.com/topjohnwu/Magisk)** — Root-решение для Android от topjohnwu
- **[Theos](https://theos.dev/)** — Фреймворк для iOS-твиков от DHowett
- **[webosbrew](https://github.com/webosbrew)** — Платформа домашних приложений для WebOS
- **[Cloudflare](https://cloudflare.com)** — CDN и инфраструктура
- **[Google](https://google.com)** — Android SDK и инструменты разработки
- **[JetBrains](https://jetbrains.com)** — IDE для разработки (GoLand, WebStorm, CLion)
- **[Tailwind CSS](https://tailwindcss.com/)** — Стилизация интерфейсов
- **[React](https://react.dev/)** — UI-фреймворк для фронтенда
- **Сообществу** — Всем, кто тестировал, сообщал об ошибках и предлагал идеи

---

## ❓ Частые вопросы

### Это безопасно?
Да. Unbound модифицирует только **ваши собственные** исходящие пакеты. Он не перехватывает чужой трафик, не ломает чужие серверы и не hack'ит что-либо. Это легальный инструмент настройки собственного сетевого стека.

### Нужен ли root/администратор?
- **Windows:** Да, права администратора обязательны (WinDivert требует)
- **Android:** Нет (APK через VpnService), но Magisk-модуль требует root
- **Linux:** Да (nftables/nfqws требуют root)
- **iOS:** Да (требуется джейлбрейк)
- **OpenWrt:** Да (root на роутере)

### Это легально?
Зависит от вашей юрисдикции. Unbound — инструмент с открытым исходным кодом (GPL-3.0). Проконсультируйтесь с местным законодательством.

### Какие сервисы работают?
YouTube, Discord, Instagram, Twitter/X, Facebook, RuTracker и другие. Список пополняется.

### Почему не Telegram?
MTProto/Telegram не поддерживается в данной конфигурации. Фокус на YouTube, Discord, Instagram, Twitter.

### Программа бесплатная?
Да, полностью бесплатна и открыта (GPL-3.0). Никакой телеметрии, никакой рекламы, никаких подписок.

---

## 📜 Лицензия

**GNU General Public License v3.0 (GPL-3.0)**

Этот проект — свободное ПО. Вы можете распространять и/или модифицировать его на условиях GPL-3.0. См. файл [LICENSE](LICENSE).

---

<p align="center">
  <sub>Сделано <a href="https://github.com/bobberdolle1"><strong>bobberdolle1</strong></a> • 2024-2026</sub>
</p>
