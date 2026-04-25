# 🎨 QUICK REFERENCE - CYBER GRID DESIGN SYSTEM

## 🎯 Быстрый доступ к ключевым элементам

---

## 🎨 ЦВЕТА (копировать и использовать)

```css
/* Фоны */
--bg-base: #010B2A;
--bg-surface: #061038;
--bg-elevated: #0A1644;

/* Основной */
--primary-500: #39FF6A;
--primary-600: #23E153;
--primary-700: #15B844;

/* Акценты */
--accent-lime: #72FF8D;
--accent-cyan: #55E7FF;

/* Текст */
--text-primary: #EAFBF1;
--text-secondary: #A9C7B5;
--text-muted: #6F8A7A;

/* Состояния */
--state-success: #39FF6A;
--state-warning: #FFD24D;
--state-error: #FF4D6D;
--state-info: #55E7FF;

/* Границы */
--border-default: #1B365F;
--border-strong: #39FF6A;
--border-subtle: #113059;

/* Эффекты */
--neon-glow: 0 0 8px rgba(57,255,106,0.55), 0 0 24px rgba(57,255,106,0.25);
--neon-glow-soft: 0 0 6px rgba(57,255,106,0.35);
```

---

## 🔤 ШРИФТЫ

```css
/* Import */
@import url('https://fonts.googleapis.com/css2?family=Orbitron:wght@400;500;600;700;900&family=Exo+2:wght@400;500;600;700;800;900&family=JetBrains+Mono:wght@400;500;600&display=swap');

/* Variables */
--font-display: 'Orbitron', 'Exo 2', sans-serif;
--font-body: 'Exo 2', sans-serif;
--font-mono: 'JetBrains Mono', monospace;
```

**Использование:**
- **Display**: Заголовки, кнопки, метки (UPPERCASE)
- **Body**: Параграфы, описания, формы
- **Mono**: Код, ID, технические данные

---

## 📏 SPACING (4px система)

```css
--space-1: 4px;
--space-2: 8px;
--space-3: 12px;
--space-4: 16px;
--space-5: 20px;
--space-6: 24px;
--space-8: 32px;
--space-10: 40px;
--space-12: 48px;
--space-16: 64px;
```

---

## 🔲 RADIUS

```css
--radius-sm: 4px;   /* Кнопки, inputs, badges */
--radius-md: 8px;   /* Карточки */
--radius-lg: 12px;  /* Модальные окна */
```

---

## ⚡ MOTION

```css
/* Duration */
--duration-fast: 120ms;   /* Hover states */
--duration-base: 180ms;   /* Transitions */
--duration-slow: 260ms;   /* Complex animations */

/* Easing */
--easing: cubic-bezier(0.2, 0.8, 0.2, 1);
```

---

## 🎯 ГОТОВЫЕ КЛАССЫ

### Кнопка Primary
```html
<button class="btn primary">
  ▶ ЗАПУСТИТЬ
</button>
```

### Кнопка Ghost
```html
<button class="btn ghost">
  ✎ РЕДАКТИРОВАТЬ
</button>
```

### Кнопка Danger
```html
<button class="btn danger">
  ✕ УДАЛИТЬ
</button>
```

### Карточка
```html
<div class="card">
  <h2>ЗАГОЛОВОК</h2>
  <p>Контент карточки</p>
</div>
```

### Статистика
```html
<div class="stat-card">
  <div class="stat-card__label">ВСЕГО БОТОВ</div>
  <div class="stat-card__value">42</div>
</div>
```

### Статус бейдж
```html
<span class="bot-card__status bot-card__status--running">
  ● РАБОТАЕТ
</span>
```

### Alert
```html
<div class="alert">
  <strong>ВНИМАНИЕ:</strong> Важная информация
</div>
```

---

## 🎬 ГОТОВЫЕ АНИМАЦИИ

### Fade In
```css
animation: fadeIn 260ms cubic-bezier(0.2, 0.8, 0.2, 1) backwards;
```

### Slide In
```css
animation: slideIn 300ms ease-out;
```

### Pulse
```css
animation: pulse 2s infinite;
```

---

## 🌐 ФОНОВАЯ СЕТКА

```css
/* Уже применена к body::before */
/* Автоматически отображается на всех страницах */
```

---

## 🎨 ПИКСЕЛЬНЫЕ УГЛЫ

```css
/* Автоматически на .card и .quick-tile */
/* Через ::before и ::after псевдоэлементы */
```

---

## 💡 ТИПИЧНЫЕ ПАТТЕРНЫ

### Заголовок страницы
```html
<header class="hero">
  <h1 class="hero__title">УПРАВЛЕНИЕ БОТАМИ</h1>
  <p class="hero__lead">
    Описание страницы
  </p>
</header>
```

### Quick Tiles (навигация)
```html
<nav class="quick-tiles">
  <a class="quick-tile" href="#section">
    <span class="quick-tile__label">ШАГ 1</span>
    <span class="quick-tile__title">ЗАГОЛОВОК</span>
    <p class="quick-tile__desc">Описание</p>
  </a>
</nav>
```

### Форма
```html
<div class="field">
  <label for="input-id">МЕТКА</label>
  <input id="input-id" type="text" placeholder="Placeholder" />
  <p class="hint">Подсказка</p>
</div>
```

### Checkbox
```html
<label class="checkbox-row">
  <input type="checkbox" />
  <span>ВКЛЮЧИТЬ ОПЦИЮ</span>
</label>
```

---

## 🚀 БЫСТРЫЙ СТАРТ

### 1. Создать новую карточку
```html
<div class="card">
  <h2>МОЯ КАРТОЧКА</h2>
  <p>Контент</p>
  <button class="btn primary">ДЕЙСТВИЕ</button>
</div>
```

### 2. Добавить статистику
```html
<div class="stats-grid">
  <div class="stat-card">
    <div class="stat-card__label">МЕТРИКА</div>
    <div class="stat-card__value">123</div>
  </div>
</div>
```

### 3. Создать форму
```html
<section class="card">
  <h2>НАСТРОЙКИ</h2>
  
  <div class="field">
    <label for="name">ИМЯ</label>
    <input id="name" type="text" />
  </div>
  
  <div class="form-actions">
    <button class="btn primary">СОХРАНИТЬ</button>
    <button class="btn ghost">ОТМЕНИТЬ</button>
  </div>
</section>
```

---

## 🎯 ПРАВИЛА СТИЛЯ

### ✅ ДЕЛАТЬ
- UPPERCASE для заголовков и кнопок
- Неоновое свечение на активных элементах
- Пиксельные углы на карточках
- Monospace для технических данных
- Иконки на кнопках (▶ ■ ✕)
- Letter-spacing: 0.08em для UPPERCASE

### ❌ НЕ ДЕЛАТЬ
- Обычный регистр для заголовков
- Округлые формы (только квадратные)
- Пастельные цвета
- Blur/glass эффекты
- Много акцентных цветов в одном экране

---

## 📱 RESPONSIVE

```css
@media (max-width: 768px) {
  /* Автоматически применяется */
  /* Навигация: вертикальная */
  /* Grid: 1 колонка */
  /* Отступы: уменьшены */
}
```

---

## 🔧 КАСТОМИЗАЦИЯ

### Изменить основной цвет
```css
:root {
  --primary-500: #FF6A39; /* Новый цвет */
}
```

### Изменить шрифт
```css
:root {
  --font-display: 'Ваш шрифт', sans-serif;
}
```

### Изменить скорость анимаций
```css
:root {
  --duration-base: 300ms; /* Медленнее */
}
```

---

## 📚 ПОЛНАЯ ДОКУМЕНТАЦИЯ

- **REDESIGN_SUMMARY.md** - Полное описание
- **VISUAL_CHANGES.md** - Визуальные изменения
- **FONT_UPDATE.md** - Информация о шрифтах
- **COMPONENT_RECOMMENDATIONS.md** - Улучшения
- **FINAL_REPORT.md** - Итоговый отчёт

---

## 🌐 ССЫЛКИ

**Dev сервер**: http://localhost:5174  
**Design system**: design-system.json  
**Google Fonts**: fonts.google.com  
**React Bits**: reactbits.dev  
**TweakCN**: tweakcn.com  

---

## ⚡ ГОРЯЧИЕ КЛАВИШИ (для разработки)

```bash
# Запуск dev сервера
cd frontend && npm run dev

# Сборка production
npm run build

# Предпросмотр production
npm run preview
```

---

**Версия**: 1.0.0  
**Дата**: 2026-04-24  
**Статус**: ✅ Готово к использованию
