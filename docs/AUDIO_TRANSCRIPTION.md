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
│   STT Server    │ ← Транскрибирует (faster-whisper)
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

### 3. **Транскрипция** (STT Server)

**Что происходит:**
- Audio Service отправляет WAV чанк на STT сервер
- STT сервер использует `faster-whisper` для распознавания речи

**HTTP запрос:**
```http
POST http://stt:9000/v1/audio/transcriptions
Content-Type: multipart/form-data

file: audio.wav (binary)
model: whisper-1
language: ru
```

**STT Server (Python FastAPI):**
```python
# app.py
model = WhisperModel("small", device="cpu", compute_type="int8")

segments, _info = model.transcribe(
    audio_file,
    language="ru",
    vad_filter=True,    # Фильтр голосовой активности
    beam_size=1         # Быстрая транскрипция
)

text = " ".join(seg.text.strip() for seg in segments)
return {"text": text}
```

**Модели Whisper:**
- `tiny` — самая быстрая, низкое качество
- `base` — быстрая, приемлемое качество
- `small` — **рекомендуется** (баланс скорости/качества)
- `medium` — медленнее, лучше качество
- `large` — самая точная, очень медленная

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

## 🔧 Настройки STT модели

```env
# В docker-compose.yml для stt сервиса:
STT_MODEL_NAME=small                  # tiny, base, small, medium, large
STT_DEVICE=cpu                        # cpu или cuda
STT_COMPUTE_TYPE=int8                 # int8, float16, float32
```

**Рекомендации:**
- **CPU**: `small` + `int8` — лучший баланс
- **GPU (CUDA)**: `medium` + `float16` — быстрее и точнее
- **Слабое железо**: `tiny` + `int8` — минимальные требования

---

## 📈 Производительность

### Время обработки (на CPU):

| Модель | Чанк 8 сек | Память | Качество |
|--------|-----------|--------|----------|
| tiny   | ~1-2 сек  | 1 GB   | ⭐⭐     |
| base   | ~2-3 сек  | 1 GB   | ⭐⭐⭐   |
| small  | ~3-5 сек  | 2 GB   | ⭐⭐⭐⭐ |
| medium | ~8-12 сек | 5 GB   | ⭐⭐⭐⭐⭐ |
| large  | ~20+ сек  | 10 GB  | ⭐⭐⭐⭐⭐ |

### Цикл работы:

```
Захват (8 сек) → Транскрипция (3-5 сек) → Ожидание (15 сек) → Повтор
```

**Итого:** Новый контекст каждые ~15 секунд

---

## 🐛 Troubleshooting

### Транскрипция не работает

**Проверка 1: STT сервер запущен**
```bash
curl http://localhost:9000/health
# Должен вернуть: {"status":"ok","model":"small",...}
```

**Проверка 2: Streamlink установлен**
```bash
docker exec twitch-bot-backend streamlink --version
```

**Проверка 3: FFmpeg установлен**
```bash
docker exec twitch-bot-backend ffmpeg -version
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
- Используйте `small` или `tiny` модель
- Включите GPU: `STT_DEVICE=cuda` (требует NVIDIA GPU)
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
  "status": "ok",
  "model": "small",
  "device": "cpu",
  "model_loaded": true
}
```

---

## 💡 Оптимизация

### Для быстрой транскрипции:
```env
STT_MODEL_NAME=small
STT_COMPUTE_TYPE=int8
AUDIO_CHUNK_SEC=6
```

### Для высокого качества:
```env
STT_MODEL_NAME=medium
STT_DEVICE=cuda
STT_COMPUTE_TYPE=float16
AUDIO_CHUNK_SEC=10
```

### Для экономии ресурсов:
```env
STT_MODEL_NAME=tiny
AUDIO_CHUNK_SEC=5
AUDIO_POLL_SEC=30
```

---

## 📝 Примечания

1. **Lazy loading модели**: Модель Whisper загружается при первом запросе, не при старте сервера
2. **VAD фильтр**: Автоматически отсекает тишину и шум
3. **Beam size = 1**: Быстрая транскрипция без перебора вариантов
4. **Дедупликация**: Одинаковые тексты подряд игнорируются
5. **Контекст**: Хранится только последние 2 минуты (120 сек)

---

**Версия:** 1.0  
**Последнее обновление:** 2026-04-24
