package main

import (
	"sync"
	"testing"
	"time"

	"twitch-smm-bot/internal/storage"
)

// TestNormalizeChannelName tests channel name normalization
func TestNormalizeChannelName(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"channel1", "channel1"},
		{"#channel2", "channel2"},
		{"https://twitch.tv/channel3", "channel3"},
		{"https://twitch.tv/channel4/something", "channel4"},
		{"", ""},
		{"   ", ""},
		{"#", ""},
	}

	for _, tt := range tests {
		got := normalizeChannelName(tt.input)
		if got != tt.expected {
			t.Errorf("normalizeChannelName(%s) = %s, want %s", tt.input, got, tt.expected)
		}
	}
}

// TestNewGlobalLimiter tests rate limiter creation
func TestNewGlobalLimiter(t *testing.T) {
	limiter := newGlobalLimiter(30, nil)
	if limiter == nil {
		t.Fatal("expected limiter to be created")
	}
	if limiter.limit != 30 {
		t.Errorf("expected limit 30, got %d", limiter.limit)
	}
	if limiter.window != time.Minute {
		t.Errorf("expected window 1 minute, got %v", limiter.window)
	}
}

// TestNewGlobalLimiterDefault tests default limit
func TestNewGlobalLimiterDefault(t *testing.T) {
	limiter := newGlobalLimiter(0, nil)
	if limiter.limit != 30 {
		t.Errorf("expected default limit 30, got %d", limiter.limit)
	}
}

// TestGlobalLimiterAllow tests the allow logic
func TestGlobalLimiterAllow(t *testing.T) {
	limiter := newGlobalLimiter(5, nil)
	now := time.Now()

	// First 5 requests should be allowed
	for i := 0; i < 5; i++ {
		if !limiter.allow(now) {
			t.Errorf("expected allow() = true for request %d", i)
		}
	}

	// 6th request should be denied
	if limiter.allow(now) {
		t.Error("expected allow() = false after limit reached")
	}
}

// TestGlobalLimiterCleanup tests event cleanup
func TestGlobalLimiterCleanup(t *testing.T) {
	limiter := newGlobalLimiter(100, nil)
	now := time.Now()

	// Add some events
	limiter.events = []time.Time{
		now.Add(-2 * time.Minute),    // should be cleaned (older than 1 min window)
		now.Add(-30 * time.Second), // should stay (within 1 min window)
		now.Add(-10 * time.Second), // should stay (within 1 min window)
		now,                         // should stay (within 1 min window)
	}

	// Run cleanup
	limiter.cleanup()

	// Should have 3 events (within window)
	if len(limiter.events) != 3 {
		t.Errorf("expected 3 events after cleanup, got %d", len(limiter.events))
	}
}

// TestGlobalLimiterMaxEvents tests max events truncation
func TestGlobalLimiterMaxEvents(t *testing.T) {
	limiter := newGlobalLimiter(10, nil)
	limiter.maxEvents = 5

	// Add more events than max
	now := time.Now()
	for i := 0; i < 10; i++ {
		limiter.events = append(limiter.events, now.Add(time.Duration(i)*time.Second))
	}

	// Run cleanup
	limiter.cleanup()

	// Should truncate to maxEvents
	if len(limiter.events) > limiter.maxEvents {
		t.Errorf("expected at most %d events, got %d", limiter.maxEvents, len(limiter.events))
	}
}

// TestBotRuntimeStruct tests the botRuntime struct
func TestBotRuntimeStruct(t *testing.T) {
	br := &botRuntime{
		cfg: storage.BotConfig{
			ID:          "test-id",
			TwitchLogin: "testlogin",
			Name:        "Test Bot",
			Personality:  "веселый_фанат",
		},
		running:    false,
		stopping:   false,
	}

	if br.cfg.ID != "test-id" {
		t.Errorf("expected ID 'test-id', got '%s'", br.cfg.ID)
	}
	if br.running {
		t.Error("expected running to be false")
	}
	if br.stopping {
		t.Error("expected stopping to be false")
	}
}

// TestMinInterval tests the minInterval function
func TestMinInterval(t *testing.T) {
	tests := []struct {
		ratePerMin int
		expected    time.Duration
	}{
		{0, time.Minute / 10},      // default
		{1, time.Minute},           // 1 msg/min
		{10, time.Minute / 10},     // 10 msgs/min
		{30, time.Minute / 30},     // 30 msgs/min
		{60, time.Second},          // 60 msgs/min
	}

	for _, tt := range tests {
		got := minInterval(tt.ratePerMin)
		if got != tt.expected {
			t.Errorf("minInterval(%d) = %v, want %v", tt.ratePerMin, got, tt.expected)
		}
	}
}

// TestRandomDelay tests the random delay function
func TestRandomDelay(t *testing.T) {
	// Test with valid range
	delay := randomDelay(10, 20)
	if delay < 10*time.Second || delay > 20*time.Second {
		t.Errorf("randomDelay(10, 20) = %v, want between 10-20s", delay)
	}

	// Test with equal min and max
	delay = randomDelay(15, 15)
	if delay != 15*time.Second {
		t.Errorf("randomDelay(15, 15) = %v, want 15s", delay)
	}

	// Test with invalid range (max < min)
	delay = randomDelay(20, 10)
	if delay < 10*time.Second || delay > 20*time.Second {
		t.Errorf("randomDelay(20, 10) = %v, want between 10-20s", delay)
	}

	// Test with zero min (should use default min=45, then max=min=45)
	delay = randomDelay(0, 10)
	if delay != 45*time.Second {
		t.Errorf("randomDelay(0, 10) = %v, want exactly 45s (default min=45, max=min)", delay)
	}
}

// TestConcurrentGlobalLimiter tests concurrent access
func TestConcurrentGlobalLimiter(t *testing.T) {
	limiter := newGlobalLimiter(100, nil)
	var wg sync.WaitGroup

	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			now := time.Now()
			limiter.allow(now)
			limiter.cleanup()
		}()
	}
	wg.Wait()

	// Should not panic or race
}

// TestBotManagerRLock tests RWMutex read locks
func TestBotManagerRLock(t *testing.T) {
	bm := &botManager{
		mu:       sync.RWMutex{},
		bots:     make(map[string]*botRuntime),
	}

	// Add a bot
	bm.bots["test-id"] = &botRuntime{
		cfg: storage.BotConfig{
			ID: "test-id",
		},
	}

	// Test read lock (should not block)
	bm.mu.RLock()
	_, exists := bm.bots["test-id"]
	bm.mu.RUnlock()

	if !exists {
		t.Error("expected bot 'test-id' to exist")
	}
}

// TestPrimaryChannelFromSettings tests channel selection
func TestPrimaryChannelFromSettings(t *testing.T) {
	// Test with debug mode and debug channel
	settings := &storage.Settings{
		DebugMode:    true,
		DebugChannel: "#debug_channel",
		Channels:     []string{"channel1", "channel2"},
	}
	ch := primaryChannelFromSettings(settings, "fallback")
	if ch != "debug_channel" {
		t.Errorf("expected 'debug_channel', got '%s'", ch)
	}

	// Test without debug mode (use first channel)
	settings = &storage.Settings{
		DebugMode: false,
		Channels:  []string{"channel1", "channel2"},
	}
	ch = primaryChannelFromSettings(settings, "fallback")
	if ch != "channel1" {
		t.Errorf("expected 'channel1', got '%s'", ch)
	}

	// Test with empty channels (use fallback)
	settings = &storage.Settings{
		DebugMode: false,
		Channels:  []string{},
	}
	ch = primaryChannelFromSettings(settings, "fallback")
	if ch != "fallback" {
		t.Errorf("expected 'fallback', got '%s'", ch)
	}
}
