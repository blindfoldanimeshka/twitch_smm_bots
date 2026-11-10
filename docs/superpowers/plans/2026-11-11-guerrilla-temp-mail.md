# Guerrilla Temp Mail Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Встроить временную почту через публичный Guerrilla Mail API в Go-бэкенд: создание ящика, список писем, чтение текста — без домена, Cloudflare и внешних контейнеров.

**Architecture:** `internal/guerrilla` — тонкий клиент `api.guerrillamail.com/ajax.php`. `internal/tempmail` — сервис: создаёт ящик, хранит `sid_token` в Redis с TTL 1 час, отдаёт наружу только свой id. API-эндпоинты под `X-API-Key`. UI — карточка в Settings, без пятой страницы.

**Tech Stack:** Go 1.26+, `net/http`, `encoding/json`, Redis (существующий `internal/cache`), React 19 + TS.

**Spec:** решение пользователя: Guerrilla вместо TempFastMail без внешних шагов.

**Testing note:** фронтенд-раннера нет — гейт `npm.cmd run build`. Backend: httptest для Guerrilla и API, fake-cache для сервиса.

## Global Constraints

- Новых зависимостей не добавлять.
- `sid_token` никогда не покидает бэкенд и не логируется.
- Тело письма — чужой HTML: рендерить как HTML нельзя, только текст (вырезать теги на бэкенде).
- Ящик живёт 1 час (Guerrilla удаляет письма через час); TTL Redis тот же.
- Все новые маршруты — под существующим `X-API-Key`.
- Никакой автоматизации регистрации/обхода верификации сторонних сервисов.

## Review Focus

- `sid_token` не попадает в ответы, ошибки и логи.
- Истёкшая сессия даёт 404, а не панику и не 500.
- Тело письма безопасно: HTML вырезан, скрипты не исполняются.
- Сбой Guerrilla (429/5xx/таймаут) даёт 502 с понятным текстом.
- UI не показывает пустой список как «писем нет», если запрос упал.

---

### Task 1: Клиент Guerrilla Mail

**Files:**
- Create: `internal/guerrilla/client.go`, `internal/guerrilla/client_test.go`

**Interfaces:**
- `NewClient(baseURL string, httpClient *http.Client) *Client`; default base `https://api.guerrillamail.com`, 10s timeout.
- `Create(ctx, alias string) (*Inbox, error)` — `get_email_address`, затем при непустом alias `set_email_user`.
- `List(ctx, sid string) ([]Message, error)` — `get_email_list&offset=0`.
- `Fetch(ctx, sid, mailID string) (*Message, error)` — `fetch_email`.
- `Delete(ctx, sid, mailID string) error` — `del_email`.
- Types: `Inbox{Email, SID string}`; `Message{ID, From, Subject, Excerpt, Body string; ReceivedAt time.Time}`.

- [ ] **Step 1: failing tests** (httptest): create returns email+sid; alias sends `set_email_user`; list parses `mail_id/mail_from/mail_subject/mail_excerpt/mail_timestamp`; fetch parses `mail_body`; non-2xx returns an error without the sid or body.
- [ ] **Step 2: run** `go test ./internal/guerrilla/ -v` → FAIL.
- [ ] **Step 3: implement** per interfaces; `mail_timestamp` unix seconds; missing/invalid timestamps become zero; body capped at 256 KiB.
- [ ] **Step 4: run tests → PASS.**
- [ ] **Step 5: commit** `feat(mail): add Guerrilla Mail API client`.

---

### Task 2: Сервис и API временной почты

**Files:**
- Create: `internal/tempmail/service.go`, `service_test.go`
- Modify: `internal/cache/tempmail.go` (new file)
- Modify: `internal/api/handlers.go`, `server.go`, `dto.go`, `handlers_test.go`
- Modify: `logic.go` (создание сервиса и проброс)

**Interfaces:**
- `tempmail.New(cache Cache, client *guerrilla.Client) *Service`.
- `Cache` interface: `SetTempMailSession(ctx, id string, data []byte, ttl time.Duration) error`, `GetTempMailSession(ctx, id string) ([]byte, error)`, `DeleteTempMailSession(ctx, id string) error`.
- `(*Service) Create(ctx, alias string) (*Inbox, error)`; `Messages(ctx, id string) (*Inbox, error)`; `Message(ctx, id, mailID string) (*Message, error)`; `Delete(ctx, id string) error`.
- DTO: `TempMailInbox{ID, Email string; ExpiresAt time.Time; Messages []TempMailMessage}`; `TempMailMessage{ID, From, Subject, Excerpt, ReceivedAt}`; `TempMailMessageBody{ID, From, Subject, Body, ReceivedAt}`.
- Routes: `POST /api/temp-mail/inbox`, `GET /api/temp-mail/inbox/{id}`, `GET /api/temp-mail/inbox/{id}/messages/{mailID}`, `DELETE /api/temp-mail/inbox/{id}`.

- [ ] **Step 1: failing service tests** (fake cache + httptest): Create stores session with 1h TTL and returns a random id (not the sid); Messages on a missing/expired key returns `ErrInboxNotFound`; Message strips HTML (script/style/tags removed, entities unescaped); Delete removes the key.
- [ ] **Step 2: failing handler tests**: create 200 and body contains `email` but not the sid; unknown id 404; provider failure 502; service nil 503; delete 200.
- [ ] **Step 3: run** focused tests → FAIL.
- [ ] **Step 4: implement** cache methods (`tempmail:<id>`, `SETEX`), service, handlers, routes, DTO mapping, wiring in `main` (service always on; no feature flag).
- [ ] **Step 5: run** `go build ./...`, `go vet ./...`, `go test ./...` → PASS.
- [ ] **Step 6: commit** `feat(mail): expose temporary inboxes via the Guerrilla API`.

---

### Task 3: UI временной почты

**Files:**
- Modify: `frontend/src/api/client.ts`
- Modify: `frontend/src/pages/Settings.tsx`
- Modify: `frontend/src/components/ui/Modal.tsx` (reuse only, no edit unless needed)

**Interfaces:**
- Client methods: `createTempInbox(alias?)`, `getTempInbox(id)`, `getTempMessage(id, mailID)`, `deleteTempInbox(id)`.
- Card replaces the old external link: «Создать ящик», адрес + копирование, «Обновить», список писем, клик → `Modal` с текстом, «Удалить».
- Inbox id persists in `localStorage` with `expires_at`; expired id is discarded.

- [ ] **Step 1: API client** types/methods.
- [ ] **Step 2: card** with states: no inbox / creating / active / provider error; empty list only when the request succeeded.
- [ ] **Step 3: message modal** renders text in `<pre>`, never HTML.
- [ ] **Step 4: gate** `cd frontend && npm.cmd run build`.
- [ ] **Step 5: commit** `feat(ui): add an in-app temporary inbox`.

---

### Task 4: TempFastMail в профиль и документация

**Files:**
- Modify: `docker-compose.yml`, `.env.example`, `frontend/Dockerfile`, `docs/TEMPORARY_MAIL.md`, `docs/CODE_GRAPH.md`, `docs/code-graph.json`

**Interfaces:**
- `tempfastmail` service gets `profiles: ["mail"]`; default `docker compose up` does not start it.
- Remove `VITE_TEMP_MAIL_URL` from compose build args, Dockerfile ARG/ENV and `.env.example`.
- Docs: Guerrilla inbox (1h, receive-only, shared domains, third-party sees content, no bypass), TempFastMail optional under `--profile mail`.
- Code graph invariants: sid never leaves backend; HTML stripped; 1h TTL.

- [ ] **Step 1: compose/profile/env/Dockerfile** cleanup.
- [ ] **Step 2: docs rewrite**.
- [ ] **Step 3: gates** `docker compose config --quiet`, `go test ./...`, `npm.cmd run build`.
- [ ] **Step 4: commit** `docs(mail): document the Guerrilla inbox and profile TempFastMail`.

---

## Self-Review

- **Spec coverage:** клиент (Task 1), хранение/API (Task 2), UI (Task 3), конфигурация/документация (Task 4).
- **Type consistency:** `Inbox`/`Message` совпадают между Task 1 и 2; DTO — между Task 2 и 3.
- **Review Focus:** пять пунктов закрыты тестами/шагами.
- **Proportion:** план короче кода; тела функций описаны требованиями.
