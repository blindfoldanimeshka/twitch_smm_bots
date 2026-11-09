package twitch

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"
)

const (
	// Public GraphQL endpoint, usable with the well-known web client ID and no OAuth token.
	publicGQLEndpoint = "https://gql.twitch.tv/gql"
	publicClientID    = "kimne78kx3ncx6brgo4mv6wki5h1ko"

	publicCacheTTL       = 15 * time.Second
	publicRequestTimeout = 5 * time.Second
)

// publicStreamQuery fetches the live stream state for a single channel login.
const publicStreamQuery = `query ($login: String!) { user(login: $login) { login displayName stream { id title createdAt viewersCount game { name } type } } }`

// PublicStreamInfo is the live state of a channel from the public GraphQL endpoint.
type PublicStreamInfo struct {
	Live        bool
	Title       string
	GameName    string
	ViewerCount int
	StartedAt   time.Time
}

// PublicStreamClient queries Twitch's public GraphQL endpoint. It requires no
// app access token, so it works when the Helix client cannot authenticate.
type PublicStreamClient struct {
	endpoint string
	clientID string
	http     *http.Client
	ttl      time.Duration

	mu    sync.RWMutex
	cache map[string]publicStreamCacheEntry
}

// publicStreamCacheEntry is a cached result keyed by normalized login.
type publicStreamCacheEntry struct {
	Info      *PublicStreamInfo
	FetchedAt time.Time
}

func NewPublicStreamClient() *PublicStreamClient {
	return &PublicStreamClient{
		endpoint: publicGQLEndpoint,
		clientID: publicClientID,
		http:     &http.Client{Timeout: publicRequestTimeout},
		ttl:      publicCacheTTL,
		cache:    make(map[string]publicStreamCacheEntry),
	}
}

// NormalizeChannel trims whitespace, lower-cases, strips a leading '#' and
// twitch.tv URL prefixes, so any channel link or mention yields the plain login.
func NormalizeChannel(channel string) string {
	login := strings.ToLower(strings.TrimSpace(channel))
	login = strings.TrimPrefix(login, "https://www.twitch.tv/")
	login = strings.TrimPrefix(login, "https://twitch.tv/")
	login = strings.TrimPrefix(login, "#")
	return login
}

// GetStream returns the live state for a channel login (raw input is normalized).
// Successful live and offline results are cached for 15 seconds; errors are not cached.
func (c *PublicStreamClient) GetStream(ctx context.Context, channelLogin string) (*PublicStreamInfo, error) {
	login := NormalizeChannel(channelLogin)

	if info, ok := c.getCached(login); ok {
		return info, nil
	}

	info, err := c.fetchStream(ctx, login)
	if err != nil {
		return nil, err
	}

	c.setCached(login, info)
	return info, nil
}

func (c *PublicStreamClient) getCached(login string) (*PublicStreamInfo, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	entry, ok := c.cache[login]
	if !ok || time.Since(entry.FetchedAt) >= c.ttl {
		return nil, false
	}
	return entry.Info, true
}

func (c *PublicStreamClient) setCached(login string, info *PublicStreamInfo) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.cache == nil {
		c.cache = make(map[string]publicStreamCacheEntry)
	}
	c.cache[login] = publicStreamCacheEntry{Info: info, FetchedAt: time.Now()}
}

type gqlStreamRequest struct {
	Query     string         `json:"query"`
	Variables map[string]any `json:"variables"`
}

type gqlStreamResponse struct {
	Data struct {
		User *struct {
			Stream *struct {
				ID           string `json:"id"`
				Title        string `json:"title"`
				CreatedAt    string `json:"createdAt"`
				ViewersCount int    `json:"viewersCount"`
				Game         *struct {
					Name string `json:"name"`
				} `json:"game"`
				Type string `json:"type"`
			} `json:"stream"`
		} `json:"user"`
	} `json:"data"`
}

func (c *PublicStreamClient) fetchStream(ctx context.Context, login string) (*PublicStreamInfo, error) {
	payload, err := json.Marshal(gqlStreamRequest{
		Query:     publicStreamQuery,
		Variables: map[string]any{"login": login},
	})
	if err != nil {
		return nil, fmt.Errorf("encode twitch graphql request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.endpoint, bytes.NewReader(payload))
	if err != nil {
		return nil, fmt.Errorf("create twitch graphql request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Client-ID", c.clientID)

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("twitch graphql request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("twitch graphql request failed with status %d", resp.StatusCode)
	}

	var out gqlStreamResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, fmt.Errorf("decode twitch graphql response: %w", err)
	}

	if out.Data.User == nil {
		return nil, fmt.Errorf("channel not found: %s", login)
	}

	if out.Data.User.Stream == nil {
		return &PublicStreamInfo{Live: false}, nil
	}

	stream := out.Data.User.Stream
	startedAt, err := time.Parse(time.RFC3339, stream.CreatedAt)
	if err != nil {
		return nil, fmt.Errorf("parse twitch stream createdAt %q: %w", stream.CreatedAt, err)
	}

	var gameName string
	if stream.Game != nil {
		gameName = stream.Game.Name
	}

	return &PublicStreamInfo{
		Live:        true,
		Title:       stream.Title,
		GameName:    gameName,
		ViewerCount: stream.ViewersCount,
		StartedAt:   startedAt,
	}, nil
}
