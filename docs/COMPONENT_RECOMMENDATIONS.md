# 🎨 КОМПОНЕНТЫ ИЗ REACTBITS.DEV И TWEAKCN.COM

## Рекомендации по интеграции

### React Bits (https://reactbits.dev/)
Библиотека анимированных UI компонентов для React

#### Подходящие компоненты для Twitch Bot Farm:

**1. Animated Cards**
- Для карточек ботов
- Hover эффекты с 3D трансформацией
- Плавные переходы

**2. Glitch Effects**
- Для статус индикаторов
- Cyber эстетика
- Эффект "сбоя" при ошибках

**3. Neon Buttons**
- Уже реализовано через CSS
- Можно улучшить анимацией

**4. Progress Bars**
- Для отображения активности ботов
- Неоновый стиль
- Анимированное заполнение

**5. Toast Notifications**
- Уже есть, можно улучшить
- Добавить glitch эффект
- Slide + fade анимации

#### Как интегрировать:

```bash
npm install @reactbits/components
```

```tsx
import { AnimatedCard, GlitchText, NeonButton } from '@reactbits/components'

// Пример использования
<AnimatedCard className="bot-card">
  <GlitchText>BOT STATUS</GlitchText>
  <NeonButton onClick={handleStart}>START</NeonButton>
</AnimatedCard>
```

### TweakCN (https://tweakcn.com/)
Редактор и генератор тем для shadcn/ui

#### Что можно взять:

**1. Cyber Theme Preset**
- Готовая тема в cyber стиле
- Неоновые цвета
- Тёмный фон

**2. Component Variants**
- Кнопки с различными эффектами
- Карточки с borders
- Input с focus states

**3. Animation Presets**
- Готовые CSS анимации
- Timing functions
- Keyframes

#### Как использовать:

1. Зайти на https://tweakcn.com/
2. Выбрать "Cyber" или "Dark Tech" тему
3. Настроить цвета под нашу палитру:
   - Primary: #39FF6A
   - Background: #010B2A
   - Surface: #061038
4. Экспортировать CSS variables
5. Скопировать в main.css

## Конкретные улучшения для проекта

### 1. Анимированные карточки ботов

```tsx
// BotCard.tsx
import { motion } from 'framer-motion'

export const BotCard = ({ bot }) => {
  return (
    <motion.div
      className="bot-card"
      whileHover={{ 
        scale: 1.02,
        boxShadow: 'var(--neon-glow)'
      }}
      whileTap={{ scale: 0.98 }}
      initial={{ opacity: 0, y: 20 }}
      animate={{ opacity: 1, y: 0 }}
      transition={{ duration: 0.3 }}
    >
      {/* Контент карточки */}
    </motion.div>
  )
}
```

### 2. Glitch эффект для ошибок

```css
/* Добавить в main.css */
@keyframes glitch {
  0% {
    transform: translate(0);
  }
  20% {
    transform: translate(-2px, 2px);
  }
  40% {
    transform: translate(-2px, -2px);
  }
  60% {
    transform: translate(2px, 2px);
  }
  80% {
    transform: translate(2px, -2px);
  }
  100% {
    transform: translate(0);
  }
}

.error-state {
  animation: glitch 0.3s infinite;
}
```

### 3. Прогресс-бар активности

```tsx
// ActivityBar.tsx
export const ActivityBar = ({ value, max }) => {
  const percentage = (value / max) * 100
  
  return (
    <div className="activity-bar">
      <div 
        className="activity-bar__fill"
        style={{ 
          width: `${percentage}%`,
          boxShadow: 'var(--neon-glow)'
        }}
      />
    </div>
  )
}
```

```css
.activity-bar {
  width: 100%;
  height: 8px;
  background: var(--bg-base);
  border: 1px solid var(--border-default);
  border-radius: var(--radius-sm);
  overflow: hidden;
}

.activity-bar__fill {
  height: 100%;
  background: linear-gradient(90deg, var(--primary-600), var(--primary-500));
  transition: width 0.5s var(--easing);
  position: relative;
}

.activity-bar__fill::after {
  content: '';
  position: absolute;
  top: 0;
  left: 0;
  right: 0;
  bottom: 0;
  background: linear-gradient(
    90deg,
    transparent,
    rgba(255,255,255,0.3),
    transparent
  );
  animation: shimmer 2s infinite;
}

@keyframes shimmer {
  0% { transform: translateX(-100%); }
  100% { transform: translateX(100%); }
}
```

### 4. Улучшенные кнопки с ripple эффектом

```tsx
// Button.tsx
import { useState } from 'react'

export const RippleButton = ({ children, onClick, ...props }) => {
  const [ripples, setRipples] = useState([])

  const handleClick = (e) => {
    const rect = e.currentTarget.getBoundingClientRect()
    const x = e.clientX - rect.left
    const y = e.clientY - rect.top
    
    const newRipple = { x, y, id: Date.now() }
    setRipples([...ripples, newRipple])
    
    setTimeout(() => {
      setRipples(ripples.filter(r => r.id !== newRipple.id))
    }, 600)
    
    onClick?.(e)
  }

  return (
    <button className="btn" onClick={handleClick} {...props}>
      {children}
      {ripples.map(ripple => (
        <span
          key={ripple.id}
          className="ripple"
          style={{ left: ripple.x, top: ripple.y }}
        />
      ))}
    </button>
  )
}
```

```css
.btn {
  position: relative;
  overflow: hidden;
}

.ripple {
  position: absolute;
  width: 20px;
  height: 20px;
  border-radius: 50%;
  background: rgba(255,255,255,0.5);
  transform: translate(-50%, -50%) scale(0);
  animation: ripple-animation 0.6s ease-out;
  pointer-events: none;
}

@keyframes ripple-animation {
  to {
    transform: translate(-50%, -50%) scale(20);
    opacity: 0;
  }
}
```

### 5. Статус индикатор с пульсацией

```tsx
// StatusIndicator.tsx
export const StatusIndicator = ({ status }) => {
  const colors = {
    running: 'var(--state-success)',
    stopped: 'var(--text-muted)',
    error: 'var(--state-error)'
  }

  return (
    <div className="status-indicator">
      <div 
        className={`status-indicator__dot ${status === 'running' ? 'pulse' : ''}`}
        style={{ background: colors[status] }}
      />
      <span className="status-indicator__label">
        {status === 'running' ? '● РАБОТАЕТ' : '○ ОСТАНОВЛЕН'}
      </span>
    </div>
  )
}
```

### 6. Cyber Grid Background (улучшенный)

```css
/* Добавить в main.css */
body::after {
  content: '';
  position: fixed;
  top: 0;
  left: 0;
  right: 0;
  bottom: 0;
  background-image:
    linear-gradient(rgba(57,255,106,0.03) 2px, transparent 2px),
    linear-gradient(90deg, rgba(57,255,106,0.03) 2px, transparent 2px),
    linear-gradient(rgba(57,255,106,0.02) 1px, transparent 1px),
    linear-gradient(90deg, rgba(57,255,106,0.02) 1px, transparent 1px);
  background-size: 100px 100px, 100px 100px, 20px 20px, 20px 20px;
  pointer-events: none;
  z-index: 0;
  animation: grid-move 20s linear infinite;
}

@keyframes grid-move {
  0% { background-position: 0 0, 0 0, 0 0, 0 0; }
  100% { background-position: 100px 100px, 100px 100px, 20px 20px, 20px 20px; }
}
```

## Библиотеки для установки

```bash
# Анимации
npm install framer-motion

# Иконки (если нужны дополнительные)
npm install lucide-react

# Утилиты для классов
npm install clsx tailwind-merge

# shadcn/ui компоненты (опционально)
npx shadcn-ui@latest init
```

## Приоритетные улучшения

1. ✅ **Шрифты** - Уже сделано (Exo 2)
2. 🔄 **Анимации карточек** - Framer Motion
3. 🔄 **Glitch эффекты** - CSS keyframes
4. 🔄 **Прогресс-бары** - Для активности ботов
5. 🔄 **Ripple кнопки** - Улучшенный feedback
6. 🔄 **Улучшенный grid** - Анимированный фон

## Следующие шаги

1. Установить framer-motion
2. Добавить анимации для карточек
3. Реализовать glitch эффект для ошибок
4. Добавить прогресс-бары
5. Улучшить кнопки с ripple
6. Анимировать grid background

---

**Источники**: reactbits.dev, tweakcn.com  
**Статус**: 📋 Рекомендации готовы  
**Дата**: 2026-04-24
