# 🎤 Как работает аудио-транскрипция

Полное объяснение системы захвата и транскрипции аудио со стримов.

---

## 📊 Архитектура системы

```
┌─────────────────┐
│  Twitch Stream  │
└────────┬────────┘
         │
         ▼
┌─────────────────┐
│   Streamlink    │ ← Захватывает аудио поток
└────────┬────────┘
         │
         ▼
┌─────────────────┐
│     FFmpeg      │ ← Нарезает на чанки (8 сек)
└────────┬────────┘
         │
         ▼
┌─────────────────┐
│   STT Server    │ ← Транскрибирует (whisper.cpp)
│  (port 9000)    │
└────────┬────────┘
         │
         ▼
┌─────────────────┐
│ Audio Service   │ ← Хранит контекст (последние 2 мин)
│   (Go backend)  │
└────────┬────────┘
         │
         ▼
┌─────────────────┐
│   LLM Client    │ ← Использует контекст для генерации
└─────────────────┘
```

---

## 🔄 Процесс работы (пошагово)

### 1. **Захват аудио потока** (Streamlink)

**Что происходит:**
- Backend запускает `streamlink` для получения прямой ссылки на аудио поток
- Streamlink парсит Twitch и возвращает URL на `.m3u8` плейлист

**Команда:**
```bash
streamlink https://twitch.tv/channel_name best --stream-url
```

**Результат:**
```
https://video-weaver.fra05.hls.twitchcdn.net/v1/playlist/...m3u8
```

**Кеширование:**
- URL кешируется на 5 минут (чтобы не спамить Twitch)
- При истечении кеша — запрашивается заново

---

### 2. **Нарезка на чанки** (FFmpeg)

**Что происходит:**
- FFmpeg читает поток по URL и нарезает на аудио-чанки
- Каждый чанк = 8 секунд аудио в формате WAV

**Команда:**
```bash
ffmpeg -hide_banner -loglevel error \
  -i "https://...m3u8" \
  -t 8 \                    # Длительность чанка
  -vn \                     # Без видео
  -acodec pcm_s16le \       # Аудио кодек
  -ar 16000 \               # Sample rate 16kHz
  -ac 1 \                   # Моно
  -f wav \                  # Формат WAV
  pipe:1                    # Вывод в stdout
```

**Параметры:**
- `ChunkSeconds: 8` — длина чанка (настраивается в .env)
- `16kHz mono WAV` — оптимально для Whisper
- Timeout: `ChunkSeconds + 20 сек` (на случай задержек)

**Результат:**
- Бинарные данные WAV в памяти ([]byte)

---

### 3. **Транскрипция** (whisper.cpp)

**Что происходит:**
- Audio Service отправляет WAV-чанк на STT-сервер
- Официальный контейнер `ghcr.io/ggml-org/whisper.cpp:main` запускает `whisper-server`
- Используется локальная модель `models/ggml-small-q5_1.bin`, смонтированная в `/models`

**HTTP запрос:**
```http
POST http://stt:9000/v1/audio/transcriptions
Content-Type: multipart/form-data

file: chunk.wav (binary)
model: whisper-1
language: ru
response_format: json
```

`model` остаётся для OpenAI-совместимой формы запроса; `whisper-server` его игнорирует.
`response_format=json` заставляет сервер вернуть `{"text": "..."}`.

**Контейнер:**
```yaml
stt:
  image: ghcr.io/ggml-org/whisper.cpp:main
  entrypoint: ["whisper-server"]
  command:
    - --host
    - 0.0.0.0
    - --port
    - "9000"
    - -m
    - /models/ggml-small-q5_1.bin
    - -t
    - "4"
    - -l
    - ru
    - --inference-path
    - /v1/audio/transcriptions
  volumes:
    - ./models:/models:ro
```

**Модель:**
- файл: `models/ggml-small-q5_1.bin` (multilingual `small`, q5_1);
- в репозиторий не коммитится (`models/` в `.gitignore`);
- путь внутри контейнера задаётся через `STT_MODEL_FILE`;
- язык по умолчанию — `STT_LANGUAGE=ru`, но поле `language` из запроса имеет приоритет.

**Результат:**
```json
{
  "text": "Привет чат как дела сегодня играем в доту"
}
```

---

### 4. **Обработка и хранение контекста**

**Что происходит:**
- Текст очищается (удаляются лишние символы)
- Проверяется на дубликаты (если такой же текст был недавно — игнорируется)
- Добавляется в список сегментов с временной меткой

**Структура сегмента:**
```go
type segment struct {
    Text string      // "Привет чат как дела"
    At   time.Time   // 2026-04-24 12:30:15
}
```

**Хранение:**
- Сегменты хранятся в памяти (массив)
- Автоматическая очистка старых сегментов (старше 2 минут)
- Максимум 320 символов в контексте

**Пример контекста:**
```
"Привет чат как дела | сегодня играем в доту | кто хочет в пати"
```

---

### 5. **Использование в LLM**

**Что происходит:**
- Когда бот генерирует сообщение, он получает аудио-контекст
- Контекст добавляется в промпт для LLM

**Промпт без аудио:**
```
System: Ты весёлый фанат стримера...
User: Напиши позитивный комментарий
```

**Промпт с аудио:**
```
System: Ты весёлый фанат стримера...
User: Напиши позитивный комментарий

Контекст из стрима:
"Привет чат как дела | сегодня играем в доту | кто хочет в пати"
```

**Результат:**
- Бот генерирует релевантное сообщение: "Го в пати, я саппорт!"

---

## ⚙️ Настройки (.env)

```env
# STT сервер
AUDIO_STT_URL=http://stt:9000/v1/audio/transcriptions
AUDIO_STT_MODEL=whisper-1
AUDIO_STT_API_KEY=                    # Не требуется для локального
AUDIO_STT_LANGUAGE=ru                 # Язык транскрипции

# Параметры захвата
AUDIO_CHUNK_SEC=8                     # Длина чанка (секунды)
AUDIO_POLL_SEC=15                     # Интервал между захватами
AUDIO_STREAM_QUALITY=best             # Качество потока

# Контекст
AUDIO_CONTEXT_TTL_SEC=120             # Время жизни сегментов (2 мин)
AUDIO_CONTEXT_MAX_CHARS=320           # Макс символов в контексте

# Бинарники
AUDIO_STREAMLINK_CMD=streamlink
AUDIO_FFMPEG_CMD=ffmpeg

# Опционально: прямой URL вместо Twitch
# AUDIO_SOURCE_URL=https://example.com/stream.m3u8
```

---

## 🔧 Настройки STT

```env
# Путь к модели внутри /models (файл лежит в ./models на хосте)
STT_MODEL_FILE=ggml-small-q5_1.bin
# Количество потоков CPU
STT_THREADS=4
# Язык по умолчанию; language из запроса имеет приоритет
STT_LANGUAGE=ru
```

Модель `ggml-small-q5_1.bin` — multilingual Whisper `small` в квантовании q5_1.
Это компромисс между качеством и скоростью на CPU. Для более высокой точности можно
положить другую GGML-модель в `models/` и указать её имя в `STT_MODEL_FILE`.

---

## 📈 Производительность

Время зависит от CPU и длины чанка. На типичном CPU `small` обрабатывает 8-секундный
чанк за несколько секунд; при четырех потоках цикл укладывается в интервал опроса.

```
Захват (8 сек) → Транскрипция (несколько сек) → Ожидание (15 сек) → Повтор
```

**Итого:** новый контекст примерно каждые 15 секунд.

---

## 🐛 Troubleshooting

### Транскрипция не работает

**Проверка 1: STT сервер запущен**
```bash
docker compose --profile audio ps stt
curl http://localhost:9000/health
# Должен вернуть: {"status":"ok"}
```

**Проверка 2: Streamlink установлен**
```bash
docker exec twitch-smm-backend streamlink --version
```

**Проверка 3: FFmpeg установлен**
```bash
docker exec twitch-smm-backend ffmpeg -version
```

**Проверка 4: Логи**
```bash
docker compose logs backend | grep audio
docker compose logs stt
```

### Пустые транскрипции

**Причины:**
- Стример молчит (нет речи в чанке)
- Музыка/шум вместо речи
- Неправильный язык (`AUDIO_STT_LANGUAGE`)

**Решение:**
- Увеличьте `AUDIO_CHUNK_SEC` до 10-15 секунд
- Проверьте что стример говорит в момент захвата
- Убедитесь что язык правильный (`ru`, `en`, etc)

### Медленная транскрипция

**Причины:**
- Слабый CPU
- Большая модель (`medium`, `large`)

**Решение:**
- Используйте модель меньше (`base`/`tiny`) через `STT_MODEL_FILE`
- Увеличьте `STT_THREADS` под количество ядер CPU
- Уменьшите `AUDIO_CHUNK_SEC` до 5-6 секунд

### Высокое использование памяти

**Причины:**
- Большая модель Whisper
- Утечки памяти

**Решение:**
- Используйте `small` вместо `medium`/`large`
- Перезапустите STT: `docker compose restart stt`

---

## 🔍 Мониторинг

### Проверка работы в реальном времени:

```bash
# Логи audio service
docker compose logs backend -f | grep "\[audio\]"

# Вывод:
# [audio] captured 128KB chunk
# [audio] transcribed: "Привет чат как дела"
# [audio] context updated (3 segments, 85 chars)
```

### Health check STT:

```bash
curl http://localhost:9000/health | jq
```

```json
{
  "status": "ok"
}
```

---

## 💡 Оптимизация

### Для быстрой транскрипции:
```env
STT_MODEL_FILE=ggml-base-q5_1.bin
STT_THREADS=4
AUDIO_CHUNK_SEC=6
```

### Для высокого качества:
```env
STT_MODEL_FILE=ggml-small-q5_1.bin
STT_THREADS=6
AUDIO_CHUNK_SEC=10
```

### Для экономии ресурсов:
```env
STT_MODEL_FILE=ggml-tiny-q5_1.bin
STT_THREADS=2
AUDIO_CHUNK_SEC=5
AUDIO_POLL_SEC=30
```

---

## 📝 Примечания

1. **Загрузка модели**: `whisper-server` загружает модель при старте контейнера; healthcheck имеет `start_period: 60s`
2. **Формат ответа**: `response_format=json` возвращает `{"text": "..."}`
3. **Тишина**: Whisper может галлюцинировать на полной тишине; пустой `text` не считается ошибкой
4. **Дедупликация**: Одинаковые тексты подряд игнорируются
5. **Контекст**: Хранится только последние 2 минуты (120 сек)

---

**Версия:** 1.0  
**Последнее обновление:** 2026-04-24
