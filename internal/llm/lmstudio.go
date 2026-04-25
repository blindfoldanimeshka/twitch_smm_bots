// Package llm provides integration with LM Studio API
package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"time"
)

// Client для LM Studio API
type Client struct {
	baseURL    string
	httpClient *http.Client
	model      string
	apiKey     string
	provider   string
}

// NewClient создаёт клиент для LM Studio API
func NewClient(baseURL, model, apiKey, provider string) *Client {
	if baseURL == "" {
		baseURL = "http://127.0.0.1:1234/v1"
	}
	if provider == "" {
		provider = "lmstudio"
	}

	client := &Client{
		baseURL:    baseURL,
		httpClient: &http.Client{Timeout: 30 * time.Second},
		model:      model,
		apiKey:     apiKey,
		provider:   provider,
	}

	// Автоматическая инициализация для LM Studio
	if provider == "lmstudio" {
		go client.autoInitializeLMStudio()
	}

	return client
}

// autoInitializeLMStudio пытается автоматически загрузить модель в LM Studio
func (c *Client) autoInitializeLMStudio() {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Проверяем доступность LM Studio
	if err := c.checkLMStudioAvailable(ctx); err != nil {
		log.Printf("[LLM] LM Studio not available: %v", err)
		return
	}

	// Получаем список доступных моделей
	models, err := c.getAvailableModels(ctx)
	if err != nil {
		log.Printf("[LLM] Failed to get models: %v", err)
		return
	}

	if len(models) == 0 {
		log.Printf("[LLM] No models found in LM Studio. Please download a model first.")
		return
	}

	// Если модель не указана, используем первую доступную
	if c.model == "" {
		c.model = models[0]
		log.Printf("[LLM] Auto-selected model: %s", c.model)
	}

	// Проверяем, загружена ли модель
	loaded, err := c.isModelLoaded(ctx)
	if err != nil {
		log.Printf("[LLM] Failed to check model status: %v", err)
		return
	}

	if !loaded {
		log.Printf("[LLM] Model %s not loaded, attempting to load...", c.model)
		if err := c.loadModel(ctx); err != nil {
			log.Printf("[LLM] Failed to load model: %v", err)
			return
		}
		log.Printf("[LLM] Model %s loaded successfully", c.model)
	} else {
		log.Printf("[LLM] Model %s already loaded", c.model)
	}
}

// checkLMStudioAvailable проверяет доступность LM Studio
func (c *Client) checkLMStudioAvailable(ctx context.Context) error {
	req, err := http.NewRequestWithContext(ctx, "GET", c.baseURL+"/models", nil)
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("LM Studio not running on %s: %w", c.baseURL, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("unexpected status: %d", resp.StatusCode)
	}

	return nil
}

// getAvailableModels получает список доступных моделей
func (c *Client) getAvailableModels(ctx context.Context) ([]string, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", c.baseURL+"/models", nil)
	if err != nil {
		return nil, err
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var result struct {
		Data []struct {
			ID string `json:"id"`
		} `json:"data"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	models := make([]string, 0, len(result.Data))
	for _, m := range result.Data {
		models = append(models, m.ID)
	}

	return models, nil
}

// isModelLoaded проверяет, загружена ли модель
func (c *Client) isModelLoaded(ctx context.Context) (bool, error) {
	// Пробуем простой запрос к модели
	testCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	_, err := c.Generate(testCtx, "test", "hi", 1)
	return err == nil, nil
}

// loadModel загружает модель в LM Studio (если API поддерживает)
func (c *Client) loadModel(ctx context.Context) error {
	// LM Studio автоматически загружает модель при первом запросе
	// Делаем тестовый запрос для инициализации
	_, err := c.Generate(ctx, "You are a test bot", "Say ok", 5)
	return err
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

	// Добавить API key если указан
	if c.apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+c.apiKey)
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

// HealthCheck проверяет доступность LM Studio API
func (c *Client) HealthCheck(ctx context.Context) error {
	testResp, err := c.Generate(ctx, "You are a test bot", "Say 'ok' only", 5)
	if err != nil {
		return fmt.Errorf("LM Studio недоступен. Убедитесь, что LM Studio запущен на %s и модель загружена: %w", c.baseURL, err)
	}
	if strings.TrimSpace(testResp) == "" {
		return fmt.Errorf("empty response from LM Studio")
	}
	return nil
}

// GetProvider возвращает название провайдера
func (c *Client) GetProvider() string {
	return c.provider
}

// GetModel возвращает название модели
func (c *Client) GetModel() string {
	return c.model
}

// GetURL возвращает базовый URL API
func (c *Client) GetURL() string {
	return c.baseURL
}
