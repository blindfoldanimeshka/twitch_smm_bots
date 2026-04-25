# 🎮 TWITCH BOT FARM - CYBER GRID REDESIGN

## 📋 Обзор

Полный редизайн UI в стиле **"retro-arcade cyber grid"** согласно design-system.json (профиль: snake-on-the-throne-neon-grid-v1).

## 🎨 Дизайн-система

### Цветовая палитра
- **Фон**: Тёмно-синий (#010B2A, #061038, #0A1644)
- **Основной**: Неоновый зелёный (#39FF6A)
- **Акценты**: Лайм (#72FF8D), Циан (#55E7FF)
- **Текст**: Светлый (#EAFBF1, #A9C7B5, #6F8A7A)

### Типографика
- **Display**: Orbitron (заголовки, кнопки, метки)
- **Body**: Rajdhani (основной текст)
- **Mono**: JetBrains Mono (код, данные)

### Ключевые особенности
- ✨ Неоновое свечение на активных элементах
- 🎯 Пиксельные декоративные углы на карточках
- 🌐 Фоновая сетка 20x20px с прозрачностью
- 🔲 Uppercase текст для заголовков и кнопок
- ⚡ Плавные анимации с cubic-bezier(0.2, 0.8, 0.2, 1)

## 📁 Обновлённые файлы

### Стили
- ✅ `frontend/src/styles/main.css` - Полностью переписан с нуля
- ✅ `frontend/src/index.css` - Обновлён импорт

### Компоненты
- ✅ `frontend/src/components/StatusBar.tsx` - Cyber стиль для статус-бара
- ✅ `frontend/src/components/Stats.tsx` - Uppercase метки
- ✅ `frontend/src/components/BotList.tsx` - Обновлённые кнопки
- ✅ `frontend/src/components/BotCard.tsx` - Иконки и стиль

### Страницы
- ✅ `frontend/src/pages/Dashboard.tsx` - Uppercase заголовок
- ✅ `frontend/src/pages/Bots.tsx` - Обновлённые формы и карточки
- ✅ `frontend/src/pages/Settings.tsx` - Полное обновление UI

## 🎯 Основные изменения

### 1. Навигация
- Неоновый зелёный логотип с пиксельными деталями
- Uppercase текст с увеличенным letter-spacing
- Активная ссылка с неоновым свечением

### 2. Карточки (Cards)
- Пиксельные углы (4x4px) с неоновым свечением
- Граница: 1px solid #39FF6A
- Hover эффект с поднятием и усилением свечения

### 3. Кнопки
- Uppercase текст с Orbitron шрифтом
- Иконки для действий (▶ СТАРТ, ■ СТОП, ✕ УДАЛИТЬ)
- Анимация "световой волны" при hover
- Неоновое свечение для primary кнопок

### 4. Формы
- Uppercase метки с Orbitron
- Тёмный фон для input (#010B2A)
- Неоновая граница при focus
- Checkbox с кастомным стилем

### 5. Статистика
- Огромные цифры (2.5rem) с неоновым свечением
- Uppercase метки
- Пиксельные углы на карточках
- Анимированный WebSocket индикатор

### 6. Статус-бар
- Компактный дизайн с monospace шрифтом
- Цветовая индикация: зелёный (успех), жёлтый (warning), красный (ошибка)
- Uppercase текст для статусов
- Пульсирующая точка для активного соединения

### 7. Toast уведомления
- Тёмный фон с неоновой границей
- Uppercase заголовки
- Slide-in анимация справа
- Иконки ✓ и ✗

## 🎬 Анимации

### Fade In (для контента страниц)
```css
@keyframes fadeIn {
  from { opacity: 0; transform: translateY(10px); }
  to { opacity: 1; transform: translateY(0); }
}
```
- Задержка между элементами: 60ms
- Создаёт эффект последовательного появления

### Slide In (для toast)
```css
@keyframes slideIn {
  from { transform: translateX(100%); opacity: 0; }
  to { transform: translateX(0); opacity: 1; }
}
```

### Pulse (для индикаторов)
```css
@keyframes pulse {
  0%, 100% { opacity: 1; transform: scale(1); }
  50% { opacity: 0.6; transform: scale(1.1); }
}
```

### Button Shine (световая волна)
- Градиент движется слева направо при hover
- Создаёт эффект "сканирования"

## 🌐 Фоновая сетка

```css
body::before {
  background-image:
    linear-gradient(rgba(57,255,106,0.12) 1px, transparent 1px),
    linear-gradient(90deg, rgba(57,255,106,0.12) 1px, transparent 1px);
  background-size: 20px 20px;
  opacity: 0.4;
}
```

## 📱 Адаптивность

- Breakpoint: 768px
- Мобильная навигация: вертикальная компоновка
- Grid: 1 колонка на мобильных
- Сохранение всех эффектов и анимаций

## 🚀 Запуск

```bash
cd frontend
npm run dev
```

Откройте: http://localhost:5174

## ✨ Особенности реализации

1. **CSS Variables** - Все цвета и размеры через переменные
2. **Semantic HTML** - Правильная структура для доступности
3. **Performance** - CSS-only анимации (GPU acceleration)
4. **Consistency** - Единый стиль во всех компонентах
5. **Scalability** - Легко добавлять новые компоненты

## 🎮 Вдохновение

- Ретро-аркадные игры 80-х
- Киберпанк эстетика
- Неоновые вывески
- Матричная сетка
- Тактические дашборды

## 📊 Результат

✅ Полностью функциональный UI
✅ Соответствие design-system.json
✅ Все компоненты обновлены
✅ Анимации и эффекты работают
✅ Адаптивный дизайн
✅ Высокая производительность

---

**Дизайн-система**: snake-on-the-throne-neon-grid-v1  
**Версия**: 1.0.0  
**Дата**: 2026-04-24
