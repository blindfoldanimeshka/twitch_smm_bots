package main

import (
	"context"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
	"unicode"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"twitch-smm-bot/internal/audio"
	"twitch-smm-bot/internal/storage"
)

// fakePresenceTimer is a presenceTimer stub that records whether Stop was
// called on it.
type fakePresenceTimer struct {
	mu      sync.Mutex
	stopped bool
}

func (f *fakePresenceTimer) Stop() bool {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.stopped = true
	return true
}

func (f *fakePresenceTimer) wasStopped() bool {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.stopped
}

// fakePresenceAfter captures scheduled presence callbacks instead of running
// them on a real clock, so tests never wait real minutes.
type fakePresenceAfter struct {
	mu        sync.Mutex
	timers    []*fakePresenceTimer
	durations []time.Duration
	callbacks []func()
}

func (f *fakePresenceAfter) after(d time.Duration, fn func()) presenceTimer {
	f.mu.Lock()
	defer f.mu.Unlock()
	timer := &fakePresenceTimer{}
	f.timers = append(f.timers, timer)
	f.durations = append(f.durations, d)
	f.callbacks = append(f.callbacks, fn)
	return timer
}

func (f *fakePresenceAfter) scheduled() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return len(f.callbacks)
}

func (f *fakePresenceAfter) callbackAt(i int) func() {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.callbacks[i]
}

// newPresenceTestManager wires a botManager without external dependencies and
// with a fake presence scheduler.
func newPresenceTestManager(t *testing.T) (*botManager, *fakePresenceAfter) {
	t.Helper()
	bm := newBotManager(nil, nil, nil, nil, context.Background(), "testchannel", 0, audio.Config{}, false, nil)
	fakeAfter := &fakePresenceAfter{}
	bm.presenceAfter = fakeAfter.after
	return bm, fakeAfter
}

// containsCyrillic reports whether s contains at least one Cyrillic letter,
// i.e. the phrase is Russian rather than empty or Latin filler.
func containsCyrillic(s string) bool {
	for _, r := range s {
		if unicode.Is(unicode.Cyrillic, r) {
			return true
		}
	}
	return false
}

// TestPickPresenceDuration verifies that the session/cooldown picker stays in
// [min, max] and returns the exact value when min == max.
func TestPickPresenceDuration(t *testing.T) {
	for i := 0; i < 200; i++ {
		d := pickPresenceDuration(20*time.Minute, 40*time.Minute)
		assert.True(t, d >= 20*time.Minute, "duration %v below minimum", d)
		assert.True(t, d <= 40*time.Minute, "duration %v above maximum", d)
	}

	assert.Equal(t, 30*time.Minute, pickPresenceDuration(30*time.Minute, 30*time.Minute),
		"min == max must return exactly that duration")

	// max < min is clamped to min.
	assert.Equal(t, 20*time.Minute, pickPresenceDuration(20*time.Minute, 5*time.Minute))
}

// TestPresenceGreetingFarewellPools verifies every personality has non-empty
// Russian greeting and farewell pools that do not overlap.
func TestPresenceGreetingFarewellPools(t *testing.T) {
	for _, personality := range []string{PersonalityCheerful, PersonalityToxic, PersonalityNeutral, PersonalityHyper} {
		greetings, ok := presenceGreetings[personality]
		require.True(t, ok, "greeting pool missing for %q", personality)
		farewells, ok := presenceFarewells[personality]
		require.True(t, ok, "farewell pool missing for %q", personality)

		assert.GreaterOrEqual(t, len(greetings), 3, "personality %q needs at least 3 greetings", personality)
		assert.GreaterOrEqual(t, len(farewells), 3, "personality %q needs at least 3 farewells", personality)

		for _, phrase := range greetings {
			assert.NotEmpty(t, strings.TrimSpace(phrase), "personality %q has an empty greeting", personality)
			assert.True(t, containsCyrillic(phrase), "greeting %q must be Russian", phrase)
		}
		for _, phrase := range farewells {
			assert.NotEmpty(t, strings.TrimSpace(phrase), "personality %q has an empty farewell", personality)
			assert.True(t, containsCyrillic(phrase), "farewell %q must be Russian", phrase)
		}

		greeting := randomPresencePhrase(personality, presenceGreeting)
		farewell := randomPresencePhrase(personality, presenceFarewell)
		assert.NotEmpty(t, greeting)
		assert.NotEmpty(t, farewell)
		assert.NotEqual(t, greeting, farewell, "greeting and farewell pools must not overlap for %q", personality)
	}
}

// TestPresenceConfigFromSettings verifies Settings fields map to presence
// durations and booleans, with sane defaults and clamping.
func TestPresenceConfigFromSettings(t *testing.T) {
	cfg := presenceConfigFromSettings(&storage.Settings{
		PresenceEnabled:            true,
		PresenceSessionMinMinutes:  15,
		PresenceSessionMaxMinutes:  45,
		PresenceCooldownMinMinutes: 5,
		PresenceCooldownMaxMinutes: 25,
		PresenceGreetingEnabled:    true,
		PresenceFarewellEnabled:    false,
	})
	assert.True(t, cfg.enabled)
	assert.Equal(t, 15*time.Minute, cfg.sessionMin)
	assert.Equal(t, 45*time.Minute, cfg.sessionMax)
	assert.Equal(t, 5*time.Minute, cfg.cooldownMin)
	assert.Equal(t, 25*time.Minute, cfg.cooldownMax)
	assert.True(t, cfg.greeting)
	assert.False(t, cfg.farewell)

	// Nil settings fall back to the built-in defaults.
	def := presenceConfigFromSettings(nil)
	assert.True(t, def.enabled)
	assert.Equal(t, defaultPresenceSessionMin, def.sessionMin)
	assert.Equal(t, defaultPresenceSessionMax, def.sessionMax)
	assert.Equal(t, defaultPresenceCooldownMin, def.cooldownMin)
	assert.Equal(t, defaultPresenceCooldownMax, def.cooldownMax)
	assert.True(t, def.greeting)
	assert.True(t, def.farewell)

	// Zero values fall back to defaults, max < min is clamped.
	zero := presenceConfigFromSettings(&storage.Settings{PresenceEnabled: true})
	assert.Equal(t, defaultPresenceSessionMin, zero.sessionMin)
	assert.Equal(t, defaultPresenceSessionMax, zero.sessionMax)

	clamped := presenceConfigFromSettings(&storage.Settings{
		PresenceEnabled:           true,
		PresenceSessionMinMinutes: 60,
		PresenceSessionMaxMinutes: 10,
	})
	assert.Equal(t, 60*time.Minute, clamped.sessionMax, "sessionMax must be clamped to sessionMin")
}

// TestManualStopCancelsPresenceTimer verifies a manual stop clears the
// presence session state and cancels any scheduled return.
func TestManualStopCancelsPresenceTimer(t *testing.T) {
	bm, _ := newPresenceTestManager(t)

	fake := newFakeTwitchClient()
	timer := &fakePresenceTimer{}
	stopped := make(chan struct{})
	close(stopped) // old loop already exited

	bm.mu.Lock()
	bm.bots["bot-1"] = &botRuntime{
		cfg: storage.BotConfig{
			ID:             "bot-1",
			TwitchLogin:    "login",
			CurrentChannel: "testchannel",
		},
		client:        fake,
		running:       true,
		cancel:        func() {},
		stopped:       stopped,
		presenceState: presenceStateOnline,
		presenceTimer: timer,
		cooldownUntil: time.Now().Add(time.Hour),
		autoPaused:    true,
	}
	bm.mu.Unlock()

	// StopBot persists through the store; use the shared stop path with
	// persist=false because the test manager has no store.
	require.NoError(t, bm.stopBot("bot-1", false))

	bm.mu.RLock()
	br := bm.bots["bot-1"]
	bm.mu.RUnlock()
	assert.Equal(t, presenceStateStopped, br.presenceState)
	assert.True(t, br.cooldownUntil.IsZero(), "cooldown must be cleared")
	assert.True(t, timer.wasStopped(), "presence timer must be stopped")
	assert.False(t, br.autoPaused)
	assert.False(t, br.stoppedByError)
}

// TestErrorStopDoesNotScheduleReturn verifies markBotError flags the runtime
// as errored, cancels the timer, and that the already-captured session-pause
// callback becomes a no-op afterwards.
func TestErrorStopDoesNotScheduleReturn(t *testing.T) {
	bm, fakeAfter := newPresenceTestManager(t)

	fake := newFakeTwitchClient()
	require.NoError(t, fake.Connect())
	bm.mu.Lock()
	br := &botRuntime{
		cfg: storage.BotConfig{
			ID:             "bot-1",
			TwitchLogin:    "login",
			Personality:    PersonalityToxic,
			CurrentChannel: "testchannel",
		},
		client:        fake,
		running:       true,
		cancel:        func() {},
		presenceState: presenceStateOnline,
	}
	bm.bots["bot-1"] = br
	bm.presence = presenceConfig{enabled: true, greeting: false, farewell: false}
	bm.mu.Unlock()

	// Model the session pause that startBot would have scheduled.
	br.presenceTimer = bm.presenceAfter(time.Hour, func() { bm.pauseBot("bot-1") })

	bm.markBotError(br)

	assert.Equal(t, presenceStateError, br.presenceState)
	assert.True(t, br.stoppedByError)
	assert.True(t, fakeAfter.timers[0].wasStopped(), "error stop must cancel the presence timer")

	// Invoking the captured session-pause callback must do nothing.
	fakeAfter.callbacks[0]()
	assert.Equal(t, presenceStateError, br.presenceState)
	assert.True(t, br.running, "pauseBot must not touch an errored runtime")
	assert.True(t, fake.IsConnected(), "client must not be closed for an errored bot")
	assert.Equal(t, 1, fakeAfter.scheduled(), "no restart may be scheduled after an error stop")
}

// TestDisablePresenceCancelsCooldown verifies ApplySettings with
// presence_enabled=false parks cooldown bots as stopped and cancels the
// scheduled return.
func TestDisablePresenceCancelsCooldown(t *testing.T) {
	bm, _ := newPresenceTestManager(t)

	fake := newFakeTwitchClient()
	timer := &fakePresenceTimer{}
	bm.mu.Lock()
	bm.bots["bot-1"] = &botRuntime{
		cfg: storage.BotConfig{
			ID:             "bot-1",
			TwitchLogin:    "login",
			CurrentChannel: "testchannel",
		},
		client:        fake,
		presenceState: presenceStateCooldown,
		presenceTimer: timer,
		cooldownUntil: time.Now().Add(time.Hour),
		autoPaused:    true,
		cancel:        func() {},
	}
	bm.mu.Unlock()

	settings := &storage.Settings{PresenceEnabled: false}
	require.NoError(t, bm.ApplySettings(context.Background(), settings))

	assert.False(t, bm.presence.enabled)
	assert.True(t, timer.wasStopped(), "cooldown timer must be stopped when presence is disabled")

	bm.mu.RLock()
	br := bm.bots["bot-1"]
	bm.mu.RUnlock()
	assert.Equal(t, presenceStateStopped, br.presenceState)
	assert.True(t, br.cooldownUntil.IsZero(), "cooldown must be cleared")
	assert.False(t, br.autoPaused)
}

// TestPauseBotEntersCooldown verifies pauseBot parks an online bot in
// cooldown, sends and records a farewell, waits for the old loop to exit
// before scheduling the return, and that the captured restart callback starts
// the bot exactly once.
func TestPauseBotEntersCooldown(t *testing.T) {
	bm, fakeAfter := newPresenceTestManager(t)

	var clientCreations int32
	bm.newIRCClient = func(login, password, channel string) TwitchClient {
		atomic.AddInt32(&clientCreations, 1)
		return newFakeTwitchClient()
	}

	fake := newFakeTwitchClient()
	stopped := make(chan struct{}) // stays open: models the old loop still running

	bm.mu.Lock()
	bm.bots["bot-1"] = &botRuntime{
		cfg: storage.BotConfig{
			ID:             "bot-1",
			TwitchLogin:    "login",
			Personality:    PersonalityCheerful,
			MinDelaySec:    1,
			MaxDelaySec:    1,
			CurrentChannel: "testchannel",
		},
		client:        fake,
		running:       true,
		cancel:        func() {},
		stopped:       stopped,
		presenceState: presenceStateOnline,
	}
	bm.presence = presenceConfig{
		enabled:     true,
		sessionMin:  time.Minute,
		sessionMax:  time.Minute,
		cooldownMin: 10 * time.Minute,
		cooldownMax: 20 * time.Minute,
		greeting:    false,
		farewell:    true,
	}
	bm.mu.Unlock()

	pauseDone := make(chan struct{})
	go func() {
		defer close(pauseDone)
		bm.pauseBot("bot-1")
	}()

	// The bot parks itself in cooldown without waiting for the loop to exit.
	require.Eventually(t, func() bool {
		bm.mu.RLock()
		defer bm.mu.RUnlock()
		return bm.bots["bot-1"].presenceState == presenceStateCooldown
	}, 2*time.Second, 5*time.Millisecond)

	// The old loop is still alive: no restart may be scheduled yet.
	assert.Equal(t, 0, fakeAfter.scheduled(), "restart must not be scheduled while the old loop is alive")

	close(stopped) // the old loop exits now
	select {
	case <-pauseDone:
	case <-time.After(3 * time.Second):
		t.Fatal("pauseBot did not finish after the old loop stopped")
	}

	bm.mu.RLock()
	br := bm.bots["bot-1"]
	cooldownUntil := br.cooldownUntil
	autoPaused := br.autoPaused
	running := br.running
	bm.mu.RUnlock()
	assert.False(t, running)
	assert.True(t, autoPaused)
	assert.True(t, cooldownUntil.After(time.Now()), "cooldownUntil must be in the future")
	assert.False(t, fake.IsConnected(), "client must be closed after the farewell")

	// Farewell recorded: exactly one send counted (greeting disabled here).
	assert.Equal(t, 1, bm.MessagesPerMin())
	sent := fake.SentMessages()
	require.Len(t, sent, 1)
	assert.True(t, containsCyrillic(sent[0]), "farewell must be a Russian phrase")

	// The return is scheduled after the cooldown.
	require.Eventually(t, func() bool {
		return fakeAfter.scheduled() == 1
	}, 2*time.Second, 5*time.Millisecond)
	restart := fakeAfter.callbackAt(0)
	require.NotNil(t, restart)

	// Firing the callback restarts the bot exactly once.
	restart()
	bm.mu.RLock()
	restartedRunning := bm.bots["bot-1"].running
	restartedState := bm.bots["bot-1"].presenceState
	bm.mu.RUnlock()
	assert.True(t, restartedRunning, "restart callback must start the bot again")
	assert.Equal(t, presenceStateOnline, restartedState)

	// A second fire must not create a second runtime.
	restart()
	assert.Equal(t, int32(1), atomic.LoadInt32(&clientCreations),
		"restart callback must not create a second runtime")

	// Cleanup: stop the restarted loop and any pending session timer.
	require.NoError(t, bm.stopBot("bot-1", false))
}
