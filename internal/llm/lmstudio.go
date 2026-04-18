// Package llm provides integration with LM Studio API (OpenAI-compatible)
package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// Client для LM Studio API
type Client struct {
	baseURL string
	httpClient *http.Client
	model   string
}

// NewClient создаёт клиент для LM Studio
func NewClient(baseURL, model string) *Client {
	if baseURL == "" {
		baseURL = "http://127.0.0.1:1234/v1"
	}
	return &Client{
		baseURL: baseURL,
		httpClient: &http.Client{Timeout: 30 * time.Second},
		model:   model,
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

// Generate отправляет запрос к LM Studio и возвращает сгенерированный текст
func (c *Client) Generate(ctx context.Context, systemPrompt, userPrompt string, maxTokens int) (string, error) {
	if maxTokens == 0 {
		maxTokens = 60
	}

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
		return "", fmt.Errorf("lm studio error (status %d): %s", resp.StatusCode, string(body))
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

// HealthCheck проверяет доступность LM Studio
func (c *Client) HealthCheck(ctx context.Context) error {
	// LM Studio не имеет endpoint /health, пробуем /models или отправляем тестовый запрос
	testResp, err := c.Generate(ctx, "You are a test bot", "Say 'ok' only", 5)
	if err != nil {
		return err
	}
	if testResp == "" {
		return fmt.Errorf("empty response from LM Studio")
	}
	return nil
}
