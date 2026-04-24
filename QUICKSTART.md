# 🚀 Быстрый старт — Twitch Bot Farm

Полная инструкция по запуску проекта с нуля до работающих ботов.

---

## Предварительные требования

### Обязательно установить:
- ✅ **Docker Desktop** — [скачать](https://www.docker.com/products/docker-desktop/)
- ✅ **Git** — [скачать](https://git-scm.com/downloads)

### Что нужно получить:
- 🔑 **Twitch аккаунты** для ботов (логин + пароль)
- 🤖 **LLM API ключ** (один из вариантов):
  - **OpenRouter** (рекомендуется) — [получить бесплатно](https://openrouter.ai/keys)
  - **LM Studio** (локально) — [скачать](https://lmstudio.ai/)
  - **OpenAI** — [получить](https://platform.openai.com/api-keys)

---

## Шаг 1: Клонирование проекта

```bash
# Клонируйте репозиторий
git clone <your-repo-url>
cd twitchSmm

# Или если уже скачали, просто перейдите в папку
cd C:\Users\blindfold\Desktop\2026\twitchSmm
```

---

## Шаг 2: Настройка конфигурации

### Создайте файл `.env` из примера:

```bash
# Windows (PowerShell)
copy .env.example .env

# Linux/Mac
cp .env.example .env
```

### Откройте `.env` и настройте:

#### Вариант A: OpenRouter (рекомендуется, бесплатные модели)

```env
# Обязательные настройки
API_KEY=your-secret-key-12345
TWITCH_CHANNEL=

# LLM настройки (OpenRouter)
LLM_PROVIDER=openrouter
LLM_API_URL=https://openrouter.ai/api/v1
LLM_MODEL=meta-llama/llama-3.1-8b-instruct:free
LLM_API_KEY=sk-or-v1-ваш-ключ-с-openrouter
```

#### Вариант B: LM Studio (локально, без интернета)

```env
# Обязательные настройки
API_KEY=your-secret-key-12345
TWITCH_CHANNEL=

# LLM настройки (LM Studio)
LLM_PROVIDER=lmstudio
LLM_API_URL=http://host.docker.internal:1234/v1
LLM_MODEL=local-model
```

**Для LM Studio:**
1. Скачайте и запустите [LM Studio](https://lmstudio.ai/)
2. Загрузите модель (например, Llama 3.1 8B)
3. Запустите локальный сервер (вкладка "Local Server")
4. Убедитесь что порт 1234 открыт

---

## Шаг 3: Запуск проекта

### Запустите Docker Compose:

```bash
docker compose up -d --build
```

Это запустит 5 сервисов:
- 🗄️ **postgres** — база данных (порт 5432)
- 🔴 **redis** — кеш и rate limiting (порт 6379)
- 🎤 **stt** — локальная транскрипция аудио (порт 9000)
- 🔧 **backend** — Go API сервер (порт 8080)
- 🎨 **frontend** — React UI (порт 3000)

### Проверьте статус:

```bash
docker compose ps
```

Все сервисы должны быть в статусе `Up (healthy)`.

---

## Шаг 4: Откройте UI и настройте ботов

### Откройте браузер:

```
http://localhost:3000
```

### Создайте первого бота:

1. Перейдите в раздел **"Боты"**
2. Нажмите **"Добавить бота"**
3. Заполните форму:
   - **Twitch Login**: имя_бота
   - **Twitch Password**: пароль_от_аккаунта
   - **Имя**: Отображаемое имя
   - **Личность**: выберите (веселый_фанат, токсик, нейтрал, хайпер)
   - **Задержка**: 45-180 секунд (рекомендуется)
   - **Лимит**: 5-10 сообщений в минуту
4. Нажмите **"Сохранить"**

### Настройте целевые каналы:

1. Перейдите в **"Настройки"**
2. В разделе **"Целевые каналы"** добавьте каналы:
   - Например: `streamer_name` (без #)
   - Можно добавить несколько каналов
3. Нажмите **"Сохранить все настройки"**

---

## Шаг 5: Запустите ботов

1. Вернитесь в раздел **"Боты"**
2. Нажмите **"Запустить всех"** или запустите ботов по отдельности
3. Статус изменится на **"Running"** 🟢

### Проверьте работу:

```bash
# Смотрите логи backend
docker compose logs backend --tail=100 -f

# Должны увидеть:
# - Подключение ботов к IRC
# - Генерацию сообщений через LLM
# - Отправку сообщений в чат
```

---

## Шаг 6: Дополнительные настройки (опционально)

### Включить аудио-транскрипцию стрима:

1. Перейдите в **"Настройки"**
2. Включите **"Включить анализ аудио со стрима"**
3. Нажмите **"Сохранить все настройки"**

Боты будут получать контекст из аудио стрима и генерировать более релевантные сообщения.

### Включить ротацию ботов:

1. В **"Настройки"** включите **"Включить ротацию ботов"**
2. Установите интервал (например, 60 минут)
3. Боты будут автоматически переключаться между каналами

---

## Проверка работы

### Откройте Dashboard:

```
http://localhost:3000
```

Вы увидите:
- 📊 Статистику (активные боты, отправленные сообщения)
- 🤖 Список ботов с их статусами
- 📡 Real-time обновления через WebSocket

### Проверьте Twitch чат:

Откройте чат целевого канала на Twitch — боты должны писать сообщения.

---

## Остановка проекта

```bash
# Остановить все сервисы
docker compose down

# Остановить и удалить данные (БД, кеш)
docker compose down -v
```

---

## Troubleshooting

### Боты не подключаются к Twitch

**Проблема:** `IRC connection failed`

**Решение:**
- Проверьте правильность логина и пароля
- Убедитесь что используете пароль, а не OAuth токен
- Проверьте что аккаунт не забанен

### LLM не отвечает

**Проблема:** `LLM API error`

**Решение для OpenRouter:**
```bash
# Проверьте API ключ
export LLM_API_KEY=sk-or-v1-your-key
./scripts/test_openrouter.sh
```

**Решение для LM Studio:**
- Убедитесь что LM Studio запущен
- Проверьте что Local Server активен на порту 1234
- Попробуйте открыть: `http://localhost:1234/v1/models`

### Сервисы не запускаются

**Проблема:** `docker compose up` падает с ошибкой

**Решение:**
```bash
# Проверьте логи
docker compose logs

# Пересоберите образы
docker compose down
docker compose up -d --build --force-recreate
```

### Frontend не открывается

**Проблема:** `localhost:3000` не отвечает

**Решение:**
```bash
# Проверьте статус
docker compose ps frontend

# Проверьте логи
docker compose logs frontend

# Перезапустите
docker compose restart frontend
```

---

## Полезные команды

```bash
# Просмотр логов всех сервисов
docker compose logs -f

# Логи конкретного сервиса
docker compose logs backend -f
docker compose logs stt -f

# Перезапуск сервиса
docker compose restart backend

# Проверка статуса
docker compose ps

# Остановка одного сервиса
docker compose stop backend

# Запуск одного сервиса
docker compose start backend
```

---

## Дополнительные инструменты

### PgAdmin (управление БД):
```
http://localhost:5050
Email: admin@local.dev
Password: admin
```

### API документация:
```
http://localhost:8080/health
```

### STT health check:
```
http://localhost:9000/health
```

---

## Следующие шаги

1. ✅ Создайте несколько ботов с разными личностями
2. ✅ Настройте ротацию между каналами
3. ✅ Включите аудио-транскрипцию для контекстных ответов
4. ✅ Мониторьте статистику в Dashboard
5. ✅ Экспериментируйте с промптами в настройках

---

## Документация

- 📖 [OpenRouter интеграция](docs/OPENROUTER.md)
- 🔄 [Миграция на OpenRouter](docs/MIGRATION_TO_OPENROUTER.md)
- ✨ [Что работает](WHAT_WORKS.md)
- 📝 [Changelog](OPENROUTER_CHANGELOG.md)

---

**Готово!** Ваши боты работают 🎉

Если возникли проблемы — проверьте секцию Troubleshooting или логи сервисов.
