package main

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"twitch-smm-bot/internal/audio"
	"twitch-smm-bot/internal/llm"
	"twitch-smm-bot/internal/storage"
)

// fakeTwitchClient is an in-memory TwitchClient implementation that records
// interactions so tests can observe what the bot loop does.
type fakeTwitchClient struct {
	mu        sync.Mutex
	connected bool
	sent      []string

	polledOnce sync.Once
	polled     chan struct{} // closed on the first IsConnected call (loop is running)

	// onWait, when set, is invoked at the start of WaitForRateLimit. It lets
	// tests simulate events landing while the bot loop sleeps in the wait.
	onWait func()
}

func newFakeTwitchClient() *fakeTwitchClient {
	return &fakeTwitchClient{polled: make(chan struct{})}
}

func (f *fakeTwitchClient) Connect() error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.connected = true
	return nil
}

func (f *fakeTwitchClient) IsConnected() bool {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.polledOnce.Do(func() { close(f.polled) })
	return f.connected
}

func (f *fakeTwitchClient) SendMessage(text string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.sent = append(f.sent, text)
	return nil
}

func (f *fakeTwitchClient) Reconnect() error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.connected = true
	return nil
}

func (f *fakeTwitchClient) Close() {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.connected = false
}

func (f *fakeTwitchClient) WaitForRateLimit() {
	if f.onWait != nil {
		f.onWait()
	}
}

// SentMessages returns a copy of the messages recorded by the fake client.
func (f *fakeTwitchClient) SentMessages() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]string(nil), f.sent...)
}

// TestMessagesPerMinCountsRecentSends verifies that MessagesPerMin counts only
// successful sends within the last 60 seconds.
func TestMessagesPerMinCountsRecentSends(t *testing.T) {
	now := time.Now()
	bm := &botManager{}
	bm.mu.Lock()
	bm.messageTimes = []time.Time{now.Add(-90 * time.Second), now.Add(-30 * time.Second), now}
	bm.mu.Unlock()
	assert.Equal(t, 2, bm.MessagesPerMin())
}

// TestStartBotLaunchesLoopAndStopsPromptly verifies that startBot actually
// launches runBotLoop (the loop polls the client) and that stopBot returns
// promptly instead of waiting out the 10 second graceful-shutdown timeout.
func TestStartBotLaunchesLoopAndStopsPromptly(t *testing.T) {
	bm := newBotManager(nil, nil, nil, nil, context.Background(), "testchannel", 0, audio.Config{}, false, nil)

	fake := newFakeTwitchClient()
	bm.newIRCClient = func(login, password, channel string) TwitchClient {
		return fake
	}

	bm.mu.Lock()
	bm.bots["bot-1"] = &botRuntime{
		cfg: storage.BotConfig{
			ID:             "bot-1",
			TwitchLogin:    "testlogin",
			Enabled:        true,
			MinDelaySec:    1,
			MaxDelaySec:    1,
			CurrentChannel: "testchannel",
		},
		client: fake,
	}
	bm.mu.Unlock()

	require.NoError(t, bm.startBot(context.Background(), "bot-1", false, true))

	// Wait until runBotLoop is actually executing: its first client poll is
	// the first observable side effect after the initial message delay.
	select {
	case <-fake.polled:
	case <-time.After(5 * time.Second):
		t.Fatal("runBotLoop did not start: client was never polled")
	}

	start := time.Now()
	require.NoError(t, bm.stopBot("bot-1", false))
	elapsed := time.Since(start)
	assert.Less(t, elapsed, 2*time.Second, "stopBot should return promptly, took %v", elapsed)

	bm.mu.RLock()
	running := bm.bots["bot-1"].running
	bm.mu.RUnlock()
	assert.False(t, running, "bot should not be running after stopBot")
}

// fakeTokenService stands in for *bottoken.Service: it records ctx.Err() at
// every EnsureFresh call (so tests can see whether the refresh context was
// alive) and returns a canned result. It holds no token material.
type fakeTokenService struct {
	mu      sync.Mutex
	errs    []error
	updated storage.BotConfig
	err     error
}

func (f *fakeTokenService) EnsureFresh(ctx context.Context, cfg storage.BotConfig) (storage.BotConfig, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.errs = append(f.errs, ctx.Err())
	if f.err != nil {
		return cfg, f.err
	}
	return f.updated, nil
}

func (f *fakeTokenService) calls() []error {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]error(nil), f.errs...)
}

// TestStartBotRefreshesOnLiveContextDespiteCanceledParent pins the StartAll
// fix: a spent shared budget must not abort an in-flight rotation (the refresh
// runs under WithoutCancel), the refreshed credential must reach
// newIRCClient, and the rotation must not hold bm.mu.
func TestStartBotRefreshesOnLiveContextDespiteCanceledParent(t *testing.T) {
	bm := newBotManager(nil, nil, nil, nil, context.Background(), "testchannel", 0, audio.Config{}, false, nil)

	expires := time.Now().Add(time.Minute) // inside tokenRefreshLead: forces a rotation
	base := storage.BotConfig{
		ID:             "bot-1",
		TwitchLogin:    "testlogin",
		Password:       "oauth:old",
		TokenExpiresAt: &expires,
		Enabled:        true,
		MinDelaySec:    1,
		MaxDelaySec:    1,
		CurrentChannel: "testchannel",
	}
	refreshed := base
	refreshed.Password = "oauth:newaccess"
	renewed := time.Now().Add(time.Hour)
	refreshed.TokenExpiresAt = &renewed

	svc := &fakeTokenService{updated: refreshed}
	bm.tokenService = svc

	var spyMu sync.Mutex
	var passwords []string
	fake := newFakeTwitchClient()
	bm.newIRCClient = func(login, password, channel string) TwitchClient {
		spyMu.Lock()
		passwords = append(passwords, password)
		spyMu.Unlock()
		return fake
	}

	bm.mu.Lock()
	bm.bots["bot-1"] = &botRuntime{cfg: base, client: fake}
	bm.mu.Unlock()

	parent, cancel := context.WithCancel(context.Background())
	cancel() // simulate the single StartAll budget already spent

	require.NoError(t, bm.startBot(parent, "bot-1", false, true))

	calls := svc.calls()
	require.Len(t, calls, 1, "an expiring token must trigger exactly one EnsureFresh")
	assert.NoError(t, calls[0], "the rotation must run on a live context despite the canceled parent")

	spyMu.Lock()
	got := append([]string(nil), passwords...)
	spyMu.Unlock()
	require.Contains(t, got, "oauth:newaccess", "newIRCClient must receive the refreshed password")

	bm.mu.RLock()
	stored := bm.bots["bot-1"].cfg
	bm.mu.RUnlock()
	assert.Equal(t, "oauth:newaccess", stored.Password, "the refreshed config must be written back to the runtime")

	require.NoError(t, bm.stopBot("bot-1", false))
}

// TestStartBotTokenRefreshFailureKeepsErrorShape pins the refusal contract: a
// failed rotation aborts the start with "bot %s: twitch token: %w" and leaves
// the runtime untouched and not running.
func TestStartBotTokenRefreshFailureKeepsErrorShape(t *testing.T) {
	bm := newBotManager(nil, nil, nil, nil, context.Background(), "testchannel", 0, audio.Config{}, false, nil)

	expires := time.Now().Add(time.Minute)
	cfg := storage.BotConfig{
		ID:             "bot-1",
		TwitchLogin:    "testlogin",
		Password:       "oauth:old",
		TokenExpiresAt: &expires,
		MinDelaySec:    1,
		MaxDelaySec:    1,
		CurrentChannel: "testchannel",
	}

	failure := errors.New("refresh unavailable")
	svc := &fakeTokenService{err: failure}
	bm.tokenService = svc

	bm.mu.Lock()
	bm.bots["bot-1"] = &botRuntime{cfg: cfg, client: newFakeTwitchClient()}
	bm.mu.Unlock()

	err := bm.startBot(context.Background(), "bot-1", false, true)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "bot bot-1: twitch token:")
	assert.ErrorIs(t, err, failure)

	bm.mu.RLock()
	br := bm.bots["bot-1"]
	running, password := br.running, br.cfg.Password
	bm.mu.RUnlock()
	assert.False(t, running, "a failed refresh must not start the bot")
	assert.Equal(t, "oauth:old", password, "a failed refresh must not rewrite the stored password")
}

// TestStopBotDoesNotWaitForNeverStartedLoop verifies that stopping a bot whose
// loop never ran returns immediately instead of waiting out the 10 second
// graceful-shutdown timeout.
func TestStopBotDoesNotWaitForNeverStartedLoop(t *testing.T) {
	bm := newBotManager(nil, nil, nil, nil, context.Background(), "testchannel", 0, audio.Config{}, false, nil)

	bm.mu.Lock()
	bm.bots["bot-1"] = &botRuntime{
		cfg:    storage.BotConfig{ID: "bot-1"},
		client: newFakeTwitchClient(),
	}
	bm.mu.Unlock()

	start := time.Now()
	require.NoError(t, bm.stopBot("bot-1", false))
	require.Less(t, time.Since(start), 500*time.Millisecond, "stopBot waited for a loop that never started")
}

// TestStaleLoopDoesNotSendAfterCancelDuringWait pins the stale-loop window:
// WaitForRateLimit runs without the client lock, so the loop's context can be
// cancelled (and stopBot return, and a newer run start) while the wait is
// sleeping. Once the wait returns, the loop must notice and exit instead of
// sending through the client it captured for this run.
func TestStaleLoopDoesNotSendAfterCancelDuringWait(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/key":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"data":{"usage_daily":0,"free_model_daily_requests":{"limit":50,"remaining":50,"used":0}}}`))
		case "/chat/completions":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"model":"openrouter/free","choices":[{"message":{"content":"hello from llm"}}],"usage":{"cost":0}}`))
		default:
			t.Errorf("unexpected llm path %s", r.URL.Path)
		}
	}))
	defer srv.Close()

	llmClient := llm.NewClient(llm.Config{BaseURL: srv.URL, APIKey: "sk-test", FreeModel: "openrouter/free"})

	bm := newBotManager(nil, llmClient, nil, nil, context.Background(), "testchannel", 0, audio.Config{}, false, nil)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	fake := newFakeTwitchClient()
	// The cancellation lands exactly while the loop is inside
	// WaitForRateLimit: the window the loop used to be blind to.
	fake.onWait = cancel

	br := &botRuntime{
		cfg: storage.BotConfig{
			ID:             "bot-1",
			TwitchLogin:    "testlogin",
			Enabled:        true,
			MinDelaySec:    1,
			MaxDelaySec:    1,
			CurrentChannel: "testchannel",
		},
		client: fake,
	}

	// Run the loop in the current goroutine so the test observes runBotLoop
	// returning after the cancellation.
	bm.runBotLoop(ctx, br)

	assert.Empty(t, fake.SentMessages(), "loop sent through its client after the run was cancelled during the rate-limit wait")
}

func TestMarkBotErrorIfActiveSkipsCancelledContext(t *testing.T) {
	bm := newBotManager(nil, nil, nil, nil, context.Background(), "testchannel", 0, audio.Config{}, false, nil)
	br := &botRuntime{cfg: storage.BotConfig{ID: "bot-1"}, running: true}
	bm.bots["bot-1"] = br

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	if bm.markBotErrorIfActive(ctx, br) {
		t.Fatal("cancelled loop must not mark the runtime as errored")
	}
	if br.presenceState == presenceStateError || br.stoppedByError || !br.running {
		t.Fatalf("cancelled loop mutated the runtime: state=%q stoppedByError=%v running=%v",
			br.presenceState, br.stoppedByError, br.running)
	}

	if !bm.markBotErrorIfActive(context.Background(), br) {
		t.Fatal("active loop must still mark the runtime as errored")
	}
	if br.presenceState != presenceStateError || !br.stoppedByError || br.running {
		t.Fatal("active path must mark the error state")
	}
}

func TestSleepCtxStopsOnCancel(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	start := time.Now()
	if sleepCtx(ctx, time.Second) {
		t.Fatal("sleepCtx must report cancellation")
	}
	if elapsed := time.Since(start); elapsed > 200*time.Millisecond {
		t.Fatalf("sleepCtx ignored cancellation for %v", elapsed)
	}
	if !sleepCtx(context.Background(), 10*time.Millisecond) {
		t.Fatal("sleepCtx must return true when the context is alive")
	}
}
