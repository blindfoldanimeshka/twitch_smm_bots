package main

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"twitch-smm-bot/internal/audio"
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

func (f *fakeTwitchClient) WaitForRateLimit() {}

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
