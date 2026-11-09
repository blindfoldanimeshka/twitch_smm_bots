# Reliability P0 Fixes Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Убрать пять подтверждённых P0-дефектов: зависание API на 5 минут из-за мьютекса в rate-limit, 10-секундный стоп неактивного бота, утечку `twitch_client_secret` через `/api/settings`, искажение `messages_per_min` presence-фразами и некорректный PONG.

**Architecture:** Точечные изменения в существующих файлах без новых зависимостей и без изменения публичных интерфейсов, кроме одного нового вычисляемого поля `twitch_client_secret_set` в настройках. Каждый фикс закрывается регрессионным тестом, который падает на старом коде.

**Tech Stack:** Go 1.26+, PostgreSQL, testify, React 19 + TypeScript (одна правка в Settings UI).

**Spec:** `docs/superpowers/specs/2026-11-09-ui-simplification-twitch-monitoring-design.md` (инварианты 2, 3, 7) и этот план.

**Design decision:** `messages_per_min` — метрика активности бота в цикле генерации. Приветствия и прощания presence-сессий в неё не входят: они одноразовые и искажают картину «сколько бот пишет сейчас». Комментарий в `logic.go:1283` становится правдой, а не наоборот.

## Global Constraints

- Go 1.26+; новых зависимостей не добавлять.
- UI-текст только на русском; никаких `alert()`/`confirm()`.
- Токены, пароли и секреты никогда не возвращаются в API и не логируются.
- Гейты перед коммитом: `go build ./...`, `go vet ./...`, `go test ./...`, `cd frontend && npm.cmd run build`.
- Публичные сигнатуры `TwitchClient`, `botManager`, `Handler` не меняются, кроме добавления поля `TwitchClientSecretSet` в `storage.Settings`.

## Review Focus

- Сохранение настроек при пустом поле секрета не должно стирать сохранённый Twitch secret.
- `stopAll` на N неактивных ботах должен завершаться меньше чем за секунду, а не N×10 с.
- Остановка бота, находящегося в 420-бэкоффе, должна укладываться в HTTP `WriteTimeout` 15 с и не блокировать `ListBots`/`GetStats`.
- PONG должен быть корректен и для `PING :tmi.twitch.tv`, и для `PING tmi.twitch.tv`.
- При `debug_mode` или `disableLLM` приветствие не отправляется и `messages_per_min` остаётся нулём.

---

### Task 1: Не держать мьютекс IRC-клиента во время rate-limit сна

**Files:**
- Modify: `internal/twitch/irc.go:175-192`
- Test: `internal/twitch/irc_test.go:101-124` (рядом с `TestWaitForRateLimit`)

**Interfaces:**
- Consumes: `IRCClient.mu`, `rateLimited`, `rateLimitUntil`, `backoffAttempts`.
- Produces: `func (c *IRCClient) WaitForRateLimit()` с прежней сигнатурой, но без удержания `c.mu` во время `time.Sleep`.

- [ ] **Step 1: Write the failing test**

Добавить в `internal/twitch/irc_test.go`:

```go
// TestWaitForRateLimitDoesNotHoldLock verifies that the rate-limit sleep does
// not keep c.mu locked: while the client is sleeping, other goroutines (Close,
// IsRateLimited, SendMessage) must be able to take the lock.
func TestWaitForRateLimitDoesNotHoldLock(t *testing.T) {
	client := NewIRCClient("testlogin", "pass", "testchannel")

	client.mu.Lock()
	client.rateLimited = true
	client.rateLimitUntil = time.Now().Add(time.Second)
	client.mu.Unlock()

	done := make(chan struct{})
	go func() {
		client.WaitForRateLimit()
		close(done)
	}()

	time.Sleep(50 * time.Millisecond)

	start := time.Now()
	client.mu.Lock()
	client.mu.Unlock()
	if elapsed := time.Since(start); elapsed > 200*time.Millisecond {
		t.Fatalf("mutex held during rate-limit sleep: lock acquisition took %v", elapsed)
	}

	select {
	case <-done:
	case <-time.After(3 * time.Second):
		t.Fatal("WaitForRateLimit did not return")
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/twitch/ -run TestWaitForRateLimitDoesNotHoldLock -v`
Expected: FAIL — `mutex held during rate-limit sleep: lock acquisition took ~1s`.

- [ ] **Step 3: Rewrite `WaitForRateLimit`**

В `internal/twitch/irc.go` заменить тело функции на: взять лок, если `!rateLimited` — разлокуть и выйти; иначе скопировать `until := c.rateLimitUntil`, разлокуть, при положительной `time.Until(until)` поспать, затем снова взять лок и сбросить `rateLimited=false`/`backoffAttempts=0` только если `!time.Now().Before(c.rateLimitUntil)`. Комментарий функции: сон идёт без удержания мьютекса, поэтому `Close` не блокируется.

- [ ] **Step 4: Run the package tests**

Run: `go test ./internal/twitch/ -v`
Expected: PASS, включая существующие `TestWaitForRateLimit` и `TestConcurrentRateLimit`.

- [ ] **Step 5: Commit**

```bash
git add internal/twitch/irc.go internal/twitch/irc_test.go
git commit -m "fix(irc): release the client lock while waiting out a rate limit"
```

---

### Task 2: Не ждать 10 секунд при остановке бота, чей цикл не запущен

**Files:**
- Modify: `logic.go:1037-1064`
- Test: `logic_bot_test.go` (рядом с `TestStartBotLaunchesLoopAndStopsPromptly`)

**Interfaces:**
- Consumes: `botRuntime.running`, `botRuntime.stopped`, `botManager.stopBot(botID string, persist bool) error`.
- Produces: та же сигнатура; ожидание `br.stopped` выполняется только если цикл реально работал.

- [ ] **Step 1: Write the failing test**

Добавить в `logic_bot_test.go`:

```go
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
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test . -run TestStopBotDoesNotWaitForNeverStartedLoop -v`
Expected: FAIL — прошло ~10 с, `stopBot waited for a loop that never started`.

- [ ] **Step 3: Guard the wait**

В `logic.go` в `stopBot` перед сбросом `br.running` сохранить `wasRunning := br.running`. После `bm.mu.Unlock()` и `bm.broadcastPresence(br)` не выполнять блок ожидания, если `!wasRunning`: сразу перейти к `if persist { return bm.store.SetBotEnabled(...) }`. Блок `select` с `gracefulTimeout` оставить только для случая, когда цикл работал (и `br.stopped != nil`).

- [ ] **Step 4: Run the root package tests**

Run: `go test . -v -run 'TestStopBot|TestStartBot'`
Expected: PASS, включая `TestStartBotLaunchesLoopAndStopsPromptly`.

- [ ] **Step 5: Commit**

```bash
git add logic.go logic_bot_test.go
git commit -m "fix(bot): stop an inactive bot without the 10s graceful wait"
```

---

### Task 3: Не отдавать `twitch_client_secret` и сохранять его при пустом поле

**Files:**
- Modify: `internal/storage/settings.go:11-43`
- Modify: `internal/api/handlers.go:549-568`, `internal/api/handlers.go:609-620`
- Modify: `frontend/src/api/client.ts` (`Settings` interface)
- Modify: `frontend/src/pages/Settings.tsx:100`, `:165`, `:355-360`
- Test: `internal/api/handlers_test.go:355-390` и новый подтест после него

**Interfaces:**
- Consumes: `storage.Settings.TwitchClientSecret`, `Handler.GetSettings`, `Handler.UpdateSettings`, `oldSettings`.
- Produces: вычисляемое поле `storage.Settings.TwitchClientSecretSet bool` с json-тегом `twitch_client_secret_set`; в JSON `twitch_client_secret` всегда пустой, `twitch_client_secret_set` отражает наличие сохранённого секрета.

- [ ] **Step 1: Write the failing tests**

В `internal/api/handlers_test.go` в подтест `GET settings` после `assert.Equal(t, "test-client-id", settings.TwitchClientID)` добавить:

```go
assert.Empty(t, settings.TwitchClientSecret, "twitch client secret must not be returned")
assert.True(t, settings.TwitchClientSecretSet, "secret presence must be reported")
```

Добавить новый подтест в `TestUpdateSettings`:

```go
t.Run("keeps stored twitch secret when incoming secret is empty", func(t *testing.T) {
	mockManager := new(MockBotManager)
	mockStore := new(MockStore)
	handler := NewHandler(mockManager, mockStore, nil, nil, nil, nil)

	oldSettings := &storage.Settings{
		ID:                         "default",
		TwitchClientID:             "old-client-id",
		TwitchClientSecret:         "stored-secret",
		PresenceEnabled:            true,
		PresenceSessionMinMinutes:  20,
		PresenceSessionMaxMinutes:  40,
		PresenceCooldownMinMinutes: 10,
		PresenceCooldownMaxMinutes: 20,
	}
	newSettings := *oldSettings
	newSettings.TwitchClientSecret = ""

	mockStore.On("GetSettings", mock.Anything).Return(oldSettings, nil)
	mockStore.On("UpdateSettings", mock.Anything, mock.MatchedBy(func(s *storage.Settings) bool {
		return s.TwitchClientSecret == "stored-secret" && s.TwitchClientSecretSet
	})).Return(nil)
	mockStore.On("CreateSettingsLog", mock.Anything, mock.Anything).Return(nil)

	body, _ := json.Marshal(&newSettings)
	req := httptest.NewRequest(http.MethodPut, "/api/settings", bytes.NewReader(body))
	w := httptest.NewRecorder()

	handler.UpdateSettings(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	mockStore.AssertCalled(t, "UpdateSettings", mock.Anything, mock.Anything)
})
```

> Preflight ruling: тест копирует `oldSettings` по значению (`newSettings := *oldSettings`) и задаёт валидные presence-диапазоны, иначе `presenceRangesValid` вернёт 400, а общий указатель затрёт сохранённый секрет ещё до обработчика.

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/api/ -run TestGetSettings -v` и `go test ./internal/api/ -run TestUpdateSettings -v`
Expected: FAIL — секрет приходит в ответе и/или пустой секрет затирает сохранённый.

- [ ] **Step 3: Add the computed field and mask the secret**

В `internal/storage/settings.go` в структуру `Settings` добавить после `TwitchClientSecret`:

```go
TwitchClientSecretSet bool `json:"twitch_client_secret_set"`
```

В `GetSettings` (`handlers.go`) после существующих присваиваний `LLM*` добавить:

```go
settings.TwitchClientSecretSet = settings.TwitchClientSecret != ""
settings.TwitchClientSecret = ""
```

В `UpdateSettings` рядом с обработкой `LLMApiKey == ""` добавить сохранение старого секрета при пустом входящем значении и пересчёт `settings.TwitchClientSecretSet = settings.TwitchClientSecret != ""`.

- [ ] **Step 4: Add the frontend field and hint**

В `frontend/src/api/client.ts` в `interface Settings` добавить `twitch_client_secret_set: boolean;` после `twitch_client_secret`. В `Settings.tsx` добавить в начальный `formState` (`:100`) `twitch_client_secret_set: false`, в синхронизацию (`:165`) — `twitch_client_secret_set: settings.twitch_client_secret_set ?? false`, а под Input секрета — placeholder `Секрет сохранён — оставьте пустым, чтобы не менять`, показываемый при `formState.twitch_client_secret_set`.

- [ ] **Step 5: Run backend and frontend gates**

Run: `go test ./internal/api/ ./internal/storage/ -v` затем `cd frontend && npm.cmd run build`
Expected: PASS; сборка без ошибок типов.

- [ ] **Step 6: Commit**

```bash
git add internal/storage/settings.go internal/api/handlers.go internal/api/handlers_test.go frontend/src/api/client.ts frontend/src/pages/Settings.tsx
git commit -m "fix(settings): never return the Twitch client secret"
```

---

### Task 4: Убрать presence-фразы из `messages_per_min`

**Files:**
- Modify: `logic.go:873-882`, `logic.go:922-931`
- Test: `logic_presence_test.go`

**Interfaces:**
- Consumes: `botManager.beginPresenceSession(botID string, disableLLM bool)`, `botManager.presence`, `botManager.recordMessageSent()`.
- Produces: `messages_per_min` считается только по сообщениям цикла генерации; сигнатуры не меняются.

- [ ] **Step 1: Write the failing test**

Добавить в `logic_presence_test.go`:

```go
// TestPresenceGreetingNotCountedInMessagesPerMin verifies that the greeting is
// still sent but does not inflate the generation-activity metric.
func TestPresenceGreetingNotCountedInMessagesPerMin(t *testing.T) {
	bm := newBotManager(nil, nil, nil, nil, context.Background(), "testchannel", 0, audio.Config{}, false, nil)
	bm.presence = presenceConfig{
		enabled:     true,
		greeting:    true,
		sessionMin:  time.Minute,
		sessionMax:  time.Minute,
		cooldownMin: time.Minute,
		cooldownMax: time.Minute,
	}
	bm.presenceAfter = func(time.Duration, func()) presenceTimer { return &fakePresenceTimer{} }

	fake := newFakeTwitchClient()
	bm.mu.Lock()
	bm.bots["bot-1"] = &botRuntime{
		cfg:     storage.BotConfig{ID: "bot-1", Personality: "нейтрал"},
		client:  fake,
		running: true,
	}
	bm.mu.Unlock()

	bm.beginPresenceSession("bot-1", false)

	require.Len(t, fake.SentMessages(), 1, "greeting must still be sent")
	assert.Equal(t, 0, bm.MessagesPerMin(), "presence phrase must not count as generation activity")
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test . -run TestPresenceGreetingNotCountedInMessagesPerMin -v`
Expected: FAIL — `MessagesPerMin()` возвращает 1.

- [ ] **Step 3: Stop counting presence phrases**

В `beginPresenceSession` и `pauseBot` оставить отправку и `slog.Warn` при ошибке, но убрать ветку `else { bm.recordMessageSent() }`, чтобы успешная presence-фраза не вызывала `recordMessageSent`. Комментарий у `logic.go:873` дополнить: фразы не входят в `messages_per_min`.

- [ ] **Step 4: Run root package tests**

Run: `go test . -v -run 'TestPresence|TestMessagesPerMin'`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add logic.go logic_presence_test.go
git commit -m "fix(stats): keep presence phrases out of messages_per_min"
```

---

### Task 5: Отвечать на PING корректным PONG

**Files:**
- Modify: `internal/twitch/chat_monitor.go:274-287`
- Test: `internal/twitch/chat_monitor_test.go`

**Interfaces:**
- Consumes: ничего.
- Produces: `func pongReply(line string) (reply string, ok bool)` — возвращает `"PONG :<payload>"` для любого `PING`-формата.

- [ ] **Step 1: Write the failing test**

Добавить в `internal/twitch/chat_monitor_test.go`:

```go
func TestPongReply(t *testing.T) {
	cases := []struct {
		line  string
		reply string
		ok    bool
	}{
		{"PING :tmi.twitch.tv", "PONG :tmi.twitch.tv", true},
		{"PING tmi.twitch.tv", "PONG :tmi.twitch.tv", true},
		{"PRIVMSG #chan :hello", "", false},
	}
	for _, c := range cases {
		reply, ok := pongReply(c.line)
		if ok != c.ok || reply != c.reply {
			t.Errorf("pongReply(%q) = (%q, %v), want (%q, %v)", c.line, reply, ok, c.reply, c.ok)
		}
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/twitch/ -run TestPongReply -v`
Expected: FAIL — `pongReply` не определена.

- [ ] **Step 3: Extract and fix the reply builder**

Добавить в `chat_monitor.go` функцию `pongReply`: отбросить префикс `PING`, `TrimSpace`, затем `TrimPrefix(..., ":")`, вернуть `"PONG :"+payload, true`; для строк без `PING` вернуть `"", false`. В `readLoop` заменить регистр `strings.HasPrefix(line, "PING")` на вызов `pongReply(line)` и запись результата через `writeLine(conn, reply)`.

- [ ] **Step 4: Run the package tests**

Run: `go test ./internal/twitch/ -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/twitch/chat_monitor.go internal/twitch/chat_monitor_test.go
git commit -m "fix(irc): reply to PING with a well-formed PONG"
```

---

### Task 6: Обновить карту кода и прогнать все гейты

**Files:**
- Modify: `docs/CODE_GRAPH.md` (раздел 8 «Инварианты и мины»)
- Modify: `docs/code-graph.json` (если в нём перечислены инварианты)

**Interfaces:**
- Consumes: результаты задач 1–5.
- Produces: документация, совпадающая с кодом.

- [ ] **Step 1: Update invariants**

В `docs/CODE_GRAPH.md` добавить пункты: `WaitForRateLimit` не держит `c.mu` во время сна; `stopBot` не ждёт цикл, который не запускался; `twitch_client_secret` не возвращается, есть `twitch_client_secret_set`; `messages_per_min` не включает presence-фразы. Обновить пункт 1, если в нём остался устаревший текст про отсутствие `runBotLoop`.

- [ ] **Step 2: Run all gates**

Run: `go build ./... && go vet ./... && go test ./...` затем `cd frontend && npm.cmd run build`
Expected: все exit 0.

- [ ] **Step 3: Commit**

```bash
git add docs/CODE_GRAPH.md docs/code-graph.json
git commit -m "docs: record the reliability invariants"
```

---

## Self-Review

- **Spec coverage:** зависание API (Task 1), стоп неактивного бота (Task 2), утечка секрета (Task 3), искажение метрики (Task 4), PONG (Task 5), документация и гейты (Task 6).
- **Type consistency:** `TwitchClientSecretSet` объявлен в Task 3 и используется только в Task 3; `pongReply` объявлен и используется в Task 5; сигнатуры публичных интерфейсов не меняются.
- **Review Focus:** каждый пункт закрыт тестом в своей задаче.
- **Proportion:** план короче кода, который меняет; тела функций описаны словами, кроме алгоритма PONG, где формат строки не выводится из сигнатуры.
