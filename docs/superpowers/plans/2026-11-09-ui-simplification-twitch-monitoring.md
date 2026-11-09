# UI Simplification and Twitch Monitoring Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Replace the heavy dark UI with a clean light four-page dashboard, add Twitch stream monitoring (uptime, viewers, chat, player), add presence simulation instead of channel rotation, and fix the known frontend/backend defects.

**Architecture:** Keep the Go backend and React frontend. Add a public GraphQL Twitch client, an anonymous read-only IRC monitor, and a `StreamMonitor` service exposed at `/api/twitch/stream`. Extend bot runtime with presence sessions and launch the currently unused `runBotLoop`. Rebuild the frontend shell and pages around a small light design system; delete the old visual components and styles.

**Tech Stack:** Go 1.26+, PostgreSQL 16, Redis 7, React 19, TypeScript, Vite, `@tanstack/react-query`, `lucide-react`, Twitch public GraphQL, Twitch IRC over TLS.

**Spec:** `docs/superpowers/specs/2026-11-09-ui-simplification-twitch-monitoring-design.md`

## Global Constraints

- UI is light-only; no dark mode. Tokens: background `#efefec`, card `#fbfbf9`, text `#181816`, muted `#6f6f68`, border `#deded6`, accent `#c9f24d`.
- Four sections only: Dashboard, Stream, Bots, Settings.
- Active viewer = unique `PRIVMSG` author in the last **300 seconds**, excluding bot logins. Passive = `max(0, viewers_total - active)`.
- Twitch GQL: `POST https://gql.twitch.tv/gql`, header `Client-ID: kimne78kx3ncx6brgo4mv6wki5h1ko`, no token. Query `user.stream.createdAt`, `viewersCount`, `game.name`, `title`, `type`.
- GQL cache: 15 seconds per channel; request timeout 5 seconds. Maximum monitored channels: 5.
- IRC monitor is anonymous and read-only: `PASS SCHMOOPIIE`, `NICK justinfan...`, `JOIN #channel`; never send `PRIVMSG`.
- Chat feed: keep last 100 messages or 10 minutes; API returns at most 50; `is_bot` is true for configured bot logins.
- Player URL: `https://player.twitch.tv/?channel=<login>&parent=<window.location.hostname>&muted=true&autoplay=true`.
- Presence defaults: enabled `TRUE`, session `20..40` minutes, cooldown `10..20` minutes, greeting/farewell enabled. Valid range `1 <= min <= max <= 240`.
- Presence simulation is not viewbotting: it only connects to IRC and sends chat messages; it must not try to change `viewer_count`.
- Rotation is removed from UI and runtime. `rotation_enabled`/`rotation_interval` stay in the DB for compatibility but are unused.
- `startBot` must launch `go bm.runBotLoop(ctx, br)`. Manual stop and error stop must never auto-restart.
- Never log or return OAuth bot tokens, passwords, or API keys. Remove payload/response console logging from the frontend.
- All new UI errors are Russian and inline/toast; no `alert()` or `window.confirm()`.
- Every task ends with `go build ./...` and the relevant tests; frontend tasks end with `npm.cmd run build`.

## Review Focus

- `startBot` must actually start the bot loop; a bot must attempt generation and `StopBot` must return without the 10-second graceful-timeout warning. Pinned in Task 5.
- Manual stop, error stop, and disabling presence must never schedule an automatic return; only a completed presence session schedules a cooldown return. Pinned in Task 6.
- GQL returns offline, missing user, malformed JSON, or HTTP 500 → endpoint still returns HTTP 200 with `configured/live/error` fields and no fake viewer numbers. Pinned in Tasks 2 and 4.
- Bot messages must be marked `is_bot` and excluded from `active_viewers`; `active + passive == viewers_total` when total is known. Pinned in Task 4.
- Switching/removing a monitored channel must stop the old IRC monitor and must not leak old messages or chatters into the new channel. Pinned in Task 3.

---

### Task 1: Presence settings schema and migration

**Files:**
- Modify: `internal/storage/settings.go`
- Modify: `internal/storage/settings_migration_test.go`
- Test: `internal/storage/settings_migration_test.go`

**Interfaces:**
- Consumes: existing `Settings`, `EnsureSettingsTable`, `GetSettings`, `UpdateSettings`.
- Produces: `Settings.PresenceEnabled bool`, `PresenceSessionMinMinutes int`, `PresenceSessionMaxMinutes int`, `PresenceCooldownMinMinutes int`, `PresenceCooldownMaxMinutes int`, `PresenceGreetingEnabled bool`, `PresenceFarewellEnabled bool` with JSON tags `presence_enabled`, `presence_session_min_minutes`, `presence_session_max_minutes`, `presence_cooldown_min_minutes`, `presence_cooldown_max_minutes`, `presence_greeting_enabled`, `presence_farewell_enabled`.

- [ ] **Step 1: Write the failing migration test**

Append to `internal/storage/settings_migration_test.go`:

```go
func TestEnsureSettingsTableAddsPresenceDefaults(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, "postgres://botfarm:botfarm@localhost:5432/botfarm?sslmode=disable")
	if err != nil {
		t.Skipf("postgres not available: %v", err)
	}
	defer pool.Close()
	if err := pool.Ping(ctx); err != nil {
		t.Skipf("postgres not available: %v", err)
	}
	store := &Store{pool: pool}
	require.NoError(t, store.EnsureSettingsTable(ctx))

	got, err := store.GetSettings(ctx)
	require.NoError(t, err)
	assert.True(t, got.PresenceEnabled)
	assert.Equal(t, 20, got.PresenceSessionMinMinutes)
	assert.Equal(t, 40, got.PresenceSessionMaxMinutes)
	assert.Equal(t, 10, got.PresenceCooldownMinMinutes)
	assert.Equal(t, 20, got.PresenceCooldownMaxMinutes)
	assert.True(t, got.PresenceGreetingEnabled)
	assert.True(t, got.PresenceFarewellEnabled)
}
```

- [ ] **Step 2: Run it to verify it fails**

Run: `go test ./internal/storage -run TestEnsureSettingsTableAddsPresenceDefaults -v`
Expected: FAIL — fields and columns do not exist.

- [ ] **Step 3: Add fields, DDL, migration, SELECT/Scan, UPDATE**

- Add fields after `RotationInterval` in `Settings`, with the JSON tags above.
- Add columns to `CREATE TABLE settings` after `rotation_interval`:
  ```sql
  presence_enabled BOOLEAN NOT NULL DEFAULT TRUE,
  presence_session_min_minutes INT NOT NULL DEFAULT 20,
  presence_session_max_minutes INT NOT NULL DEFAULT 40,
  presence_cooldown_min_minutes INT NOT NULL DEFAULT 10,
  presence_cooldown_max_minutes INT NOT NULL DEFAULT 20,
  presence_greeting_enabled BOOLEAN NOT NULL DEFAULT TRUE,
  presence_farewell_enabled BOOLEAN NOT NULL DEFAULT TRUE,
  ```
- Append the matching `ALTER TABLE settings ADD COLUMN IF NOT EXISTS ...` statements to `settingsTableSQL`.
- Add the seven columns to `GetSettings` SELECT and `Scan` in the same order.
- Add them to `UpdateSettings` and renumber all placeholders; keep argument order identical to the SELECT order.
- Do not remove `rotation_enabled`/`rotation_interval`.

- [ ] **Step 4: Run the test to verify it passes**

Run: `go test ./internal/storage -run 'TestEnsureSettingsTableAddsPresenceDefaults|TestEnsureSettingsTableMigratesLMStudioSettings' -v`
Expected: PASS both (Postgres is running).

- [ ] **Step 5: Build, test, commit**

```bash
go build ./...
go test ./internal/storage -run 'Settings|EnsureSettingsTable'
git add internal/storage/settings.go internal/storage/settings_migration_test.go
git commit -m "feat(settings): add presence simulation fields"
```

---

### Task 2: Public Twitch GraphQL client

**Files:**
- Create: `internal/twitch/public_stream.go`
- Test: `internal/twitch/public_stream_test.go`

**Interfaces:**
- Consumes: standard library only.
- Produces:
  - `type PublicStreamInfo struct { Live bool; Title string; GameName string; ViewerCount int; StartedAt time.Time }`
  - `func NewPublicStreamClient() *PublicStreamClient`
  - `func (c *PublicStreamClient) GetStream(ctx context.Context, channelLogin string) (*PublicStreamInfo, error)`
  - Unexported fields `endpoint string`, `clientID string`, `ttl time.Duration`, cache keyed by normalized login.

- [ ] **Step 1: Write the failing tests**

Use `httptest.NewServer` and set `client.endpoint = srv.URL` in the same package. Tests:

- `TestPublicStreamLive` — response with `createdAt`, `viewersCount`, `game.name`, `title`; assert all fields and `Live=true`.
- `TestPublicStreamOffline` — `stream: null`; assert `Live=false`, zero fields.
- `TestPublicStreamMissingUser` — `user: null`; assert error contains `channel not found`.
- `TestPublicStreamHTTPError` — 500; assert error.
- `TestPublicStreamMalformedJSON` — `not json`; assert error.
- `TestPublicStreamCachesForTTL` — two calls, server counter must be 1; after setting `client.cache` entry `FetchedAt` to 16 seconds ago, second call hits server.
- `TestNormalizeTwitchChannel` — `#Foo`, `https://www.twitch.tv/Foo`, ` Foo ` all normalize to `foo`.

- [ ] **Step 2: Run to verify failure**

Run: `go test ./internal/twitch -run TestPublicStream -v`
Expected: FAIL — types/methods do not exist.

- [ ] **Step 3: Implement the client**

- Normalize login: trim, lower-case, strip leading `#`, strip `https://www.twitch.tv/` / `https://twitch.tv/`.
- POST JSON `{query, variables:{login}}` with the exact query from the spec.
- Headers: `Client-ID: kimne78kx3ncx6brgo4mv6wki5h1ko`, `Content-Type: application/json`.
- Timeout 5 s per request; TLS via default transport.
- Parse `data.user`; null → error. `stream == null` → `Live=false`. Parse `createdAt` as RFC3339.
- Cache success and offline results for 15 s. Do not cache errors.
- Mutex protects the cache map.

- [ ] **Step 4: Run tests to verify pass**

Run: `go test ./internal/twitch -run TestPublicStream -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
go build ./...
git add internal/twitch/public_stream.go internal/twitch/public_stream_test.go
git commit -m "feat(twitch): add public GraphQL stream client"
```

---

### Task 3: Anonymous read-only IRC chat monitor

**Files:**
- Create: `internal/twitch/chat_monitor.go`
- Test: `internal/twitch/chat_monitor_test.go`

**Interfaces:**
- Consumes: existing IRC host constant `irc.chat.twitch.tv:6697`; `NormalizeChannel` from Task 2.
- Produces:
  - `type Chatter struct { Login string; LastMessageAt time.Time }` with JSON tags `login`, `last_message_at`.
  - `type ChatMessage struct { Login string; Text string; At time.Time; IsBot bool }` with JSON tags `login`, `text`, `at`, `is_bot`.
  - `func NewChatMonitor() *ChatMonitor`
  - `func (m *ChatMonitor) SetChannel(channel string)`
  - `func (m *ChatMonitor) ActiveChatters(window time.Duration) []Chatter`
  - `func (m *ChatMonitor) RecentMessages(limit int) []ChatMessage`
  - `func (m *ChatMonitor) Connected() bool`
  - `func (m *ChatMonitor) Close()`
  - Test hooks: unexported `dial func(ctx context.Context, address string) (net.Conn, error)` and `now func() time.Time`.

- [ ] **Step 1: Write the failing parser/store tests**

```go
func TestParsePrivmsg(t *testing.T) {
	login, text, ok := parsePrivmsg(":alice!alice@alice.tmi.twitch.tv PRIVMSG #chan :hello world")
	assert.True(t, ok)
	assert.Equal(t, "alice", login)
	assert.Equal(t, "hello world", text)

	_, _, ok = parsePrivmsg(":alice!alice@alice.tmi.twitch.tv JOIN #chan")
	assert.False(t, ok)
}

func TestChatMonitorTracksUniquesAndPrunes(t *testing.T) {
	now := time.Date(2026, 11, 9, 12, 0, 0, 0, time.UTC)
	m := NewChatMonitor()
	m.now = func() time.Time { return now }
	m.record("alice", "hi")
	m.record("bob", "yo")
	m.record("alice", "again")
	assert.Len(t, m.ActiveChatters(5*time.Minute), 2)

	now = now.Add(6 * time.Minute)
	assert.Empty(t, m.ActiveChatters(5*time.Minute))
}

func TestChatMonitorCapsMessages(t *testing.T) {
	m := NewChatMonitor()
	for i := 0; i < 120; i++ {
		m.record("user", fmt.Sprintf("m%d", i))
	}
	msgs := m.RecentMessages(50)
	require.Len(t, msgs, 50)
	assert.Equal(t, "m119", msgs[0].Text)
}
```

Add `TestChatMonitorSetChannelStopsPrevious` using `net.Pipe` and a fake dial: feed `PRIVMSG` lines for channel A, then call `SetChannel("b")`, assert `Connected()` false for the old connection and no messages leak after channel switch.

- [ ] **Step 2: Run to verify failure**

Run: `go test ./internal/twitch -run 'TestParsePrivmsg|TestChatMonitor' -v`
Expected: FAIL — parser/types missing.

- [ ] **Step 3: Implement**

- `parsePrivmsg` uses a regexp equivalent to `^:([^!]+)![^@]*@[^ ]+ PRIVMSG #[^ ]+ :(.*)$`.
- `record` stores `lastMessageAt` and appends `ChatMessage`; prune chatter map older than 5 min and messages older than 10 min; cap at 100 messages.
- `SetChannel("")` cancels; `SetChannel(same)` no-op; different channel cancels old goroutine and starts a new one.
- Connect with TLS `irc.chat.twitch.tv:6697`; send `PASS SCHMOOPIIE`, `NICK justinfan<random>`, `JOIN #channel`.
- Read loop: handle `PING` with `PONG`, otherwise parse `PRIVMSG`; ignore everything else.
- Reconnect loop with backoff 5s, 10s, 20s, 40s, 60s max + jitter; stop on context cancel.
- Never write `PRIVMSG`/`JOIN` beyond the initial anonymous join.
- `Connected()` returns whether the current read loop has a live connection.

- [ ] **Step 4: Run tests**

Run: `go test ./internal/twitch -run 'TestParsePrivmsg|TestChatMonitor' -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
go build ./...
git add internal/twitch/chat_monitor.go internal/twitch/chat_monitor_test.go
git commit -m "feat(twitch): add anonymous read-only chat monitor"
```

---

### Task 4: StreamMonitor and `/api/twitch/stream`

**Files:**
- Create: `internal/twitch/stream_monitor.go`
- Test: `internal/twitch/stream_monitor_test.go`
- Modify: `internal/api/handlers.go`
- Modify: `internal/api/server.go`
- Modify: `internal/api/handlers_test.go`
- Modify: `logic.go` (create/pass monitor, update channels in `ApplySettings`)

**Interfaces:**
- Consumes: `PublicStreamClient` (Task 2), `ChatMonitor`/`Chatter`/`ChatMessage` (Task 3), `api.BotInfo`.
- Produces:
  - `type StreamStatus struct { ... }` with exact JSON tags from the spec.
  - `func NewStreamMonitor() *StreamMonitor`
  - `func (m *StreamMonitor) SetChannels(channels []string)`
  - `func (m *StreamMonitor) Channels() []string`
  - `func (m *StreamMonitor) Snapshot(ctx context.Context, channel string, excludeLogins map[string]struct{}) StreamStatus`
  - `func (m *StreamMonitor) Close()`
  - Test seams: `type streamProvider interface { GetStream(context.Context, string) (*PublicStreamInfo, error) }`, `type chatProvider interface { ActiveChatters(time.Duration) []Chatter; RecentMessages(int) []ChatMessage; Connected() bool; Close() }`, and `newStreamMonitor(public streamProvider, newChat func(string) chatProvider) *StreamMonitor`.
  - `type StreamStatusProvider interface { Channels() []string; Snapshot(context.Context, string, map[string]struct{}) twitch.StreamStatus }` in `internal/api`.
  - `GET /api/twitch/stream?channel=<login>`.

- [ ] **Step 1: Write failing StreamMonitor tests**

- `TestStreamMonitorNoChannels` — `SetChannels(nil)` then `Snapshot` returns `Configured=false`, no error.
- `TestStreamMonitorLive` — fake public provider returns live 120s ago, 100 viewers; fake chat has 3 chatters (one bot excluded); assert uptime >= 120, active=2, passive=98, `total=active+passive`, `ChatConnected=true`.
- `TestStreamMonitorOffline` — provider live=false; assert zero viewers and `Live=false`.
- `TestStreamMonitorProviderError` — provider error; `Error` non-empty, `Live=false`, no panic.
- `TestStreamMonitorSetChannelsNormalizesAndCaps` — input `#A`, `https://twitch.tv/b`, 6 channels; `Channels()` has 2 unique normalized values and caps at 5.
- `TestStreamMonitorMessagesMarkBots` — fake chat returns messages from `alice` and bot `bot1`; assert `bot1.IsBot=true`, `alice.IsBot=false`.

- [ ] **Step 2: Run to verify failure**

Run: `go test ./internal/twitch -run TestStreamMonitor -v`
Expected: FAIL — types missing.

- [ ] **Step 3: Implement StreamMonitor**

- Keep `map[string]*ChatMonitor` and `map[string]*PublicStreamClient` shared; one client with cache is enough.
- `NewStreamMonitor()` calls `newStreamMonitor(NewPublicStreamClient(), func(ch string) chatProvider { m := NewChatMonitor(); m.SetChannel(ch); return m })`.
- `SetChannels` normalizes, dedupes, caps at 5, stops removed monitors, starts new ones.
- `Snapshot`: choose explicit channel or first; if empty → `Configured=false`. Call public client with context timeout 5 s; fill status. Active window constant 300 s. Filter `excludeLogins`. Set `IsBot` on messages. Sort chatters desc, cap 20; cap messages 50.
- Uptime = `time.Since(StartedAt).Seconds()` when live and StartedAt non-zero.
- `Error` is a short safe string (`twitch unavailable`, `channel not found`), not a raw body.

- [ ] **Step 4: Write failing API tests**

In `internal/api/handlers_test.go` add a fake `StreamStatusProvider` and tests:

- `TestGetStreamStatusNoChannel` — provider returns `Configured=false`; 200 and `configured:false`.
- `TestGetStreamStatusLive` — JSON has `uptime_seconds`, `viewers_total`, `active_viewers`, `passive_viewers`, `messages`.
- `TestGetStreamStatusRejectsUnconfiguredChannel` — provider channels `["a"]`, request `?channel=b`; 400.
- `TestGetStreamStatusExcludesBots` — fake manager has bot login `bot1`; provider asserts exclude map contains `bot1`.

Update `MockBotManager` with `MessagesPerMin` in Task 5, not here; only add the stream provider plumbing now.

- [ ] **Step 5: Wire handler/server/main**

- Add `streamMonitor StreamStatusProvider` to `Handler`; `NewHandler(manager, store, hub, llmClient, cacheClient, streamMonitor)`.
- Add `streamMonitor` to `NewServer(manager, store, llmClient, cacheClient, streamMonitor, apiKey, port, corsOrigins)`.
- Route `GET /api/twitch/stream` → `GetStreamStatus`.
- `main`: `monitor := twitch.NewStreamMonitor(); defer monitor.Close()`; after settings load `monitor.SetChannels(settings.Channels)`; pass to `NewServer` and `newBotManager`.
- `botManager` gets `streamMonitor *twitch.StreamMonitor`; `ApplySettings` calls `SetChannels(settings.Channels)`.
- `GetStreamStatus`: normalize query channel via `twitch.NormalizeChannel`; if query non-empty and not in `Channels()` → 400; build exclude set from `h.manager.ListBots()` using `Config.TwitchLogin`; call `Snapshot`. If `h.streamMonitor == nil`, return 200 `{"configured":false}`.
- Update every `NewHandler`/`NewServer` call in tests and `main`.

- [ ] **Step 6: Run tests and commit**

```bash
go test ./internal/twitch ./internal/api
go build ./...
git add internal/twitch internal/api logic.go
git commit -m "feat(twitch): add stream monitor endpoint"
```

---

### Task 5: Fix bot loop launch, graceful stop, and messages/min

**Files:**
- Modify: `logic.go`
- Modify: `internal/api/handlers.go` (`BotManager` interface, `GetStats`)
- Modify: `internal/api/handlers_test.go` (`MockBotManager`)
- Test: create `logic_bot_test.go`

**Interfaces:**
- Produces: `BotManager.MessagesPerMin() int`; `botManager.messageTimes []time.Time`; `recordMessageSent()`.
- Fixes: `startBot` calls `go bm.runBotLoop(ctx, br)` exactly once per start.

- [ ] **Step 1: Write failing messages/min test**

In `logic_bot_test.go` (package `main`):

```go
func TestMessagesPerMinCountsRecentSends(t *testing.T) {
	now := time.Now()
	bm := &botManager{}
	bm.mu.Lock()
	bm.messageTimes = []time.Time{now.Add(-90 * time.Second), now.Add(-30 * time.Second), now}
	bm.mu.Unlock()
	assert.Equal(t, 2, bm.MessagesPerMin())
}
```

- [ ] **Step 2: Write failing loop-launch test**

Use a fake `TwitchClient`:

```go
type fakeTwitchClient struct {
	mu       sync.Mutex
	connected bool
	sent     []string
}
func (f *fakeTwitchClient) Connect() error { f.connected = true; return nil }
func (f *fakeTwitchClient) IsConnected() bool { return f.connected }
func (f *fakeTwitchClient) SendMessage(text string) error { f.sent = append(f.sent, text); return nil }
func (f *fakeTwitchClient) Reconnect() error { return nil }
func (f *fakeTwitchClient) Close() { f.connected = false }
func (f *fakeTwitchClient) WaitForRateLimit() {}
```

Construct a `botManager` with one `botRuntime` whose `client` is the fake, set `bm.newIRCClient = func(login, password, channel string) TwitchClient { return fakeClient }`, `cfg.Enabled=true`, `cfg.MinDelaySec=1`, `cfg.MaxDelaySec=1`, `disableLLM=true`, `runtimeCtx=context.Background()`. Call `startBot(ctx, id, false, true)`, wait until `runBotLoop` has started (add an unexported `loopStarted chan struct{}` to `botRuntime` or use a fake client connection plus a short sleep with retry), then call `stopBot(id, false)` and assert it returns in under 2 seconds and `br.running` is false. Without the `go bm.runBotLoop` line, `stopBot` waits 10 seconds and the test fails.

- [ ] **Step 3: Run to verify failure**

Run: `go test . -run 'TestMessagesPerMin|TestStartBotLaunchesLoop' -v`
Expected: FAIL — `MessagesPerMin` missing and stop takes 10s.

- [ ] **Step 4: Implement**

- Add `messageTimes []time.Time` (max 512) to `botManager`; add `recordMessageSent()` that appends under `bm.mu` and prunes older than 60s.
- Call `bm.recordMessageSent()` immediately after a successful `br.client.SendMessage(msg)` (not in debug mode, not for rate-limited failures).
- Add `MessagesPerMin() int` to `botManager` and `BotManager` interface; `MockBotManager` delegates via `m.Called()`.
- `GetStats`: `MessagesPerMin: h.manager.MessagesPerMin()`.
- Add `newIRCClient func(login, password, channel string) TwitchClient` to `botManager`, defaulting to `twitch.NewIRCClient`; use it in `startBot`, `CreateBot`, and `loadBots` so tests can inject a fake.
- In `startBot`, after `bm.mu.Unlock()` and before `return nil`, call `go bm.runBotLoop(ctx, br)`.
- Ensure `runBotLoop`'s deferred `close(br.stopped)` is the only place closing it; `stopBot` must not double-close.
- Remove the `time.Sleep(br.cfg.MinDelaySec)` from `runBotLoop`'s deferred cleanup. Messages are sent synchronously, so the sleep only makes every stop wait up to 3 minutes; with it removed, `stopBot` returns promptly and `br.stopped` closes immediately.
- Replace the `time.Sleep(...)` in the `disableLLM` branch with a `select` on `ctx.Done()` and a timer, so monitor-only bots stop promptly.

- [ ] **Step 5: Run tests and commit**

```bash
go test . ./internal/api
go build ./...
git add logic.go internal/api/handlers.go internal/api/handlers_test.go logic_bot_test.go
git commit -m "fix(bot): launch the bot loop and track messages per minute"
```

---

### Task 6: Presence simulation, bot presence API, and rotation removal

**Files:**
- Modify: `logic.go`
- Modify: `internal/api/dto.go`
- Modify: `internal/api/handlers.go`
- Modify: `internal/api/websocket.go`
- Modify: `internal/api/handlers_test.go`
- Test: `logic_presence_test.go`
- Test: `internal/api/handlers_test.go` (presence fields)

**Interfaces:**
- Consumes: Task 1 presence settings; Task 5 loop launch.
- Produces:
  - `botManager.presence` struct with enabled/session min/max/cooldown min/max/greeting/farewell.
  - `botRuntime.presenceState string` (`online|cooldown|stopped|error`), `sessionEndsAt`, `cooldownUntil`, `presenceTimer`, `autoPaused`, `stoppedByError`.
  - `api.BotInfo` fields `PresenceState string`, `SessionEndsAt *time.Time`, `CooldownUntil *time.Time`.
  - `BotDTO` JSON fields `presence_state`, `session_ends_at`, `cooldown_until`.
  - `Hub.BroadcastBotPresence(botID, login, state string, at time.Time)` → WS type `bot_presence_changed`.

- [ ] **Step 1: Write failing presence helper tests**

In `logic_presence_test.go`:

- `TestPickPresenceDuration` — min=20/max=40 always in range; min=max returns that value.
- `TestPresenceGreetingFarewell` — pools are non-empty, all Russian strings, and greeting != farewell for every personality.
- `TestPresenceSettingsUpdate` — `ApplySettings` with presence disabled cancels a scheduled `presenceTimer` and leaves a cooldown bot `stopped`.
- `TestManualStopCancelsPresence` — a bot in `online` with a live timer, `StopBot` sets `presence_state=stopped`, clears `cooldown_until`, and no timer fires afterwards.
- `TestErrorStopDoesNotReturn` — set `stoppedByError=true`, run the deferred cleanup, assert no `startBot` call.

Use a fake `TwitchClient` and an injectable `presenceAfter func(time.Duration, func()) *time.Timer` or a `time.AfterFunc` wrapper so tests do not wait 20 minutes.

- [ ] **Step 2: Write failing API test**

`TestListBotsIncludesPresence` — manager returns one bot with `PresenceState:"cooldown"`, `CooldownUntil` set; decoded `BotDTO` has `presence_state:"cooldown"` and `cooldown_until`.

- [ ] **Step 3: Implement presence runtime**

- Add `presenceAfter` hook (default `time.AfterFunc`).
- `ApplySettings`: store presence config; if disabled, stop timers for bots in cooldown and set them `stopped`; do not stop online bots.
- `startBot`: if presence enabled, choose session duration, set `sessionEndsAt`, state `online`; send greeting if enabled and `!debugMode && !disableLLM`; schedule `pauseBot` at session end. Always launch `runBotLoop` (Task 5).
- `pauseBot`: under lock, set `autoPaused=true`, state `cooldown`, `running=false`, cancel loop, close client, compute `cooldownUntil`; don't persist `enabled=false`; wait for `br.stopped` with a 5s timeout (loop exits promptly after Task 5 removes the sleep), then schedule `startBot(ctx, id, false, false)` after cooldown. If the old loop has not exited before the timer fires, skip the restart and log a warning rather than starting a duplicate.
- `StopBot`: cancel presence timer, set `stoppedByError=false`, `autoPaused=false`, `presenceState="stopped"`, clear cooldown, then existing stop.
- On `maxConsecutiveErrors` path in `runBotLoop`, set `stoppedByError=true`, `presenceState="error"` before return.
- `StopAll` cancels timers.
- `DeleteBot` cancels the presence timer before deleting.
- Add backend validation in `UpdateSettings`: presence min/max must satisfy `1 <= min <= max <= 240`; otherwise HTTP 400 with a Russian error. Test it in Task 6.
- Remove `startRotation`, `stopRotation`, `rotateBots`, `rotationEnabled`, `rotationCancel`; remove calls in `main`/`ApplySettings`; remove rotation change log entries/messages in `handlers.go`. Keep DB fields.
- Greeting/farewell pools: 3–4 Russian phrases per personality, sent via `br.client.SendMessage`; skip in debug/`disableLLM`; count via `recordMessageSent`.
- Broadcast presence events on online/cooldown/stopped/error.

- [ ] **Step 4: API presence fields and WS**

- Extend `api.BotInfo`, `BotDTO`.
- `ListBots`/`GetBot` map the fields.
- Add `BotPresencePayload` and `Hub.BroadcastBotPresence`.
- `MockBotManager`/tests updated.

- [ ] **Step 5: Run tests and commit**

```bash
go test . ./internal/api
go build ./...
git add logic.go logic_presence_test.go internal/api
git commit -m "feat(bots): add presence simulation and remove channel rotation"
```

---

### Task 7: Frontend API types, hooks, and WebSocket

**Files:**
- Modify: `frontend/src/api/client.ts`
- Modify: `frontend/src/hooks/useWebSocket.ts`
- Create: `frontend/src/hooks/useStreamStatus.ts`
- Create: `frontend/src/hooks/useStats.ts`

**Interfaces:**
- Consumes: Task 4 `StreamStatus` JSON; Task 6 `BotDTO` presence fields and WS `bot_presence_changed`.
- Produces: `StreamStatus`, `Chatter`, `ChatMessage` TS types; `api.getStreamStatus(channel?)`; `useStreamStatus(channel, enabled)`; `useStats()`.

- [ ] **Step 1: Extend types and client**

- Add `presence_state`, `session_ends_at?`, `cooldown_until?` to `Bot`.
- Add `presence_enabled`, `presence_session_min_minutes`, `presence_session_max_minutes`, `presence_cooldown_min_minutes`, `presence_cooldown_max_minutes`, `presence_greeting_enabled`, `presence_farewell_enabled` to `Settings`.
- Add `StreamStatus`, `Chatter`, `ChatMessage` interfaces matching the Go JSON.
- Add `getStreamStatus: (channel?: string) => client.get<StreamStatus>('/api/twitch/stream', { params: channel ? { channel } : {} })`.
- Remove both `client.interceptors` blocks (they log request bodies, including OAuth tokens).
- Add `export function getErrorMessage(error: unknown): string` returning Russian messages for 401/403/404/500/network, no raw stack.

- [ ] **Step 2: Add hooks**

`useStreamStatus(channel: string | undefined, enabled: boolean)`:

```ts
return useQuery({
  queryKey: ['twitch-stream', channel ?? ''],
  queryFn: async () => (await api.getStreamStatus(channel)).data,
  enabled,
  refetchInterval: 15_000,
  retry: 1,
});
```

`useStats()` returns React Query for `/api/stats`.

- [ ] **Step 3: WebSocket**

- Refactor to a single connection: `WebSocketProvider` in `App.tsx`; `useWebSocketStatus()` for Topbar; `useWebSocketSubscription(handler)` for pages. No page may open its own socket; only `AudioTestModal` may open a temporary test socket.
- Remove `console.log`/`console.error` payload logging.
- Exponential reconnect: 1s, 2s, 4s, 8s, 16s, 30s max + jitter; reset on open.
- Handle `bot_presence_changed` by adding a system entry to the log store (new `addPresenceEvent` action) with `bot_id`, `login`, `state`, `at`.

- [ ] **Step 4: Build and commit**

Run: `cd frontend && npm.cmd run build`
Expected: success.

```bash
git add frontend/src/api/client.ts frontend/src/hooks
git commit -m "feat(ui): add Twitch stream types and polling hooks"
```

---

### Task 8: Light design system and app shell

**Files:**
- Create: `frontend/src/styles/app.css`
- Rewrite: `frontend/src/index.css`
- Rewrite: `frontend/src/App.tsx`
- Create: `frontend/src/components/layout/Sidebar.tsx`
- Create: `frontend/src/components/layout/Topbar.tsx`
- Create: `frontend/src/components/layout/PageHeader.tsx`
- Create: `frontend/src/components/ui/Card.tsx`
- Create: `frontend/src/components/ui/StatCard.tsx`
- Create: `frontend/src/components/ui/Button.tsx`
- Create: `frontend/src/components/ui/Input.tsx`
- Create: `frontend/src/components/ui/Select.tsx`
- Create: `frontend/src/components/ui/Badge.tsx`
- Create: `frontend/src/components/ui/Alert.tsx`
- Create: `frontend/src/components/ui/Modal.tsx`
- Create: `frontend/src/components/ui/EmptyState.tsx`
- Create: `frontend/src/pages/Stream.tsx` (minimal placeholder, rewritten in Task 9)

**Interfaces:**
- Produces: page type `'dashboard' | 'stream' | 'bots' | 'settings'`; `Sidebar({currentPage,onNavigate})`; `Topbar({title,onNavigate})`; UI primitives with `children`, `title`, `variant`, `size`, `onClick`, `disabled`, `loading`.
- Consumes: `useWebSocketStatus` for WS chip; `/api/llm/status` for OpenRouter chip; `settings.audio_enabled` for STT chip.

- [ ] **Step 1: Implement tokens and shell**

`app.css` defines the tokens from Global Constraints, reset, layout grid, sidebar/topbar, cards, tables, forms, chat feed, responsive breakpoint 900px. No glow/gradients/snow.

`App.tsx` renders `WebSocketProvider`, `Sidebar`, `Topbar`, one page, `ErrorBoundary` in Russian. No PixelSnow/TargetCursor/Dock/BGPattern/framer-motion page transitions. Until Task 9, the `Stream` page is a minimal placeholder so the shell compiles.

`Sidebar`: brand “Twitch SMM”, nav Dashboard/Stream/Bots/Settings with lucide icons, active lime pill, version at bottom.

`Topbar`: page title, status chips:
- OpenRouter from `/api/llm/status`: green “подключён”, amber “ключ не задан”, red only for real auth/network errors.
- STT from `audio_enabled`: “вкл/выкл”.
- WS from `useWebSocketStatus`: “online/offline”.
- DEBUG chip when `debug_mode` is true.
- Refresh time.

`ErrorBoundary` copy: «Что-то пошло не так», error text, «Перезагрузить».

- [ ] **Step 2: Build and commit**

Run: `cd frontend && npm.cmd run build`
Expected: success; no import errors (old pages still referenced until Task 9–11, so keep temporary page imports compiling).

```bash
git add frontend/src/App.tsx frontend/src/index.css frontend/src/styles/app.css frontend/src/components/layout frontend/src/components/ui
git commit -m "feat(ui): add light app shell and design primitives"
```

---

### Task 9: Dashboard, Stream, and chat feed

**Files:**
- Rewrite: `frontend/src/pages/Dashboard.tsx`
- Rewrite: `frontend/src/pages/Stream.tsx`
- Create: `frontend/src/components/StreamCard.tsx`
- Create: `frontend/src/components/ChatFeed.tsx`
- Create: `frontend/src/components/SetupBanner.tsx`
- Modify: `frontend/src/components/ui/*` as needed

**Interfaces:**
- Consumes: `useStreamStatus`, `useStats`, `api.getBots`, `api.startBot/stopBot/startAll/stopAll`, `useWebSocketSubscription`, `useLogStore` presence events, settings channels.
- Produces: `StreamCard({status, channel, compact})`, `ChatFeed({messages, events})`.

- [ ] **Step 1: Dashboard**

- Fetch bots, stats, settings channels, stream status for the first channel.
- Setup banners: no channel → link to Settings; no OpenRouter key → link to Settings.
- Stat cards: total bots, running, messages/min, viewers (`—` if unknown).
- StreamCard compact: player or placeholder, uptime, total/active/passive.
- Active bots table: name, login, status badge (online/cooldown/stopped/error), actions start/stop.
- Empty state: «Пока нет ботов» + button.

- [ ] **Step 2: Stream page**

- Channel select from settings channels.
- Full StreamCard with 16:9 iframe:
  `https://player.twitch.tv/?channel=${channel}&parent=${window.location.hostname}&muted=true&autoplay=true`.
- Metrics row: uptime, total, active, passive, game.
- `ChatFeed`:
  - bot messages: lime-tinted background and left border, `@login` bold;
  - regular messages: plain;
  - presence events from WS: centered gray system line «@login зашёл/вышел»;
  - auto-scroll to newest unless user scrolled up.
- Offline/error/no-channel states per spec, no red blocks.

- [ ] **Step 3: Build and commit**

Run: `cd frontend && npm.cmd run build`
Expected: success.

```bash
git add frontend/src/pages/Dashboard.tsx frontend/src/pages/Stream.tsx frontend/src/components/StreamCard.tsx frontend/src/components/ChatFeed.tsx frontend/src/components/SetupBanner.tsx
git commit -m "feat(ui): add dashboard and stream monitoring pages"
```

---

### Task 10: Bots page, modal, and logs drawer

**Files:**
- Rewrite: `frontend/src/pages/Bots.tsx`
- Create: `frontend/src/components/BotsTable.tsx`
- Create: `frontend/src/components/BotModal.tsx`
- Create: `frontend/src/components/BotLogsDrawer.tsx`
- Rewrite: `frontend/src/components/BotForm.tsx`
- Rewrite: `frontend/src/components/BotFormField.tsx`
- Modify: `frontend/src/hooks/useBotForm.ts` only if needed
- Delete: `BotFormModal.tsx`, `BotCard.tsx`, `BotList.tsx`, `LaunchBotModal.tsx`, `BotLogsModal.tsx`, `BotLogPanel.tsx`, and their CSS

**Interfaces:**
- Consumes: `api.getBots/createBot/updateBot/deleteBot/startBot/stopBot`, `useBotForm`, `Bot` presence fields.
- Produces: table with columns login, name, personality, presence badge, delays, limit, actions; modal with create/edit; drawer with logs.

- [ ] **Step 1: Implement**

- Table rows use `presence_state`: `в эфире` (green), `пауза до HH:MM` (amber), `остановлен` (gray), `ошибка` (red).
- Actions: start, stop, edit, delete, logs.
- Start uses `api.startBot`; stop uses `api.stopBot`.
- Add/edit modal uses `useBotForm`; OAuth field is `type=password` and placeholder «оставьте пустым, чтобы не менять» when editing.
- Delete uses `Modal` confirmation; no `window.confirm`.
- Errors inline via `Alert`; no `alert()`.
- Logs drawer calls `api.getBotLogs(id, 100)`, shows level, time, message; empty state.

- [ ] **Step 2: Build and commit**

Run: `cd frontend && npm.cmd run build`
Expected: success.

```bash
git add frontend/src/pages/Bots.tsx frontend/src/components/BotsTable.tsx frontend/src/components/BotModal.tsx frontend/src/components/BotLogsDrawer.tsx frontend/src/components/BotForm.tsx frontend/src/components/BotFormField.tsx
git rm frontend/src/components/BotFormModal.tsx frontend/src/components/BotCard.tsx frontend/src/components/BotList.tsx frontend/src/components/LaunchBotModal.tsx frontend/src/components/BotLogsModal.tsx frontend/src/components/BotLogPanel.tsx frontend/src/components/BotFormModal.css frontend/src/components/BotForm.css frontend/src/components/BotFormField.css
git commit -m "feat(ui): rebuild bots table and editing flow"
```

---

### Task 11: Settings page and audio test fix

**Files:**
- Rewrite: `frontend/src/pages/Settings.tsx`
- Modify: `frontend/src/components/AudioTestModal.tsx`
- Delete: `frontend/src/components/SettingsLogPanel.tsx`

**Interfaces:**
- Consumes: `api.getSettings/updateSettings/getLLMStatus`, `Settings` presence fields, `AudioTestModal`.
- Produces: sections Channels, Twitch monitoring, OpenRouter, Audio, Behavior, Presence simulation; disabled/read-only env overrides.

- [ ] **Step 1: Implement sections**

- Channels: editable list, add/remove, hint that one channel is normal for individual use.
- Twitch monitoring: explain public GraphQL + read-only IRC; Client ID/Secret fields marked “не требуется для мониторинга” (future Helix fallback).
- OpenRouter: free model, paid model, daily cap, API key (empty keeps stored), key source badge, effective override hints.
- Audio: `audio_enabled`, language, open `AudioTestModal`.
- Behavior: debug mode/channel, positive/negative prompts, max tokens.
- Presence: enabled switch, session min/max, cooldown min/max, greeting/farewell switches. Validate `1 <= min <= max <= 240` before save; inline error.
- Remove all rotation fields.
- Save via one button; success/error toast or inline alert; no `alert()`.

- [ ] **Step 2: Fix `AudioTestModal`**

- Replace `ws://localhost:8080` with `config.wsUrl` and `config.apiKey`.
- Replace `fetch('http://localhost:8080/api/settings/test-llm')` with a client method or `config.apiUrl` + `config.apiKey`.
- Never hardcode `secret-api-key-12345`.
- Remove console logging.

- [ ] **Step 3: Build and commit**

Run: `cd frontend && npm.cmd run build`
Expected: success.

```bash
git add frontend/src/pages/Settings.tsx frontend/src/components/AudioTestModal.tsx
git rm frontend/src/components/SettingsLogPanel.tsx
git commit -m "feat(ui): rebuild settings with presence simulation"
```

---

### Task 12: Remove old UI, update docs, and clean rotation

**Files:**
- Delete: `frontend/src/components/Navigation.tsx`, `StatusBar.tsx`, `FloatingProviderStatus.tsx`, `Stats.tsx`, `DebugMessagesPanel.tsx`, `DebugModeNotification.tsx`, `FloatingPanel.tsx`, `FloatingPanel.css`
- Delete: `frontend/src/components/ui/PixelSnow.*`, `TargetCursor.*`, `GooeyNav.*`, `dock.tsx`, `dock-tabs.tsx`, `grid-beam.tsx`, `bg-pattern.tsx`, `tilt-card.tsx`, `interactive-hover-button.tsx`
- Delete: `frontend/src/pages/BGPatternDemo.tsx`, `frontend/src/styles/main.css`, `frontend/src/App.css`
- Delete unused assets after checking imports: `frontend/src/assets/react.svg`, `vite.svg`
- Modify: `README.md`, `docs/AGENTS.md`
- Modify: `internal/api/handlers.go` (remove rotation change log entries/messages)
- Modify: `frontend/src/api/client.ts` (remove proxy methods if no UI uses them? keep API, remove only if unused)

**Interfaces:**
- Consumes: Tasks 8–11.
- Produces: no imports of deleted components; no rotation UI/docs; docs describe Stream and presence.

- [ ] **Step 1: Delete and grep**

Delete the listed files only after `grep` shows no remaining imports. Run:

```powershell
Get-ChildItem frontend/src -Recurse -Include *.tsx,*.ts | Select-String -Pattern 'PixelSnow|TargetCursor|GooeyNav|Dock|BGPattern|StatusBar|FloatingProviderStatus|SettingsLogPanel|DebugMessagesPanel|DebugModeNotification|FloatingPanel|BotList|BotCard|LaunchBotModal|BotLogsModal|BotLogPanel'
```

Expected after cleanup: no matches.

Run:

```powershell
Get-ChildItem README.md,docs/AGENTS.md -File | Select-String -Pattern 'rotation|Rotation|ротац'
Get-ChildItem frontend/src -Recurse -Include *.tsx,*.ts | Select-String -Pattern 'rotation|Rotation|ротац'
```

Expected: no user-facing rotation references (backend DB fields may remain).

- [ ] **Step 2: Update docs**

- README: four sections, Stream monitoring, presence simulation, public GQL caveat, no rotation, model file note.
- `docs/AGENTS.md`: UI structure, stream monitoring, presence simulation; remove rotation.
- Do not touch historical `docs/superpowers/specs/*` records.

- [ ] **Step 3: Full build/test and commit**

```bash
go build ./...
go test ./...
cd frontend && npm.cmd run build && cd ..
git add -A
git commit -m "chore(ui): remove legacy components and document the new dashboard"
```

---

### Task 13: End-to-end verification

**Files:** none (verification only; fix failures in the owning task).

- [ ] **Step 1: Build and test**

```bash
go build ./...
go test ./...
cd frontend && npm.cmd run build && cd ..
```

Expected: all green.

- [ ] **Step 2: Rebuild stack**

```bash
docker compose up -d --build --wait
docker compose ps
```

Expected: backend, frontend, postgres, redis healthy.

- [ ] **Step 3: API checks**

With `X-API-Key: secret-api-key-12345`:

- `GET /api/twitch/stream` with no channels → `configured:false`.
- Add a live channel in Settings, then:
  - `live:true`, `uptime_seconds > 0`, `viewers_total > 0`;
  - `viewers_total >= active_viewers + passive_viewers` and equality when total known;
  - active chatters appear while bots are stopped (anonymous IRC);
  - messages include `is_bot:true` for bot logins.
- Offline channel → `live:false`, 200, no fake numbers.
- Invalid channel → 400.
- `/api/stats` `messages_per_min` increases after a bot sends.
- `GET /api/llm/status` unchanged; no key → amber UI state.

- [ ] **Step 4: Presence checks**

Set session min/max to 1/1 minutes and cooldown 1/1 in Settings, start a bot:

- state goes `online` → `cooldown` → `online`;
- greeting and farewell appear in chat feed and Twitch;
- `StopBot` during online → `stopped`, no return;
- force an error stop → `error`, no return;
- disable presence while in cooldown → bot stays stopped.

- [ ] **Step 5: UI checks**

- Four sections render; light theme; no console logs with request bodies/OAuth tokens; no `alert()`/`confirm()`.
- Stream player loads for a live channel.
- Bot messages are lime-highlighted; regular messages are not.
- Missing channel/OpenRouter key shows calm setup banners.
- Mobile width < 900px: sidebar collapses.

- [ ] **Step 6: Report evidence**

Record command output and any skipped live checks. Do not claim a step passed without output.
