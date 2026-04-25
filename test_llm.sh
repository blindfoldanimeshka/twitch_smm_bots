#!/bin/bash
echo "Testing LM Studio auto-initialization..."
echo ""
echo "Available models:"
curl -s http://localhost:1234/v1/models | jq -r '.data[].id' 2>/dev/null
echo ""
echo "Testing chat completion with google/gemma-3-4b:"
curl -s http://localhost:1234/v1/chat/completions \
  -H "Content-Type: application/json" \
  -d '{
    "model": "google/gemma-3-4b",
    "messages": [
      {"role": "system", "content": "You are a helpful assistant."},
      {"role": "user", "content": "Say hello in one word"}
    ],
    "max_tokens": 10
  }' | jq -r '.choices[0].message.content' 2>/dev/null
