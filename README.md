# 🚀 UNBOUND

**Next-Gen GUI для Zapret 2 с Auto-Tune сканером и встроенным Lua редактором**

![Windows](https://img.shields.io/badge/Windows-0078D6?style=for-the-badge&logo=windows&logoColor=white)
![Go](https://img.shields.io/badge/Go-00ADD8?style=for-the-badge&logo=go&logoColor=white)
![React](https://img.shields.io/badge/React-20232A?style=for-the-badge&logo=react&logoColor=61DAFB)
![TypeScript](https://img.shields.io/badge/TypeScript-007ACC?style=for-the-badge&logo=typescript&logoColor=white)

---

## 🎯 Что это?

**Unbound** — премиум GUI-обёртка для движка **Zapret 2** (nfqws.exe + WinDivert), которая автоматически подбирает оптимальную стратегию обхода DPI для вашего провайдера. Забудьте про консоль и ручной подбор параметров — просто нажмите **CONNECT**.

### ⚡ Ключевые фичи

- **🧠 Smart Auto-Tune Scanner** — автоматически тестирует все профили и выбирает лучший для вашей сети
- **📝 Advanced Lua Editor** — пишите и сохраняйте кастомные Zapret 2 скрипты прямо в приложении
- **🎨 Premium Dark UI** — glassmorphic интерфейс с real-time телеметрией и динамической подсветкой статуса
- **🔒 Zero-Zombie Engine** — корректное завершение WinDivert драйверов при закрытии/сворачивании в трей
- **📊 Live Telemetry** — мониторинг работы движка в реальном времени с фильтрацией логов

---

## 📥 Установка

1. Скачайте `unbound.exe` из [Releases](https://github.com/bobberdolle1/unbound/releases/latest)
2. Запустите **от имени администратора** (требуется для WinDivert)
3. Нажмите кнопку **CONNECT** или запустите **Auto-Tune**

> ⚠️ **Важно:** Закройте все другие DPI-bypass инструменты (GoodbyeDPI, Zapret CLI и т.д.) перед запуском Unbound

---

## 🎮 Как пользоваться

### Быстрый старт
1. Выберите профиль из списка (например, `Unbound Ultimate (God Mode)`)
2. Нажмите большую кнопку **TAP TO CONNECT**
3. Проверьте доступ к заблокированным ресурсам

### Auto-Tune (рекомендуется)
1. Нажмите кнопку **Auto-Tune** (иконка радара)
2. Подождите 2-3 минуты, пока система протестирует все профили
3. Unbound автоматически выберет и запустит лучший вариант

### Advanced Lua Editor
1. Нажмите иконку **Code** в правом верхнем углу
2. Напишите свой Lua скрипт для Zapret 2
3. Нажмите **Save & Apply** — профиль `Custom Profile` будет автоматически выбран
4. Скрипт сохраняется в `%APPDATA%/Unbound/custom_profile.lua`

---

## 🛠️ Встроенные профили

| Профиль | Описание |
|---------|----------|
| **Unbound Ultimate (God Mode)** | Универсальный профиль с агрессивным обходом TLS + QUIC |
| **Discord Voice Optimized** | Оптимизирован для голосовых каналов Discord (UDP 3478, 50000-65535) |
| **YouTube QUIC Aggressive** | Максимальная производительность для YouTube через QUIC |
| **Telegram API Bypass** | Специально для Telegram (порты 5222, 5223, 5228) |
| **Fake TLS & QUIC** | Базовая подмена TLS/QUIC пакетов |
| **Multi-Strategy Chaos** | Комбинация fake, multidisorder и badseq |
| **Standard Split** | Классический split на позиции 1 |
| **Fake Packets + BadSeq** | Fake пакеты + некорректная последовательность |
| **Disorder** | Перестановка фрагментов пакетов |
| **Split Handshake** | Split на середине домена (midsld) |
| **Flowseal Legacy** | Совместимость со старыми версиями Zapret |
| **Custom Profile** | Ваш собственный Lua скрипт |

---

## 🏗️ Архитектура

```
Unbound (Wails v2)
├── Go Backend
│   ├── engine/
│   │   ├── assets.go          # Embedded nfqws.exe + WinDivert + Lua scripts
│   │   ├── config.go          # Persistent storage для custom scripts
│   │   ├── scanner.go         # Auto-Tune логика
│   │   ├── healthcheck.go     # Проверка доступности ресурсов
│   │   └── providers/
│   │       └── zapret2_windows.go  # Запуск nfqws.exe с WinDivert
│   ├── app.go                 # Wails bindings
│   └── app_windows.go         # System Tray интеграция
│
└── React Frontend (TypeScript + Tailwind)
    └── src/
        └── App.tsx            # Glassmorphic UI с real-time телеметрией
```

---

## 🔧 Сборка из исходников

### Требования
- Go 1.21+
- Node.js 18+
- Wails CLI v2.11.0+

### Команды
```bash
# Установка зависимостей
go mod download
cd frontend && npm install

# Dev режим
wails dev

# Production сборка
wails build -clean
```

Готовый `unbound.exe` появится в `build/bin/`

---

## 🐛 Troubleshooting

### "WinDivert Error/Binding Failure"
- Закройте все другие DPI-bypass инструменты
- Перезапустите Unbound от имени администратора
- Проверьте, что WinDivert драйвер не заблокирован антивирусом

### "Administrator/root privileges required"
- Запустите `unbound.exe` через ПКМ → "Запуск от имени администратора"

### Профиль не работает
- Попробуйте **Auto-Tune** — он автоматически найдёт рабочий вариант
- Проверьте логи в нижней панели (Telemetry)

---

## 📜 Лицензия

MIT License — делайте что хотите, но без гарантий.

---

## 🙏 Благодарности

- **[Zapret](https://github.com/bol-van/zapret)** — за мощный DPI bypass движок
- **[Wails](https://wails.io)** — за возможность писать desktop GUI на Go + React
- **[WinDivert](https://reqrypt.org/windivert.html)** — за низкоуровневый перехват пакетов

---

## 🔗 Ссылки

- [Releases](https://github.com/bobberdolle1/unbound/releases)
- [Issues](https://github.com/bobberdolle1/unbound/issues)
- [Zapret Documentation](https://github.com/bol-van/zapret)

---

**Made with 🔥 by bobberdolle1**
