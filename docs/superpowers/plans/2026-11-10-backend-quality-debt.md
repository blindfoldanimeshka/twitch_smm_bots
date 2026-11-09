# Backend Quality Debt Implementation Plan (Package C)

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Закрыть backend-долг: `UpdateBot` возвращает 400 на невалидных данных вместо 500, журнал настроек отражает изменения Twitch-полей без утечки секрета, сохранённый Twitch-секрет можно осознанно очистить, а тесты настроек проверяют реальный round-trip вместо пустых утверждений.

**Architecture:** Валидация в handler-слое поверх текущего состояния бота; маскирование секрета в change-log по образцу `llm_api_key`; явный транзиентный флаг `twitch_client_secret_clear` (не persisted); интеграционные тесты storage с восстановлением строки настроек.

**Tech Stack:** Go 1.26+, pgx/v5, testcontainers не используем — интеграция с локальным Postgres из docker-compose.

**Spec:** итоговые ревью планов A/B и `docs/CODE_GRAPH.md` (инварианты секретов и настроек).

**Testing note:** storage-тесты интеграционные: `postgres://botfarm:botfarm@localhost:5432/botfarm?sslmode=disable`, `t.Skip` если база недоступна. Каждый тест восстанавливает исходную строку `settings` в `defer`.

## Global Constraints

- Новых зависимостей не добавлять.
- Секреты (OAuth-токены, `llm_api_key`, `twitch_client_secret`) никогда не логируются и не попадают в ответы/журнал настроек.
- `POST /api/settings` остаётся полной заменой строки; новые поля — только транзиентные.
- Русский текст ошибок в UI; API-сообщения могут остаться английскими.
- Гейты: `go build ./...`, `go vet ./...`, `go test ./...`; для фронтенда — `cd frontend && npm.cmd run build`.

## Review Focus

- Невалидный `UpdateBot` не доходит до БД и возвращает 400 с понятной причиной.
- Журнал настроек показывает факт смены Twitch-секрета, но нигде не содержит его значение.
- Очистка секрета работает только по явному флагу; пустое поле по-прежнему сохраняет старое значение.
- Интеграционные тесты не оставляют за собой изменённые настройки и не падают при отсутствии Postgres.

---

### Task 1: Валидация `UpdateBot` и Twitch-поля в журнале настроек

**Files:**
- Modify: `internal/api/handlers.go:285-332`
- Modify: `internal/api/handlers.go:682-890` (`calculateSettingsChanges`, `formatSettingsChangeMessage`)
- Test: `internal/api/handlers_test.go`

**Interfaces:**
- Consumes: `BotManager.GetBot(id) (*BotInfo, error)` (`handlers.go:22`), `ValidatePersonality` (`validation.go:55`).
- Produces: `UpdateBot` возвращает 400 на невалидных `personality`/`min_delay_sec`/`max_delay_sec`/`rate_limit_per_min`/пустом `name`; 404 на отсутствующего бота; 500 только на ошибках хранилища.
- Produces: `calculateSettingsChanges` включает `twitch_client_id` (`old`/`new`) и `twitch_client_secret` (`{"changed": true}` без значений); `formatSettingsChangeMessage` добавляет человекочитаемые строки про Twitch.

- [ ] **Step 1: Write the failing tests**

В `internal/api/handlers_test.go` добавить табличный тест `TestUpdateBotRejectsInvalidConfig`:
- кейсы: `min_delay_sec = 0`; `max_delay_sec < min_delay_sec`; `rate_limit_per_min = 0`; `personality = "unknown"`; `name = ""`.
- для каждого: `MockManager.GetBot` возвращает валидный `BotInfo`, запрос `PUT /api/bots/{id}` с одним невалидным полем, ожидание `400`, и `AssertNotCalled(t, "UpdateBot", ...)`.
- отдельный кейс: валидный частичный апдейт (`max_delay_sec` больше текущего min) → `200` и `UpdateBot` вызван.

Добавить `TestCalculateSettingsChangesIncludesTwitchFields`:
- старые настройки с `TwitchClientID="old-id"`, `TwitchClientSecret="old-secret"`;
- новые с `TwitchClientID="new-id"`, `TwitchClientSecret="new-secret"`;
- `changes["twitch_client_id"]` содержит `old`/`new`; `changes["twitch_client_secret"]` равно `map[string]interface{}{"changed": true}`; сериализованный `changes` не содержит подстрок `old-secret`/`new-secret`.
- `formatSettingsChangeMessage(changes)` содержит «Twitch» и не содержит значений секрета.

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/api/ -run 'TestUpdateBotRejectsInvalidConfig|TestCalculateSettingsChangesIncludesTwitchFields' -v`
Expected: FAIL — сейчас невалидные значения уходят в `UpdateBot`, Twitch-полей в changes нет.

- [ ] **Step 3: Implement `UpdateBot` validation**

В `UpdateBot` после декодирования запроса:
1. `info, err := h.manager.GetBot(botID)`; при ошибке — 404.
2. Собрать эффективные значения: взять `info.Config` и применить предоставленные поля.
3. Проверить: `personality` (если задано) через `ValidatePersonality`; `min_delay_sec > 0`; `max_delay_sec >= min_delay_sec`; `rate_limit_per_min > 0`; `name` не пустой, если задан.
4. При ошибке — `http.Error(w, fmt.Sprintf(`{"error":%q}`, err.Error()), http.StatusBadRequest)` и выход до вызова `manager.UpdateBot`.

- [ ] **Step 4: Implement Twitch change log**

В `calculateSettingsChanges` добавить сравнения `TwitchClientID` (старое/новое) и `TwitchClientSecret` (только `{"changed": true}`). В `formatSettingsChangeMessage` добавить строки по образцу существующих, с явной маскировкой: «Twitch Client Secret: изменён».

- [ ] **Step 5: Run tests**

Run: `go test ./internal/api/ -v`
Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add internal/api/handlers.go internal/api/handlers_test.go
git commit -m "fix(api): reject invalid bot updates and log Twitch setting changes"
```

---

### Task 2: Явная очистка сохранённого Twitch-секрета

**Files:**
- Modify: `internal/storage/settings.go:11-43`
- Modify: `internal/api/handlers.go:609-640`
- Test: `internal/api/handlers_test.go`
- Modify: `frontend/src/api/client.ts` (`Settings`)
- Modify: `frontend/src/pages/Settings.tsx` (форма, dirty, `onSuccess`, секрет-поле)

**Interfaces:**
- Consumes: поле `TwitchClientSecretSet` и правило «пустое значение сохраняет старое» из плана A.
- Produces: `storage.Settings.TwitchClientSecretClear bool` с json-тегом `twitch_client_secret_clear`, не persists.
- Produces: `POST /api/settings` с `twitch_client_secret_clear: true` записывает пустой секрет; без флага пустое поле сохраняет сохранённый секрет.

- [ ] **Step 1: Write the failing tests**

Два подтеста в `TestUpdateSettings`:
1. `clears stored twitch secret on explicit flag`: старый секрет `stored-secret`, входящий `TwitchClientSecret=""`, `TwitchClientSecretClear=true`; `MockStore.UpdateSettings` ожидает `TwitchClientSecret == ""` и `TwitchClientSecretSet == false`.
2. `empty secret without clear flag keeps stored value` — уже существует с плана A; убедиться, что он не сломан.

- [ ] **Step 2: Run tests to verify the new one fails**

Run: `go test ./internal/api/ -run TestUpdateSettings -v`
Expected: FAIL — флаг отсутствует, секрет сохраняется.

- [ ] **Step 3: Implement the flag**

Добавить поле в `storage.Settings`. В `UpdateSettings` рядом с обработкой секрета:
```go
if settings.TwitchClientSecretClear {
    settings.TwitchClientSecret = ""
} else if settings.TwitchClientSecret == "" {
    settings.TwitchClientSecret = oldSettings.TwitchClientSecret
}
settings.TwitchClientSecretClear = false
settings.TwitchClientSecretSet = settings.TwitchClientSecret != ""
```
Убедиться, что `UpdateSettings` в storage не пишет это поле (список колонок уже явный).

- [ ] **Step 4: Frontend control**

В `api/client.ts` добавить `twitch_client_secret_clear: boolean` в `Settings`. В `Settings.tsx`:
- добавить поле в начальный `formState` и в `normalizeSettings` (`false`);
- рядом с полем секрета показывать кнопку «Очистить секрет», если `formState.twitch_client_secret_set`;
- по клику: `update('twitch_client_secret', '')` и `update('twitch_client_secret_clear', true)`;
- `dirty` учитывает `formState.twitch_client_secret_clear`;
- `reconcileAfterSave` сбрасывает флаг в `false`;
- baseline после сохранения содержит `twitch_client_secret_clear: false`.

- [ ] **Step 5: Run gates**

Run: `go test ./internal/api/ ./internal/storage/ -v` и `cd frontend && npm.cmd run build`
Expected: PASS, exit 0.

- [ ] **Step 6: Commit**

```bash
git add internal/storage/settings.go internal/api/handlers.go internal/api/handlers_test.go frontend/src/api/client.ts frontend/src/pages/Settings.tsx
git commit -m "feat(settings): allow clearing the stored Twitch client secret"
```

---

### Task 3: Реальные интеграционные тесты настроек

**Files:**
- Rewrite: `internal/storage/settings_test.go`

**Interfaces:**
- Consumes: `Store.GetSettings`, `Store.UpdateSettings`, `Store.EnsureSettingsTable`; DSN и паттерн `t.Skip` из `settings_migration_test.go:12-25`.
- Produces: тесты, которые читают строку `settings`, меняют её, проверяют round-trip и восстанавливают исходные значения в `defer`.

- [ ] **Step 1: Replace the meaningless sqlmock tests**

Удалить `sqlmock` целиком (он не перехватывает `pgxpool`). Написать:
- `TestSettingsRoundTrip`:
  - открыть pool по DSN, `Ping`, иначе `t.Skip`;
  - `EnsureSettingsTable`;
  - сохранить исходные `GetSettings`;
  - `defer` восстановить исходные через `UpdateSettings`;
  - изменить `PresenceEnabled` (инверсия), `PresenceSessionMinMinutes/Max`, `TwitchClientID`, `TwitchClientSecret`, `LLMApiKey`;
  - `UpdateSettings`, затем `GetSettings`;
  - проверить каждое изменённое поле, включая секреты;
  - убедиться, что `TwitchClientSecretSet` не пишется в БД (поле не влияет на чтение).
- `TestUpdateSettingsMissingRowDoesNotPanic`:
  - временно `UPDATE settings SET id='moved' WHERE id='default'`; вызвать `UpdateSettings`; вернуть строку обратно; тест не должен паниковать (поведение `RowsAffected==0` фиксируем как есть).

- [ ] **Step 2: Run tests**

Run: `go test ./internal/storage/ -run 'TestSettingsRoundTrip|TestUpdateSettingsMissingRow' -v`
Expected: PASS при живом Postgres; `SKIP` без него.

- [ ] **Step 3: Run the full backend suite**

Run: `go test ./...`
Expected: PASS.

- [ ] **Step 4: Commit**

```bash
git add internal/storage/settings_test.go
git commit -m "test(storage): exercise the real settings round-trip"
```

---

## Self-Review

- **Spec coverage:** 400 вместо 500 (Task 1), журнал Twitch-полей без секрета (Task 1), явная очистка секрета (Task 2), реальные тесты настроек (Task 3).
- **Type consistency:** `TwitchClientSecretClear` объявлен в Task 2 и используется только там; интеграционные тесты Task 3 не зависят от флага.
- **Review Focus:** каждый пункт закрыт шагом своей задачи.
- **Proportion:** план короче кода; тела функций описаны требованиями.
