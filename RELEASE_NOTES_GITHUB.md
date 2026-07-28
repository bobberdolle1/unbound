# UNBOUND Refresh v0.1.0-refresh

Первый официальный релиз версии **UNBOUND Refresh** — десктопного инструмента десинхронизации и обхода DPI-блокировок для **Windows**, **macOS** и **Linux**.

---

## 📦 Файлы релиза (Release Assets)

| Файл | Описание | Платформа |
| :--- | :--- | :--- |
| **`unbound-v0.1.0-refresh-win64.zip`** | Архив с GUI-приложением и инструкцией | Windows 10 / 11 64-bit |
| **`unbound-windows-amd64.exe`** | Исполняемый GUI-файл Windows | Windows 10 / 11 64-bit |
| **`Unbound-macOS-universal.zip`** | Приложение `.app` (Universal Binary) | macOS (Intel / M1 / M2 / M3) |
| **`unbound-linux-amd64`** | Исполняемый файл Linux x86_64 | Linux (Ubuntu, Debian, Fedora, Arch) |
| **`unbound-linux-arm64`** | Исполняемый файл Linux ARM64 | Linux ARM64 / Raspberry Pi |

---

## 📖 Инструкция по установке и запуску

### 🪟 Windows (Windows 10 / 11 64-bit)
1. Скачайте архив **`unbound-v0.1.0-refresh-win64.zip`** и распакуйте его (или скачайте `unbound-windows-amd64.exe`).
2. Запустите **`unbound.exe`** от имени Администратора.
3. В интерфейсе нажмите кнопку **«Автоподбор» (AutoTune)** или выберите профиль обхода (`Recommended` / `Alternative 2`).

> 🛡 **Замечание по Защитнику Windows (Windows Defender):**
> Драйвер перехвата пакетов L3/L4 `WinDivert` может классифицироваться антивирусами как `Not-a-virus:RiskTool.Multi.WinDivert`. Это **100% ложное срабатывание (False Positive)** на компонент перехвата трафика.
> Если Защитник Windows блокирует запуск:
> 1. Зайдите в *Защитник Windows -> Параметры защиты -> Исключения*.
> 2. Добавьте в исключения папку с распакованным `unbound.exe`.
> 3. При появлении окна SmartScreen нажмите **«Подробнее» ➔ «Выполнить в любом случае»**.

---

### 🍎 macOS (macOS 12+ Intel / Apple Silicon M1/M2/M3)
1. Скачайте архив **`Unbound-macOS-universal.zip`**.
2. Распакуйте архив и переместите `Unbound.app` в папку `Программы` (`/Applications`).
3. Если macOS блокирует запуск (Gatekeeper), выполните в терминале:
   ```bash
   xattr -cr /Applications/Unbound.app
   ```
4. Нажмите **ПОДКЛЮЧИТЬ!** (приложению понадобятся права sudo для настройки `pf`).

---

### 🐧 Linux (Ubuntu / Debian / Arch / Fedora)
1. Скачайте файл **`unbound-linux-amd64`** (или `unbound-linux-arm64`).
2. Сделайте файл исполняемым и запустите с правами root:
   ```bash
   chmod +x unbound-linux-amd64
   sudo ./unbound-linux-amd64
   ```

---

## ✨ Ключевые возможности версии Refresh:
- **Современный движок Zapret 2**: Работа на базе `winws2` / `nfqws2` / `tpws` с полной поддержкой Lua-скриптов (`zapret-antidpi.lua`).
- **Автоподбор стратегий (AutoTune)**: Проверка доступности YouTube, Discord, Cloudflare и Ozon в реальном времени с точным подбором профиля под вашего провайдера.
- **Безопасность**: Без MITM и подмены Root CA. Контроль целостности бинарников по SHA256 (`ENGINE_ASSETS.sha256`).
- **SecureDNS**: Встроенная поддержка Cloudflare DoH (1.1.1.1).
