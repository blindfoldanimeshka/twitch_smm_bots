// Package llm provides integration with OpenAI-compatible APIs (LM Studio, Omniroute, OpenAI, OpenRouter, etc.)
package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// Client для OpenAI-compatible API (LM Studio, Omniroute, OpenAI, OpenRouter, etc.)
type Client struct {
	baseURL    string
	httpClient *http.Client
	model      string
	apiKey     string
	provider   string
}

// NewClient создаёт клиент для OpenAI-compatible API
// provider: "lmstudio", "omniroute", "openai", "openrouter", "custom"
func NewClient(baseURL, model, apiKey, provider string) *Client {
	if baseURL == "" {
		baseURL = "http://127.0.0.1:1234/v1"
	}
	if provider == "" {
		provider = "lmstudio"
	}

	// Автоматически устанавливаем URL для OpenRouter
	if provider == "openrouter" && baseURL == "http://127.0.0.1:1234/v1" {
		baseURL = "https://openrouter.ai/api/v1"
	}

	return &Client{
		baseURL:    baseURL,
		httpClient: &http.Client{Timeout: 30 * time.Second},
		model:      model,
		apiKey:     apiKey,
		provider:   provider,
	}
}

type chatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type chatRequest struct {
	Model       string        `json:"model"`
	Messages    []chatMessage `json:"messages"`
	Temperature float64       `json:"temperature,omitempty"`
	MaxTokens   int           `json:"max_tokens,omitempty"`
	Stream      bool          `json:"stream"`
}

type chatResponse struct {
	Choices []struct {
		Message chatMessage `json:"message"`
	} `json:"choices"`
	Error *struct {
		Message string `json:"message"`
	} `json:"error,omitempty"`
}

// Generate отправляет запрос к API и возвращает сгенерированный текст
// Автоматически повторяет запрос до 3 раз при ошибках
func (c *Client) Generate(ctx context.Context, systemPrompt, userPrompt string, maxTokens int) (string, error) {
	if maxTokens == 0 {
		maxTokens = 60
	}

	var lastErr error
	for attempt := 1; attempt <= 3; attempt++ {
		result, err := c.generateOnce(ctx, systemPrompt, userPrompt, maxTokens)
		if err == nil {
			return result, nil
		}

		lastErr = err
		if attempt < 3 {
			// Exponential backoff: 1s, 2s
			time.Sleep(time.Duration(attempt) * time.Second)
		}
	}

	return "", fmt.Errorf("failed after 3 attempts: %w", lastErr)
}

func (c *Client) generateOnce(ctx context.Context, systemPrompt, userPrompt string, maxTokens int) (string, error) {

	reqBody := chatRequest{
		Model: c.model,
		Messages: []chatMessage{
			{Role: "system", Content: systemPrompt},
			{Role: "user", Content: userPrompt},
		},
		Temperature: 0.8,
		MaxTokens:   maxTokens,
		Stream:      false,
	}

	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		return "", fmt.Errorf("marshal error: %w", err)
	}

	url := fmt.Sprintf("%s/chat/completions", c.baseURL)
	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(jsonData))
	if err != nil {
		return "", err
	}

	req.Header.Set("Content-Type", "application/json")

	// Добавить API key если указан (для Omniroute, OpenAI, OpenRouter и т.д.)
	if c.apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+c.apiKey)
	}

	// OpenRouter требует дополнительные заголовки
	if c.provider == "openrouter" {
		req.Header.Set("HTTP-Referer", "https://github.com/twitch-smm-bot")
		req.Header.Set("X-Title", "Twitch SMM Bot")
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("http error: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("%s API error (status %d): %s", c.provider, resp.StatusCode, string(body))
	}

	var chatResp chatResponse
	if err := json.Unmarshal(body, &chatResp); err != nil {
		return "", fmt.Errorf("unmarshal error: %w", err)
	}

	if chatResp.Error != nil {
		return "", fmt.Errorf("api error: %s", chatResp.Error.Message)
	}

	if len(chatResp.Choices) == 0 {
		return "", fmt.Errorf("no response generated")
	}

	return chatResp.Choices[0].Message.Content, nil
}

// HealthCheck проверяет доступность API
func (c *Client) HealthCheck(ctx context.Context) error {
	testResp, err := c.Generate(ctx, "You are a test bot", "Say 'ok' only", 5)
	if err != nil {
		return fmt.Errorf("%s health check failed: %w", c.provider, err)
	}
	if strings.TrimSpace(testResp) == "" {
		return fmt.Errorf("empty response from %s", c.provider)
	}
	return nil
}

// GetProvider возвращает название провайдера
func (c *Client) GetProvider() string {
	return c.provider
}
