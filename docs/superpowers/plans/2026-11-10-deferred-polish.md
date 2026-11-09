# Deferred Polish Implementation Plan (A + B)

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Закрыть отложенные хвосты честности фронтенда (типизированные WS-события, состояние ошибки сокета, честные формулировки, сохранение секретов) и остаточную гонку backend, где устаревший цикл бота может изменить состояние нового запуска.

**Architecture:** Discriminated union для WS-сообщений по образцу backend DTO; явное состояние `error` в общем WS-хуке; координация фокус-трапов через стек; реконсиляция формы после сохранения без потери правок, сделанных во время запроса; в backend — единый ctx-guard перед `markBotError`, ctx-aware сон и защищённый `lastSent`.

**Tech Stack:** Go 1.26+, React 19 + TypeScript + Vite, TanStack Query v5.

**Spec:** `docs/superpowers/specs/2026-11-09-ui-simplification-twitch-monitoring-design.md` и итоговые ревью планов A/B.

**Testing note:** у фронтенда нет тест-раннера; гейт — `npm.cmd run build` и перечисленные grep-проверки. Backend — `go test ./...` с новыми юнит-тестами.

## Global Constraints

- Новых зависимостей не добавлять.
- Русский UI; никаких `alert()`/`confirm()`.
- Один WebSocket на приложение; `new WebSocket(` только в `hooks/useWebSocket.ts`.
- Секреты не логируются и не возвращаются API.
- Публичные сигнатуры `botManager` не меняются; новые хелперы — неэкспортируемые.
- Работа на `main`.

## Review Focus

- Отменённый цикл бота не меняет `presenceState`/`running` нового запуска и не помечает его ошибкой.
- `markBotError` на живом цикле по-прежнему срабатывает после 5 ошибок.
- Неизвестный тип WS-сообщения явно обработан, а не молча проглочен.
- Аудиомодалка различает «подключение», «ошибка соединения» и «подключено».
- Два одновременно открытых диалога: Tab и Escape обрабатывает только верхний.
- Неудачное сохранение настроек не снимает `dirty` и не выдаёт себя за успех.

---

### Task 1: Типизированные WS-события и состояние ошибки сокета

**Files:**
- Modify: `frontend/src/hooks/useWebSocket.ts:4-10`, `:54-78`, `:120-158`
- Modify: `frontend/src/components/LiveDataBridge.tsx:16-33`
- Modify: `frontend/src/pages/Stream.tsx:54-71`
- Modify: `frontend/src/components/AudioTestModal.tsx:24`, `:34-42`, `:64-70`
- Modify: `frontend/src/components/layout/Topbar.tsx:30`, `:81-83`

**Interfaces:**
- Produces: `WSMessage` — discriminated union `{ type: K; payload: WSPayloads[K] }` для ключей `bot_status_changed`, `bot_presence_changed`, `bot_message_sent`, `stats_updated`, `bot_log`, `settings_log`, `audio_context`.
- Produces: payload-типы по backend DTO: `BotStatusPayload {bot_id, status}`, `BotPresencePayload {bot_id, login, state, at}`, `BotMessagePayload {bot_id, message, personality, channel?, timestamp}`, `BotLogPayload {bot_id, log_type, level, message, context?, timestamp}`, `SettingsLogPayload {level, message, changes?, timestamp}`, `Stats` (существующий тип из `api/client.ts`), `audio_context: {context: string}`.
- Produces: `useWebSocketStatus(): { connected: boolean; error: boolean }`; `error` становится `true` после `onerror` и сбрасывается в `false` в `onopen`.

- [ ] **Step 1: Union и статус**

В `useWebSocket.ts` заменить `WSMessage` на discriminated union и добавить модульный флаг `socketError` с сеттером, аналогичным `setConnected`. `ws.onerror` выставляет `error=true` (без логирования payload), `ws.onopen` — `connected=true, error=false`, `ws.onclose` — `connected=false`. `WebSocketContextValue` расширить до `{ connected, error }`, провайдер и `useWebSocketStatus` вернуть оба поля.

- [ ] **Step 2: Исчерпывающая обработка**

`LiveDataBridge` переписать на `switch (msg.type)` с явными ветками для всех семи типов: пять текущих инвалидаций, `bot_log` и `audio_context` — `break` с комментарием «handled elsewhere / intentionally ignored». Добавить `default: { const _exhaustive: never = msg; }` для компиляторной проверки полноты.

- [ ] **Step 3: Убрать ручные касты**

`Stream.tsx` берёт `msg.payload` уже типизированным для `bot_presence_changed`; удалить `as { login?: unknown; ... }` и проверки `typeof`, оставив валидацию только там, где поле допускает `string` из backend (`state`). `AudioTestModal` использует `msg.payload.context` без `any`.

- [ ] **Step 4: Статус в UI**

`AudioTestModal`: `connected` → «подключено», `error` → «ошибка соединения», иначе «подключение…».
`Topbar`: при `error` показывать чип WS в тревожном тоне («WebSocket: ошибка»), при `connected` — как сейчас.

- [ ] **Step 5: Проверить**

Run: `cd frontend && npm.cmd run build`
Expected: exit 0; `tsc -b` ловит необработанный тип.

- [ ] **Step 6: Commit**

```bash
git add frontend/src/hooks/useWebSocket.ts frontend/src/components/LiveDataBridge.tsx frontend/src/pages/Stream.tsx frontend/src/components/AudioTestModal.tsx frontend/src/components/layout/Topbar.tsx
git commit -m "feat(ui): type WebSocket events and expose connection errors"
```

---

### Task 2: Честные формулировки, секреты и сохранение без потери правок

**Files:**
- Modify: `frontend/src/pages/Settings.tsx:100-112`, `:226-274`, `:321-348`, `:760-764`
- Modify: `frontend/src/pages/Dashboard.tsx:132-146`, `:172-185`, `:207-216`
- Modify: `frontend/src/components/StreamCard.tsx:137-154`

**Interfaces:**
- Consumes: `useWebSocketStatus` из Task 1 не требуется.
- Produces: `reconcileAfterSave(form, payload): SettingsType` — чистая функция, возвращающая форму после успешного сохранения: очищенный `llm_api_key`/`twitch_client_secret` только если значение не менялось с момента клика; нормализованные `channels`/`max_tokens` только если они не менялись.

- [ ] **Step 1: Сохранение без преждевременной нормализации**

В `handleSave` убрать `setFormState` до `mutate`. `payload` строится как сейчас (trim каналов, clamp токенов, свежий `updated_at`), но форма остаётся введённой пользователем.

- [ ] **Step 2: Реконсиляция в `onSuccess`**

Добавить чистую `reconcileAfterSave(form, payload)`:
- `channels`: если `form.channels.map(trim).filter(Boolean)` совпадает с `payload.channels`, заменить на отображаемую форму (`payload.channels.length ? payload.channels : ['']`), иначе оставить форму;
- `max_tokens`: если `form.max_tokens === payload.max_tokens`, оставить; иначе оставить форму;
- `llm_api_key` и `twitch_client_secret`: очищать только если `form.<field> === payload.<field>`, иначе сохранить введённое;
- `updated_at`: взять из `payload`.
В `onSuccess` вызвать `setFormState(prev => reconcileAfterSave(prev, variables))` и `setBaseline(normalizeSettings(variables))` с clamp токенов. Ошибка сохранения (`onError`) не трогает форму и baseline, поэтому `dirty` остаётся.

- [ ] **Step 3: Два «Загружаем настройки…»**

В `Dashboard` оставить текст только в одном месте: слот баннера показывает loading/error, а плейсхолдер карточки «Трансляция» в состоянии `settingsPending` показывает нейтральный скелет без повторного текста (например, `loading` без `loadingText` либо короткое «…»).

- [ ] **Step 4: Подсказки статистики**

`hint`: если запрос упал и данные есть — «данные не обновляются»; если данных нет — «нет данных»; иначе `undefined`. Применить к трём стат-карточкам.

- [ ] **Step 5: Текст StreamCard при сбое**

В ветке `fetchFailed` не утверждать, что данные показаны, если `showDetails === false`. Использовать: «Обновить не удалось. Последняя попытка: HH:MM:SS» без слова «показаны». Ветку `gqlFailed` не менять.

- [ ] **Step 6: Канал аудиомодалки**

В `Settings.tsx` передавать первый непустой канал из сохранённых настроек (`settings?.channels`), а не из `formState.channels`.

- [ ] **Step 7: Проверить**

Run: `cd frontend && npm.cmd run build`
Expected: exit 0.
Проверка глазами: сохранить список каналов, отредактировать поле во время запроса — правка не исчезает; очистить список каналов и сохранить — `dirty` снимается.

- [ ] **Step 8: Commit**

```bash
git add frontend/src/pages/Settings.tsx frontend/src/pages/Dashboard.tsx frontend/src/components/StreamCard.tsx
git commit -m "fix(ui): keep edits during save and stop overstating stale data"
```

---

### Task 3: Согласованность диалогов и ARIA

**Files:**
- Modify: `frontend/src/hooks/useFocusTrap.ts:20-65`
- Modify: `frontend/src/components/ui/Modal.tsx:19-54`
- Modify: `frontend/src/components/BotLogsDrawer.tsx:44-93`
- Modify: `frontend/src/components/ui/Input.tsx:25-28`
- Modify: `frontend/src/components/ui/Select.tsx:32-37`

**Interfaces:**
- Produces: `useFocusTrap(ref, active, onEscape?)` — Tab/Shift+Tab и Escape обрабатывает только верхний активный трап; при деактивации возвращает фокус.
- Produces: контейнеры диалогов с `tabIndex={-1}`.

- [ ] **Step 1: Стек трапов**

В `useFocusTrap.ts` добавить модульный стек активных трапов (массив уникальных id). Keydown-обработчик игнорирует событие, если его трап не последний в стеке. Escape вызывает переданный `onEscape`, только если трап верхний. При деактивации удалять id из стека и возвращать фокус.

- [ ] **Step 2: Контейнеры**

`Modal.tsx` и `BotLogsDrawer.tsx`: добавить `tabIndex={-1}` на контейнер; перенести Escape из локального `window`-слушателя в `onEscape` хука (или оставить слушатель, но гейтить верхним трапом). Поведение закрытия по Escape и оверлею сохранить.

- [ ] **Step 3: Слияние ARIA**

`Input.tsx` и `Select.tsx`: при ошибке объединять внешний и внутренний `aria-describedby` (`[rest['aria-describedby'], errorId].filter(Boolean).join(' ')`), а `aria-invalid` выставлять `true` только при ошибке, не затирая переданное значение.

- [ ] **Step 4: Проверить**

Run: `cd frontend && npm.cmd run build`
Expected: exit 0.
Проверка: открыть в `Bots` модалку и дровер одновременно, Tab не перескакивает между ними, Escape закрывает верхний.

- [ ] **Step 5: Commit**

```bash
git add frontend/src/hooks/useFocusTrap.ts frontend/src/components/ui/Modal.tsx frontend/src/components/BotLogsDrawer.tsx frontend/src/components/ui/Input.tsx frontend/src/components/ui/Select.tsx
git commit -m "fix(ui): coordinate stacked dialogs and merge field ARIA"
```

---

### Task 4: Устаревший цикл не меняет состояние нового запуска (backend)

**Files:**
- Modify: `logic.go:1111-1126`, `:1156-1174`, `:1176-1188`, `:1218-1232`
- Test: `logic_bot_test.go`, `logic_presence_test.go`

**Interfaces:**
- Produces: `func (bm *botManager) markBotErrorIfActive(ctx context.Context, br *botRuntime) bool` — вызывает `markBotError` и возвращает `true` только если `ctx.Err() == nil`.
- Produces: `func sleepCtx(ctx context.Context, d time.Duration) bool` — `true`, если сон завершился, `false`, если контекст отменён.
- Produces: `func (bm *botManager) touchLastSent(br *botRuntime, t time.Time)` и `func (bm *botManager) lastSentAt(br *botRuntime) time.Time` — доступ к `lastSent` под `bm.mu`.

- [ ] **Step 1: Write the failing tests**

В `logic_bot_test.go`:

```go
func TestMarkBotErrorIfActiveSkipsCancelledContext(t *testing.T) {
	bm := newBotManager(nil, nil, nil, nil, context.Background(), "testchannel", 0, audio.Config{}, false, nil)
	br := &botRuntime{cfg: storage.BotConfig{ID: "bot-1"}, running: true}
	bm.bots["bot-1"] = br

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	if bm.markBotErrorIfActive(ctx, br) {
		t.Fatal("cancelled loop must not mark the runtime as errored")
	}
	if br.presenceState == presenceStateError || br.stoppedByError || !br.running {
		t.Fatalf("cancelled loop mutated the runtime: state=%q stoppedByError=%v running=%v",
			br.presenceState, br.stoppedByError, br.running)
	}

	if !bm.markBotErrorIfActive(context.Background(), br) {
		t.Fatal("active loop must still mark the runtime as errored")
	}
	if br.presenceState != presenceStateError || !br.stoppedByError || br.running {
		t.Fatal("active path must mark the error state")
	}
}

func TestSleepCtxStopsOnCancel(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	start := time.Now()
	if sleepCtx(ctx, time.Second) {
		t.Fatal("sleepCtx must report cancellation")
	}
	if elapsed := time.Since(start); elapsed > 200*time.Millisecond {
		t.Fatalf("sleepCtx ignored cancellation for %v", elapsed)
	}
	if !sleepCtx(context.Background(), 10*time.Millisecond) {
		t.Fatal("sleepCtx must return true when the context is alive")
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test . -run 'TestMarkBotErrorIfActive|TestSleepCtx' -v`
Expected: FAIL — `markBotErrorIfActive`/`sleepCtx` не определены.

- [ ] **Step 3: Implement the helpers**

Реализовать хелперы рядом с `markBotError`. `markBotErrorIfActive` сначала проверяет `ctx.Err()`, затем вызывает `markBotError` под `bm.mu` (использовать существующий метод). `sleepCtx` — `select` на `time.After(d)` и `ctx.Done()`. `touchLastSent`/`lastSentAt` — под `bm.mu`.

- [ ] **Step 4: Use them in the loop**

- Reconnect-ветка: `bm.markBotErrorIfActive(ctx, br)`; `if !sleepCtx(ctx, errorRetryDelay) { return }`.
- LLM-error ветка: `bm.markBotErrorIfActive(ctx, br)`.
- После успешного `generateMessage` и перед записью `br.lastSent`: `if ctx.Err() != nil { return }`; запись через `bm.touchLastSent`.
- Чтение темпа — через `bm.lastSentAt`.
- В `defer` читать `br.stopping` под `bm.mu`.

- [ ] **Step 5: Run tests**

Run: `go test . -run 'TestMarkBotErrorIfActive|TestSleepCtx|TestStaleLoop|TestStartBot|TestStopBot|TestPresence' -v` и `go test ./...`
Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add logic.go logic_bot_test.go
git commit -m "fix(bot): never let a cancelled loop touch a newer run"
```

---

## Self-Review

- **Spec coverage:** типизация WS и ошибка сокета (Task 1), честные формулировки и сохранение (Task 2), стек диалогов и ARIA (Task 3), остаточная гонка (Task 4).
- **Type consistency:** `WSMessage` объявлен в Task 1 и потребляется только там; `reconcileAfterSave` объявлена и использована в Task 2; backend-хелперы объявлены и использованы в Task 4.
- **Review Focus:** каждый пункт закрыт шагом своей задачи.
- **Proportion:** план короче кода; алгоритмы описаны там, где сигнатура не определяет поведение (реконсиляция, стек трапов).
