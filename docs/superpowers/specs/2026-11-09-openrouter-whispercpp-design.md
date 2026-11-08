---
name: OpenRouter LLM and whisper.cpp STT Migration
description: Replace LM Studio with OpenRouter (free-first with paid fallback) and replace the Python faster-whisper service with whisper.cpp
type: design
date: 2026-11-09
---

# OpenRouter LLM and whisper.cpp STT Migration - Design

## 1. Context

The bot farm currently generates chat messages through an LM Studio server running on the
host (`http://host.docker.internal:1234/v1`). The LLM client in `internal/llm/lmstudio.go`
contains LM Studio-specific auto-initialization (model discovery, model loading, availability
probing) and hard-coded defaults.

Optional stream transcription is provided by `stt_server/`, a Python FastAPI service that
wraps `faster-whisper` and exposes an OpenAI-compatible `/v1/audio/transcriptions` endpoint
on port 9000. It is only started with the `audio` Docker Compose profile.

Both services are now being replaced:

- LM Studio is removed entirely. The project will use OpenRouter.
- The Python STT service is removed. The project will use `whisper-server` from
  whisper.cpp with the local GGML model `ggml-small-q5_1.bin`.

## 2. Intent and Why

The owner wants a bot farm that runs without a locally managed model server, uses free
OpenRouter capacity first (`openrouter/free`), and falls back to exactly one paid model
(`z-ai/glm-5.3-flash`) when free capacity is unavailable. The owner has donated roughly
$20 to OpenRouter, which puts the account on the 1000 free requests/day tier.

The STT goal is to keep local transcription with the already downloaded Whisper `small`
quantized model, but without maintaining a Python service.

The migration must preserve the existing bot behavior: message generation, debug mode,
rotation, rate limits, and audio context injection. The broader backend rewrite continues
incrementally after this change; this design only swaps the two external integrations.

## 3. Goals

1. Remove all LM Studio code, defaults, configuration, and documentation.
2. Use `openrouter/free` as the only free model and `z-ai/glm-5.3-flash` as the only paid
   fallback model.
3. Let OpenRouter perform per-request fallback (`models` array + `route: "fallback"`).
4. Prevent unbounded spending: paid fallback is enabled by default but always bounded by a
   daily USD cap, visible in status, and switchable off in the UI or by environment
   variable.
5. Use OpenRouter's key endpoint as the authoritative source for free-quota and daily spend.
6. Keep the existing HTTP contract between the Go backend and STT: multipart upload with
   `file`, `language`, and `response_format`, returning `{"text": "..."}`.
7. Replace the Python STT image with the official whisper.cpp image and the local model.
8. Fix the environment-vs-database settings precedence bug that made deployment defaults
   ineffective.

## 4. Non-Goals

- Rewriting `botManager`, storage, or the frontend architecture.
- Streaming LLM responses.
- Per-bot or per-channel model selection.
- Automatic model discovery or a generic multi-provider abstraction.
- Sending real Twitch messages during verification (debug mode is used).
- Multi-language auto-detection tuning beyond the existing `AUDIO_STT_LANGUAGE` setting.

## 5. Anti-Goals

- Paid spend is never silent: paid fallback defaults to enabled with a $1/day cap, every
  paid completion is logged with its model and `cost_usd`, status exposes today's spend,
  and an environment variable can force it off. A paid request may only happen when paid
  fallback is enabled and the daily cap has not been reached (best-effort under
  concurrency; see 9.3).
- Never log or return the API key.
- Never break debug mode or the existing "stop bot after N consecutive errors" behavior.
- Never make the LLM health check itself billable.

## 6. Constraints

- The STT model file is `C:\Users\blindfold\Downloads\ggml-small-q5_1.bin`
  (190,085,487 bytes, multilingual Whisper `small`, q5_1 quantization).
- `whisper-server` does **not** expose OpenAI's `/v1/audio/transcriptions` path by default.
  It uses `/inference`, but the path can be renamed with
  `--inference-path /v1/audio/transcriptions`. Accepted multipart fields include `file`,
  `language`, `response_format` (`json` returns `{"text": ...}`), `temperature`, and
  `no_speech_thold`. Unknown fields such as `model` are ignored.
- The official image `ghcr.io/ggml-org/whisper.cpp:main` was verified to contain
  `whisper-server`, `curl`, and `ffmpeg`, and is reachable from this environment.
- The backend container already produces 16 kHz mono WAV chunks with ffmpeg, so STT
  conversion is not required (`--convert` can stay off).
- OpenRouter chat completions accepts `models: string[]` and `route: "fallback"`.
  The response contains the model actually used and `usage.cost`.
- `GET https://openrouter.ai/api/v1/key` returns `usage_daily` and
  `free_model_daily_requests.{limit,remaining,used}`.
- Go 1.26+ toolchain, PostgreSQL, Redis, React 19 frontend remain unchanged.

## 7. Current State

### LLM

- `internal/llm/lmstudio.go` implements a `Client` with `NewClient`, `Generate`,
  `HealthCheck`, `GetProvider`, `GetModel`, and `GetURL`.
- `NewClient` starts `autoInitializeLMStudio` in a goroutine, which probes `/models` and
  attempts to load a model. This must be deleted.
- `Generate` retries three times with 1s/2s backoff and does not inspect usage or cost.
- `HealthCheck` performs a real generation request, which would be billable against
  OpenRouter. It is called by `/api/health` and `/api/llm/status`.
- `logic.go` builds a client from env at startup, then `ApplySettings` rebuilds it from the
  database row, so stale database values override env.

### Settings

- `settings` has `llm_provider`, `llm_api_url`, `llm_model`, and `llm_api_key`.
- Table defaults are `lmstudio`, `http://127.0.0.1:1234/v1`, and `''`.
- `GetSettings` returns the API key in plaintext.

### STT

- `stt_server/app.py` loads `faster-whisper`, exposes `/health` and
  `/v1/audio/transcriptions`.
- `internal/audio/service.go` posts multipart form data with `model`, `language`, `file`,
  and parses `{"text": "..."}`.
- `docker-compose.yml` builds `stt_server` and stores a Hugging Face cache volume.

## 8. Approaches Considered

### 8.1 LLM routing

**A. Native OpenRouter fallback plus app-level guards (chosen).** Send one request with
`models: ["openrouter/free", "z-ai/glm-5.3-flash"]` and `route: "fallback"`. The client
reads `/key` to know whether free quota remains and how much was spent today, and removes
the paid model from the request when fallback is disabled or the cap is reached. On a
response that `usage.cost` identifies as paid, the client puts the free model on a short
cooldown so subsequent requests do not pay the fallback latency on every call.

- Pros: one request, provider handles rate limits and outages, minimal code, official
  mechanism, exact cost available in the response.
- Cons: fallback also triggers on moderation refusals, so a benign-but-refused free
  request can incur a paid call. The daily cap bounds this.

**B. App-level orchestration.** Make one request per model, handle 429/503 and moderation
locally, and keep per-model circuit breakers.

- Pros: full control and per-model metrics.
- Cons: more code, extra round trips on failure, reimplements a supported feature.

**C. Free-only.** Use only `openrouter/free` and fail when the daily quota is exhausted.

- Pros: no spend at all.
- Cons: the farm stops generating for the rest of the day once 1000 requests are used.
  The owner explicitly wants a paid fallback.

Decision: **A**.

### 8.2 STT deployment

**A. Docker sidecar with the official image (chosen).** Add the `stt` service using
`ghcr.io/ggml-org/whisper.cpp:main`, mount `./models` read-only, and keep the existing
`audio` profile.

- Pros: no build, reproducible, fits the existing Compose workflow, the backend already
  points at `stt:9000`.
- Cons: pulls an external image from ghcr.io; the image is CPU-only.

**B. Host process.** Run a Windows `whisper-server.exe` on the host and point the backend
at `host.docker.internal:9000`.

- Pros: no image pull; easy GPU use.
- Cons: manual setup, not reproducible, Compose cannot manage it.

**C. Build a custom image.** Keep a Dockerfile that compiles whisper.cpp.

- Pros: controls compiler flags.
- Cons: slow builds, no functional benefit for this CPU-only use case.

Decision: **A**.

## 9. Design

### 9.1 Settings model and migration

Reuse existing columns where the meaning still fits:

| Column | New meaning | Default |
|---|---|---|
| `llm_provider` | always `openrouter` | `openrouter` |
| `llm_api_url` | OpenRouter base URL | `https://openrouter.ai/api/v1` |
| `llm_model` | free model ID | `openrouter/free` |
| `llm_api_key` | database fallback key | `''` |

Add columns:

| Column | Meaning | Default |
|---|---|---|
| `llm_paid_model` | paid fallback model ID | `z-ai/glm-5.3-flash` |
| `llm_paid_enabled` | allow paid fallback | `TRUE` |
| `llm_paid_daily_cap_usd` | daily USD cap for paid calls; `<= 0` means unlimited | `1.0` |

`EnsureSettingsTable` runs idempotent DDL:

```sql
ALTER TABLE settings ADD COLUMN IF NOT EXISTS llm_paid_model TEXT NOT NULL DEFAULT 'z-ai/glm-5.3-flash';
ALTER TABLE settings ADD COLUMN IF NOT EXISTS llm_paid_enabled BOOLEAN NOT NULL DEFAULT TRUE;
ALTER TABLE settings ADD COLUMN IF NOT EXISTS llm_paid_daily_cap_usd DOUBLE PRECISION NOT NULL DEFAULT 1.0;

UPDATE settings
SET llm_provider = 'openrouter',
    llm_api_url = 'https://openrouter.ai/api/v1',
    llm_model = 'openrouter/free'
WHERE llm_provider = 'lmstudio'
   OR llm_api_url LIKE '%:1234%'
   OR llm_model = ''
   OR llm_model = 'local-model';
```

The `CREATE TABLE IF NOT EXISTS` defaults change to the new values as well. The migration
runs before `GetSettings`, so the first load already sees OpenRouter settings.

Precedence rules:

| Setting | Precedence |
|---|---|
| API key | `OPENROUTER_API_KEY` env > `llm_api_key` DB > legacy `LLM_API_KEY` env |
| Provider, URL, free model, paid model | DB non-empty > env > built-in default |
| `llm_paid_enabled`, `llm_paid_daily_cap_usd` | env when present and non-empty > DB > default |

Rationale:

- A rotated `OPENROUTER_API_KEY` must always beat a stale database value. `LLM_API_KEY` is
  the old variable name and is kept only as a last-resort fallback for existing
  deployments.
- Provider, URL, and model IDs are UI-managed, so the database is authoritative once a
  value exists. Env values fill empty columns and seed a brand-new row.
- Paid enable and daily cap are deployment safety controls. When `LLM_PAID_ENABLED` or
  `LLM_PAID_DAILY_CAP_USD` is explicitly set in the environment, it wins over the UI. This
  gives the operator a kill switch that a stale UI value cannot override. When the
  variable is absent or empty, the UI/database value is effective.
- `ApplySettings` must stop rebuilding the client with only database values. It resolves
  env + database into one `llm.Config` and applies it to the existing client.

`Settings` gains `LLMPaidModel`, `LLMPaidEnabled`, and `LLMPaidDailyCapUSD` with matching
JSON tags. It also gains read-only computed fields:
`llm_paid_enabled_effective`, `llm_paid_daily_cap_usd_effective`,
`llm_api_key_set` (true when any resolved key exists), and `llm_api_key_source`
(`env`, `database`, or `none`). `GetSettings`/`UpdateSettings` include the stored new
columns; updates to `llm_paid_enabled`/`llm_paid_daily_cap_usd` are accepted but reported
as overridden when the corresponding env variable is set. The API key is masked:
`GetSettings` always returns `llm_api_key: ""`; an empty incoming key means "keep the
stored key". Because the key is never returned, the UI cannot clear a database key by
submitting empty; clearing is an explicit future action, not part of this change. Masking
is an API-boundary concern only: server-side config resolution reads the stored column
directly and never round-trips the key through the settings JSON response.

`EnsureSettingsTable` keeps the column defaults at `TRUE` / `1.0` on purpose: the owner
explicitly asked for a paid fallback, and a $1/day cap makes the default spend bounded and
observable. This is not "opt-in" in the strict sense; it is "on, capped, and switchable
off".

### 9.2 OpenRouter client

`internal/llm/lmstudio.go` is replaced by `internal/llm/openrouter.go`. The package stays
`llm` so the rest of the codebase keeps its import.

```go
type Config struct {
    BaseURL          string
    APIKey           string
    FreeModel        string
    PaidModel        string
    PaidEnabled      bool
    PaidDailyCapUSD  float64
    Referer          string
    Title            string
    HTTPClient       *http.Client
}

func NewClient(cfg Config) *Client
func (c *Client) Reconfigure(cfg Config) // swaps config in place; keeps caches/state
func (c *Client) Generate(ctx context.Context, systemPrompt, userPrompt string, maxTokens int) (string, error)
func (c *Client) HealthCheck(ctx context.Context) error
func (c *Client) Status(ctx context.Context) Status
func (c *Client) GetProvider() string
func (c *Client) GetModel() string // free model, kept for existing callers
func (c *Client) GetURL() string
```

`Reconfigure` is the only way settings are applied after startup. `logic.go` keeps one
`*llm.Client` for the lifetime of the process and calls `Reconfigure` from
`ApplySettings`. `/api/health` and `/api/llm/status` therefore observe live settings
instead of a client captured at startup. `Reconfigure` takes the same mutex used by
`Generate` and `Status`, so a request either sees the old config or the new one, never a
half-updated mix.

Request headers:

- `Authorization: Bearer <key>`
- `Content-Type: application/json`
- `HTTP-Referer: <Referer>` (default `http://localhost:3000`)
- `X-OpenRouter-Title: <Title>` (default `Twitch SMM Bot Farm`)

Request bodies:

- Free only, or paid disabled/capped and free available:
  ```json
  {"model": "openrouter/free", "messages": [...], "temperature": 0.8, "max_tokens": 60, "stream": false}
  ```
- Free then paid:
  ```json
  {"models": ["openrouter/free", "z-ai/glm-5.3-flash"], "route": "fallback", "messages": [...], "temperature": 0.8, "max_tokens": 60, "stream": false}
  ```
- Paid only (free quota exhausted):
  ```json
  {"model": "z-ai/glm-5.3-flash", "messages": [...], "temperature": 0.8, "max_tokens": 60, "stream": false}
  ```

Response parsing: take `choices[0].message.content`; if `error` is present, return it.
Read `model`, `usage.cost`, `usage.prompt_tokens`, and `usage.completion_tokens`.

Free-vs-paid detection must **not** compare `model` with the literal string
`openrouter/free`. `openrouter/free` is a router: the response `model` is the concrete
resolved free model (for example `upstage/solar-pro-3:free`). The rule is:

```text
paidServed = usage.cost > 0 || model == cfg.PaidModel
freeServed = !paidServed
```

`usage.cost > 0` is authoritative because OpenRouter reports the actual charge. The
`model == PaidModel` check is a belt-and-braces fallback for responses that omit `usage`.
A `:free` suffix alone is not used, because it does not prove the request was free and the
paid model could be promoted later.

Empty or missing completions are treated as errors and are **not retried**:
`choices` empty, `content` empty after trimming, or
`finish_reason == "content_filter"` all return an error such as
`openrouter: empty completion (model=..., cost_usd=...)`. This avoids paying twice for a
request the provider already completed.

Log one structured line per successful call:

```text
level=INFO msg="llm completion" requested="openrouter/free" model="z-ai/glm-5.3-flash" free=false cost_usd=0.0000159 prompt_tokens=420 completion_tokens=28
```

Retries: at most two attempts, only for transport errors and HTTP 5xx, with a 500 ms pause
between attempts and respect for `ctx`. No retry on 400/401/402/429, because the native
fallback already handled per-model failures and a retry could double-charge.

`HealthCheck` performs `GET {baseURL}/key` and caches the result (and the parsed key data)
for 30 seconds. It never calls `chat/completions`, so health checks stay free. The response
is the OpenRouter `{"data": {...}}` envelope; the client reads `data.usage_daily` and
`data.free_model_daily_requests.{limit,remaining,used}`. If no API key is configured, it
returns `OpenRouter API key is not configured (set OPENROUTER_API_KEY)`.

### 9.3 Routing, limits, and cost

The client keeps a mutex-protected cache of key information and local spend:

```go
type keyInfo struct {
    UsageDaily    float64
    FreeLimit     int
    FreeRemaining int
    FreeUsed      int
    FetchedAt     time.Time
    OK            bool
}

type budgetState struct {
    localDailyCost   float64 // actual spend observed from usage.cost
    reservedUSD      float64 // worst-case reservations for in-flight paid-eligible calls
    inflightPaid     int
    freeBlockedUntil time.Time
    freeBackoff      time.Duration
    day              string // UTC date for localDailyCost
}
```

Because `/key` can lag behind the response that just completed, the client adds
`usage.cost` to `localDailyCost` immediately and uses the larger of the two:

```text
spentToday   = max(localDailyCost, keyInfo.UsageDaily) + reservedUSD
paidAllowed  = effectivePaidEnabled && (capUSD <= 0 || spentToday < capUSD)
freeAvailable = keyInfo unknown || keyInfo.FreeRemaining > 0
cooldownActive = now < freeBlockedUntil
```

Reservations make the cap tighter under concurrency. Before dispatching a request that
could fall back to paid, the client adds a conservative reservation of `$0.0005` (roughly
an order of magnitude above the observed cost of a `z-ai/glm-5.3-flash` bot message) and
increments `inflightPaid`. When the response arrives, it replaces the reservation with the actual
`usage.cost`. This is still **best-effort**: the cap can be exceeded by at most the
in-flight reservations that were issued before spend was observed. It cannot be exceeded
unboundedly, because every dispatch re-checks `spentToday`. The estimate is a constant in
the client and is documented in a comment; it does not need pricing lookup.

Routing for each `Generate` call:

```text
if freeAvailable && (!cooldownActive || !paidAllowed):
    if paidAllowed: free-first with paid fallback
    else:           free only
else if paidAllowed:
    paid only
else:
    return LLM budget exhausted: free daily limit reached and paid fallback is disabled or capped
```

Consequences:

- The cooldown only suppresses free when paid is actually usable. If paid is disabled or
  the cap is reached, the client keeps trying free rather than failing or skipping it.
- Paid-only is used when free quota is known to be zero, or when the cooldown is active
  and paid is usable.
- If `/key` is unavailable, the client assumes free is available and falls back to
  `localDailyCost` for the cap. It never assumes paid is the only option.

`localDailyCost` accumulates `usage.cost` in memory and resets when the UTC date changes.
`usage_daily` from `/key` is authoritative for spend outside this process.

OpenRouter also enforces a free-tier rate limit (20 requests/minute). Native fallback
triggers on 429, so a burst above that rate consumes paid capacity even while daily free
quota remains. The `$1/day` cap bounds that spend. This change does not add a local free
token bucket; the existing per-bot message delays are expected to stay below the burst
limit. A token bucket is the natural follow-up if the farm grows.

Cooldown: when a response is `paidServed` while free was requested, set
`freeBlockedUntil = now + cooldown`, starting at 60 seconds and doubling on consecutive
fallbacks up to 15 minutes. A `freeServed` response resets the cooldown and backoff. This
avoids paying fallback latency on every message after free quota is exhausted.

`Status` distinguishes authentication/connectivity from the ability to generate:

- `available` = `GET /key` succeeded; the URL is reachable and the key is valid.
- `generation_available` = at least one path can generate right now (free quota remains,
  or paid is allowed).

`Status` exposes at least:

```go
type Status struct {
    Available             bool
    GenerationAvailable   bool
    Provider              string
    FreeModel             string
    PaidModel             string
    PaidEnabled           bool // effective value after env override
    PaidDailyCapUSD       float64 // effective value after env override
    PaidSpentTodayUSD     float64
    FreeRequestsLimit     int
    FreeRequestsRemaining int
    FreeRequestsUsed      int
    FreeCooldownUntil     time.Time
    InflightPaidRequests  int
    LastError             string
    CheckedAt             time.Time
}
```

### 9.4 API and UI

`GetLLMStatus` returns the `Status` fields plus `url`, `model` (free model), and `error`.
Existing UI behavior (`available`, `provider`, `model`, `url`) is preserved. The settings
page "test connection" succeeds when `available` is true, and also displays
`generation_available` plus the reason when it is false. The dashboard uses
`generation_available` to decide whether the farm can currently post.

`logic.go` changes:

- Remove `LMSTUDIO_URL`/`LMSTUDIO_MODEL` fallbacks and the "LM Studio must be running"
  messages.
- Build `llm.Config` from env, merge the database row using the 9.1 precedence table, and
  call `llmClient.Reconfigure(cfg)` from `ApplySettings`. Do not swap `bm.llmClient`.
- `ApplySettings` logs the resolved free/paid models, whether paid fallback is effectively
  enabled, and the effective cap. It does not log the key.

Frontend changes:

- `frontend/src/api/client.ts`: extend the `Settings` interface with the three new stored
  fields (`llm_paid_model`, `llm_paid_enabled`, `llm_paid_daily_cap_usd`), the computed
  fields (`llm_paid_enabled_effective`, `llm_paid_daily_cap_usd_effective`,
  `llm_api_key_set`, `llm_api_key_source`), and extend the LLM status type with
  `generation_available`, `paid_spent_today_usd`, `free_requests_remaining`,
  `free_cooldown_until`, and `inflight_paid_requests`.
- `frontend/src/pages/Settings.tsx`:
  - Defaults and reset behavior use OpenRouter (`https://openrouter.ai/api/v1`,
    `openrouter/free`, `z-ai/glm-5.3-flash`).
  - Provider select has one option: `OpenRouter`.
  - "МОДЕЛЬ" becomes "FREE MODEL"; API key field is enabled with placeholder
    `sk-or-v1-...` and stays empty when a key is already stored. A badge shows whether the
    effective key comes from the environment, the database, or is missing.
  - New fields: paid model, paid fallback toggle, daily cap in USD. When
    `llm_paid_enabled_effective` / `llm_paid_daily_cap_usd_effective` differ from the stored
    values, the field is marked as overridden by the environment and is read-only.
  - Status area shows today's paid spend, free requests remaining, and a cooldown note when
    free is temporarily skipped.
  - The LM Studio instruction block is replaced with an OpenRouter explanation and a
    note that the paid cap of `0` or less means unlimited.
- The connection test keeps calling `/api/llm/status`.

### 9.5 STT with whisper.cpp

Delete `stt_server/` (`app.py`, `Dockerfile`, `requirements.txt`).

Model placement: create `./models/` in the repository, copy
`ggml-small-q5_1.bin` into it, and add `models/` to `.gitignore`. The model is not
committed.

Compose service:

```yaml
stt:
  profiles: ["audio"]
  image: ghcr.io/ggml-org/whisper.cpp:main
  container_name: twitch-smm-stt
  restart: unless-stopped
  command:
    - whisper-server
    - --host
    - 0.0.0.0
    - --port
    - "9000"
    - -m
    - /models/${STT_MODEL_FILE:-ggml-small-q5_1.bin}
    - -t
    - ${STT_THREADS:-4}
    - -l
    - ${STT_LANGUAGE:-ru}
    - --inference-path
    - /v1/audio/transcriptions
  ports:
    - "127.0.0.1:9000:9000"
  volumes:
    - ./models:/models:ro
  healthcheck:
    test: ["CMD", "curl", "-fsS", "http://127.0.0.1:9000/health"]
    interval: 15s
    timeout: 5s
    retries: 10
    start_period: 60s
```

Notes:

- The `language` multipart field sent by the backend overrides `-l` per request.
- The default `response_format` in `whisper-server` is `json`, which returns
  `{"text": "..."}`. The Go client will send `response_format=json` explicitly for
  clarity.
- `--convert` stays off because the backend already sends 16 kHz mono WAV. Enabling it
  forces an ffmpeg conversion for every chunk.
- The `whisper_cache` volume is removed.
- The backend keeps `AUDIO_STT_URL=http://stt:9000/v1/audio/transcriptions`; only the
  service behind it changes.

`internal/audio/service.go` changes: add
`writer.WriteField("response_format", "json")`. Keep the `model` field because
whisper-server ignores unknown fields and the field keeps the request OpenAI-shaped.

### 9.6 Compose and environment

Backend environment in `docker-compose.yml`:

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

The empty default for the two paid variables is deliberate: Compose still passes them, but
the Go code treats an empty value as "not set", so the database/UI value is used. A
non-empty `LLM_PAID_ENABLED=false` or `LLM_PAID_DAILY_CAP_USD=0.25` is an operator
kill-switch/override that wins over the UI.

`.env.example` documents `LLM_PAID_ENABLED` and `LLM_PAID_DAILY_CAP_USD` as optional
overrides (commented out), and is rewritten for OpenRouter, whisper.cpp, and the new STT
variables (`STT_MODEL_FILE`, `STT_THREADS`, `STT_LANGUAGE`). The file currently has broken
CP1251/UTF-8 encoding; it will be rewritten as UTF-8.

Documentation to update: root `README.md` and `docs/AGENTS.md` (both currently reference
LM Studio).

### 9.7 Files touched

- `internal/llm/lmstudio.go` → deleted; `internal/llm/openrouter.go` → added.
- `internal/llm/openrouter_test.go` → added (routing, request bodies, response parsing,
  paid detection via `usage.cost`, empty-completion errors, cost/reservation accumulation,
  cooldown, `Reconfigure`).
- `internal/storage/settings.go` → new fields, new defaults, migration DDL.
- `internal/api/handlers.go` → `GetLLMStatus`, settings masking, change log entries.
- `logic.go` → env/config wiring, `ApplySettings`, remove LM Studio messages.
- `internal/audio/service.go` → `response_format=json`.
- `internal/audio/service_test.go` → request shape test (added if absent).
- `docker-compose.yml` → LLM env, `stt` service, model volume.
- `.env.example`, `.gitignore`, `README.md`, `docs/AGENTS.md`.
- `frontend/src/api/client.ts`, `frontend/src/pages/Settings.tsx`.
- `stt_server/` → deleted.
- `models/ggml-small-q5_1.bin` → local file, gitignored.

## 10. Verification

1. `go build ./...` and `go test ./...` pass.
2. `docker compose up -d --build --remove-orphans --wait` brings all default services to
   healthy.
3. Without a key, `/api/health` reports the LLM as unavailable with the clear
   `OpenRouter API key is not configured (set OPENROUTER_API_KEY)` message; the rest of the
   system stays healthy. `/api/llm/status` shows `available=false` and
   `generation_available=false`, and `/api/settings` shows `llm_api_key_source=none`.
4. With a key:
   - `/api/llm/status` shows the free/paid models, `free_requests_remaining`,
     `paid_spent_today_usd`, `generation_available`, and any active cooldown.
   - Settings shows `llm_api_key_source=env` when the key comes from the environment.
   - The settings page "test connection" succeeds.
   - A debug-mode bot generates a message; logs show the resolved concrete model,
     `free=true`, and `cost_usd=0` while free quota remains.
   - Edit the paid toggle/cap in the UI and confirm `/api/llm/status` reflects the change
     without restarting the backend (proves `Reconfigure` works).
   - With `LLM_PAID_ENABLED=false` in the environment, confirm the UI toggle is reported as
     overridden and no paid model is requested.
5. STT:
   - `docker compose --profile audio up -d --wait` brings `twitch-smm-stt` to healthy.
   - `curl -F file=@sample.wav -F response_format=json -F language=ru
     http://127.0.0.1:9000/v1/audio/transcriptions` returns `{"text": "..."}`.
   - Enable `audio_enabled` in the UI and confirm transcription appears in bot context.
6. Debug mode stays on for all manual runs so no real Twitch messages are sent.
7. Free-quota and cap behavior are verified with unit tests and, if possible, by pointing
   the client at a stub HTTP server rather than by spending real money.

## 11. Rollout and Rollback

Rollout: one commit for the LLM migration and one for STT, so either can be reverted
independently. The database migration is additive; rolling back the binary leaves the new
columns unused. The old `llm_provider` value is overwritten, so a rollback to LM Studio
requires restoring settings manually; this is acceptable because LM Studio is being
removed.

## 12. Left to the Builder

- Exact placement of the "resolve env + database settings" helper (storage vs. `logic.go`).
- Whether `Status` should include `usage` breakdowns beyond cost and tokens.
- Thread count default for whisper.cpp on this machine (`4` is the starting point).
- Whether to expose the new paid-budget fields in the Dashboard status card or only in
  Settings.
- Test file organization and whether to use `httptest` or an interface for the OpenRouter
  HTTP client.
