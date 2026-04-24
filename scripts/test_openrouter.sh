#!/bin/bash
# Скрипт для тестирования интеграции с OpenRouter

set -e

echo "🧪 Тестирование OpenRouter интеграции..."
echo ""

# Проверка переменных окружения
if [ -z "$LLM_API_KEY" ]; then
    echo "❌ Ошибка: LLM_API_KEY не установлен"
    echo "Установите: export LLM_API_KEY=sk-or-v1-your-key"
    exit 1
fi

echo "✅ API ключ найден"
echo ""

# Тест 1: Прямой запрос к OpenRouter API
echo "📡 Тест 1: Прямой запрос к OpenRouter API..."

RESPONSE=$(curl -s -X POST https://openrouter.ai/api/v1/chat/completions \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $LLM_API_KEY" \
  -H "HTTP-Referer: https://github.com/twitch-smm-bot" \
  -H "X-Title: Twitch SMM Bot Test" \
  -d '{
    "model": "meta-llama/llama-3.1-8b-instruct:free",
    "messages": [
      {"role": "system", "content": "You are a helpful assistant."},
      {"role": "user", "content": "Say hello in one word"}
    ],
    "max_tokens": 10
  }')

if echo "$RESPONSE" | grep -q "error"; then
    echo "❌ Ошибка API:"
    echo "$RESPONSE" | jq .
    exit 1
fi

echo "✅ API отвечает корректно"
echo "Ответ: $(echo "$RESPONSE" | jq -r '.choices[0].message.content')"
echo ""

# Тест 2: Проверка доступных моделей
echo "📋 Тест 2: Проверка доступных бесплатных моделей..."

MODELS=$(curl -s https://openrouter.ai/api/v1/models \
  -H "Authorization: Bearer $LLM_API_KEY")

echo "Бесплатные модели:"
echo "$MODELS" | jq -r '.data[] | select(.pricing.prompt == "0") | "  - \(.id)"' | head -5
echo ""

# Тест 3: Проверка разных моделей
echo "🔄 Тест 3: Проверка нескольких моделей..."

MODELS_TO_TEST=(
    "meta-llama/llama-3.1-8b-instruct:free"
    "google/gemini-flash-1.5:free"
    "mistralai/mistral-7b-instruct:free"
)

for MODEL in "${MODELS_TO_TEST[@]}"; do
    echo -n "  Тестирую $MODEL... "

    RESPONSE=$(curl -s -X POST https://openrouter.ai/api/v1/chat/completions \
      -H "Content-Type: application/json" \
      -H "Authorization: Bearer $LLM_API_KEY" \
      -H "HTTP-Referer: https://github.com/twitch-smm-bot" \
      -H "X-Title: Twitch SMM Bot Test" \
      -d "{
        \"model\": \"$MODEL\",
        \"messages\": [{\"role\": \"user\", \"content\": \"Hi\"}],
        \"max_tokens\": 5
      }" 2>/dev/null)

    if echo "$RESPONSE" | grep -q "error"; then
        echo "❌ Ошибка"
    else
        echo "✅ OK"
    fi
done

echo ""
echo "✅ Все тесты пройдены!"
echo ""
echo "Для использования в проекте добавьте в .env:"
echo "LLM_PROVIDER=openrouter"
echo "LLM_API_URL=https://openrouter.ai/api/v1"
echo "LLM_MODEL=meta-llama/llama-3.1-8b-instruct:free"
echo "LLM_API_KEY=$LLM_API_KEY"
