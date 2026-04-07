# Unbound Web — Расширение для браузера

Кросс-браузерное расширение для обхода DPI и цензуры с двумя интеллектуальными режимами:

## Режимы работы

### 1. Режим компаньона
Выступает в роли панели управления, которая связывается с локальным демоном Unbound Desktop через Native Messaging. Десктопное приложение занимается реальной работой с сетью.

### 2. Автономный режим прокси
Использует `proxy` API браузера для динамического применения PAC-скриптов, направляя только определённые домены через внешний прокси-сервер (HTTPS/SOCKS5).

## Возможности

- **Два режима работы**: Переключение между режимом компаньона (нативное приложение) и автономным режимом (прокси)
- **Динамическая генерация PAC**: Интеллектуальная маршрутизация только определённых доменов
- **Поддержка тем**:
  - Doodle Jump Минимализм (светлая тема)
  - Modern Dark (тёмная тема)
- **Кросс-браузерность**: Сборка для Chrome (MV3) и Firefox (MV3)
- **Управление доменами**: Добавление/удаление доменов для обхода цензуры

## Разработка

### Требования
- Node.js 18+
- npm или yarn

### Установка

```bash
cd extension-web
npm install
```

### Режим разработки

```bash
# Chrome (режим наблюдения)
npm run dev:chrome

# Firefox (режим наблюдения)
npm run dev:firefox
```

### Сборка

```bash
# Сборка для обоих браузеров
npm run build

# Отдельные сборки
npm run build:chrome
npm run build:firefox
```

### Установка в браузер

#### Chrome
1. Откройте `chrome://extensions/`
2. Включите «Режим разработчика»
3. Нажмите «Загрузить распакованное расширение»
4. Выберите папку `dist/chrome`

#### Firefox
1. Откройте `about:debugging#/runtime/this-firefox`
2. Нажмите «Загрузить временное дополнение»
3. Выберите любой файл из папки `dist/firefox`

## Архитектура

```
extension-web/
├── src/
│   ├── background/        # Service worker (MV3)
│   │   ├── index.ts       # Главная точка входа
│   │   ├── companion.ts   # Логика нативного обмена сообщениями
│   │   └── standalone.ts  # Логика PAC/прокси
│   ├── popup/             # React UI
│   │   ├── main.tsx       # Точка входа
│   │   └── App.tsx        # Главный компонент
│   ├── components/        # React-компоненты
│   │   ├── ConnectToggle.tsx
│   │   ├── ModeSelector.tsx
│   │   ├── DomainList.tsx
│   │   ├── ThemeSwitcher.tsx
│   │   └── ProxyConfigPanel.tsx
│   ├── utils/             # Утилиты
│   │   ├── storage.ts     # Сохранение состояния
│   │   └── proxy.ts       # Генерация PAC-скриптов
│   ├── types/             # TypeScript типы
│   └── styles/            # Глобальные стили
├── manifest.chrome.ts     # Манифест Chrome MV3
├── manifest.firefox.ts    # Манифест Firefox MV3
├── vite.config.ts         # Конфигурация Vite
└── host_manifest.json     # Манифест хоста нативного обмена сообщениями
```

## Интеграция с Native Messaging

Расширение взаимодействует с демоном Unbound Desktop через Native Messaging API Chrome.

### Настройка (Windows)

1. Соберите бинарник хоста (Go/Rust)
2. Разместите его в директории расширения
3. Зарегистрируйте в реестре Windows:

```reg
Windows Registry Editor Version 5.00

[HKEY_CURRENT_USER\SOFTWARE\Google\Chrome\NativeMessagingHosts\com.unbound.desktop]
@="C:\\path\\to\\host_manifest.json"
```

### Настройка (macOS)

```bash
mkdir -p ~/Library/Application\ Support/Google/Chrome/NativeMessagingHosts
cp host_manifest.json ~/Library/Application\ Support/Google/Chrome/NativeMessagingHosts/com.unbound.desktop.json
```

### Протокол

**Расширение → Хост:**
```json
{"command": "start", "domains": ["*.youtube.com", "*.discord.com"]}
{"command": "stop"}
{"command": "status"}
{"command": "update_domains", "domains": [...]}
```

**Хост → Расширение:**
```json
{"status": "running", "version": "1.0.0"}
{"status": "stopped"}
{"status": "error", "message": "Описание ошибки"}
```

## Пример PAC-скрипта

В автономном режиме расширение генерирует PAC-скрипт вроде:

```javascript
function FindProxyForURL(url, host) {
  if (dnsDomainIs(host, '.youtube.com') ||
      dnsDomainIs(host, '.discord.com')) {
    return "PROXY proxy.example.com:8080";
  }
  return "DIRECT";
}
```

## Лицензия

Та же, что и у основного проекта Unbound.
