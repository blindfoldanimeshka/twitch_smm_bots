package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"math"
	"net/http"
	"strings"
	"sync"
	"time"
)

const (
	keyCacheTTL       = 30 * time.Second
	keyRefreshTimeout = 3 * time.Second
	retryAttempts     = 2
	retryDelay        = 500 * time.Millisecond
	cooldownInitial   = 60 * time.Second
	cooldownMax       = 15 * time.Minute
	// paidReservation is a conservative in-flight cost estimate for a paid-eligible
	// request, roughly an order of magnitude above the observed cost of a
	// z-ai/glm-5.3-flash bot message. It is replaced by the actual usage.cost.
	paidReservation = 0.0005
)

// keyInfo caches the parsed GET /key response for 30 seconds.
type keyInfo struct {
	UsageDaily    float64
	FreeLimit     int
	FreeRemaining int
	FreeUsed      int
	FetchedAt     time.Time
	OK            bool
}

// budgetState tracks local paid spend, in-flight reservations and free cooldown.
type budgetState struct {
	localDailyCost   float64 // actual spend observed from usage.cost
	reservedUSD      float64 // worst-case reservations for in-flight paid-eligible calls
	inflightPaid     int
	freeBlockedUntil time.Time
	freeBackoff      time.Duration
	day              string // UTC date for localDailyCost
}

// Client is the OpenRouter client with free-first routing, budget and cooldown.
type Client struct {
	mu         sync.Mutex
	cfg        Config
	httpClient *http.Client
	key        keyInfo
	budget     budgetState
	now        func() time.Time
}

// withDefaults fills empty string fields with the built-in defaults. It leaves
// PaidEnabled and PaidDailyCapUSD untouched: false and 0 are meaningful values.
func withDefaults(cfg Config) Config {
	if cfg.BaseURL == "" {
		cfg.BaseURL = DefaultBaseURL
	}
	if cfg.FreeModel == "" {
		cfg.FreeModel = DefaultFreeModel
	}
	if cfg.PaidModel == "" {
		cfg.PaidModel = DefaultPaidModel
	}
	if cfg.Referer == "" {
		cfg.Referer = DefaultReferer
	}
	if cfg.Title == "" {
		cfg.Title = DefaultTitle
	}
	if cfg.Provider == "" {
		cfg.Provider = ProviderOpenRouter
	}
	return cfg
}

// NewClient returns a configured Client. Empty string fields are filled from the
// built-in defaults. A 30 s HTTP client is used unless cfg.HTTPClient was provided.
func NewClient(cfg Config) *Client {
	cfg = withDefaults(cfg)

	httpClient := cfg.HTTPClient
	if httpClient == nil {
		httpClient = &http.Client{Timeout: 30 * time.Second}
	}

	return &Client{
		cfg:        cfg,
		httpClient: httpClient,
		now:        time.Now,
	}
}

// Reconfigure swaps the config in place. The key cache is reset when the API key
// or base URL changed, forcing a fresh /key lookup.
func (c *Client) Reconfigure(cfg Config) {
	cfg = withDefaults(cfg)

	c.mu.Lock()
	defer c.mu.Unlock()

	if cfg.APIKey != c.cfg.APIKey || cfg.BaseURL != c.cfg.BaseURL {
		c.key = keyInfo{}
	}
	c.cfg = cfg

	if cfg.HTTPClient != nil {
		c.httpClient = cfg.HTTPClient
	} else if c.httpClient == nil {
		c.httpClient = &http.Client{Timeout: 30 * time.Second}
	}
}

// GetProvider returns the configured provider name.
func (c *Client) GetProvider() string {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.cfg.Provider
}

// GetModel returns the free model, kept for existing callers.
func (c *Client) GetModel() string {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.cfg.FreeModel
}

// GetURL returns the base URL.
func (c *Client) GetURL() string {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.cfg.BaseURL
}

// chatMessage is a single chat message.
type chatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// chatRequest covers the three request body shapes: free only (Model), free-first
// fallback (Models + Route), and paid only (Model).
type chatRequest struct {
	Model       string        `json:"model,omitempty"`
	Models      []string      `json:"models,omitempty"`
	Route       string        `json:"route,omitempty"`
	Messages    []chatMessage `json:"messages"`
	Temperature float64       `json:"temperature"`
	MaxTokens   int           `json:"max_tokens"`
	Stream      bool          `json:"stream"`
}

type completionResult struct {
	content          string
	model            string
	cost             float64
	promptTokens     int
	completionTokens int
}

// transportError wraps a network-level failure and is retryable.
type transportError struct {
	err error
}

func (e *transportError) Error() string { return e.err.Error() }
func (e *transportError) Unwrap() error { return e.err }

// httpStatusError wraps a non-2xx HTTP response.
type httpStatusError struct {
	status int
	body   string
}

func (e *httpStatusError) Error() string {
	return fmt.Sprintf("openrouter API error (status %d): %s", e.status, e.body)
}

// isRetryable reports whether a completion error may be retried: transport errors
// and HTTP 5xx. 4xx (including 429), parsing errors and empty completions are not.
func isRetryable(err error) bool {
	var te *transportError
	if errors.As(err, &te) {
		return true
	}
	var se *httpStatusError
	if errors.As(err, &se) {
		return se.status >= 500
	}
	return false
}

// sleepCtx waits for d, returning early (with ctx.Err()) if ctx is cancelled.
func sleepCtx(ctx context.Context, d time.Duration) error {
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-t.C:
		return nil
	}
}

// Generate produces a chat completion, routing free-first with paid fallback
// according to the current budget, quota and cooldown state.
func (c *Client) Generate(ctx context.Context, systemPrompt, userPrompt string, maxTokens int) (string, error) {
	c.mu.Lock()
	apiKey := c.cfg.APIKey
	c.mu.Unlock()
	if apiKey == "" {
		return "", errors.New("OpenRouter API key is not configured (set OPENROUTER_API_KEY)")
	}

	// Refresh /key before routing if the cached info is stale. A failure is ignored
	// and generation continues under the "free available" assumption.
	refreshCtx, cancel := context.WithTimeout(ctx, keyRefreshTimeout)
	_ = c.refreshKeyInfo(refreshCtx)
	cancel()

	now := c.now()

	c.mu.Lock()
	// Reset the local daily spend accumulator when the UTC date changes.
	if day := now.UTC().Format("2006-01-02"); c.budget.day != day {
		c.budget.localDailyCost = 0
		c.budget.day = day
	}

	cfg := c.cfg
	key := c.key

	spentToday := math.Max(c.budget.localDailyCost, key.UsageDaily) + c.budget.reservedUSD
	paidAllowed := cfg.PaidEnabled && (cfg.PaidDailyCapUSD <= 0 || spentToday < cfg.PaidDailyCapUSD)
	freeAvailable := !key.OK || key.FreeRemaining > 0
	cooldownActive := !c.budget.freeBlockedUntil.IsZero() && now.Before(c.budget.freeBlockedUntil)

	messages := []chatMessage{
		{Role: "system", Content: systemPrompt},
		{Role: "user", Content: userPrompt},
	}

	var (
		reqBody        chatRequest
		freeRequested  bool
		paidEligible   bool
		requestedModel string
	)

	switch {
	case freeAvailable && (!cooldownActive || !paidAllowed):
		if paidAllowed {
			reqBody = chatRequest{Models: []string{cfg.FreeModel, cfg.PaidModel}, Route: "fallback", Messages: messages, Temperature: 0.8, MaxTokens: maxTokens, Stream: false}
			freeRequested = true
			paidEligible = true
			requestedModel = cfg.FreeModel
		} else {
			reqBody = chatRequest{Model: cfg.FreeModel, Messages: messages, Temperature: 0.8, MaxTokens: maxTokens, Stream: false}
			freeRequested = true
			requestedModel = cfg.FreeModel
		}
	case paidAllowed:
		reqBody = chatRequest{Model: cfg.PaidModel, Messages: messages, Temperature: 0.8, MaxTokens: maxTokens, Stream: false}
		paidEligible = true
		requestedModel = cfg.PaidModel
	default:
		c.mu.Unlock()
		return "", errors.New("LLM budget exhausted: free daily limit reached and paid fallback is disabled or capped")
	}

	if paidEligible {
		c.budget.reservedUSD += paidReservation
		c.budget.inflightPaid++
	}
	baseURL := cfg.BaseURL
	referer := cfg.Referer
	title := cfg.Title
	paidModel := cfg.PaidModel
	c.mu.Unlock()

	result, err := c.dispatch(ctx, apiKey, baseURL, referer, title, reqBody)

	c.mu.Lock()
	if paidEligible {
		// Release the reservation, replacing it with the actual cost on success.
		c.budget.reservedUSD -= paidReservation
		c.budget.inflightPaid--
		if err == nil {
			c.budget.localDailyCost += result.cost
			paidServed := result.cost > 0 || result.model == paidModel
			if freeRequested {
				if paidServed {
					c.applyFallbackCooldown(c.now())
				} else {
					c.budget.freeBackoff = 0
					c.budget.freeBlockedUntil = time.Time{}
				}
			}
		}
	}
	c.mu.Unlock()

	if err != nil {
		return "", err
	}

	paidServed := result.cost > 0 || result.model == paidModel
	slog.Info("llm completion",
		"requested", requestedModel,
		"model", result.model,
		"free", !paidServed,
		"cost_usd", result.cost,
		"prompt_tokens", result.promptTokens,
		"completion_tokens", result.completionTokens,
	)

	return result.content, nil
}

// applyFallbackCooldown starts or extends the free cooldown. It must be called with
// c.mu held.
func (c *Client) applyFallbackCooldown(now time.Time) {
	if c.budget.freeBackoff <= 0 {
		c.budget.freeBackoff = cooldownInitial
	} else {
		c.budget.freeBackoff *= 2
		if c.budget.freeBackoff > cooldownMax {
			c.budget.freeBackoff = cooldownMax
		}
	}
	c.budget.freeBlockedUntil = now.Add(c.budget.freeBackoff)
}

// dispatch sends the completion request, retrying transport errors and HTTP 5xx.
func (c *Client) dispatch(ctx context.Context, apiKey, baseURL, referer, title string, body chatRequest) (completionResult, error) {
	jsonData, err := json.Marshal(body)
	if err != nil {
		return completionResult{}, fmt.Errorf("openrouter: marshal request: %w", err)
	}

	url := strings.TrimSuffix(baseURL, "/") + "/chat/completions"

	var lastErr error
	for attempt := 1; attempt <= retryAttempts; attempt++ {
		res, err := c.doCompletion(ctx, url, apiKey, referer, title, jsonData)
		if err == nil {
			return res, nil
		}
		lastErr = err
		if !isRetryable(err) || attempt == retryAttempts {
			return completionResult{}, err
		}
		if err := sleepCtx(ctx, retryDelay); err != nil {
			return completionResult{}, err
		}
	}
	return completionResult{}, lastErr
}

// doCompletion performs a single completion POST and parses the response.
func (c *Client) doCompletion(ctx context.Context, url, apiKey, referer, title string, jsonData []byte) (completionResult, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewBuffer(jsonData))
	if err != nil {
		return completionResult{}, &transportError{err: err}
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+apiKey)
	if referer != "" {
		req.Header.Set("HTTP-Referer", referer)
	}
	if title != "" {
		req.Header.Set("X-OpenRouter-Title", title)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return completionResult{}, &transportError{err: err}
	}
	defer resp.Body.Close()

	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return completionResult{}, &transportError{err: err}
	}

	if resp.StatusCode != http.StatusOK {
		return completionResult{}, &httpStatusError{status: resp.StatusCode, body: strings.TrimSpace(string(raw))}
	}

	var parsed struct {
		Model   string `json:"model"`
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
			FinishReason string `json:"finish_reason"`
		} `json:"choices"`
		Usage struct {
			Cost             float64 `json:"cost"`
			PromptTokens     int     `json:"prompt_tokens"`
			CompletionTokens int     `json:"completion_tokens"`
		} `json:"usage"`
		Error *struct {
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.Unmarshal(raw, &parsed); err != nil {
		return completionResult{}, fmt.Errorf("openrouter: unmarshal response: %w", err)
	}

	if parsed.Error != nil {
		return completionResult{}, fmt.Errorf("openrouter API error: %s", parsed.Error.Message)
	}

	if len(parsed.Choices) == 0 {
		return completionResult{}, fmt.Errorf("openrouter: empty completion (model=%s, cost_usd=%.7f)", parsed.Model, parsed.Usage.Cost)
	}

	content := strings.TrimSpace(parsed.Choices[0].Message.Content)
	if content == "" {
		return completionResult{}, fmt.Errorf("openrouter: empty completion (model=%s, cost_usd=%.7f)", parsed.Model, parsed.Usage.Cost)
	}

	if parsed.Choices[0].FinishReason == "content_filter" {
		return completionResult{}, fmt.Errorf("openrouter: content filtered (model=%s, cost_usd=%.7f)", parsed.Model, parsed.Usage.Cost)
	}

	return completionResult{
		content:          content,
		model:            parsed.Model,
		cost:             parsed.Usage.Cost,
		promptTokens:     parsed.Usage.PromptTokens,
		completionTokens: parsed.Usage.CompletionTokens,
	}, nil
}

// refreshKeyInfo fetches GET /key, caching the parsed result for 30 s. It returns
// nil (without a request) when the cached info is still fresh.
func (c *Client) refreshKeyInfo(ctx context.Context) error {
	c.mu.Lock()
	if c.key.OK && c.now().Sub(c.key.FetchedAt) <= keyCacheTTL {
		c.mu.Unlock()
		return nil
	}
	apiKey := c.cfg.APIKey
	baseURL := c.cfg.BaseURL
	c.mu.Unlock()

	if apiKey == "" {
		return errors.New("OpenRouter API key is not configured (set OPENROUTER_API_KEY)")
	}

	url := strings.TrimSuffix(baseURL, "/") + "/key"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+apiKey)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("openrouter /key request failed (status %d): %s", resp.StatusCode, strings.TrimSpace(string(raw)))
	}

	var envelope struct {
		Data struct {
			UsageDaily            float64 `json:"usage_daily"`
			FreeModelDailyRequest struct {
				Limit     int `json:"limit"`
				Remaining int `json:"remaining"`
				Used      int `json:"used"`
			} `json:"free_model_daily_requests"`
		} `json:"data"`
	}
	if err := json.Unmarshal(raw, &envelope); err != nil {
		return fmt.Errorf("openrouter: unmarshal key response: %w", err)
	}

	c.mu.Lock()
	c.key = keyInfo{
		UsageDaily:    envelope.Data.UsageDaily,
		FreeLimit:     envelope.Data.FreeModelDailyRequest.Limit,
		FreeRemaining: envelope.Data.FreeModelDailyRequest.Remaining,
		FreeUsed:      envelope.Data.FreeModelDailyRequest.Used,
		FetchedAt:     c.now(),
		OK:            true,
	}
	c.mu.Unlock()

	return nil
}

// HealthCheck performs GET /key (the only health check) and returns its error.
func (c *Client) HealthCheck(ctx context.Context) error {
	return c.refreshKeyInfo(ctx)
}

// Status is a snapshot of the client's availability, budget and quota state.
type Status struct {
	Available             bool      `json:"available"`
	GenerationAvailable   bool      `json:"generation_available"`
	Provider              string    `json:"provider"`
	FreeModel             string    `json:"free_model"`
	PaidModel             string    `json:"paid_model"`
	PaidEnabled           bool      `json:"paid_enabled"`
	PaidDailyCapUSD       float64   `json:"paid_daily_cap_usd"`
	PaidSpentTodayUSD     float64   `json:"paid_spent_today_usd"`
	FreeRequestsLimit     int       `json:"free_requests_limit"`
	FreeRequestsRemaining int       `json:"free_requests_remaining"`
	FreeRequestsUsed      int       `json:"free_requests_used"`
	FreeCooldownUntil     time.Time `json:"free_cooldown_until"`
	InflightPaidRequests  int       `json:"inflight_paid_requests"`
	LastError             string    `json:"last_error"`
	CheckedAt             time.Time `json:"checked_at"`
}

// Status computes the current client status. It best-effort refreshes /key but
// never mutates config or spend.
func (c *Client) Status(ctx context.Context) Status {
	c.mu.Lock()
	apiKey := c.cfg.APIKey
	c.mu.Unlock()

	var lastErr error
	if apiKey != "" {
		lastErr = c.refreshKeyInfo(ctx)
	} else {
		lastErr = errors.New("OpenRouter API key is not configured (set OPENROUTER_API_KEY)")
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	now := c.now()
	key := c.key
	cfg := c.cfg
	budget := c.budget

	paidSpent := math.Max(budget.localDailyCost, key.UsageDaily)
	spentWithReservation := paidSpent + budget.reservedUSD

	paidAllowed := cfg.PaidEnabled && (cfg.PaidDailyCapUSD <= 0 || spentWithReservation < cfg.PaidDailyCapUSD)
	freeAvailable := !key.OK || key.FreeRemaining > 0

	st := Status{
		Available:             key.OK,
		GenerationAvailable:   cfg.APIKey != "" && (freeAvailable || paidAllowed),
		Provider:              cfg.Provider,
		FreeModel:             cfg.FreeModel,
		PaidModel:             cfg.PaidModel,
		PaidEnabled:           cfg.PaidEnabled,
		PaidDailyCapUSD:       cfg.PaidDailyCapUSD,
		PaidSpentTodayUSD:     paidSpent,
		FreeRequestsLimit:     key.FreeLimit,
		FreeRequestsRemaining: key.FreeRemaining,
		FreeRequestsUsed:      key.FreeUsed,
		FreeCooldownUntil:     budget.freeBlockedUntil,
		InflightPaidRequests:  budget.inflightPaid,
		CheckedAt:             now,
	}
	if lastErr != nil {
		st.LastError = lastErr.Error()
	}
	return st
}
