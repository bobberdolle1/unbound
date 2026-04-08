# Changelog — Unbound Website

Все значимые изменения сайта документируются в этом файле.

## [1.0.0] - 2026-04-07
### Initial Release — Landing Page
- **Astro 5**: Статическая генерация, мгновенная загрузка, zero JavaScript runtime.
- **Современная тёмная тема**: Элегантная тёмная палитра (#0a0a0f) с фиолетовыми градиентами (#667eea → #764ba2).
- **Автоопределение ОС**: Клиентский JS определяет Windows / macOS / Linux / Android / iOS и подсвечивает соответствующую карточку загрузки с бейджем «Recommended».
- **Живые ссылки на релизы**: `fetch-releases.js` запрашивает `api.github.com/repos/bobberdolle1/unbound/releases/latest`, парсит ассеты и подставляет реальные URL загрузок + номер версии в кнопки.
- **Пасхальные темы** (переключатель в правом нижнем углу):
  - 🌑 **Modern Dark** — по умолчанию, минималистичный dark с backdrop-blur.
  - 🎨 **iOS 6 Skeuomorphic** — текстуры, leather gradients, glossy кнопки, inset shadows, реалистичные переключатели.
  - 🎮 **Doodle Jump** — Comic Sans, dashed borders, rainbow gradient animation, bounce-эффекты, box-shadow «смещение».
- **Секции лендинга**:
  - **Hero** — заголовок «Total War Against Censorship», статистика (70+ профилей, 0% потери скорости, 1-click), CTA-кнопки.
  - **Features** — 6 карточек: Multidisorder, Zero Speed Loss, Auto-Tune, Open Source, Conflict Detection, Silent Auto-Start.
  - **Download** — 5 платформ (Windows, macOS, Linux, Android, iOS), Windows подсвечен как primary.
  - **How It Works** — 3 шага + техническое объяснение (zapret2, TCP desynchronization, WinDivert).
  - **FAQ** — 6 раскрывающихся вопросов-ответов.
  - **Footer** — ссылки на GitHub, Releases, Issues, Changelog, кредиты (Zapret2, Wails, WinDivert).
- **Навигация**: Фиксированная с backdrop-blur, логотип ⚡ UNBOUND, якорные ссылки, кнопка GitHub.
- **SEO**: Meta-теги, Open Graph, Google Fonts (Inter + JetBrains Mono).
- **Адаптивный дизайн**: Полная поддержка мобильных экранов (grid → single column на < 640px).
- **Локальное развёртывание**: Скрипт `npm run deploy` → `astro build` + `gh-pages -d dist`. Без CI/CD.
- **Конфигурация**: `base: /unbound`, `site: https://bobberdolle1.github.io` — готово к GitHub Pages.

---
*UNBOUND Website: Fast, static, and fun.*
