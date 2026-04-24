# OpenRouter Integration - Changelog

**Дата:** 2026-04-24  
**Версия:** 1.1.0

## Что добавлено

### 1. Поддержка OpenRouter в LLM клиенте
**Файл:** `internal/llm/lmstudio.go`

- Добавлен провайдер `openrouter` в список поддерживаемых
- Автоматическая установка URL `https://openrouter.ai/api/v1` для OpenRouter
- Добавлены обязательные заголовки для OpenRouter:
  - `HTTP-Referer: https://github.com/twitch-smm-bot`
  - `X-Title: Twitch SMM Bot`

### 2. Обновлена документация

**Файл:** `.env.example`
- Добавлен вариант 2 с настройками OpenRouter
- Пример бесплатной модели: `meta-llama/llama-3.1-8b-instruct:free`

**Файл:** `README.md`
- Добавлен пример конфигурации для OpenRouter
- Инструкции по быстрому старту

**Файл:** `WHAT_WORKS.md`
- Обновлен список поддерживаемых провайдеров
- Добавлена информация о новых возможностях

### 3. Новая документация

**Файл:** `docs/OPENROUTER.md`
- Полное руководство по интеграции с OpenRouter
- Список рекомендуемых моделей (бесплатных и платных)
- Сравнение с другими провайдерами
- Troubleshooting секция

**Файл:** `docs/MIGRATION_TO_OPENROUTER.md`
- Пошаговая инструкция миграции с LM Studio
- Примеры конфигурации до/после
- Инструкции по откату
- Рекомендации по выбору моделей

### 4. Тестовый скрипт

**Файл:** `scripts/test_openrouter.sh`
- Автоматическое тестирование API ключа
- Проверка доступных бесплатных моделей
- Тестирование нескольких моделей
- Готовые команды для добавления в `.env`

## Как использовать

### Быстрый старт

1. Получите API ключ на [openrouter.ai/keys](https://openrouter.ai/keys)

2. Обновите `.env`:
```env
LLM_PROVIDER=openrouter
LLM_API_URL=https://openrouter.ai/api/v1
LLM_MODEL=meta-llama/llama-3.1-8b-instruct:free
LLM_API_KEY=sk-or-v1-your-key
```

3. Перезапустите backend:
```bash
docker compose restart backend
```

### Тестирование

```bash
export LLM_API_KEY=sk-or-v1-your-key
./scripts/test_openrouter.sh
```

## Преимущества OpenRouter

✅ **Множество моделей** — доступ к 100+ моделям через единый API  
✅ **Бесплатные модели** — Llama 3.1, Gemini Flash, Mistral и другие  
✅ **Не требует локального железа** — работает через облако  
✅ **Быстрое переключение** — меняйте модели без перезапуска  
✅ **Мониторинг** — отслеживайте использование в dashboard  

## Обратная совместимость

Все существующие провайдеры продолжают работать:
- ✅ LM Studio (локально)
- ✅ Omniroute
- ✅ OpenAI
- ✅ Custom OpenAI-compatible APIs

## Технические детали

### Изменения в коде

```go
// Автоматическая настройка URL для OpenRouter
if provider == "openrouter" && baseURL == "http://127.0.0.1:1234/v1" {
    baseURL = "https://openrouter.ai/api/v1"
}

// Добавление специфичных заголовков
if c.provider == "openrouter" {
    req.Header.Set("HTTP-Referer", "https://github.com/twitch-smm-bot")
    req.Header.Set("X-Title", "Twitch SMM Bot")
}
```

### API совместимость

OpenRouter полностью совместим с OpenAI API:
- Endpoint: `/v1/chat/completions`
- Формат запроса/ответа: идентичен OpenAI
- Авторизация: `Bearer` токен

## Следующие шаги

1. Протестируйте интеграцию с вашим API ключом
2. Выберите подходящую модель из списка
3. Мониторьте использование в dashboard
4. При необходимости переключитесь на платную модель для лучшего качества

## Полезные ссылки

- [OpenRouter Dashboard](https://openrouter.ai/activity)
- [Список моделей](https://openrouter.ai/models)
- [Документация API](https://openrouter.ai/docs)
- [Получить API ключ](https://openrouter.ai/keys)

## Поддержка

При возникновении проблем:
1. Проверьте `docs/OPENROUTER.md` — troubleshooting секция
2. Запустите `./scripts/test_openrouter.sh` для диагностики
3. Проверьте логи: `docker compose logs backend --tail=100`
