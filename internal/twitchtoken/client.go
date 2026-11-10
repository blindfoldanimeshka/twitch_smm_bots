// Package twitchtoken is a minimal client for the Twitch Token Generator v2 API
// (https://twitchtokengenerator.com/docs/openapi.json). It covers token
// validation and refresh against the service's shared Twitch application.
package twitchtoken

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

// DefaultBaseURL is the public host of the Twitch Token Generator v2 API.
const DefaultBaseURL = "https://twitchtokengenerator.com"

// defaultTimeout bounds a single API request when the caller does not supply a
// client.
const defaultTimeout = 10 * time.Second

// maxResponseBytes bounds the response body we are willing to read.
const maxResponseBytes = 1 << 20

// maxDrainBytes bounds how much of an error response body is drained so the
// connection can be reused. The content is never surfaced.
const maxDrainBytes = 4 << 10

// Validation is the result of validating an access token.
type Validation struct {
	Username  string   `json:"username"`
	UserID    string   `json:"user_id"`
	ClientID  string   `json:"client_id"`
	ExpiresIn int      `json:"expires_in"`
	Scopes    []string `json:"scopes"`
}

// Tokens is a refreshed access/refresh token pair.
type Tokens struct {
	AccessToken  string   `json:"access_token"`
	RefreshToken string   `json:"refresh_token"`
	ClientID     string   `json:"client_id"`
	ExpiresIn    int      `json:"expires_in"`
	Scopes       []string `json:"scopes"`
}

// Client is a Twitch Token Generator v2 API client. It is safe for concurrent
// use.
type Client struct {
	baseURL    string
	httpClient *http.Client
}

// NewClient returns a client for baseURL (DefaultBaseURL when empty). When
// httpClient is nil a client with a 10 s timeout is used. A trailing slash in
// baseURL is ignored.
func NewClient(baseURL string, httpClient *http.Client) *Client {
	if baseURL == "" {
		baseURL = DefaultBaseURL
	}
	if httpClient == nil {
		httpClient = &http.Client{Timeout: defaultTimeout}
	}

	return &Client{
		baseURL:    strings.TrimSuffix(baseURL, "/"),
		httpClient: httpClient,
	}
}

// Validate inspects an access token and returns the account, application and
// granted scopes behind it.
func (c *Client) Validate(ctx context.Context, accessToken string) (*Validation, error) {
	var out Validation
	if err := c.post(ctx, "validate", "/api/v2/tokens/validate",
		map[string]string{"access_token": accessToken}, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// Refresh exchanges a refresh token for a new access/refresh token pair. The
// client_id and client_secret are omitted so the generator's shared Twitch
// application is used.
func (c *Client) Refresh(ctx context.Context, refreshToken string) (*Tokens, error) {
	var out Tokens
	if err := c.post(ctx, "refresh", "/api/v2/tokens/refresh",
		map[string]string{"refresh_token": refreshToken}, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// apiError is a non-2xx HTTP response. It reports only the operation and the
// status code: request bodies and response bodies carry tokens and are never
// included.
type apiError struct {
	op     string
	status int
}

func (e *apiError) Error() string {
	return fmt.Sprintf("twitchtoken: %s failed with status %d", e.op, e.status)
}

// post performs a JSON POST and decodes the response into out.
func (c *Client) post(ctx context.Context, op, path string, body, out any) error {
	data, err := json.Marshal(body)
	if err != nil {
		return fmt.Errorf("twitchtoken: marshal %s request: %w", op, err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+path, bytes.NewReader(data))
	if err != nil {
		return fmt.Errorf("twitchtoken: build %s request: %w", op, err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("twitchtoken: %s request: %w", op, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, maxDrainBytes))
		return &apiError{op: op, status: resp.StatusCode}
	}

	raw, err := io.ReadAll(io.LimitReader(resp.Body, maxResponseBytes))
	if err != nil {
		return fmt.Errorf("twitchtoken: read %s response: %w", op, err)
	}
	if err := json.Unmarshal(raw, out); err != nil {
		return fmt.Errorf("twitchtoken: decode %s response: %w", op, err)
	}

	return nil
}
