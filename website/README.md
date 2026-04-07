# Сайт Unbound

Высококонверсионная посадочная страница для инструмента обхода DPI **Unbound**. Построена на [Astro](https://astro.build/) для молниеносной статической производительности.

## 🌐 Живой сайт

Развёрнут на GitHub Pages: `https://bobberdolle1.github.io/unbound/`

## ✨ Возможности

- **Автоопределение ОС** — Подсвечивает кнопку загрузки, соответствующую ОС посетителя (Windows, macOS, Linux, Android, iOS)
- **Живые релизы GitHub** — Получает URL-адреса загрузок напрямую из GitHub API
- **Современная тёмная тема** — Элегантная тёмная эстетика с фиолетовыми градиентными акцентами
- **Пасхальные темы** — Переключение между:
  - 🌑 **Modern Dark** (по умолчанию)
  - 🎨 **iOS 6 Skeuomorphic** — Реалистичная кожа, строчка и эффекты глянца
  - 🎮 **Doodle Jump** — Comic Sans, пунктирные границы, прыгучие анимации
- **Полностью статический** — Без сервера, без бэкенда, мгновенная загрузка

## 🚀 Быстрый старт

### Установка зависимостей
```bash
cd website
npm install
```

### Сервер разработки
```bash
npm run dev
```
Откройте `http://localhost:4321` в браузере.

### Сборка для продакшена
```bash
npm run build
```
Результат — в `dist/`.

### Предпросмотр продакшен-сборки
```bash
npm run preview
```

## 📦 Локальное развёртывание на GitHub Pages (без CI/CD)

Этот проект использует пакет [`gh-pages`](https://www.npmjs.com/package/gh-pages) для **локального развёртывания**. Никаких GitHub Actions, облачного CI — всё выполняется с вашей машины.

### Одноразовая настройка
```bash
cd website
npm install   # gh-pages уже в devDependencies
```

### Развёртывание
```bash
npm run deploy
```

Этот скрипт:
1. Запускает `npm run build` для генерации статического сайта в `dist/`
2. Отправляет содержимое `dist/` в ветку `gh-pages` вашего репозитория
3. GitHub Pages автоматически обслуживает сайт из этой ветки

### ⚠️ Важные заметы
- Вы должны иметь **права на запись** в репозиторий GitHub
- Базовый путь `base` в `astro.config.mjs` установлен в `/unbound` — это соответствует имени репозитория. Если ваш URL Pages отличается (например, кастомный домен), обновите значение `base`
- После первого推送 перейдите в **GitHub → Settings → Pages** и установите источник на ветку `gh-pages`
- Последующие вызовы `npm run deploy` обновят ветку автоматически

### Кастомный домен (опционально)
Добавьте файл `CNAME` в папку `public/` с вашим доменом:
```
unbound.example.com
```
Затем настройте DNS для указания на GitHub Pages.

## 📁 Структура проекта

```
website/
├── astro.config.mjs      # Конфигурация Astro
├── package.json          # Зависимости + скрипт развёртывания
├── public/
│   └── scripts/          # Клиентский JS (определение ОС, релизы, темы)
│       ├── os-detect.js
│       ├── fetch-releases.js
│       └── theme-manager.js
├── src/
│   ├── components/       # Компоненты Astro
│   │   ├── BaseHead.astro
│   │   ├── Navbar.astro
│   │   ├── Hero.astro
│   │   ├── Features.astro
│   │   ├── Download.astro
│   │   ├── HowItWorks.astro
│   │   ├── FAQ.astro
│   │   ├── Footer.astro
│   │   └── ThemeToggle.astro
│   ├── pages/
│   │   └── index.astro   # Посадочная страница
│   └── styles/
│       ├── global.css    # Глобальные стили
│       └── themes.css    # Тёмная + пасхальные темы
└── dist/                 # Результат сборки (игнорируется git, развёртывается в gh-pages)
```

## 🎨 Темы

Переключатель тем циклически проходит три темы:

| Тема | Класс | Описание |
|------|-------|----------|
| Modern Dark | `theme-dark` | По умолчанию — элегантная тёмная с фиолетовыми градиентами |
| iOS 6 Skeuomorphic | `theme-skeuomorphic` | Реалистичные текстуры, кожа, глянцевые эффекты |
| Doodle Jump | `theme-doodle` | Comic Sans, пунктирные границы, радужные анимации |

Предпочтение темы сохраняется в `localStorage` и сохраняется между визитами.

## 🔧 Настройка

### Репозиторий GitHub
URL репозитория настроен в:
- `public/scripts/fetch-releases.js` — `REPO_OWNER` и `REPO_NAME`
- `astro.config.mjs` — `site` и `base`

### Базовый путь GitHub Pages
Если ваш сайт доступен по адресу `https://username.github.io/repo-name/`, `base` должен быть `/repo-name`. Для кастомных доменов в корне установите `base: "/"`.

## 📄 Лицензия

GPL-3.0 — та же, что и у основного проекта Unbound.
