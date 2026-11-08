# OpenRouter LLM and whisper.cpp STT Migration Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Replace LM Studio with an OpenRouter client (free-first with a capped paid fallback) and replace the Python STT service with whisper.cpp, without breaking the existing bot farm behavior.

**Architecture:** Keep the Go backend and React frontend. Add OpenRouter settings columns and a migration to the existing `settings` row. Replace `internal/llm/lmstudio.go` with a mutex-protected OpenRouter client configured from env + database through one pure resolution function, then wire it into the existing single client instance via `Reconfigure`. Keep the backend-to-STT HTTP contract unchanged and swap the sidecar image to the official whisper.cpp image with the local GGML model mounted read-only.

**Tech Stack:** Go 1.26+, PostgreSQL 16, Redis 7, React 19 + TypeScript + Vite, OpenRouter chat completions API, official `ghcr.io/ggml-org/whisper.cpp:main` image.

**Spec:** `docs/superpowers/specs/2026-11-09-openrouter-whispercpp-design.md`

## Global Constraints

- Free model is exactly `openrouter/free`; paid fallback model is exactly `z-ai/glm-5.3-flash`.
- Paid fallback defaults to **enabled** with a **$1.00/day** cap. A cap `<= 0` means unlimited.
- Precedence for the API key: `OPENROUTER_API_KEY` env > `llm_api_key` database column > legacy `LLM_API_KEY` env.
- Precedence for provider, base URL, free model, paid model: non-empty database value > env > built-in default.
- Precedence for `llm_paid_enabled` and `llm_paid_daily_cap_usd`: non-empty env value > database > default. An empty env value means "not set".
- `GET {baseURL}/key` is the only health check. It must never call `chat/completions`.
- Never log, return, or echo the API key. `GetSettings` always returns `llm_api_key: ""`.
- A response is paid when `usage.cost > 0 || model == paid model`. Never compare against the literal `openrouter/free`.
- Empty `choices`, empty/whitespace `content`, or `finish_reason == "content_filter"` is an error and is **never retried**.
- Retries: at most 2 attempts total, only for transport errors and HTTP 5xx, 500 ms apart, respecting `ctx`. No retry on 400/401/402/429.
- Paid spend tracking is best-effort: `$0.0005` in-flight reservation per paid-eligible request, replaced by actual `usage.cost`.
- Free cooldown starts at 60 s, doubles per consecutive paid-served response, caps at 15 min, and resets on a free-served response. It only suppresses free when paid is usable.
- STT stays at `http://stt:9000/v1/audio/transcriptions`. Multipart fields: `file`, `language`, `response_format=json`; keep `model` for OpenAI-shaped requests.
- STT model source: `C:\Users\blindfold\Downloads\ggml-small-q5_1.bin` (190,085,487 bytes), copied to `./models/ggml-small-q5_1.bin`. `models/` is gitignored and the model is never committed.
- Each task ends with a green `go build ./...` (and `go test ./...` where applicable) and one commit.

## Review Focus

- OpenRouter omits `usage` or returns `cost: null`/absent → generation still succeeds, is not misclassified as paid, and does not charge a reservation.
- `GET /key` times out or returns 5xx → `Generate` still attempts free-only and does not block for more than the request timeout.
- Settings are saved from the masked UI with `llm_api_key: ""` → the stored database key must be preserved, not wiped.
- `LLM_PAID_ENABLED` or `LLM_PAID_DAILY_CAP_USD` contains invalid text → resolution falls back to the database/default; a valid negative/zero cap means unlimited.
- whisper.cpp returns `{"text": ""}` for silence or non-JSON on failure → empty text is a successful no-op, non-JSON is a clear error, and neither crashes the audio loop.

---

### Task 1: Settings storage, paid columns, and migration

**Files:**
- Modify: `internal/storage/settings.go`
- Create: `internal/storage/settings_migration_test.go`
- Test: `internal/storage/settings_migration_test.go`

**Interfaces:**
- Consumes: existing `Store`, `EnsureSettingsTable(ctx)`, `GetSettings(ctx)`, `UpdateSettings(ctx, *Settings)`.
- Produces: `Settings.LLMPaidModel string`, `LLMPaidEnabled bool`, `LLMPaidDailyCapUSD float64`, plus computed JSON-only fields `LLMPaidEnabledEffective bool`, `LLMPaidDailyCapUSDEffective float64`, `LLMApiKeySet bool`, `LLMApiKeySource string`.

- [ ] **Step 1: Write the failing migration test**

Append to `internal/storage/settings_migration_test.go` (package `storage`). Skip when Postgres is unavailable so `go test -short ./...` stays green.

```go
func TestEnsureSettingsTableMigratesLMStudioSettings(t *testing.T) {
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

	_, err = pool.Exec(ctx, `UPDATE settings SET llm_provider='lmstudio',
		llm_api_url='http://127.0.0.1:1234/v1', llm_model='', llm_api_key='' WHERE id='default'`)
	require.NoError(t, err)
	require.NoError(t, store.EnsureSettingsTable(ctx))

	got, err := store.GetSettings(ctx)
	require.NoError(t, err)
	assert.Equal(t, "openrouter", got.LLMProvider)
	assert.Equal(t, "https://openrouter.ai/api/v1", got.LLMApiURL)
	assert.Equal(t, "openrouter/free", got.LLMModel)
	assert.Equal(t, "z-ai/glm-5.3-flash", got.LLMPaidModel)
	assert.True(t, got.LLMPaidEnabled)
	assert.Equal(t, 1.0, got.LLMPaidDailyCapUSD)
}
```

- [ ] **Step 2: Run it to verify it fails**

Run: `go test ./internal/storage -run TestEnsureSettingsTableMigratesLMStudioSettings -v`
Expected: FAIL — unknown fields/columns `LLMPaidModel`, `llm_paid_model`, `LLMPaidEnabled`, `llm_paid_daily_cap_usd`.

- [ ] **Step 3: Add fields, DDL defaults, and idempotent migration**

In `internal/storage/settings.go`:

1. Add to `Settings` after `LLMApiKey`:

```go
LLMPaidModel                string `json:"llm_paid_model"`
LLMPaidEnabled              bool   `json:"llm_paid_enabled"`
LLMPaidDailyCapUSD          float64 `json:"llm_paid_daily_cap_usd"`
LLMPaidEnabledEffective     bool   `json:"llm_paid_enabled_effective"`
LLMPaidDailyCapUSDEffective float64 `json:"llm_paid_daily_cap_usd_effective"`
LLMApiKeySet                bool   `json:"llm_api_key_set"`
LLMApiKeySource             string `json:"llm_api_key_source"`
```

2. Change `settingsTableSQL` defaults to `llm_provider='openrouter'`, `llm_api_url='https://openrouter.ai/api/v1'`, `llm_model='openrouter/free'`, and add the three new columns with defaults `'z-ai/glm-5.3-flash'`, `TRUE`, `1.0`.
3. Append the `ALTER TABLE ... ADD COLUMN IF NOT EXISTS` statements and the `UPDATE settings SET ... WHERE llm_provider='lmstudio' OR llm_api_url LIKE '%:1234%' OR llm_model='' OR llm_model='local-model'` statement from spec 9.1 to `settingsTableSQL`.
4. Add the three new columns to the `GetSettings` SELECT and `Scan` list, in the same order as the INSERT defaults.
5. Add them to the `UpdateSettings` UPDATE statement and argument list; insert placeholders at the end before `twitch_client_id` so the numbering stays readable (`llm_paid_model`, `llm_paid_enabled`, `llm_paid_daily_cap_usd` become `$14`, `$15`, `$16`; Twitch fields shift to `$17`, `$18`).
6. Do **not** persist the computed fields; they remain JSON-only.

- [ ] **Step 4: Run the test to verify it passes**

Run: `go test ./internal/storage -run TestEnsureSettingsTableMigratesLMStudioSettings -v`
Expected: PASS when Postgres is up; `SKIP` when it is not.

- [ ] **Step 5: Run the package tests and build, then commit**

```bash
go build ./...
go test ./internal/storage
git add internal/storage/settings.go internal/storage/settings_migration_test.go
git commit -m "feat(settings): add OpenRouter paid fallback columns and migration"
```

---

### Task 2: Pure env + database config resolution

**Files:**
- Create: `internal/llm/config.go`
- Test: `internal/llm/config_test.go`

**Interfaces:**
- Consumes: `*storage.Settings`.
- Produces:
  - `type Config struct { BaseURL, APIKey, FreeModel, PaidModel string; PaidEnabled bool; PaidDailyCapUSD float64; Referer, Title string; HTTPClient *http.Client }`
  - `type EnvConfig struct { APIKey, LegacyAPIKey, Provider, BaseURL, FreeModel, PaidModel, PaidEnabled, PaidDailyCapUSD, Referer, Title string }`
  - `type DBConfig struct { Provider, BaseURL, FreeModel, PaidModel, APIKey string; PaidEnabled *bool; PaidDailyCapUSD *float64 }` — pointers distinguish "no row" (`nil`) from a stored `false`/`0`.
  - `const KeySourceEnv = "env"`, `KeySourceDatabase = "database"`, `KeySourceNone = "none"`
  - `func EnvFromLookup(lookup func(string) string) EnvConfig`
  - `func DBConfigFromSettings(s *storage.Settings) DBConfig` — returns a zero `DBConfig` when `s == nil`
  - `func ResolveConfig(env EnvConfig, db DBConfig) (Config, string)` — second value is the key source.
  - Defaults: provider `openrouter`, URL `https://openrouter.ai/api/v1`, free `openrouter/free`, paid `z-ai/glm-5.3-flash`, paid enabled `true`, cap `1.0`, referer `http://localhost:3000`, title `Twitch SMM Bot Farm`.

- [ ] **Step 1: Write the failing table test**

In `internal/llm/config_test.go`, test `ResolveConfig` with a table covering:

- all empty → built-in defaults (including paid enabled `true`, cap `1.0`), key source `none`;
- env key present and DB key present → env key wins, source `env`;
- env key empty, legacy key and DB key present → DB wins, source `database`;
- only legacy key → source `env`;
- DB provider/url/free/paid non-empty vs env → DB wins;
- empty DB provider/url/models → env fills them;
- empty DB model → default fills it;
- env `LLM_PAID_ENABLED=false` overrides DB `true`; env `"true"` overrides DB `false`;
- invalid env paid bool `"maybe"` → DB value;
- env cap `"0.25"` → 0.25; env cap `"-1"` → -1 (unlimited); invalid `"abc"` → DB value;
- empty env paid values → DB pointer values; nil DB pointers → defaults;
- a stored DB `PaidEnabled=false` / cap `0` (non-nil pointers) survives when no env override.

Core assertion shape:

```go
cfg, source := ResolveConfig(env, db)
assert.Equal(t, "z-ai/glm-5.3-flash", cfg.PaidModel)
assert.Equal(t, KeySourceEnv, source)
```

Also add `TestEnvFromLookupTreatsMissingAsEmpty` using a map-backed lookup and `TestDBConfigFromSettingsCopiesAllFields`.

- [ ] **Step 2: Run it to verify it fails**

Run: `go test ./internal/llm -run 'TestResolveConfig|TestEnvFromLookup|TestDBConfigFromSettings' -v`
Expected: FAIL — `internal/llm/config.go` and the types do not exist.

- [ ] **Step 3: Implement `internal/llm/config.go`**

Implement the three constants and defaults. `EnvFromLookup` reads exactly these keys:

```text
OPENROUTER_API_KEY, LLM_API_KEY, LLM_PROVIDER, LLM_API_URL, LLM_MODEL,
LLM_PAID_MODEL, LLM_PAID_ENABLED, LLM_PAID_DAILY_CAP_USD,
OPENROUTER_REFERER, OPENROUTER_TITLE
```

`ResolveConfig` rules:

1. Key: `env.APIKey`, else `db.APIKey`, else `env.LegacyAPIKey`; set source accordingly.
2. Provider/URL/free/paid: first non-empty of `db`, `env`, default. `Config` does not expose `Provider`; the client always reports `openrouter` (add a `Provider` constant if the implementer finds it clearer).
3. Paid enabled/cap: if the env string is non-empty **and parses**, use it; otherwise if the DB pointer is non-nil use the pointed value; otherwise use the default.
4. Referer/title: env non-empty else default; never from DB.
5. `HTTPClient` stays nil; `NewClient` supplies the 30 s default.

Do not read `os.Getenv` inside `ResolveConfig`; only `EnvFromLookup` takes a lookup function.

- [ ] **Step 4: Run the tests to verify they pass**

Run: `go test ./internal/llm -run 'TestResolveConfig|TestEnvFromLookup|TestDBConfigFromSettings' -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
go build ./...
git add internal/llm/config.go internal/llm/config_test.go
git commit -m "feat(llm): resolve OpenRouter config from env and database"
```

---

### Task 3: OpenRouter client, routing, budget, cooldown, and startup wiring

**Files:**
- Delete: `internal/llm/lmstudio.go`
- Create: `internal/llm/openrouter.go`
- Test: `internal/llm/openrouter_test.go`
- Modify: `logic.go` (`main`, `ApplySettings`)

**Interfaces:**
- Consumes: `Config`, `EnvConfig`, `DBConfig`, `EnvFromLookup`, `DBConfigFromSettings`, `ResolveConfig` from Task 2.
- Produces:
  - `func NewClient(cfg Config) *Client`
  - `func (c *Client) Reconfigure(cfg Config)`
  - `func (c *Client) Generate(ctx context.Context, systemPrompt, userPrompt string, maxTokens int) (string, error)`
  - `func (c *Client) HealthCheck(ctx context.Context) error`
  - `func (c *Client) Status(ctx context.Context) Status`
  - `func (c *Client) GetProvider() string`, `GetModel() string` (free model), `GetURL() string`
  - `type Status struct` with JSON tags: `available`, `generation_available`, `provider`, `free_model`, `paid_model`, `paid_enabled`, `paid_daily_cap_usd`, `paid_spent_today_usd`, `free_requests_limit`, `free_requests_remaining`, `free_requests_used`, `free_cooldown_until`, `inflight_paid_requests`, `last_error`, `checked_at`.

- [ ] **Step 1: Write the failing routing and parsing tests**

Create `internal/llm/openrouter_test.go` with a helper:

```go
func newTestClient(t *testing.T, h http.HandlerFunc) (*Client, *httptest.Server) {
	srv := httptest.NewServer(h)
	t.Cleanup(srv.Close)
	return NewClient(Config{
		BaseURL: srv.URL, APIKey: "sk-test",
		FreeModel: "openrouter/free", PaidModel: "z-ai/glm-5.3-flash",
		PaidEnabled: true, PaidDailyCapUSD: 1.0,
	}), srv
}
```

Add tests named and asserting:

- `TestGenerateFallbackRequest` — default config (paid enabled); server receives `"models":["openrouter/free","z-ai/glm-5.3-flash"]` and `"route":"fallback"`; returns `{"model":"upstage/solar-pro-3:free","choices":[{"message":{"content":"hi"}}],"usage":{"cost":0}}`; result `"hi"`, `free=true`.
- `TestGenerateDropsPaidModelWhenDisabled` — `PaidEnabled=false`; server receives `"model":"openrouter/free"`, no `models`/`route`.
- `TestGenerateRefreshesStaleKeyInfo` — a key cache older than 30 s is refreshed before routing; exhausted free quota makes the body paid-only.
- `TestGenerateContinuesWhenKeyFetchFails` — `/key` returns 500; generation still sends the free-first request and succeeds.
- `TestGeneratePaidOnlyWhenFreeExhausted` — seed key cache with `FreeRemaining=0`; body is `{"model":"z-ai/glm-5.3-flash"}`.
- `TestGenerateBudgetExhausted` — free remaining 0, paid disabled; error string is exactly `LLM budget exhausted: free daily limit reached and paid fallback is disabled or capped`, and no HTTP request is made.
- `TestGenerateClassifiesPaidFromCost` — model `upstage/solar-pro-3:free`, `usage.cost=0.00002`; then `Status` shows paid spend > 0 and a cooldown in the future.
- `TestGenerateClassifiesPaidFromModelWhenUsageMissing` — no `usage` object, model equals paid model; treated as paid, no panic.
- `TestGenerateEmptyCompletionIsNotRetried` — server returns empty `choices`; counter stays 1; error contains `openrouter: empty completion`.
- `TestGenerateContentFilterIsError` — `finish_reason:"content_filter"`; error, one attempt.
- `TestGenerateRetriesOnly5xx` — first response 503, second 200; counter 2. A 429 case stays at counter 1.
- `TestGenerateCooldownDoublesAndCaps` — first paid-served fallback sets a 60 s block; advance the injected clock past it, trigger a second fallback, assert 120 s; a free-served response resets the block and backoff to zero.
- `TestHealthCheckParsesKeyAndNeverCallsCompletions` — `/key` returns the envelope; assert parsed limits, and fail the test if `/chat/completions` is hit.
- `TestStatusGenerationAvailable` — with a key configured, a `/key` failure gives `Available=false`, `GenerationAvailable=true`, `FreeRequestsRemaining=0`, and `LastError` mentioning the `/key` error.
- `TestStatusNotConfigured` — an empty API key gives `Available=false` and `GenerationAvailable=false`; no HTTP request is made.
- `TestReconfigureUpdatesFieldsAndResetsKeyCache` — change URL/models/key; getters reflect it and a subsequent health check hits the new URL.

To make time deterministic, add an unexported `now func() time.Time` field on `Client` (default `time.Now`) and set it in cooldown tests.

- [ ] **Step 2: Run to verify the tests fail**

Run: `go test ./internal/llm -run TestGenerate -v`
Expected: FAIL — the old `lmstudio.go` has a different `NewClient` signature and no routing/status.

- [ ] **Step 3: Implement `internal/llm/openrouter.go`**

Delete `internal/llm/lmstudio.go` in the same step. Implement:

- `Client` holds `mu sync.Mutex`, `cfg Config`, `httpClient *http.Client`, `key keyInfo`, `budget budgetState`, `now func() time.Time`.
- `NewClient` fills **empty string** fields (`BaseURL`, `FreeModel`, `PaidModel`, `Referer`, `Title`) from the built-in defaults and sets a 30 s HTTP client unless `cfg.HTTPClient` was provided. It must keep `PaidEnabled` and `PaidDailyCapUSD` exactly as given: `false` and `0` are meaningful values, not "unset".
- `Reconfigure` locks, replaces `cfg`, and resets `key.OK`/`key.FetchedAt` when `APIKey` or `BaseURL` changed.
- `Generate` refreshes `/key` first when the cached key info is older than 30 s; a refresh failure is ignored and generation continues with the "free available" assumption. The refresh gets its own 3 s timeout and must not consume the caller's whole LLM timeout.
- `Generate` locks only long enough to snapshot config/limits/reserve budget, then performs HTTP outside the lock, then locks to apply cost/cooldown. Reservations are released with the actual cost (or dropped) on every error path.
- Before using the budget, reset `localDailyCost` to zero when `now().UTC()` has a different date than `budget.day`.
- `Status.PaidSpentTodayUSD` is actual observed spend (`max(localDailyCost, keyInfo.UsageDaily)`), excluding reservations; `InflightPaidRequests` reports the reservations separately.
- Request dispatch follows the routing table in spec 9.3 exactly. Build the body from the three shapes in spec 9.2. Always send `temperature: 0.8`, `stream: false`, and the caller's `maxTokens`.
- Headers: `Authorization: Bearer <key>`, `Content-Type`, `HTTP-Referer`, `X-OpenRouter-Title`. If the key is empty, return `OpenRouter API key is not configured (set OPENROUTER_API_KEY)` before any HTTP call.
- Parse `{choices:[{message:{content},finish_reason}], model, usage:{cost,prompt_tokens,completion_tokens}, error}`. Apply the paid detection, empty-completion, retry, reservation, and cooldown rules from Global Constraints.
- Log one `slog.Info("llm completion", requested, model, free, cost_usd, prompt_tokens, completion_tokens)` line per success.
- `refreshKeyInfo(ctx)` does `GET {BaseURL}/key`, caches for 30 s, parses `data.usage_daily` and `data.free_model_daily_requests.{limit,remaining,used}`. `HealthCheck` calls it and returns only its error. `Status` calls it (best-effort), then computes the fields below.
- `GenerationAvailable = cfg.APIKey != "" && (freeAvailable || paidAllowed)`. Without a key no path can generate, even though free quota is unknown.
- `Status` must never mutate config or spend.

- [ ] **Step 4: Run the client tests to verify they pass**

Run: `go test ./internal/llm -v`
Expected: PASS.

- [ ] **Step 5: Wire `logic.go`**

In `main()`:

- Delete `llmURL`, `llmModel`, `llmAPIKey`, `llmProvider` and their `LMSTUDIO_*` fallbacks.
- Create `llmEnv := llm.EnvFromLookup(os.Getenv)` and `llmClient := llm.NewClient(mustResolve(llmEnv, llm.DBConfig{}))`. Add this tiny helper at the bottom of `main`'s file scope:

```go
func mustResolve(env llm.EnvConfig, db llm.DBConfig) llm.Config {
	cfg, _ := llm.ResolveConfig(env, db)
	return cfg
}
```

- After `store.GetSettings(ctx)` succeeds, call `llmClient.Reconfigure(mustResolve(llmEnv, llm.DBConfigFromSettings(settings)))`, then keep the existing informational health log (it is now a free `/key` call). Remove the "LM Studio must be running" wording.
- In `ApplySettings`, remove the new-client/health-check block. Resolve from `llm.EnvFromLookup(os.Getenv)` + `llm.DBConfigFromSettings(settings)`, call `bm.llmClient.Reconfigure(cfg)` (guard nil), and log provider, free model, paid model, effective paid enabled, effective cap, and `api_key_set` (boolean only).
- Leave `generateMessage` and `TestLLMGeneration` unchanged; they already call `Generate`.

- [ ] **Step 6: Build, test, and commit**

```bash
go build ./...
go test ./...
git add internal/llm logic.go
git commit -m "feat(llm): replace LM Studio with OpenRouter fallback client"
```

---

### Task 4: Settings masking, effective fields, and LLM status API

**Files:**
- Modify: `internal/api/handlers.go`
- Test: `internal/api/handlers_test.go`

**Interfaces:**
- Consumes: `llm.ResolveConfig`, `llm.EnvFromLookup`, `llm.DBConfigFromSettings`, `llm.KeySource*`, `llm.Client.Status`.
- Produces: `Handler.envLookup func(string) string` (set to `os.Getenv` by `NewHandler`); `GetSettings` response with masked key and computed fields; `GetLLMStatus` response with `generation_available` and budget fields.

- [ ] **Step 1: Write the failing handler tests**

Extend `internal/api/handlers_test.go` (package `api`) with a fixed env lookup:

```go
func fixedEnv(values map[string]string) func(string) string {
	return func(key string) string { return values[key] }
}
```

Add tests:

- `TestGetSettingsMasksAPIKeyAndReportsSource` — DB key `sk-db`; `handler.envLookup = fixedEnv(nil)`; response has `llm_api_key == ""`, `llm_api_key_set == true`, `llm_api_key_source == "database"`.
- `TestGetSettingsReportsEnvKey` — `OPENROUTER_API_KEY=sk-env` plus DB key; source `env`, body never contains `sk-env`.
- `TestGetSettingsReportsPaidEnvOverride` — DB paid enabled `false`, cap `0.5`; env `LLM_PAID_ENABLED=true`, `LLM_PAID_DAILY_CAP_USD=0.25`; computed effective values are `true` and `0.25`.
- `TestUpdateSettingsPreservesStoredKeyWhenEmpty` — request body has `llm_api_key: ""`; `MockStore.UpdateSettings` receives `sk-db`; the settings manager receives `sk-db`.
- `TestUpdateSettingsUsesIncomingKeyWhenProvided` — body `sk-new`; store and settings manager receive `sk-new`.
- `TestGetLLMStatusIncludesGenerationFields` — use an `httptest` OpenRouter stub via `llm.NewClient`; assert JSON keys `available`, `generation_available`, `model`, `free_model`, `url`, `paid_spent_today_usd`, `free_requests_remaining`, `inflight_paid_requests`, and `generation_available=true`.
- `TestCalculateSettingsChangesIncludesPaidFields` — paid model/enabled/cap changes appear in the change map.

Use the existing `MockStore`. Add `type MockSettingsManager struct{ mock.Mock }` implementing `ApplySettings`, `GetAudioContext`, and `TestLLMGeneration` via `m.Called(...)`. `MockBotManager` does not implement `SettingsManager`, so assign the mock directly after construction: `handler.settingsManager = mockSettings`. Assert the `ApplySettings` argument with `mock.MatchedBy`.

- [ ] **Step 2: Run to verify the tests fail**

Run: `go test ./internal/api -run 'TestGetSettings|TestUpdateSettings|TestGetLLMStatus|TestCalculateSettingsChangesIncludesPaidFields' -v`
Expected: FAIL — key still returned, computed fields absent, status fields absent.

- [ ] **Step 3: Implement the handler changes**

- Add `envLookup func(string) string` to `Handler`; initialize it to `os.Getenv` in `NewHandler`.
- Add a helper `resolveSettings(settings *storage.Settings) llm.Config` that calls `llm.ResolveConfig(llm.EnvFromLookup(h.envLookup), llm.DBConfigFromSettings(settings))`.
- `GetSettings`: after loading, call the helper; set `LLMApiKey=""`, `LLMApiKeySet = cfg.APIKey != ""`, `LLMApiKeySource = source`, `LLMPaidEnabledEffective = cfg.PaidEnabled`, `LLMPaidDailyCapUSDEffective = cfg.PaidDailyCapUSD`; encode.
- `UpdateSettings`: load `oldSettings`; if `settings.LLMApiKey == ""`, set it to `oldSettings.LLMApiKey`; clear/ignore incoming computed fields by overwriting them from resolution before calling `ApplySettings`. Keep all existing change logging and error behavior.
- Add change entries and messages for `llm_paid_model`, `llm_paid_enabled`, `llm_paid_daily_cap_usd`, and an `llm_api_key` entry that reports `{old:"***", new:"***"}` only when the stored key changed. Never include the key value.
- `GetLLMStatus`: when `h.llmClient == nil` keep the current fallback but add `generation_available:false`. Otherwise call `Status(ctx)` and build the response map with the exact keys listed in the Interfaces block; add `error` only when `LastError != ""`. Keep `model` as the free model for existing callers.

- [ ] **Step 4: Run the tests and the package build**

Run: `go test ./internal/api -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
go build ./...
go test ./...
git add internal/api/handlers.go internal/api/handlers_test.go
git commit -m "feat(api): mask keys and expose OpenRouter budget status"
```

---

### Task 5: Frontend settings and status UI

**Files:**
- Modify: `frontend/src/api/client.ts`
- Modify: `frontend/src/pages/Settings.tsx`
- Modify: `frontend/src/components/StatusBar.tsx`
- Modify: `frontend/src/components/FloatingProviderStatus.tsx`

**Interfaces:**
- Consumes: the Task 4 JSON contract.
- Produces: TypeScript types and UI fields that round-trip the new settings without sending the masked key back as a change; dashboard indicators driven by `generation_available`.

- [ ] **Step 1: Extend the TypeScript types**

In `frontend/src/api/client.ts`:

- Add to `Settings`: `llm_paid_model: string`, `llm_paid_enabled: boolean`, `llm_paid_daily_cap_usd: number`, `llm_paid_enabled_effective: boolean`, `llm_paid_daily_cap_usd_effective: number`, `llm_api_key_set: boolean`, `llm_api_key_source: 'env' | 'database' | 'none'`.
- Extend `LLMStatus` with `generation_available: boolean`, `free_model: string`, `paid_model: string`, `paid_enabled: boolean`, `paid_daily_cap_usd: number`, `paid_spent_today_usd: number`, `free_requests_limit: number`, `free_requests_remaining: number`, `free_requests_used: number`, `free_cooldown_until: string`, `inflight_paid_requests: number`, `last_error?: string`.

- [ ] **Step 2: Update `Settings.tsx` defaults and data mapping**

- Default state: `llm_provider: 'openrouter'`, `llm_api_url: 'https://openrouter.ai/api/v1'`, `llm_model: 'openrouter/free'`, `llm_paid_model: 'z-ai/glm-5.3-flash'`, `llm_paid_enabled: true`, `llm_paid_daily_cap_usd: 1`, `llm_api_key: ''`, `llm_api_key_set: false`, `llm_api_key_source: 'none'`, effective fields equal to stored values.
- In the settings-load `useEffect`, use `??` for booleans/numbers (so `false` and `0` survive) and always set `llm_api_key: ''`.
- The provider select shows a single `OpenRouter` option and no longer rewrites the URL to `127.0.0.1:1234`.
- The API key input has placeholder `sk-or-v1-...` when no key is set and `•••••••• (already set)` when `llm_api_key_set` is true. Add a badge: `env` → "Ключ из окружения", `database` → "Ключ сохранён в БД", `none` → "Ключ не задан".
- Add paid model text input, paid fallback `Switch`, and numeric daily cap input with hint «0 или меньше — без лимита». When the corresponding effective value differs from the stored value, render the field read-only and show «Переопределено окружением».

- [ ] **Step 3: Update the connection test and status area**

- Keep calling `/api/llm/status`.
- On success (`available === true`) show the provider and free model; if `generation_available === false`, additionally show `last_error`/reason.
- Add a status row: today's spend `paid_spent_today_usd`, free remaining `free_requests_remaining`, paid enabled/cap, and a cooldown note when `free_cooldown_until` is in the future.
- Replace the LM Studio instruction block with OpenRouter text: free model is used first; paid fallback is bounded by the cap; key comes from `OPENROUTER_API_KEY` or the database.

- [ ] **Step 4: Drive dashboard indicators from `generation_available`**

- `StatusBar.tsx`: guard `status == null` (render the connection-error branch). Keep the `!status.available` branch as a connection problem. Insert a second branch for `status.available && !status.generation_available` using the warning style, with the text `{status.provider.toUpperCase()} НЕ МОЖЕТ ГЕНЕРИРОВАТЬ`, the `last_error` reason, `free_requests_remaining`, and the existing settings button. Only the final success branch means the farm can post.
- `FloatingProviderStatus.tsx`: replace the two-state `available` dot/label with three states — green `READY` when `available && generation_available`, amber `LIMITED` when `available && !generation_available`, red `DISCONNECTED` otherwise. Show `last_error`, `paid_spent_today_usd`, and `free_requests_remaining` in the detail panel. Keep polling `/api/llm/status`.

- [ ] **Step 5: Verify the frontend builds**

Run: `cd frontend && npm run build`
Expected: `tsc -b` and `vite build` complete with exit code 0.

- [ ] **Step 6: Commit**

```bash
git add frontend/src/api/client.ts frontend/src/pages/Settings.tsx \
  frontend/src/components/StatusBar.tsx frontend/src/components/FloatingProviderStatus.tsx
git commit -m "feat(ui): configure OpenRouter free and paid fallback"
```

---

### Task 6: whisper.cpp STT sidecar

**Files:**
- Modify: `internal/audio/service.go`
- Create: `internal/audio/service_test.go`
- Modify: `docker-compose.yml` (`stt` service and volumes)
- Modify: `.gitignore`
- Delete: `stt_server/app.py`, `stt_server/Dockerfile`, `stt_server/requirements.txt`

**Interfaces:**
- Consumes: existing `audio.Service.transcribe(ctx, audioData)` and `audio.Config`.
- Produces: `transcribe` sends `response_format=json`; Compose `stt` runs `whisper-server` on port 9000 with `/v1/audio/transcriptions`.

- [ ] **Step 1: Write the failing transcription request test**

Create `internal/audio/service_test.go` (package `audio`):

```go
func TestTranscribeSendsWhisperFields(t *testing.T) {
	var got map[string]string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.NoError(t, r.ParseMultipartForm(1<<20))
		got = map[string]string{
			"model": r.FormValue("model"), "language": r.FormValue("language"),
			"response_format": r.FormValue("response_format"),
		}
		_, file, err := r.FormFile("file")
		require.NoError(t, err)
		require.NotNil(t, file)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"text":"привет"}`))
	}))
	defer srv.Close()

	s := NewService(Config{STTURL: srv.URL, STTModel: "whisper-1", STTLanguage: "ru"})
	text, err := s.transcribe(context.Background(), []byte("wav"))
	require.NoError(t, err)
	assert.Equal(t, "привет", text)
	assert.Equal(t, "whisper-1", got["model"])
	assert.Equal(t, "ru", got["language"])
	assert.Equal(t, "json", got["response_format"])
}
```

Add `TestTranscribeEmptyTextIsNotAnError` returning `{"text":""}` and asserting `""`, `nil`. Add `TestTranscribeNonJSONIsError` returning `not json` with HTTP 200 and asserting a non-nil error containing `unmarshal stt response`.

- [ ] **Step 2: Run it to verify it fails**

Run: `go test ./internal/audio -run TestTranscribe -v`
Expected: FAIL — `response_format` is empty.

- [ ] **Step 3: Add the field and replace the Compose service**

- In `transcribe`, after the `language` field, add `_ = writer.WriteField("response_format", "json")`. Keep the `model` field.
- Replace the `stt` service in `docker-compose.yml` with the YAML from spec 9.5 (official image, `profiles: ["audio"]`, `--inference-path /v1/audio/transcriptions`, `./models:/models:ro`, `curl /health` check).
- Remove `whisper_cache` from the top-level `volumes:` block.
- Add `models/` to `.gitignore`.
- Delete the `stt_server/` directory.

- [ ] **Step 4: Put the model in place and verify the sidecar**

```powershell
New-Item -ItemType Directory -Force models | Out-Null
Copy-Item "C:\Users\blindfold\Downloads\ggml-small-q5_1.bin" "models\ggml-small-q5_1.bin"
(Get-Item "models\ggml-small-q5_1.bin").Length   # expect 190085487
docker compose --profile audio up -d stt
docker compose ps stt
curl.exe -fsS http://localhost:9000/health
```

Expected: model length `190085487`, container healthy, `/health` returns HTTP 200.

Optionally smoke-test the endpoint with a generated silent chunk:

```powershell
ffmpeg -y -f lavfi -i anullsrc=r=16000:cl=mono -t 1 -c:a pcm_s16le chunk.wav
curl.exe -fsS -X POST http://localhost:9000/v1/audio/transcriptions -F "file=@chunk.wav" -F "language=ru" -F "response_format=json"
```

Expected: a JSON object containing a `text` field.

- [ ] **Step 5: Run audio tests, build, and commit**

```bash
go test ./internal/audio -v
go build ./...
git add internal/audio/service.go internal/audio/service_test.go docker-compose.yml .gitignore
git add -A stt_server
git commit -m "feat(stt): replace Python service with whisper.cpp sidecar"
```

Confirm `git status --short` does not list `models/`.

---

### Task 7: Compose environment, `.env.example`, and documentation

**Files:**
- Modify: `docker-compose.yml` (backend `environment`)
- Rewrite: `.env.example` (UTF-8)
- Modify: `README.md`
- Modify: `docs/AGENTS.md`
- Modify: `docs/AUDIO_TRANSCRIPTION.md`

**Interfaces:**
- Consumes: env variable names from spec 9.6.
- Produces: a deployment where Compose provides correct defaults and every document describes OpenRouter + whisper.cpp.

- [ ] **Step 1: Replace the backend LLM environment**

In `docker-compose.yml`, replace the `LLM_*` block with exactly:

```yaml
- LLM_PROVIDER=${LLM_PROVIDER:-openrouter}
- LLM_API_URL=${LLM_API_URL:-https://openrouter.ai/api/v1}
- LLM_MODEL=${LLM_MODEL:-openrouter/free}
- LLM_PAID_MODEL=${LLM_PAID_MODEL:-z-ai/glm-5.3-flash}
- LLM_PAID_ENABLED=${LLM_PAID_ENABLED:-}
- LLM_PAID_DAILY_CAP_USD=${LLM_PAID_DAILY_CAP_USD:-}
- OPENROUTER_API_KEY=${OPENROUTER_API_KEY:-}
- LLM_API_KEY=${LLM_API_KEY:-}
- OPENROUTER_REFERER=${OPENROUTER_REFERER:-http://localhost:3000}
- OPENROUTER_TITLE=${OPENROUTER_TITLE:-Twitch SMM Bot Farm}
```

Keep the existing `extra_hosts` entry; it is harmless and still useful for host-run services.

- [ ] **Step 2: Rewrite `.env.example` as UTF-8**

Use the existing section style but replace the LM Studio block with OpenRouter and document the paid overrides as optional:

```env
# === LLM (OpenRouter) ===
OPENROUTER_API_KEY=
# Optional: legacy name, used only when OPENROUTER_API_KEY and the DB key are empty
LLM_API_KEY=
LLM_PROVIDER=openrouter
LLM_API_URL=https://openrouter.ai/api/v1
LLM_MODEL=openrouter/free
LLM_PAID_MODEL=z-ai/glm-5.3-flash
# Optional operator overrides. Leave commented out to use the UI/database values.
# LLM_PAID_ENABLED=false
# LLM_PAID_DAILY_CAP_USD=1.0
OPENROUTER_REFERER=http://localhost:3000
OPENROUTER_TITLE=Twitch SMM Bot Farm
```

Add STT variables: `STT_MODEL_FILE=ggml-small-q5_1.bin`, `STT_THREADS=4`, `STT_LANGUAGE=ru`. Keep `AUDIO_STT_URL=http://stt:9000/v1/audio/transcriptions`, `AUDIO_STT_MODEL=whisper-1`, `AUDIO_STT_LANGUAGE=ru`. Remove `STT_MODEL_NAME`, `STT_DEVICE`, `STT_COMPUTE_TYPE`.

- [ ] **Step 3: Update the docs**

- `README.md` LLM section: OpenRouter is the only provider; explain free-first behavior, the paid fallback and default $1/day cap, where to put `OPENROUTER_API_KEY`, and that the farm still starts without a key. Note that bursts above OpenRouter's 20 free requests/minute can consume paid capacity, bounded by the cap. Project layout: `internal/audio` + whisper.cpp sidecar instead of `stt_server/`. Add a one-line note that `models/ggml-small-q5_1.bin` must exist before `--profile audio`.
- `docs/AGENTS.md`: tech stack line becomes `OpenRouter API (free-first with paid fallback)`; feature 2 and Important Notes updated; remove "LM Studio must be running".
- `docs/AUDIO_TRANSCRIPTION.md`: replace `faster-whisper` with `whisper.cpp`, describe the official image, `--inference-path`, the model mount, and the new STT env variables. Keep the Go capture/ffmpeg/context sections intact. Update troubleshooting to check `docker compose --profile audio ps stt` and `curl http://localhost:9000/health`.

- [ ] **Step 4: Verify no stale references remain and Compose parses**

```powershell
Select-String -Path README.md,docs/AGENTS.md,docs/AUDIO_TRANSCRIPTION.md,.env.example,docker-compose.yml -Pattern 'LM Studio|lmstudio|faster-whisper|stt_server|127\.0\.0\.1:1234|STT_MODEL_NAME'
docker compose config --quiet
```

Expected: no matches; Compose exits 0.

- [ ] **Step 5: Commit**

```bash
git add docker-compose.yml .env.example README.md docs/AGENTS.md docs/AUDIO_TRANSCRIPTION.md
git commit -m "docs: document OpenRouter and whisper.cpp deployment"
```

---

### Task 8: End-to-end verification

**Files:** none (verification only; fix any failure in the task that owns the code).

**Interfaces:**
- Consumes: all previous tasks.
- Produces: recorded evidence that the migrated system builds, starts, and behaves correctly.

- [ ] **Step 1: Build and test the backend and frontend**

```bash
go build ./...
go test ./...
cd frontend && npm run build
```

Expected: exit code 0 for all; storage integration tests may `SKIP` when Postgres is unavailable.

- [ ] **Step 2: Rebuild the stack**

```bash
docker compose up -d --build --remove-orphans --wait
docker compose ps
```

Expected: `postgres`, `redis`, `backend`, `frontend` are healthy; no `stt` container unless the audio profile is enabled.

- [ ] **Step 3: Check the API without an OpenRouter key**

```bash
curl.exe -fsS -H "X-API-Key: secret-api-key-12345" http://localhost:8080/api/health
curl.exe -fsS -H "X-API-Key: secret-api-key-12345" http://localhost:8080/api/settings
curl.exe -fsS -H "X-API-Key: secret-api-key-12345" http://localhost:8080/api/llm/status
```

Expected: health is `degraded` only because of the LLM; settings has `llm_api_key:""`, `llm_api_key_source:"none"`, `llm_api_url:"https://openrouter.ai/api/v1"`, `llm_model:"openrouter/free"`; status has `available:false`, `generation_available:false`, and the error `OpenRouter API key is not configured (set OPENROUTER_API_KEY)`.

- [ ] **Step 4: Check the API with a key**

Set `OPENROUTER_API_KEY` in `.env`, then `docker compose up -d backend` and repeat Step 3.

Expected: status `available:true`, `generation_available:true`, free/paid models correct, `free_requests_remaining` populated, `paid_spent_today_usd` numeric; settings reports `llm_api_key_source:"env"` and never returns the key.

- [ ] **Step 5: Prove runtime reconfiguration**

In the UI (**Settings → LLM**), change the daily cap to `0.5` and save, then:

```bash
curl.exe -fsS -H "X-API-Key: secret-api-key-12345" http://localhost:8080/api/llm/status
```

Expected: the status shows the new cap without restarting the backend. With `LLM_PAID_ENABLED=false` in the environment, settings marks the toggle as overridden and status shows paid disabled.

Do not send a partial JSON body to `POST /api/settings` for this check: the endpoint replaces the whole settings row, so a partial body would wipe channels and prompts.

- [ ] **Step 6: Debug-mode bot generation**

Enable a bot in debug mode against the configured channel, start it, and inspect backend logs.

Expected: one `msg="llm completion"` line per generated message with the concrete model, `free=true` while free quota remains, and `cost_usd=0`. No Twitch message is sent in debug mode.

- [ ] **Step 7: STT smoke test**

```bash
docker compose --profile audio up -d stt
docker compose ps stt
curl.exe -fsS http://localhost:9000/health
```

Expected: healthy, and the Task 6 transcription call returns a JSON `text` field. Then stop the profile if it is not needed: `docker compose --profile audio stop stt`.

- [ ] **Step 8: Report evidence**

Record command output for Steps 1–7 in the task discussion. Do not claim completion for any step that was skipped; mark live-key steps as pending until a key is available.
