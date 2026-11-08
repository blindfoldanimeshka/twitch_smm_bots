package llm

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

// newTestClient spins up a test server and returns a client configured against it.
func newTestClient(t *testing.T, h http.HandlerFunc) (*Client, *httptest.Server) {
	srv := httptest.NewServer(h)
	t.Cleanup(srv.Close)
	return NewClient(Config{
		BaseURL: srv.URL, APIKey: "sk-test",
		FreeModel: "openrouter/free", PaidModel: "z-ai/glm-5.3-flash",
		PaidEnabled: true, PaidDailyCapUSD: 1.0,
	}), srv
}

// writeKey writes the OpenRouter GET /key envelope.
func writeKey(w http.ResponseWriter, remaining, limit, used int, usageDaily float64) {
	w.Header().Set("Content-Type", "application/json")
	fmt.Fprintf(w, `{"data":{"usage_daily":%f,"free_model_daily_requests":{"limit":%d,"remaining":%d,"used":%d}}}`,
		usageDaily, limit, remaining, used)
}

// writeCompletion writes a successful completion response.
func writeCompletion(w http.ResponseWriter, model, content string, cost float64) {
	w.Header().Set("Content-Type", "application/json")
	fmt.Fprintf(w, `{"model":%q,"choices":[{"message":{"content":%q}}],"usage":{"cost":%f}}`,
		model, content, cost)
}

// seedKey seeds a fresh key cache so Generate does not refresh it first.
func seedKey(c *Client, remaining, limit, used int, usageDaily float64) {
	c.mu.Lock()
	c.key = keyInfo{
		UsageDaily:    usageDaily,
		FreeLimit:     limit,
		FreeRemaining: remaining,
		FreeUsed:      used,
		FetchedAt:     c.now(),
		OK:            true,
	}
	c.mu.Unlock()
}

// capturedBody is the shape the client sends to /chat/completions.
type capturedBody struct {
	Model       string   `json:"model"`
	Models      []string `json:"models"`
	Route       string   `json:"route"`
	Temperature float64  `json:"temperature"`
	MaxTokens   int      `json:"max_tokens"`
	Stream      bool     `json:"stream"`
}

func TestGenerateFallbackRequest(t *testing.T) {
	var body capturedBody
	c, _ := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/key":
			writeKey(w, 50, 50, 0, 0)
		case "/chat/completions":
			_ = json.NewDecoder(r.Body).Decode(&body)
			writeCompletion(w, "upstage/solar-pro-3:free", "hi", 0)
		default:
			t.Errorf("unexpected path %s", r.URL.Path)
		}
	})

	got, err := c.Generate(context.Background(), "sys", "user", 60)
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	if got != "hi" {
		t.Fatalf("got %q, want %q", got, "hi")
	}
	if len(body.Models) != 2 || body.Models[0] != "openrouter/free" || body.Models[1] != "z-ai/glm-5.3-flash" {
		t.Fatalf("models = %v, want [openrouter/free z-ai/glm-5.3-flash]", body.Models)
	}
	if body.Route != "fallback" {
		t.Fatalf("route = %q, want fallback", body.Route)
	}

	// Free-served: no cooldown and no local spend.
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.budget.freeBackoff != 0 || !c.budget.freeBlockedUntil.IsZero() {
		t.Fatalf("expected no cooldown after free-served, backoff=%v blocked=%v", c.budget.freeBackoff, c.budget.freeBlockedUntil)
	}
	if c.budget.localDailyCost != 0 {
		t.Fatalf("localDailyCost = %v, want 0", c.budget.localDailyCost)
	}
}

func TestGenerateDropsPaidModelWhenDisabled(t *testing.T) {
	var body capturedBody
	c, _ := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/key":
			writeKey(w, 50, 50, 0, 0)
		case "/chat/completions":
			_ = json.NewDecoder(r.Body).Decode(&body)
			writeCompletion(w, "upstage/solar-pro-3:free", "hi", 0)
		default:
			t.Errorf("unexpected path %s", r.URL.Path)
		}
	})

	c.mu.Lock()
	c.cfg.PaidEnabled = false
	c.mu.Unlock()

	if _, err := c.Generate(context.Background(), "sys", "user", 60); err != nil {
		t.Fatalf("Generate: %v", err)
	}
	if body.Model != "openrouter/free" {
		t.Fatalf("model = %q, want openrouter/free", body.Model)
	}
	if len(body.Models) != 0 || body.Route != "" {
		t.Fatalf("unexpected fallback fields: models=%v route=%q", body.Models, body.Route)
	}
}

func TestGenerateRefreshesStaleKeyInfo(t *testing.T) {
	var body capturedBody
	c, _ := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/key":
			writeKey(w, 0, 50, 50, 0) // exhausted free quota
		case "/chat/completions":
			_ = json.NewDecoder(r.Body).Decode(&body)
			writeCompletion(w, "z-ai/glm-5.3-flash", "hi", 0.00001)
		default:
			t.Errorf("unexpected path %s", r.URL.Path)
		}
	})

	// Seed a stale cache claiming free quota remains; the refresh must overwrite it.
	c.mu.Lock()
	c.key = keyInfo{
		UsageDaily: 0, FreeLimit: 50, FreeRemaining: 50, FreeUsed: 0,
		FetchedAt: time.Now().Add(-time.Minute), OK: true,
	}
	c.mu.Unlock()

	if _, err := c.Generate(context.Background(), "sys", "user", 60); err != nil {
		t.Fatalf("Generate: %v", err)
	}
	if body.Model != "z-ai/glm-5.3-flash" {
		t.Fatalf("model = %q, want paid-only z-ai/glm-5.3-flash", body.Model)
	}
	if len(body.Models) != 0 || body.Route != "" {
		t.Fatalf("unexpected fallback fields: models=%v route=%q", body.Models, body.Route)
	}
}

func TestGenerateContinuesWhenKeyFetchFails(t *testing.T) {
	var body capturedBody
	c, _ := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/key":
			w.WriteHeader(http.StatusInternalServerError)
		case "/chat/completions":
			_ = json.NewDecoder(r.Body).Decode(&body)
			writeCompletion(w, "upstage/solar-pro-3:free", "hi", 0)
		default:
			t.Errorf("unexpected path %s", r.URL.Path)
		}
	})

	got, err := c.Generate(context.Background(), "sys", "user", 60)
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	if got != "hi" {
		t.Fatalf("got %q, want %q", got, "hi")
	}
	// Key unknown -> assume free available, paid allowed -> free-first fallback.
	if len(body.Models) != 2 || body.Models[0] != "openrouter/free" {
		t.Fatalf("models = %v, want free-first fallback", body.Models)
	}
}

func TestGeneratePaidOnlyWhenFreeExhausted(t *testing.T) {
	var body capturedBody
	c, _ := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/key":
			writeKey(w, 0, 50, 50, 0)
		case "/chat/completions":
			_ = json.NewDecoder(r.Body).Decode(&body)
			writeCompletion(w, "z-ai/glm-5.3-flash", "hi", 0.00001)
		default:
			t.Errorf("unexpected path %s", r.URL.Path)
		}
	})

	seedKey(c, 0, 50, 50, 0)

	if _, err := c.Generate(context.Background(), "sys", "user", 60); err != nil {
		t.Fatalf("Generate: %v", err)
	}
	if body.Model != "z-ai/glm-5.3-flash" {
		t.Fatalf("model = %q, want z-ai/glm-5.3-flash", body.Model)
	}
	if len(body.Models) != 0 || body.Route != "" {
		t.Fatalf("unexpected fallback fields: models=%v route=%q", body.Models, body.Route)
	}
}

func TestGenerateBudgetExhausted(t *testing.T) {
	var calls int32
	c, _ := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&calls, 1)
	})

	c.mu.Lock()
	c.cfg.PaidEnabled = false
	c.mu.Unlock()
	seedKey(c, 0, 50, 50, 0)

	_, err := c.Generate(context.Background(), "sys", "user", 60)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	want := "LLM budget exhausted: free daily limit reached and paid fallback is disabled or capped"
	if err.Error() != want {
		t.Fatalf("error = %q, want %q", err.Error(), want)
	}
	if atomic.LoadInt32(&calls) != 0 {
		t.Fatalf("expected no HTTP requests, got %d", calls)
	}
}

func TestGenerateClassifiesPaidFromCost(t *testing.T) {
	c, _ := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/key":
			writeKey(w, 50, 50, 0, 0)
		case "/chat/completions":
			writeCompletion(w, "upstage/solar-pro-3:free", "hi", 0.00002)
		default:
			t.Errorf("unexpected path %s", r.URL.Path)
		}
	})

	if _, err := c.Generate(context.Background(), "sys", "user", 60); err != nil {
		t.Fatalf("Generate: %v", err)
	}

	st := c.Status(context.Background())
	if st.PaidSpentTodayUSD <= 0 {
		t.Fatalf("PaidSpentTodayUSD = %v, want > 0", st.PaidSpentTodayUSD)
	}
	if !st.FreeCooldownUntil.After(c.now()) {
		t.Fatalf("FreeCooldownUntil = %v, want in the future", st.FreeCooldownUntil)
	}
}

func TestGenerateClassifiesPaidFromModelWhenUsageMissing(t *testing.T) {
	c, _ := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/key":
			writeKey(w, 50, 50, 0, 0)
		case "/chat/completions":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"model":"z-ai/glm-5.3-flash","choices":[{"message":{"content":"hi"}}]}`))
		default:
			t.Errorf("unexpected path %s", r.URL.Path)
		}
	})

	got, err := c.Generate(context.Background(), "sys", "user", 60)
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	if got != "hi" {
		t.Fatalf("got %q, want %q", got, "hi")
	}

	// Treated as paid via model match even though usage is missing.
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.budget.freeBackoff == 0 {
		t.Fatal("expected a cooldown backoff for a paid model response")
	}
}

func TestGenerateEmptyCompletionIsNotRetried(t *testing.T) {
	var calls int32
	c, _ := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/key":
			writeKey(w, 50, 50, 0, 0)
		case "/chat/completions":
			atomic.AddInt32(&calls, 1)
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"model":"upstage/solar-pro-3:free","choices":[],"usage":{"cost":0}}`))
		default:
			t.Errorf("unexpected path %s", r.URL.Path)
		}
	})

	_, err := c.Generate(context.Background(), "sys", "user", 60)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "openrouter: empty completion") {
		t.Fatalf("error = %q, want mention of empty completion", err.Error())
	}
	if atomic.LoadInt32(&calls) != 1 {
		t.Fatalf("calls = %d, want 1", calls)
	}
}

func TestGenerateContentFilterIsError(t *testing.T) {
	var calls int32
	c, _ := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/key":
			writeKey(w, 50, 50, 0, 0)
		case "/chat/completions":
			atomic.AddInt32(&calls, 1)
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"model":"upstage/solar-pro-3:free","choices":[{"message":{"content":"blocked"},"finish_reason":"content_filter"}],"usage":{"cost":0}}`))
		default:
			t.Errorf("unexpected path %s", r.URL.Path)
		}
	})

	_, err := c.Generate(context.Background(), "sys", "user", 60)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if atomic.LoadInt32(&calls) != 1 {
		t.Fatalf("calls = %d, want 1", calls)
	}
}

func TestGenerateRetriesOnly5xx(t *testing.T) {
	t.Run("503 retries once", func(t *testing.T) {
		var calls int32
		c, _ := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
			switch r.URL.Path {
			case "/key":
				writeKey(w, 50, 50, 0, 0)
			case "/chat/completions":
				n := atomic.AddInt32(&calls, 1)
				if n == 1 {
					w.WriteHeader(http.StatusServiceUnavailable)
					return
				}
				writeCompletion(w, "upstage/solar-pro-3:free", "hi", 0)
			default:
				t.Errorf("unexpected path %s", r.URL.Path)
			}
		})

		got, err := c.Generate(context.Background(), "sys", "user", 60)
		if err != nil {
			t.Fatalf("Generate: %v", err)
		}
		if got != "hi" {
			t.Fatalf("got %q, want %q", got, "hi")
		}
		if atomic.LoadInt32(&calls) != 2 {
			t.Fatalf("calls = %d, want 2", calls)
		}
	})

	t.Run("429 no retry", func(t *testing.T) {
		var calls int32
		c, _ := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
			switch r.URL.Path {
			case "/key":
				writeKey(w, 50, 50, 0, 0)
			case "/chat/completions":
				atomic.AddInt32(&calls, 1)
				w.WriteHeader(http.StatusTooManyRequests)
			default:
				t.Errorf("unexpected path %s", r.URL.Path)
			}
		})

		_, err := c.Generate(context.Background(), "sys", "user", 60)
		if err == nil {
			t.Fatal("expected error, got nil")
		}
		if atomic.LoadInt32(&calls) != 1 {
			t.Fatalf("calls = %d, want 1", calls)
		}
	})
}

func TestGenerateCooldownDoublesAndCaps(t *testing.T) {
	// Completion costs in order: paid fallbacks, a free-served reset, then paid
	// fallbacks to drive the backoff up to the 15-minute cap.
	costs := []float64{0.00001, 0.00001, 0, 0.00001, 0.00001, 0.00001, 0.00001, 0.00001, 0.00001}
	want := []time.Duration{
		60 * time.Second, 120 * time.Second, 0,
		60 * time.Second, 120 * time.Second, 240 * time.Second,
		480 * time.Second, 900 * time.Second, 900 * time.Second,
	}

	var callIdx int32
	var now atomic.Value
	now.Store(time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC))

	c, _ := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/key":
			writeKey(w, 50, 50, 0, 0)
		case "/chat/completions":
			idx := atomic.AddInt32(&callIdx, 1) - 1
			if idx >= int32(len(costs)) {
				t.Errorf("too many completion calls")
				return
			}
			if costs[idx] > 0 {
				writeCompletion(w, "z-ai/glm-5.3-flash", "hi", costs[idx])
			} else {
				writeCompletion(w, "upstage/solar-pro-3:free", "hi", 0)
			}
		default:
			t.Errorf("unexpected path %s", r.URL.Path)
		}
	})
	c.now = func() time.Time { return now.Load().(time.Time) }

	var prev time.Duration
	for i := range costs {
		if i > 0 && prev > 0 {
			cur := now.Load().(time.Time)
			now.Store(cur.Add(prev + time.Second))
		}
		if _, err := c.Generate(context.Background(), "sys", "user", 60); err != nil {
			t.Fatalf("call %d: %v", i, err)
		}
		c.mu.Lock()
		got := c.budget.freeBackoff
		c.mu.Unlock()
		if got != want[i] {
			t.Fatalf("call %d: backoff = %v, want %v", i, got, want[i])
		}
		prev = want[i]
	}

	// After the free-served reset (index 2) the block was cleared; verify the block
	// is still in place after the final paid fallback.
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.budget.freeBlockedUntil.IsZero() {
		t.Fatal("expected freeBlockedUntil to be set after final paid fallback")
	}
}

func TestHealthCheckParsesKeyAndNeverCallsCompletions(t *testing.T) {
	var completions int32
	c, _ := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/key":
			writeKey(w, 25, 50, 25, 0.0042)
		case "/chat/completions":
			atomic.AddInt32(&completions, 1)
			t.Errorf("unexpected call to /chat/completions")
		default:
			t.Errorf("unexpected path %s", r.URL.Path)
		}
	})

	if err := c.HealthCheck(context.Background()); err != nil {
		t.Fatalf("HealthCheck: %v", err)
	}

	c.mu.Lock()
	defer c.mu.Unlock()
	if c.key.FreeLimit != 50 || c.key.FreeRemaining != 25 || c.key.FreeUsed != 25 {
		t.Fatalf("key parsed incorrectly: %+v", c.key)
	}
	if c.key.UsageDaily != 0.0042 {
		t.Fatalf("usage_daily = %v, want 0.0042", c.key.UsageDaily)
	}
	if atomic.LoadInt32(&completions) != 0 {
		t.Fatalf("completions called %d times", completions)
	}
}

func TestStatusGenerationAvailable(t *testing.T) {
	c, _ := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/key" {
			w.WriteHeader(http.StatusInternalServerError)
			_, _ = w.Write([]byte("internal error"))
			return
		}
		t.Errorf("unexpected path %s", r.URL.Path)
	})

	st := c.Status(context.Background())
	if st.Available {
		t.Fatal("expected Available=false")
	}
	if !st.GenerationAvailable {
		t.Fatal("expected GenerationAvailable=true")
	}
	if st.FreeRequestsRemaining != 0 {
		t.Fatalf("FreeRequestsRemaining = %d, want 0", st.FreeRequestsRemaining)
	}
	if !strings.Contains(st.LastError, "/key") {
		t.Fatalf("LastError = %q, want mention of /key", st.LastError)
	}
}

func TestStatusNotConfigured(t *testing.T) {
	var calls int32
	c, _ := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&calls, 1)
	})

	c.mu.Lock()
	c.cfg.APIKey = ""
	c.mu.Unlock()

	st := c.Status(context.Background())
	if st.Available {
		t.Fatal("expected Available=false")
	}
	if st.GenerationAvailable {
		t.Fatal("expected GenerationAvailable=false")
	}
	if atomic.LoadInt32(&calls) != 0 {
		t.Fatalf("expected no HTTP requests, got %d", calls)
	}
}

func TestReconfigureUpdatesFieldsAndResetsKeyCache(t *testing.T) {
	c, _ := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	})

	seedKey(c, 50, 50, 0, 0)

	var hits int32
	srvB := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/key" {
			atomic.AddInt32(&hits, 1)
			writeKey(w, 10, 20, 10, 0)
		}
	}))
	t.Cleanup(srvB.Close)

	c.Reconfigure(Config{
		BaseURL: srvB.URL, APIKey: "new-key",
		FreeModel: "openrouter/free2", PaidModel: "z-ai/glm-5.3-flash2",
		PaidEnabled: true, PaidDailyCapUSD: 1.0,
	})

	if c.GetURL() != srvB.URL {
		t.Fatalf("GetURL = %q, want %q", c.GetURL(), srvB.URL)
	}
	if c.GetModel() != "openrouter/free2" {
		t.Fatalf("GetModel = %q, want openrouter/free2", c.GetModel())
	}
	if c.GetProvider() != "openrouter" {
		t.Fatalf("GetProvider = %q, want openrouter", c.GetProvider())
	}

	c.mu.Lock()
	if c.key.OK {
		c.mu.Unlock()
		t.Fatal("key cache should have been reset by Reconfigure")
	}
	c.mu.Unlock()

	if err := c.HealthCheck(context.Background()); err != nil {
		t.Fatalf("HealthCheck: %v", err)
	}
	if atomic.LoadInt32(&hits) != 1 {
		t.Fatalf("server B /key hits = %d, want 1", hits)
	}
}
