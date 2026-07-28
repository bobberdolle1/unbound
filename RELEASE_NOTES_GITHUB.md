# UNBOUND Refresh v0.1.0-refresh

Первый официальный релиз версии **UNBOUND Refresh** — десктопного инструмента десинхронизации и обхода DPI-блокировок для **Windows**, **macOS** и **Linux**.

---

## 📦 Файлы релиза (Release Assets)

| Файл | Описание | Платформа |
| :--- | :--- | :--- |
| **`unbound-v0.1.0-refresh-win64.zip`** | Архив с GUI-приложением и инструкцией | Windows 10 / 11 64-bit |
| **`unbound-windows-amd64.exe`** | Исполняемый GUI-файл Windows | Windows 10 / 11 64-bit |
| **`Unbound-macOS-universal.zip`** | Приложение `.app` GUI (Universal Binary) | macOS (Intel / M1 / M2 / M3) |
| **`unbound-darwin-arm64`** | Исполняемый CLI-файл macOS (Apple Silicon) | macOS M1 / M2 / M3 / M4 |
| **`unbound-darwin-amd64`** | Исполняемый CLI-файл macOS (Intel) | macOS Intel |
| **`unbound-linux-amd64`** | Исполняемый CLI-файл Linux x86_64 | Linux (Ubuntu, Debian, Fedora, Arch) |
| **`unbound-linux-arm64`** | Исполняемый CLI-файл Linux ARM64 | Linux ARM64 / Raspberry Pi |

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
- **GUI приложение**: Скачайте архив **`Unbound-macOS-universal.zip`**, распакуйте и переместите `Unbound.app` в `/Applications`.
- **CLI режим**: Скачайте **`unbound-darwin-arm64`** (M1/M2/M3) или **`unbound-darwin-amd64`** (Intel), сделайте исполняемым:
  ```bash
  chmod +x unbound-darwin-arm64
  sudo ./unbound-darwin-arm64 --cli --autotune
  ```

---

### 🐧 Linux (Ubuntu / Debian / Arch / Fedora)
1. Скачайте файл **`unbound-linux-amd64`** (или `unbound-linux-arm64`).
2. Сделайте файл исполняемым и запустите с правами root:
   ```bash
   chmod +x unbound-linux-amd64
   sudo ./unbound-linux-amd64 --cli --autotune
   ```

---

## ⚡ Новые возможности CLI режима (`--cli`):
- **`--cli --autotune`**: Автоматический запуск автоподбора лучших профилей прямо из терминала с красивым текстовым прогресс-баром `[████████░░░░] 60%`.
- **`--list-profiles --json`**: Вывод доступных профилей обхода в JSON-формате для скриптов и систем мониторинга.
- **Живой мониторинг пинга**: Вывод задержки к YouTube, Discord и Instagram каждые 5 секунд во время работы.
