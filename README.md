# Twitch Bot Farm

Система управления Twitch-ботами с UI, LLM-генерацией сообщений и локальной аудио-транскрипцией для контекстных ответов.

## Быстрый старт (Docker)

1. Убедитесь, что Docker запущен.
2. Запустите проект:

```bash
docker compose up -d --build
```

3. Откройте:
- Frontend: `http://localhost:3000`
- Backend API: `http://localhost:8080`
- STT health: `http://localhost:9000/health`

## Важная логика каналов

- Основной канал теперь задается в UI: `Settings -> channels`.
- `TWITCH_CHANNEL` в `.env` используется только как fallback.
- Если в UI каналы пустые, backend не падает, но предупреждает в логах.

## Локальная аудио-транскрипция

Используется связка:
- `streamlink` (получение аудио потока)
- `ffmpeg` (нарезка аудио-чанков)
- `stt_server` (faster-whisper, OpenAI-compatible endpoint `/v1/audio/transcriptions`)

Включение:
1. В UI включить `Включить анализ аудио со стрима`.
2. Нажать `Сохранить все настройки`.
3. Проверить логи backend/stt.

## Минимальные переменные `.env`

```env
API_KEY=your-secret-key
TWITCH_CHANNEL=
LLM_PROVIDER=lmstudio
LLM_API_URL=http://host.docker.internal:1234/v1
LLM_MODEL=local-model
```

**Для OpenRouter:**
```env
LLM_PROVIDER=openrouter
LLM_API_URL=https://openrouter.ai/api/v1
LLM_MODEL=meta-llama/llama-3.1-8b-instruct:free
LLM_API_KEY=sk-or-v1-your-key
```

## Полезные команды

```bash
docker compose ps
docker compose logs backend --tail=100
docker compose logs stt --tail=100
docker compose up -d --build
```

## Примечание по STT_DEVICE

- `STT_DEVICE=cpu` — стабильный дефолт.
- `STT_DEVICE=cuda` — используйте, если GPU корректно проброшена в контейнер и хотите ускорение транскрипции.

## Ответственность

Используйте проект в рамках правил Twitch и местного законодательства.
