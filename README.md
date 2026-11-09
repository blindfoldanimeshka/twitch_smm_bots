# Twitch SMM Bot Farm

Система управления Twitch-ботами: веб-UI, LLM-генерация сообщений, ротация по каналам и опциональная транскрипция аудио стрима.

## Быстрый старт

Нужен только [Docker Desktop](https://www.docker.com/products/docker-desktop/).

**Windows:** двойной клик по `start.cmd`.

**Linux/macOS:**

```bash
./start.sh
```

Либо вручную:

```bash
docker compose up -d --build --wait
```

Скрипт сам запускает Docker Desktop (если он закрыт), собирает образы и ждёт, пока сервисы станут здоровыми. Первый запуск занимает 5–10 минут, последующие — секунды.

После запуска откройте **http://localhost:3000**.

Файл `.env` **не обязателен**: в `docker-compose.yml` уже есть рабочие значения по умолчанию. Нужен только если хотите что-то поменять — скопируйте `.env.example` в `.env`.

## Первый бот

1. **Settings → channels** — добавьте канал (имя без `#`), нажмите «Сохранить».
2. **Bots → Добавить бота** — Twitch login, OAuth-токен (`oauth:...`, получить на https://twitchapps.com/tmi/), имя, личность, задержки, лимит.
3. **Запустить** бота или «Запустить всех».

## LLM

Используется OpenRouter:

- бесплатная модель `openrouter/free` — первая попытка;
- платный fallback `z-ai/glm-5.3-flash` — только если бесплатная недоступна;
- дневной лимит платных расходов по умолчанию — `$1`, меняется в UI (**Settings → LLM**);
- ключ задаётся через `OPENROUTER_API_KEY` в `.env` или в UI. Ключ из окружения имеет приоритет.

Без ключа backend и UI запускаются, но боты не смогут генерировать сообщения. `/api/llm/status` покажет причину.

OpenRouter ограничивает бесплатный тариф 20 запросами в минуту. Всплески выше лимита могут уходить в платный fallback, но дневной лимит ограничивает расход.

## Сервисы

| Сервис    | Адрес                          |
|-----------|--------------------------------|
| UI        | http://localhost:3000          |
| API       | http://localhost:8080/api/health |
| PostgreSQL| `localhost:5432` (botfarm/botfarm) |
| Redis     | `localhost:6379`               |

API-ключ по умолчанию: `secret-api-key-12345` (меняется через `API_KEY` в `.env`).

## Управление

```bash
docker compose ps                     # статус
docker compose logs -f backend        # логи backend
docker compose logs -f frontend       # логи UI
docker compose down                   # остановить (данные сохраняются)
docker compose down -v                # остановить и удалить данные
```

Windows: `stop.cmd`.

## Опциональные сервисы

Они не собираются при обычном запуске.

```bash
# Локальная транскрипция аудио стрима (whisper.cpp)
# Перед запуском положите модель в models/ggml-small-q5_1.bin
docker compose --profile audio up -d

# pgAdmin: http://localhost:5050 (admin@local.dev / admin)
docker compose --profile tools up -d
```

После включения профиля `audio` включите анализ аудио в UI (**Settings**). Подробности: [docs/AUDIO_TRANSCRIPTION.md](docs/AUDIO_TRANSCRIPTION.md).

## Если Docker Hub недоступен

Укажите зеркало в `.env`:

```env
BASE_IMAGE_PREFIX=mirror.yandex.ru/mirrors/docker/library/
```

## Разработка без Docker

Нужны Go 1.26+ и Node.js 22+.

```bash
docker compose up -d postgres redis
go run .
cd frontend && npm ci && npm run dev
```

UI для разработки: http://localhost:5173 (CORS уже разрешён).

## Архитектура

```
frontend/      React + TypeScript + Vite (nginx в проде)
logic.go       точка входа backend: менеджер ботов, планировщик, консоль
internal/api   REST API, WebSocket, middleware
internal/twitch IRC-клиент Twitch
internal/llm   клиент OpenRouter (free-first + paid fallback)
internal/audio streamlink + ffmpeg + whisper.cpp
internal/storage PostgreSQL, схема создаётся автоматически при старте
models/        локальная модель whisper.cpp (не коммитится)
```

Схема БД создаётся и обновляется самим приложением при старте (`EnsureSchema` / `EnsureSettingsTable`), поэтому init-скрипты в `db/` для запуска не нужны.

## Траблшутинг

**`docker compose up` падает на сборке.** Проверьте, что Docker Desktop запущен и есть интернет. При блокировке Docker Hub используйте `BASE_IMAGE_PREFIX`.

**Порт занят.** Поменяйте левую часть порта в `docker-compose.yml` (`3000:80`, `8080:8080`, `5432:5432`, `6379:6379`).

**UI открывается, но запросы к API падают.** Убедитесь, что `API_KEY` в `.env` совпадает с ключом, с которым собран frontend: после изменения выполните `docker compose up -d --build`.

**Сбросить всё.** `docker compose down -v` — удалит базу и Redis.
