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

По умолчанию используется LM Studio на вашем компьютере:

- запустите LM Studio → Local Server на порту `1234`;
- из контейнера адрес — `http://host.docker.internal:1234/v1`.

Провайдер, URL, модель и ключ можно поменять в UI (**Settings → LLM**) — подойдёт любой OpenAI-совместимый API.

Если LLM недоступен, backend и UI всё равно запускаются, но боты не смогут генерировать сообщения.

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
# Локальная транскрипция аудио стрима (faster-whisper, тяжёлый образ)
docker compose --profile audio up -d --build

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
internal/llm   OpenAI-совместимый LLM-клиент
internal/audio streamlink + ffmpeg + STT
internal/storage PostgreSQL, схема создаётся автоматически при старте
stt_server/    FastAPI + faster-whisper (опциональный профиль audio)
```

Схема БД создаётся и обновляется самим приложением при старте (`EnsureSchema` / `EnsureSettingsTable`), поэтому init-скрипты в `db/` для запуска не нужны.

## Траблшутинг

**`docker compose up` падает на сборке.** Проверьте, что Docker Desktop запущен и есть интернет. При блокировке Docker Hub используйте `BASE_IMAGE_PREFIX`.

**Порт занят.** Поменяйте левую часть порта в `docker-compose.yml` (`3000:80`, `8080:8080`, `5432:5432`, `6379:6379`).

**UI открывается, но запросы к API падают.** Убедитесь, что `API_KEY` в `.env` совпадает с ключом, с которым собран frontend: после изменения выполните `docker compose up -d --build`.

**Сбросить всё.** `docker compose down -v` — удалит базу, redis и кэш моделей.
