# Twitch Bot Farm

Система управления Twitch-ботами с UI, LLM-генерацией сообщений и локальной аудио-транскрипцией.

---

## 🚀 Быстрый старт

### 1. Установите Docker
Скачайте [Docker Desktop](https://www.docker.com/products/docker-desktop/)

### 2. Установите LM Studio

- Скачайте [LM Studio](https://lmstudio.ai/)
- Загрузите модель (например, Google Gemma 3 4B или Llama 3.2 3B)
- Запустите Local Server на порту 1234

### 3. Настройте проект

```bash
# Создайте .env файл
copy .env.example .env
```

Откройте `.env` и настройте:

```env
API_KEY=secret-key-12345
LLM_PROVIDER=lmstudio
LLM_API_URL=http://127.0.0.1:1234/v1
LLM_MODEL=google/gemma-3-4b
```

### 4. Запустите проект

```bash
docker compose up -d --build
```

Подождите 2-3 минуты пока все сервисы запустятся.

### 5. Создайте бота

1. Откройте: **http://localhost:3000**

2. **Настройки** → добавьте канал:
   - Введите имя канала (без #): `имя_стримера`
   - Нажмите **"Сохранить все настройки"**

3. **Боты** → **"Добавить бота"**:
   - **Twitch Login**: `имя_вашего_бота`
   - **Twitch Password**: `пароль_от_аккаунта`
   - **Имя**: `Любое имя`
   - **Личность**: `веселый_фанат`
   - **Min/Max задержка**: `45` / `180`
   - **Лимит**: `5`
   - Нажмите **"Сохранить"**

4. Нажмите **"Запустить всех"**

✅ **Готово!** Бот начнет писать в чат.

---

## 🎯 Возможности

✅ Управление ботами через UI  
✅ 4 типа личностей (веселый_фанат, токсик, нейтрал, хайпер)  
✅ Автоматическая ротация между каналами  
✅ Аудио-транскрипция стримов (Whisper)  
✅ Контекстные ответы через LLM  
✅ WebSocket real-time обновления  
✅ Rate limiting (Redis)  
✅ Темная тема UI  

---

## 💡 Поддерживаемые LLM

- **LM Studio** — локально, бесплатно, автоматическая инициализация моделей

---

## 🔧 Настройка каналов

Каналы задаются через UI: **Settings → channels**

`TWITCH_CHANNEL` в `.env` используется только как fallback.

---

## 🎤 Аудио-транскрипция (опционально)

Боты могут слушать стрим и генерировать контекстные ответы.

**Включение:**
1. В UI включить **"Включить анализ аудио со стрима"**
2. Нажать **"Сохранить все настройки"**
3. Проверить логи: `docker compose logs backend stt`

**Технологии:**
- `streamlink` — захват аудио потока
- `ffmpeg` — нарезка чанков
- `faster-whisper` — транскрипция (OpenAI-compatible API)

📖 **Детальная документация:** [docs/AUDIO_TRANSCRIPTION.md](docs/AUDIO_TRANSCRIPTION.md)

---

## 📊 Полезные команды

```bash
# Проверить статус
docker compose ps

# Логи
docker compose logs backend --tail=100 -f
docker compose logs stt --tail=100 -f

# Перезапуск
docker compose restart backend

# Остановка
docker compose down

# Полная пересборка
docker compose down
docker compose up -d --build --force-recreate
```

---

## 🔗 Доступные сервисы

- **Frontend**: http://localhost:3000
- **Backend API**: http://localhost:8080
- **STT Health**: http://localhost:9000/health
- **PgAdmin**: http://localhost:5050 (admin@local.dev / admin)

---

## 🐛 Troubleshooting

### Боты не подключаются к Twitch
- Проверьте правильность логина и пароля
- Используйте **пароль**, не OAuth токен
- Убедитесь что аккаунт не забанен

### LLM не отвечает

**OpenRouter:**
```bash
# Проверьте API ключ
export LLM_API_KEY=sk-or-v1-your-key
./scripts/test_openrouter.sh
```

**LM Studio:**
- Убедитесь что LM Studio запущен
- Проверьте Local Server на порту 1234
- Откройте: http://localhost:1234/v1/models

### Сервисы не запускаются
```bash
docker compose down
docker compose up -d --build --force-recreate
docker compose logs
```

### Frontend не открывается
```bash
docker compose logs frontend
docker compose restart frontend
```

---

## 📚 Дополнительная документация

### OpenRouter
- **Интеграция**: `docs/OPENROUTER.md`
- **Миграция с LM Studio**: `docs/MIGRATION_TO_OPENROUTER.md`
- **Тест API**: `scripts/test_openrouter.sh`

### Аудио-транскрипция
- **Полное руководство**: `docs/AUDIO_TRANSCRIPTION.md`

### Конфигурация
- **Пример .env**: `.env.example`
- **Docker Compose**: `docker-compose.yml`

---

## ⚙️ Переменные окружения

Полный список в `.env.example`. Основные:

```env
# Обязательные
API_KEY=your-secret-key
TWITCH_CHANNEL=                    # fallback, основной канал в UI

# LLM (выберите провайдер)
LLM_PROVIDER=openrouter            # lmstudio, openrouter, openai, omniroute
LLM_API_URL=https://openrouter.ai/api/v1
LLM_MODEL=meta-llama/llama-3.1-8b-instruct:free
LLM_API_KEY=sk-or-v1-your-key

# База данных (defaults)
DATABASE_URL=postgres://botfarm:botfarm@postgres:5432/twitch_bot_farm?sslmode=disable
REDIS_URL=redis://redis:6379

# STT (опционально)
AUDIO_STT_URL=http://stt:9000/v1/audio/transcriptions
STT_DEVICE=cpu                     # или cuda для GPU
STT_MODEL_NAME=small               # tiny, base, small, medium, large
```

---

## 🏗️ Архитектура

```
┌─────────────┐
│   Frontend  │ React + TypeScript + Vite
│  (port 3000)│
└──────┬──────┘
       │
       ▼
┌─────────────┐
│   Backend   │ Go + Gorilla Mux + WebSocket
│  (port 8080)│
└──────┬──────┘
       │
       ├──────► PostgreSQL (база данных)
       ├──────► Redis (кеш, rate limiting)
       ├──────► STT Server (faster-whisper)
       └──────► LLM API (OpenRouter/LM Studio/OpenAI)
```

**Компоненты:**
- `internal/api` — REST API, WebSocket, handlers
- `internal/twitch` — IRC клиент для Twitch
- `internal/llm` — интеграция с LLM провайдерами
- `internal/audio` — захват и транскрипция аудио
- `internal/storage` — PostgreSQL, настройки
- `internal/cache` — Redis, rate limiting
- `frontend` — React UI
- `stt_server` — Python FastAPI + faster-whisper

---

## 📝 Примечания

### STT Device
- `STT_DEVICE=cpu` — стабильный дефолт
- `STT_DEVICE=cuda` — для GPU ускорения (требует NVIDIA GPU)

### Личности ботов
- **веселый_фанат** — позитивные комментарии
- **токсик** — провокационные вопросы (не оскорбления)
- **нейтрал** — обычные вопросы и комментарии
- **хайпер** — энергичные сообщения со сленгом

### Rate Limiting
- Глобальный лимит: 30 сообщений/минуту (настраивается)
- Лимит на бота: настраивается индивидуально
- Используется Redis для синхронизации между ботами

---

## ⚠️ Ответственность

Используйте проект в рамках правил Twitch и местного законодательства.

---

## 🔗 Полезные ссылки

- **OpenRouter**: https://openrouter.ai
- **LM Studio**: https://lmstudio.ai
- **Twitch IRC**: https://dev.twitch.tv/docs/irc
- **Docker**: https://www.docker.com

---

**Версия:** 1.1.0  
**Последнее обновление:** 2026-04-24