# Twitch Token Import Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Добавить ручной импорт OAuth-токенов из TwitchTokenGenerator: проверка, шифрованное хранение refresh-токена, автообновление перед запуском бота и статус токена в UI — без регистрации собственного Twitch-приложения.

**Architecture:** Новый пакет `internal/twitchtoken` инкапсулирует API генератора (`/api/v2/tokens/validate`, `/api/v2/tokens/refresh`). Новый пакет `internal/tokencrypt` шифрует refresh-токены AES-256-GCM ключом из `TOKEN_ENCRYPTION_KEY`. `internal/bottoken` связывает хранилище, шифр и клиент. Access-токен живёт в существующей колонке `bots.password` в формате `oauth:<token>`; refresh-токен — в новой колонке в зашифрованном виде.

**Tech Stack:** Go 1.26+, pgx/v5, стандартный `crypto/aes`+`cipher.GCM`, React 19 + TypeScript.

**Spec:** итог разговора: ручной импорт токенов из twitchtokengenerator, письма отложены.

**Testing note:** фронтенд-раннера нет — гейт `npm.cmd run build`. Backend — `go test ./...`, httptest для внешнего API, fake-хранилище для сервиса.

## Global Constraints

- Новых зависимостей не добавлять.
- Токены никогда не логируются, не возвращаются API и не попадают в ошибки.
- Refresh-токен хранится только в зашифрованном виде; без `TOKEN_ENCRYPTION_KEY` refresh-импорт отклоняется с понятной ошибкой, access-импорт работает.
- Access-токен нормализуется к `oauth:<token>` (как ждёт IRC `PASS`).
- Один импорт меняет только одного бота; логин из токена обязан совпадать с `twitch_login` бота (регистр не важен).
- Работа на `main`.

## Review Focus

- Refresh-токен не утекает ни в JSON, ни в логи, ни в сообщения об ошибках.
- Обновлённый refresh-токен сохраняется до использования нового access-токена; при сбое записи бот не остаётся с недействительной парой.
- Импорт токена чужого аккаунта отклоняется 409, а не молча привязывается к боту.
- Бот с истёкшим токеном не запускается молча: либо обновление, либо внятная ошибка.
- `TOKEN_ENCRYPTION_KEY` не коммитится; в примерах только placeholder.

---

### Task 1: Шифр и клиент генератора

**Files:**
- Create: `internal/tokencrypt/cipher.go`
- Create: `internal/tokencrypt/cipher_test.go`
- Create: `internal/twitchtoken/client.go`
- Create: `internal/twitchtoken/client_test.go`

**Interfaces:**
- Produces: `tokencrypt.NewCipher(keyBase64 string) (*Cipher, error)`; `(*Cipher) Encrypt(plain string) (string, error)`; `(*Cipher) Decrypt(enc string) (string, error)`.
- Produces: `twitchtoken.NewClient(baseURL string, httpClient *http.Client) *Client`; `(*Client) Validate(ctx context.Context, accessToken string) (*Validation, error)`; `(*Client) Refresh(ctx context.Context, refreshToken string) (*Tokens, error)`.
- Types: `Validation{Username, UserID, ClientID string; ExpiresIn int; Scopes []string}`; `Tokens{AccessToken, RefreshToken, ClientID string; ExpiresIn int; Scopes []string}`.

- [ ] **Step 1: Write failing tests**

`cipher_test.go`: round-trip; wrong key fails; tampered ciphertext fails; empty key returns error.
`client_test.go` (httptest): validate sends `{"access_token": ...}` to `/api/v2/tokens/validate` and parses username/expires/scopes; refresh sends only `{"refresh_token": ...}` to `/api/v2/tokens/refresh` and parses the new pair; non-200 returns an error that does **not** contain the token substring.

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/tokencrypt/ ./internal/twitchtoken/ -v`
Expected: FAIL — packages/functions undefined.

- [ ] **Step 3: Implement**

`tokencrypt`: decode base64 key, require exactly 32 bytes; AES-GCM with random nonce; output `base64(nonce||ciphertext)`.
`twitchtoken`: base URL default `https://twitchtokengenerator.com`, 10s timeout; JSON POST; map non-2xx to an error with status only; never include request bodies in errors.

- [ ] **Step 4: Run tests**

Run: `go test ./internal/tokencrypt/ ./internal/twitchtoken/ -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/tokencrypt internal/twitchtoken
git commit -m "feat(token): add token encryption and generator API client"
```

---

### Task 2: Колонки токена и запись пары

**Files:**
- Modify: `internal/storage/postgres.go:16-105` (schema + DO migrations), `:116-130` (BotConfig), SELECT/scan/INSERT (`:170`, `:184`, `:225-243`)
- Modify: `internal/api/dto.go` (`BotDTO`)
- Test: `internal/storage/postgres_test.go`

**Interfaces:**
- Produces: `BotConfig` fields `TokenRefreshEnc string`, `TokenExpiresAt *time.Time`, `TokenClientID string`, `TokenScopes []string`.
- Produces: `(*Store) SetBotToken(ctx context.Context, id, accessToken, refreshTokenEnc string, expiresAt time.Time, clientID string, scopes []string) error`.
- Produces: `BotDTO` fields `TokenExpiresAt *time.Time`, `TokenHasRefresh bool`, `TokenScopes []string`, `TokenValid bool` (json: `token_expires_at`, `token_has_refresh`, `token_scopes`, `token_valid`).

- [ ] **Step 1: Write failing tests**

Интеграционный тест (паттерн `settings_migration_test.go`): после `EnsureSchema` колонки существуют; `SetBotToken` записывает access в `password`, зашифрованный refresh и expiry; `GetBot` возвращает все поля; повторный вызов перезаписывает пару.

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/storage/ -run TestBotTokenRoundTrip -v`
Expected: FAIL — колонок и метода нет.

- [ ] **Step 3: Implement**

Добавить `ALTER TABLE bots ADD COLUMN IF NOT EXISTS token_refresh_enc TEXT NOT NULL DEFAULT ''`, `token_expires_at TIMESTAMPTZ`, `token_client_id TEXT NOT NULL DEFAULT ''`, `token_scopes JSONB NOT NULL DEFAULT '[]'`. Расширить `BotConfig`, все SELECT/scan/INSERT. `SetBotToken` обновляет `password = $2, token_refresh_enc = $3, token_expires_at = $4, token_client_id = $5, token_scopes = $6, updated_at = NOW() WHERE id = $1`; проверять `RowsAffected`.

- [ ] **Step 4: Run tests**

Run: `go test ./internal/storage/ -run TestBotTokenRoundTrip -v`
Expected: PASS (или SKIP без Postgres).

- [ ] **Step 5: Commit**

```bash
git add internal/storage/postgres.go internal/storage/postgres_test.go internal/api/dto.go
git commit -m "feat(storage): persist imported Twitch token metadata"
```

---

### Task 3: Сервис токенов, автообновление и API

**Files:**
- Create: `internal/bottoken/service.go`, `service_test.go`
- Modify: `logic.go:327-372` (поле `tokenService`), `:764-821` (`startBot` — `EnsureFresh` перед подключением), wiring в `main`
- Modify: `internal/api/handlers.go` (Handler поле + setter), `internal/api/server.go` (роуты)
- Modify: `internal/api/dto.go`, `internal/api/handlers_test.go`

**Interfaces:**
- Produces: `bottoken.New(store TokenStore, cipher *tokencrypt.Cipher, client *twitchtoken.Client) *Service`.
- `TokenStore` interface: `SetBotToken(...) error`; `GetBot(ctx, id) (*storage.BotConfig, error)`.
- `(*Service) Import(ctx, botID, login, accessToken, refreshToken string) (*Status, error)`.
- `(*Service) EnsureFresh(ctx, cfg storage.BotConfig) (storage.BotConfig, error)`.
- `(*Service) RefreshNow(ctx, botID string) (*Status, error)`.
- `(*Service) Status(cfg storage.BotConfig) Status` — `Valid`, `ExpiresAt *time.Time`, `Scopes []string`, `ClientID string`, `HasRefresh bool`, `Login string`.
- Routes: `POST /api/bots/{id}/token`, `GET /api/bots/{id}/token`, `POST /api/bots/{id}/token/refresh`.

- [ ] **Step 1: Write failing tests**

`service_test.go` with a fake store and httptest TTG:
- Import validates and stores `oauth:<token>`; login mismatch returns a sentinel error;
- Import without cipher rejects a refresh token but accepts access-only;
- EnsureFresh with a token valid for >5 minutes is a no-op; with an expiring token it refreshes, stores the new pair, and returns the new password;
- A store failure after refresh returns an error and does not report success.

Handler tests: import returns 200 and never echoes tokens; invalid token → 400; login mismatch → 409; TTG down → 502; refresh without stored refresh → 400.

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/bottoken/ ./internal/api/ -run 'TestService|TestImportToken|TestRefreshToken' -v`
Expected: FAIL.

- [ ] **Step 3: Implement service**

Import: strip `oauth:`; call `Validate`; compare `strings.EqualFold(validation.Username, login)`; encrypt refresh if present and cipher configured; `SetBotToken`; return status.
EnsureFresh: if `TokenExpiresAt` is nil or > now+5min → return cfg unchanged. Else require refresh + cipher; call `Refresh`; encrypt new refresh; `SetBotToken`; return cfg with `Password = "oauth:"+access`, new refresh, expiry `now+ExpiresIn`, scopes.
Errors are sentinel values (`ErrLoginMismatch`, `ErrNoRefreshToken`, `ErrEncryptionUnavailable`) for handler mapping; никогда не включают токены.

- [ ] **Step 4: Wire the manager**

`botManager.tokenService *bottoken.Service`; in `startBot`, before creating the IRC client: `cfg, err := bm.ensureFreshToken(ctx, br)`; on error return it; assign `br.cfg = cfg` and build the client with the new password. `EnsureFresh` is a no-op when the service is nil (tests and installations without a key).

- [ ] **Step 5: Handlers and routes**

`Handler` gets `tokenService`; `SetTokenService`. `Initialize` call in `main`. DTO mapping fills token status from `BotConfig` + `Status()`. Responses contain only status.

- [ ] **Step 6: Run gates**

Run: `go build ./...`, `go vet ./...`, `go test ./...`
Expected: PASS.

- [ ] **Step 7: Commit**

```bash
git add internal/bottoken internal/api logic.go
git commit -m "feat(bot): import and refresh Twitch tokens before start"
```

---

### Task 4: UI импорта и статуса токена

**Files:**
- Modify: `frontend/src/api/client.ts` (типы + `importBotToken`, `getBotToken`, `refreshBotToken`)
- Modify: `frontend/src/components/BotModal.tsx` (секция «Токен Twitch»)
- Modify: `frontend/src/pages/Bots.tsx` / `components/BotsTable.tsx` (бейдж статуса)
- Modify: `frontend/src/hooks/useBotForm.ts` (не отправлять токены в общем payload)

**Interfaces:**
- Consumes: endpoints Task 3; `Bot` fields `token_expires_at`, `token_has_refresh`, `token_scopes`, `token_valid`.
- Produces: форма с двумя password-полями и кнопкой «Проверить и сохранить»; ошибки — `Alert`; после сохранения поля очищаются, показывается срок действия.

- [ ] **Step 1: API client**

Добавить методы и типы; токены передаются только в этих вызовах.

- [ ] **Step 2: Modal section**

В `BotModal` (режим редактирования) — блок «Токен Twitch»: поля `access_token`, `refresh_token`, кнопка импорта, статус («действует до HH:MM», «истёк», «обновляется автоматически»). Поля не предзаполняются. Ошибки 409/400 показываются рядом.

- [ ] **Step 3: Table badge**

В строке бота — бейдж: `токен: OK` / `истекает` / `нет токена`, с `title` о времени.

- [ ] **Step 4: Guard**

`useBotForm`/`handleSave` не включает токены в обычный update.

- [ ] **Step 5: Verify**

Run: `cd frontend && npm.cmd run build`
Expected: exit 0.

- [ ] **Step 6: Commit**

```bash
git add frontend/src/api/client.ts frontend/src/components/BotModal.tsx frontend/src/pages/Bots.tsx frontend/src/components/BotsTable.tsx frontend/src/hooks/useBotForm.ts
git commit -m "feat(ui): import Twitch tokens and show their status"
```

---

### Task 5: Конфигурация, документация и интеграционный прогон

**Files:**
- Modify: `.env.example`, `docker-compose.yml`
- Modify: `docs/CODE_GRAPH.md`, `docs/code-graph.json`
- Test: `internal/storage/postgres_test.go` (расширить)

**Interfaces:**
- Produces: `TOKEN_ENCRYPTION_KEY` в примере окружения и compose (placeholder, не значение).

- [ ] **Step 1: Env**

Добавить `TOKEN_ENCRYPTION_KEY=` в `.env.example` с комментарием, как сгенерировать (`openssl rand -base64 32`), и пробросить в backend-контейнер.

- [ ] **Step 2: Docs**

Обновить инварианты: refresh-токен только зашифрованным; токены не логируются/не возвращаются; access-токен в `password` как `oauth:`; автообновление за 5 минут до истечения.

- [ ] **Step 3: Full gates**

Run: `go build ./...`, `go vet ./...`, `go test ./...`, `cd frontend && npm.cmd run build`.
Expected: exit 0.

- [ ] **Step 4: Commit**

```bash
git add .env.example docker-compose.yml docs/CODE_GRAPH.md docs/code-graph.json
git commit -m "docs: document token encryption and import flow"
```

---

## Self-Review

- **Spec coverage:** импорт (Task 3), проверка (Task 1), шифрование (Task 1), хранение (Task 2), обновление (Task 3), UI (Task 4), конфигурация/документация (Task 5).
- **Type consistency:** `SetBotToken` объявлен в Task 2 и потребляется в Task 3; `Status`/`Tokens`/`Validation` согласованы между Task 1 и 3.
- **Review Focus:** пять пунктов закрыты тестами/шагами.
- **Proportion:** план короче кода; тела функций описаны требованиями.
