package twitch

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

const testPublicLivePayload = `{"data":{"user":{"login":"foo","displayName":"Foo","stream":{"id":"123","title":"Hello world","createdAt":"2026-11-09T12:00:00Z","viewersCount":42,"game":{"name":"Just Chatting"},"type":"LIVE"}}}}`

const testPublicOfflinePayload = `{"data":{"user":{"login":"foo","displayName":"Foo","stream":null}}}`

const testPublicMissingUserPayload = `{"data":{"user":null}}`

// newTestPublicClient returns a client whose endpoint points at the test server.
func newTestPublicClient(srv *httptest.Server) *PublicStreamClient {
	client := NewPublicStreamClient()
	client.endpoint = srv.URL
	return client
}

// TestPublicStreamLive tests a live stream response: all fields parsed, Live=true.
func TestPublicStreamLive(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("request method = %q, want POST", r.Method)
		}
		if got := r.Header.Get("Client-ID"); got != "kimne78kx3ncx6brgo4mv6wki5h1ko" {
			t.Errorf("Client-ID header = %q, want %q", got, "kimne78kx3ncx6brgo4mv6wki5h1ko")
		}
		if got := r.Header.Get("Content-Type"); got != "application/json" {
			t.Errorf("Content-Type header = %q, want application/json", got)
		}

		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Errorf("read request body: %v", err)
		}
		var req struct {
			Variables struct {
				Login string `json:"login"`
			} `json:"variables"`
		}
		if err := json.Unmarshal(body, &req); err != nil {
			t.Errorf("unmarshal request body %q: %v", string(body), err)
		} else if req.Variables.Login != "foo" {
			t.Errorf("variables.login = %q, want normalized %q", req.Variables.Login, "foo")
		}

		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(testPublicLivePayload))
	}))
	defer srv.Close()

	client := newTestPublicClient(srv)
	info, err := client.GetStream(context.Background(), "#Foo")
	if err != nil {
		t.Fatalf("GetStream() error = %v", err)
	}
	if !info.Live {
		t.Error("expected Live to be true")
	}
	if info.Title != "Hello world" {
		t.Errorf("Title = %q, want %q", info.Title, "Hello world")
	}
	if info.GameName != "Just Chatting" {
		t.Errorf("GameName = %q, want %q", info.GameName, "Just Chatting")
	}
	if info.ViewerCount != 42 {
		t.Errorf("ViewerCount = %d, want 42", info.ViewerCount)
	}
	wantStartedAt := time.Date(2026, 11, 9, 12, 0, 0, 0, time.UTC)
	if !info.StartedAt.Equal(wantStartedAt) {
		t.Errorf("StartedAt = %v, want %v", info.StartedAt, wantStartedAt)
	}
}

// TestPublicStreamOffline tests stream == null: Live=false, zero fields, no error.
func TestPublicStreamOffline(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(testPublicOfflinePayload))
	}))
	defer srv.Close()

	client := newTestPublicClient(srv)
	info, err := client.GetStream(context.Background(), "Foo")
	if err != nil {
		t.Fatalf("GetStream() error = %v", err)
	}
	if info.Live {
		t.Error("expected Live to be false")
	}
	if info.Title != "" {
		t.Errorf("Title = %q, want empty", info.Title)
	}
	if info.GameName != "" {
		t.Errorf("GameName = %q, want empty", info.GameName)
	}
	if info.ViewerCount != 0 {
		t.Errorf("ViewerCount = %d, want 0", info.ViewerCount)
	}
	if !info.StartedAt.IsZero() {
		t.Errorf("StartedAt = %v, want zero time", info.StartedAt)
	}
}

// TestPublicStreamMissingUser tests user == null: error contains "channel not found".
func TestPublicStreamMissingUser(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(testPublicMissingUserPayload))
	}))
	defer srv.Close()

	client := newTestPublicClient(srv)
	_, err := client.GetStream(context.Background(), "Foo")
	if err == nil {
		t.Fatal("expected error for user == null")
	}
	if !strings.Contains(err.Error(), "channel not found") {
		t.Errorf("error = %v, want it to contain %q", err, "channel not found")
	}
}

// TestPublicStreamHTTPError tests a non-200 response: error, nothing cached.
func TestPublicStreamHTTPError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "boom", http.StatusInternalServerError)
	}))
	defer srv.Close()

	client := newTestPublicClient(srv)
	if _, err := client.GetStream(context.Background(), "Foo"); err == nil {
		t.Fatal("expected error for HTTP 500")
	}

	client.mu.RLock()
	defer client.mu.RUnlock()
	if len(client.cache) != 0 {
		t.Errorf("cache size = %d, want 0 (errors must not be cached)", len(client.cache))
	}
}

// TestPublicStreamMalformedJSON tests an invalid JSON body: error, nothing cached.
func TestPublicStreamMalformedJSON(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte("not json"))
	}))
	defer srv.Close()

	client := newTestPublicClient(srv)
	if _, err := client.GetStream(context.Background(), "Foo"); err == nil {
		t.Fatal("expected error for malformed JSON")
	}

	client.mu.RLock()
	defer client.mu.RUnlock()
	if len(client.cache) != 0 {
		t.Errorf("cache size = %d, want 0 (errors must not be cached)", len(client.cache))
	}
}

// TestPublicStreamCachesForTTL tests that repeated calls hit the cache until it expires.
func TestPublicStreamCachesForTTL(t *testing.T) {
	var calls int64
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt64(&calls, 1)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(testPublicLivePayload))
	}))
	defer srv.Close()

	client := newTestPublicClient(srv)
	ctx := context.Background()

	for i := 0; i < 2; i++ {
		if _, err := client.GetStream(ctx, "Foo"); err != nil {
			t.Fatalf("GetStream() call %d error = %v", i+1, err)
		}
	}
	if got := atomic.LoadInt64(&calls); got != 1 {
		t.Fatalf("server calls after two GetStream() calls = %d, want 1 (cached)", got)
	}

	// Age the cache entry past the TTL (16 seconds ago > 15s ttl).
	client.mu.Lock()
	for login, entry := range client.cache {
		entry.FetchedAt = time.Now().Add(-16 * time.Second)
		client.cache[login] = entry
	}
	client.mu.Unlock()

	if _, err := client.GetStream(ctx, "Foo"); err != nil {
		t.Fatalf("GetStream() after expiry error = %v", err)
	}
	if got := atomic.LoadInt64(&calls); got != 2 {
		t.Fatalf("server calls after cache expiry = %d, want 2", got)
	}
}

// TestNormalizeTwitchChannel tests channel login normalization.
func TestNormalizeTwitchChannel(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"#Foo", "foo"},
		{"https://www.twitch.tv/Foo", "foo"},
		{"https://twitch.tv/Foo", "foo"},
		{" Foo ", "foo"},
		{"Foo", "foo"},
		{"Bar_Baz123", "bar_baz123"},
	}
	for _, tt := range tests {
		if got := NormalizeChannel(tt.input); got != tt.want {
			t.Errorf("NormalizeChannel(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}
