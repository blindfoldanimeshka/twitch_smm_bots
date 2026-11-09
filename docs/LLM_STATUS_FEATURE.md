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
  "generation_available": true,
  "model": "openrouter/free",
  "url": "https://openrouter.ai/api/v1",
  "paid_enabled": true,
  "paid_daily_cap_usd": 1.0,
  "paid_spent_today_usd": 0.0,
  "free_requests_remaining": 1000
}
```

### 2. Файлы изменены:
- `internal/llm/openrouter.go` - методы `Status()`, `GetModel()` и `GetURL()`
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
LLMProvider      string    `json:"llm_provider"`      // openrouter
LLMApiURL        string    `json:"llm_api_url"`       // https://openrouter.ai/api/v1
LLMModel         string    `json:"llm_model"`         // openrouter/free
LLMPaidModel     string    `json:"llm_paid_model"`    // z-ai/glm-5.3-flash
LLMApiKey        string    `json:"llm_api_key"`       // never returned by the API
```

### Frontend изменения

**Файлы изменены:**
- `frontend/src/api/client.ts` - добавлены поля LLM в интерфейс `Settings`
- `frontend/src/pages/Settings.tsx` - новая секция "Настройки LLM провайдера"

**UI секция включает:**
- Провайдер: OpenRouter (единственный поддерживаемый)
- Поля: URL, бесплатная модель, платная модель, дневной лимит, API ключ
- Ключ никогда не возвращается в UI; показывается источник (`env`, `database`, `none`)

### Применение настроек в runtime

При сохранении настроек через UI:
1. Настройки сохраняются в PostgreSQL
2. `ApplySettings()` пересчитывает конфиг из env + БД и вызывает `llmClient.Reconfigure(cfg)`
3. Один и тот же клиент продолжает работать, без замены указателя
4. `/api/llm/status` сразу видит новые модели/лимиты
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
3. Выбрать OpenRouter (единственный вариант)
4. Указать API ключ (`OPENROUTER_API_KEY` имеет приоритет над ключом из БД)
5. Оставить бесплатную модель `openrouter/free` и платный fallback `z-ai/glm-5.3-flash`
6. Нажать "Сохранить все настройки"
7. StatusBar покажет обновлённый статус подключения
