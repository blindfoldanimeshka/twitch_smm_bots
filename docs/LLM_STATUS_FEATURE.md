# LLM Provider Status Feature

## Описание
Добавлена система мониторинга статуса LLM провайдера с отображением в UI и автоматическим fallback при недоступности.

## Backend изменения

### 1. API endpoint для проверки статуса
- **Endpoint**: `GET /api/llm/status`
- **Response**:
```json
{
  "provider": "openrouter",
  "available": true,
  "model": "meta-llama/llama-3.1-8b-instruct:free",
  "url": "https://openrouter.ai/api/v1"
}
```

### 2. Файлы изменены:
- `internal/llm/lmstudio.go` - добавлены методы `GetModel()` и `GetURL()`
- `internal/api/dto.go` - добавлен `LLMStatusDTO`
- `internal/api/handlers.go` - добавлен handler `GetLLMStatus()`
- `internal/api/server.go` - добавлен route `/api/llm/status`
- `logic.go` - добавлена проверка `llmClient != nil` перед генерацией

## Frontend изменения

### 1. StatusBar компонент
- Расположение: под Navigation header
- Автоматическое обновление каждые 30 секунд
- Три состояния:
  - 🟢 Зеленый: провайдер подключен и работает
  - 🟡 Желтый: провайдер недоступен (показывает ошибку)
  - 🔴 Красный: не удалось получить статус

### 2. Файлы изменены:
- `frontend/src/api/client.ts` - добавлен интерфейс `LLMStatus` и метод `getLLMStatus()`
- `frontend/src/components/StatusBar.tsx` - новый компонент
- `frontend/src/App.tsx` - добавлен `<StatusBar />` под Navigation

## Fallback логика

Если LLM провайдер недоступен:
1. Backend логирует ошибку: `[bot_id] llm error: ...`
2. Бот пропускает отправку сообщения
3. После 5 последовательных ошибок бот останавливается
4. UI показывает статус недоступности провайдера
5. Пользователь видит уведомление и может переключить провайдера в настройках

## Тестирование

1. Запустить проект: `docker-compose up`
2. Открыть UI: `http://localhost:3000`
3. Проверить StatusBar под header
4. Остановить LLM провайдер и убедиться, что статус меняется на желтый/красный
5. Проверить логи ботов на наличие сообщений об ошибках LLM

## Обновление: Настройки LLM провайдера в UI

### Backend изменения

**Файлы изменены:**
- `internal/storage/settings.go` - добавлены поля `LLMProvider`, `LLMApiURL`, `LLMModel`, `LLMApiKey`
- `logic.go` - метод `ApplySettings()` теперь пересоздаёт LLM клиент при изменении настроек
- `internal/api/handlers.go` - обновление LLM клиента в handler при сохранении настроек
- `migrations/002_add_llm_settings.sql` - SQL миграция для добавления полей

**Новые поля в Settings:**
```go
LLMProvider      string    `json:"llm_provider"`      // lmstudio, openrouter, openai, omniroute, custom
LLMApiURL        string    `json:"llm_api_url"`       // http://127.0.0.1:1234/v1
LLMModel         string    `json:"llm_model"`         // meta-llama/llama-3.1-8b-instruct:free
LLMApiKey        string    `json:"llm_api_key"`       // sk-...
```

### Frontend изменения

**Файлы изменены:**
- `frontend/src/api/client.ts` - добавлены поля LLM в интерфейс `Settings`
- `frontend/src/pages/Settings.tsx` - новая секция "Настройки LLM провайдера"

**UI секция включает:**
- Выбор провайдера (LM Studio, OpenRouter, OpenAI, Omniroute, Custom)
- Автоматическая подстановка URL при выборе известного провайдера
- Поля для API URL, модели и API ключа
- Подсказки для каждого провайдера

### Применение настроек в runtime

При сохранении настроек через UI:
1. Настройки сохраняются в PostgreSQL
2. `ApplySettings()` в botManager пересоздаёт LLM клиент
3. Handler также обновляет свой LLM клиент
4. Выполняется health check нового клиента
5. StatusBar автоматически обновляется и показывает новый статус

### Миграция базы данных

Для существующих установок выполнить:
```bash
docker-compose exec postgres psql -U postgres -d twitchsmm -f /migrations/002_add_llm_settings.sql
```

Или пересоздать контейнеры:
```bash
docker-compose down
docker-compose up --build
```

### Пример использования

1. Открыть Settings в UI
2. Перейти в секцию "Настройки LLM провайдера"
3. Выбрать провайдера (например, OpenRouter)
4. Указать API ключ
5. Указать модель (например, `meta-llama/llama-3.1-8b-instruct:free`)
6. Нажать "Сохранить все настройки"
7. StatusBar покажет обновлённый статус подключения
