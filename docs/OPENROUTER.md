# OpenRouter Integration Guide

OpenRouter предоставляет доступ к множеству LLM моделей через единый API, включая бесплатные модели.

## Быстрый старт

1. Получите API ключ на [openrouter.ai](https://openrouter.ai/keys)

2. Настройте `.env`:
```env
LLM_PROVIDER=openrouter
LLM_API_URL=https://openrouter.ai/api/v1
LLM_MODEL=meta-llama/llama-3.1-8b-instruct:free
LLM_API_KEY=sk-or-v1-your-openrouter-key
```

3. Перезапустите сервисы:
```bash
docker compose restart backend
```

## Рекомендуемые модели

### Бесплатные модели
- `meta-llama/llama-3.1-8b-instruct:free` — быстрая, хорошо подходит для чата
- `google/gemini-flash-1.5:free` — очень быстрая от Google
- `mistralai/mistral-7b-instruct:free` — компактная и эффективная

### Платные модели (низкая стоимость)
- `anthropic/claude-3-haiku` — быстрый Claude
- `openai/gpt-4o-mini` — экономичный GPT-4
- `google/gemini-pro-1.5` — мощный Gemini

## Особенности интеграции

### Автоматическая настройка URL
Если указан `LLM_PROVIDER=openrouter`, URL автоматически устанавливается в `https://openrouter.ai/api/v1`

### Дополнительные заголовки
OpenRouter требует заголовки для аналитики:
- `HTTP-Referer`: идентификатор приложения
- `X-Title`: название приложения

Эти заголовки добавляются автоматически.

## Проверка работы

```bash
# Проверить логи backend
docker compose logs backend --tail=50

# Должны увидеть успешные запросы к OpenRouter
```

## Сравнение с другими провайдерами

| Провайдер | Плюсы | Минусы |
|-----------|-------|--------|
| **OpenRouter** | Множество моделей, есть бесплатные, единый API | Требует интернет |
| **LM Studio** | Полностью локально, бесплатно | Требует мощное железо |
| **OpenAI** | Высокое качество | Дорого |
| **Omniroute** | Балансировка нагрузки | Требует подписку |

## Troubleshooting

### Ошибка 401 Unauthorized
- Проверьте правильность API ключа
- Убедитесь что ключ начинается с `sk-or-v1-`

### Ошибка 429 Rate Limit
- Бесплатные модели имеют лимиты
- Попробуйте другую бесплатную модель
- Или перейдите на платную модель

### Медленные ответы
- Выберите более быструю модель (gemini-flash, llama-3.1-8b)
- Проверьте интернет соединение

## Полезные ссылки

- [OpenRouter Dashboard](https://openrouter.ai/activity)
- [Список моделей](https://openrouter.ai/models)
- [Документация API](https://openrouter.ai/docs)
- [Цены](https://openrouter.ai/models?order=newest)
