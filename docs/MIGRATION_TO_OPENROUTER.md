# Миграция на OpenRouter

Быстрая инструкция по переходу с LM Studio на OpenRouter.

## Зачем переходить?

- ✅ Не нужно локальное железо для LLM
- ✅ Доступ к множеству моделей (включая бесплатные)
- ✅ Быстрее чем локальные модели на слабом железе
- ✅ Можно переключаться между моделями без перезагрузки

## Шаги миграции

### 1. Получите API ключ
Зарегистрируйтесь на [openrouter.ai](https://openrouter.ai/keys) и создайте ключ.

### 2. Обновите `.env`

**Было (LM Studio):**
```env
LLM_PROVIDER=lmstudio
LLM_API_URL=http://host.docker.internal:1234/v1
LLM_MODEL=local-model
```

**Стало (OpenRouter):**
```env
LLM_PROVIDER=openrouter
LLM_API_URL=https://openrouter.ai/api/v1
LLM_MODEL=meta-llama/llama-3.1-8b-instruct:free
LLM_API_KEY=sk-or-v1-ваш-ключ-здесь
```

### 3. Перезапустите backend

```bash
docker compose restart backend
```

### 4. Проверьте работу

```bash
# Проверить логи
docker compose logs backend --tail=50

# Должны увидеть успешные запросы к openrouter.ai
```

## Тестирование перед миграцией

Протестируйте API ключ перед изменением `.env`:

```bash
export LLM_API_KEY=sk-or-v1-ваш-ключ
./scripts/test_openrouter.sh
```

## Откат назад

Если что-то пошло не так, верните старые настройки:

```env
LLM_PROVIDER=lmstudio
LLM_API_URL=http://host.docker.internal:1234/v1
LLM_MODEL=local-model
# LLM_API_KEY= (закомментируйте)
```

```bash
docker compose restart backend
```

## Рекомендуемые модели для начала

1. **meta-llama/llama-3.1-8b-instruct:free** — лучший баланс качества и скорости
2. **google/gemini-flash-1.5:free** — самая быстрая
3. **mistralai/mistral-7b-instruct:free** — компактная

Полный список: [openrouter.ai/models](https://openrouter.ai/models?order=newest)

## Мониторинг использования

Отслеживайте использование API на [openrouter.ai/activity](https://openrouter.ai/activity)

## Troubleshooting

**Ошибка: "Invalid API key"**
- Проверьте что ключ начинается с `sk-or-v1-`
- Убедитесь что нет лишних пробелов

**Ошибка: "Rate limit exceeded"**
- Бесплатные модели имеют лимиты
- Попробуйте другую бесплатную модель
- Или добавьте кредиты на аккаунт

**Медленные ответы**
- Выберите модель с меткой "fast"
- Проверьте интернет соединение
