# Frontend Honesty & Cleanup Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Убрать врущие состояния интерфейса, сделать обновление данных единым для всех страниц, защитить несохранённые настройки и удалить мёртвый код и зависимости.

**Architecture:** Один владелец WebSocket-инвалидации на уровне провайдеров, один примитив `AsyncState` для загрузки/ошибки/пустоты, dirty-guard в форме настроек, навигация через контекст вместо DOM-хака, общий `Modal` и общая WS-подписка вместо второго сокета. Никаких новых зависимостей; только удаление.

**Tech Stack:** React 19, TypeScript, Vite, TanStack Query v5, Zustand (удаляется), Tailwind v4 (остаётся), framer-motion (удаляется).

**Spec:** `docs/superpowers/specs/2026-11-09-ui-simplification-twitch-monitoring-design.md` (разделы про честные состояния и лёгкий UI).

**Testing note:** в проекте нет фронтенд-тест-раннера. Гейт каждой задачи — `npm.cmd run build` (`tsc -b` + vite) плюс перечисленные grep-проверки и ручная проверка в браузере `http://localhost:3000`. Новый тест-раннер не добавляем.

## Global Constraints

- Новых зависимостей не добавлять; только удаление.
- Русский UI; никаких `alert()`/`confirm()`.
- Один WebSocket на приложение: в `frontend/src` должен остаться ровно один `new WebSocket(`.
- Все `/api/*` запросы идут через `api/client.ts`; ключ не попадает в логи.
- Не менять Go-бэкенд.
- Гейт: `cd frontend && npm.cmd run build`.
- После удаления зависимостей обязателен `npm.cmd install` для обновления `package-lock.json`.

## Review Focus

- Несохранённые правки в Settings не затираются фоновым refetch.
- Страница Bots видит `bot_presence_changed` без смонтированного Dashboard.
- Ошибка запроса никогда не рендерится как «пусто» или бесконечная загрузка.
- В приложении остаётся одно WS-соединение, включая модалку аудиотеста.
- Escape закрывает модалку, фокус возвращается на кнопку-открывашку.
- После снятия зависимостей сборка проходит без мёртвых импортов.

---

### Task 1: Единый владелец live-данных и навигация через контекст

**Files:**
- Modify: `frontend/src/App.tsx:11-88`
- Create: `frontend/src/components/LiveDataBridge.tsx`
- Create: `frontend/src/components/layout/navigation.tsx`
- Modify: `frontend/src/components/SetupBanner.tsx:1-60`
- Modify: `frontend/src/pages/Dashboard.tsx:113-123`, `:202`
- Modify: `frontend/src/components/StreamCard.tsx:110`
- Modify: `frontend/src/pages/Stream.tsx:26-30`

**Interfaces:**
- Produces: `LiveDataBridge()` — компонент без пропсов, живёт под `QueryClientProvider` и `WebSocketProvider`.
- Produces: `NavigationProvider({ value, children })` и `useAppNavigate(): (page: Page) => void` из `components/layout/navigation.tsx`; тип `Page` импортируется из `../Sidebar`.
- `SetupBanner` теряет DOM-поиск и вызывает `useAppNavigate()`.

- [ ] **Step 1: Переставить провайдеры**

В `App.tsx` сделать `QueryClientProvider` внешним, затем `WebSocketProvider`, затем `LiveDataBridge` и shell. Провайдер навигации оборачивает shell и получает `navigate: setCurrentPage`.

- [ ] **Step 2: Создать `LiveDataBridge`**

Компонент подписывается через `useWebSocketSubscription` и инвалидирует:

| `msg.type` | Ключи |
|---|---|
| `bot_status_changed` | `['bots']`, `['stats']` |
| `stats_updated` | `['stats']` |
| `bot_presence_changed` | `['bots']` |
| `bot_message_sent` | `['stats']` |
| `settings_log` | `['settings']` |

Колбэк оборачивается в `useCallback` с зависимостью `[queryClient]`.

- [ ] **Step 3: Убрать локальную инвалидацию и дубли polling**

Удалить подписку `Dashboard.tsx:113-123`. Убрать `refetchInterval: 30_000` у запроса `['settings']` в `Dashboard.tsx:94-98` и `Stream.tsx:26-30`: владельцем остаётся `Topbar`.

- [ ] **Step 4: Навигация через контекст**

Создать `NavigationProvider`/`useAppNavigate`. В `SetupBanner` удалить `navigateToPage` и `document.querySelectorAll`; кнопка вызывает `useAppNavigate()(actionPage)`. В `Dashboard.tsx:202` и `StreamCard.tsx:110` удалить импорт и вызов `navigateToPage` — CTA внутри `SetupBanner` уже навигирует сам.

- [ ] **Step 5: Проверить**

Run: `cd frontend && npm.cmd run build`
Expected: exit 0.
Run: `Select-String -Path frontend/src -Pattern "querySelectorAll\('\.sidebar__nav-item'" -Recurse`
Expected: 0 совпадений.

- [ ] **Step 6: Commit**

```bash
git add frontend/src/App.tsx frontend/src/components/LiveDataBridge.tsx frontend/src/components/layout/navigation.tsx frontend/src/components/SetupBanner.tsx frontend/src/pages/Dashboard.tsx frontend/src/components/StreamCard.tsx frontend/src/pages/Stream.tsx
git commit -m "feat(ui): own live invalidation at the app level and navigate by context"
```

---

### Task 2: Settings dirty-guard и честный аудиотест

**Files:**
- Modify: `frontend/src/pages/Settings.tsx:139-192`, `:238-255`, `:560-567`, `:653-657`
- Modify: `frontend/src/components/AudioTestModal.tsx` (переписать)
- Modify: `frontend/src/components/ui/Modal.tsx` (только переиспользование пропсов, если нужно)

**Interfaces:**
- Consumes: `Modal` из `components/ui/Modal.tsx`, `useWebSocketSubscription` из `hooks/useWebSocket.ts`, `useAppNavigate` не нужен.
- Produces: `AudioTestModal({ isOpen, onClose, channel })` — канал это первый непустой канал из формы, а не `debug_channel`.

- [ ] **Step 1: Baseline и dirty**

Добавить `baselineRef` с нормализованными значениями после каждого успешного `GET`/сохранения. `dirty` = глубокое сравнение `formState` с baseline (без `updated_at` и `llm_api_key`). Эффект синхронизации применяет серверные данные только когда `dirty === false`.

- [ ] **Step 2: Баннер и сохранение**

Пока `dirty`, показывать info-баннер «Есть несохранённые изменения». На `onSuccess` обновлять baseline и сбрасывать `dirty`. Существующие таймеры `setSaveSuccess/setSaveError` сохранить.

- [ ] **Step 3: `max_tokens`**

При `onChange` пустую строку игнорировать (оставлять прежнее значение), число клампить в `20..200`.

- [ ] **Step 4: Переписать `AudioTestModal`**

Убрать `new WebSocket` (`:26`), `createPortal`, `framer-motion` и инлайн-тёмную палитру. Подписаться на `audio_context` через `useWebSocketSubscription`, рендерить через общий `Modal` с классами лёгкой темы. Сохранить: кнопку «Тест LLM» (`POST /api/settings/test-llm`, `personality: 'нейтрал'`), ленту транскрипции с ограничением 200 записей.

- [ ] **Step 5: Передать правильный канал**

В `Settings.tsx:653-657` передавать `channel={formState.channels.map(c => c.trim()).find(Boolean) ?? ''}`.

- [ ] **Step 6: Проверить**

Run: `cd frontend && npm.cmd run build`
Expected: exit 0.
Run: `Select-String -Path frontend/src -Pattern "new WebSocket\(" -Recurse`
Expected: ровно 1 совпадение (`hooks/useWebSocket.ts`).
Run: `Select-String -Path frontend/src -Pattern "framer-motion" -Recurse`
Expected: 0 совпадений.

- [ ] **Step 7: Commit**

```bash
git add frontend/src/pages/Settings.tsx frontend/src/components/AudioTestModal.tsx
git commit -m "feat(settings): protect unsaved edits and move audio test onto the shared socket"
```

---

### Task 3: Доступность примитивов

**Files:**
- Modify: `frontend/src/components/ui/Modal.tsx:14-60`
- Create: `frontend/src/hooks/useFocusTrap.ts`
- Modify: `frontend/src/components/BotLogsDrawer.tsx:49-78`
- Modify: `frontend/src/components/ui/Input.tsx:11-26`
- Modify: `frontend/src/components/ui/Select.tsx:18-34`
- Modify: `frontend/src/components/ui/Button.tsx:24-34`

**Interfaces:**
- Produces: `useFocusTrap(ref: RefObject<HTMLElement | null>, active: boolean)` — ловит Tab/Shift+Tab внутри контейнера, ставит начальный фокус и возвращает его на элемент, активный до открытия.

- [ ] **Step 1: Focus trap**

Реализовать хук: найти фокусируемые элементы `a[href], button:not([disabled]), input, select, textarea, [tabindex]:not([tabindex="-1"])`; на `keydown` с `Tab` циклически переводить фокус; при `active` запоминать `document.activeElement` и фокусировать первый элемент; при снятии возвращать фокус.

- [ ] **Step 2: Modal**

Добавить `useId()` для `aria-labelledby`, повесить `id` на `h3`, применить `useFocusTrap`. Escape сохранить.

- [ ] **Step 3: Drawer**

Применить `useFocusTrap` к `aside`; `role="dialog"`, `aria-modal`, `aria-label` уже есть.

- [ ] **Step 4: Input/Select/Button**

`Input` и `Select`: при `error` ставить `aria-invalid` и `aria-describedby` на id текста ошибки (id из `useId`). `Button`: при `loading` ставить `aria-busy="true"` и `disabled`.

- [ ] **Step 5: Проверить**

Run: `cd frontend && npm.cmd run build`
Expected: exit 0.
Run: `Select-String -Path frontend/src -Pattern "aria-describedby|aria-invalid|aria-labelledby" -Recurse`
Expected: не 0 совпадений.

- [ ] **Step 6: Commit**

```bash
git add frontend/src/components/ui/Modal.tsx frontend/src/hooks/useFocusTrap.ts frontend/src/components/BotLogsDrawer.tsx frontend/src/components/ui/Input.tsx frontend/src/components/ui/Select.tsx frontend/src/components/ui/Button.tsx
git commit -m "feat(ui): add focus trap and accessible field errors"
```

---

### Task 4: Метаданные страницы и версия

**Files:**
- Modify: `frontend/index.html:2`, `:7`
- Modify: `frontend/vite.config.ts`
- Create: `frontend/src/vite-env.d.ts` (или дописать в существующий)
- Modify: `frontend/src/components/layout/Sidebar.tsx:6`, `:52`
- Modify: `frontend/package.json:4`

**Interfaces:**
- Produces: глобальная константа `__APP_VERSION__: string`, объявленная в `vite-env.d.ts`.

- [ ] **Step 1: index.html**

`lang="ru"`, `<title>Twitch SMM</title>`.

- [ ] **Step 2: Версия из package.json**

В `vite.config.ts` прочитать `package.json` (`readFileSync(new URL('./package.json', import.meta.url))`) и задать `define: { __APP_VERSION__: JSON.stringify(pkg.version) }`. В `package.json` поставить `"version": "0.1.0"`. Объявить `declare const __APP_VERSION__: string` в `vite-env.d.ts`. Обновить lock-файл: `npm.cmd install --package-lock-only`.

- [ ] **Step 3: Sidebar**

Удалить константу `APP_VERSION` и выводить `v{__APP_VERSION__}`.

- [ ] **Step 4: Проверить**

Run: `cd frontend && npm.cmd run build`
Expected: exit 0; в `dist/index.html` `<html lang="ru">` и `<title>Twitch SMM</title>`.

- [ ] **Step 5: Commit**

```bash
git add frontend/index.html frontend/vite.config.ts frontend/src/vite-env.d.ts frontend/src/components/layout/Sidebar.tsx frontend/package.json frontend/package-lock.json
git commit -m "chore(ui): set Russian document metadata and derive the displayed version"
```

---

### Task 5: Честные состояния загрузки, ошибки и пустоты

**Files:**
- Create: `frontend/src/components/ui/AsyncState.tsx`
- Modify: `frontend/src/pages/Dashboard.tsx:86-101`, `:133-203`
- Modify: `frontend/src/pages/Stream.tsx:26-37`, `:81-117`
- Modify: `frontend/src/pages/Bots.tsx:25-32`, `:187-212`
- Modify: `frontend/src/components/StreamCard.tsx:59-78`, `:103-177`
- Modify: `frontend/src/components/ChatFeed.tsx:13-19`, `:124-140`
- Modify: `frontend/src/hooks/useStreamStatus.ts:8-15`

**Interfaces:**
- Produces: `AsyncState({ loading, error, isEmpty, loadingText, errorText, emptyTitle, emptyDescription, onRetry, children })`.
- `StreamCard` получает пропсы `isLoading?: boolean`, `isError?: boolean`, `onRetry?: () => void`.
- `ChatFeed` получает `loading?: boolean`, `error?: boolean`, `onRetry?: () => void`.

- [ ] **Step 1: Примитив**

`AsyncState` рендерит: `loading` → calm-текст с `role="status"`; `error` → `Alert variant="error"` с кнопкой «Повторить» при `onRetry`; `isEmpty` → `EmptyState`; иначе `children`.

- [ ] **Step 2: Dashboard**

Разделить `isLoading`/`isError` для `['bots']`, `['stats']`, `['settings']`:
- боты: загрузка → «Загружаем ботов…», ошибка → Alert с retry, пусто → нынешний EmptyState;
- статистика: при ошибке показывать `—` с подписью «нет данных», а не молча;
- баннер «Канал не задан» показывать только когда `settings` загружены без ошибки и каналов действительно нет.

- [ ] **Step 3: Stream**

Те же три состояния для settings и `useStreamStatus`. Чаттеры: загрузка → «Загружаем чат…», ошибка → Alert + retry, пусто → «Пока никто не писал». Чат: передать `loading`/`error` в `ChatFeed`.

- [ ] **Step 4: Bots**

Пока `isLoading`, показывать «Загружаем ботов…» вместо EmptyState; ошибка уже обрабатывается.

- [ ] **Step 5: StreamCard**

Принять `isLoading`/`isError`/`onRetry` от вызывающих. HTTP-ошибка → Alert «Не удалось получить данные Twitch» с retry, не «Загружаем статус…». Строку метрик и бейдж чата не показывать, когда данные не live и есть ошибка. `configured: false` без ошибки — отдельный текст «Канал не отслеживается» с CTA в настройки.

- [ ] **Step 6: ChatFeed**

Добавить `aria-live="polite"` на список сообщений и отдельные ветки loading/error/empty.

- [ ] **Step 7: Проверить**

Run: `cd frontend && npm.cmd run build`
Expected: exit 0.
Ручная проверка: остановить backend (`docker compose stop backend`), открыть каждую страницу — должны быть тексты ошибок с retry, а не «Пока нет ботов», не бесконечная загрузка и не «Канал не задан». Затем `docker compose start backend`.

- [ ] **Step 8: Commit**

```bash
git add frontend/src/components/ui/AsyncState.tsx frontend/src/pages/Dashboard.tsx frontend/src/pages/Stream.tsx frontend/src/pages/Bots.tsx frontend/src/components/StreamCard.tsx frontend/src/components/ChatFeed.tsx frontend/src/hooks/useStreamStatus.ts
git commit -m "feat(ui): distinguish loading, failure and emptiness on every page"
```

---

### Task 6: Удаление мёртвого кода и зависимостей

**Files:**
- Delete: `frontend/src/stores/logStore.ts`, `frontend/src/lib/utils.ts`, `frontend/tailwind.config.js`
- Modify: `frontend/src/hooks/useWebSocket.ts:1-3`, `:46-92`, `:123`, `:240-243`
- Modify: `frontend/src/hooks/useBotForm.ts:59`, `:144`, `:152-153`
- Modify: `frontend/src/api/client.ts` (удалить `getBot`, `getProxies`, `createProxy`, `deleteProxy`, `Proxy`, `CreateProxyRequest`)
- Modify: `frontend/package.json`

**Interfaces:**
- Consumes: Task 2 удалил `framer-motion`; Task 5 не использует удаляемые сущности.
- Produces: `useWebSocket.ts` экспортирует только `startWebSocket`, `subscribeWebSocket`, `onWebSocketStatus`, `getWebSocketConnected`, `WebSocketProvider`, `useWebSocketStatus`, `useWebSocketSubscription`.

- [ ] **Step 1: Убрать мёртвый стор**

Удалить импорт `logStore`, функцию `handleStoreEvents`, её вызов на каждом сообщении и обёртку `useWebSocket`/`BotPresenceEvent`. Удалить файл `stores/logStore.ts`.

- [ ] **Step 2: Мелкий мёртвый код**

Удалить `lib/utils.ts`, `showAdvanced`/`setShowAdvanced` из `useBotForm.ts`, неиспользуемые методы и типы прокси из `client.ts`, `tailwind.config.js`.

- [ ] **Step 3: Снять зависимости**

Run: `cd frontend && npm.cmd uninstall gsap three @types/three motion tailwind-merge zustand framer-motion`
Expected: `package.json` и `package-lock.json` обновлены.

- [ ] **Step 4: Проверить**

Run: `cd frontend && npm.cmd run build`
Expected: exit 0, вывод без предупреждений о неиспользуемых импортах.
Run: `Select-String -Path frontend/src -Pattern "logStore|lib/utils|framer-motion|gsap|three|tailwind-merge|zustand" -Recurse`
Expected: 0 совпадений.

- [ ] **Step 5: Commit**

```bash
git add frontend/package.json frontend/package-lock.json frontend/src/hooks/useWebSocket.ts frontend/src/hooks/useBotForm.ts frontend/src/api/client.ts frontend/src/stores/logStore.ts frontend/src/lib/utils.ts frontend/tailwind.config.js
git commit -m "chore(ui): remove dead stores, helpers and unused dependencies"
```

---

## Self-Review

- **Spec coverage:** честные состояния (Task 5), единое обновление данных (Task 1), защита правок (Task 2), лёгкий UI и один сокет (Task 2), навигация без DOM (Task 1), доступность (Task 3), метаданные (Task 4), чистка (Task 6).
- **Type consistency:** `Page` один и тот же из `Sidebar.tsx`; `AsyncState` пропсы совпадают во всех вызовах; `StreamCard`/`ChatFeed` получают новые пропсы только из Task 5 и не конфликтуют с Task 1.
- **Review Focus:** каждый пункт закрыт шагом соответствующей задачи.
- **Proportion:** план короче кода; тела компонентов описаны требованиями, а не транскриптом.
