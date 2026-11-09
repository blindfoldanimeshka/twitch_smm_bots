# Упрощение UI и Twitch-мониторинг — дизайн

Дата: 2026-11-09
Статус: на утверждение

## 1. Контекст

Проект — панель управления фермой Twitch-ботов: Go backend, React 19 frontend,
PostgreSQL, Redis, OpenRouter для генерации сообщений и whisper.cpp для
транскрипции стрима.

Текущий UI — тёмная «киберпанк»-тема с PixelSnow, TargetCursor, Dock,
BGPattern и тяжёлыми анимациями. Он перегружен, дублирует статус-компоненты,
а ошибки конфигурации выглядят как красные сбои. Нужна спокойная светлая
панель в стиле референса: боковое меню, карточки, таблицы, минимум декора.

Одновременно нужен нативный мониторинг Twitch-стрима:

1. сколько длится стрим (uptime);
2. сколько зрителей всего, активных и пассивных;
3. встроенный плеер, чтобы смотреть стрим из панели.

Twitch Developer App у владельца нет. Поэтому мониторинг использует публичный
GraphQL Twitch (`gql.twitch.tv/gql`) с публичным web Client-ID, без OAuth.
Это неофициальный путь: он может измениться или быть заблокирован. Дизайн
изолирует его за одним клиентом, чтобы позже заменить на Helix без переделки UI.

## 2. Цели

1. Заменить тёмный перегруженный shell на светлую панель с боковым меню.
2. Свести приложение к четырём разделам: Dashboard, Stream, Bots, Settings.
3. Показывать uptime, общее число зрителей, активных и пассивных зрителей.
4. Встроить Twitch-плеер в панель.
5. Считать активных зрителей как уникальных авторов сообщений за 5 минут,
   независимо от того, запущены ли боты.
6. Убрать ротацию по каналам. Вместо неё — presence simulation: бот реально
   подключается, работает сессию, прощается, отключается, выжидает паузу и
   возвращается.
7. Показывать чат стрима в панели; сообщения наших ботов подсвечивать
   акцентным цветом.
8. Убрать найденные ошибки frontend/backend и не показывать конфигурационные
   состояния как сбои.
9. Сохранить существующее поведение ботов, настроек, аудио и OpenRouter.

## 3. Не-цели

- Twitch OAuth, регистрация приложения, Helix API.
- Twitch chat embed.
- Тёмная тема или переключатель тем.
- История стримов, графики за дни, экспорт.
- Новые страницы сверх четырёх.
- Переписывание bot runtime целиком; presence-сессии добавляются точечно.
- Переписывание storage, OpenRouter-клиента, whisper.cpp.
- Обучение личности бота на экспорте Telegram-чата (`realhuman_simulation_behaour_idea.txt`) — отдельный проект, не входит в эту итерацию.
- Мобильное приложение.
- Накрутка зрителей или попытка влиять на `viewer_count`.

## 4. Анти-цели

- Выдуманные цифры зрителей. Если данных нет — прочерк и причина.
- Накрутка зрителей: presence simulation только подключается к IRC и пишет
  сообщения, но не влияет на просмотры и `viewer_count`.
- Логирование OAuth-токенов, паролей ботов и ключей API.
- Красная ошибка на экране, когда просто не задан канал или ключ.
- Потеря функций: CRUD ботов, start/stop, каналы, OpenRouter, аудио.
- Тяжёлые анимации, кастомный курсор, снег, dock, параллакс.

## 5. Ограничения

- Backend: Go 1.26, текущие пакеты и API-контракт; без новых тяжёлых зависимостей.
- Frontend: React 19, TypeScript, Vite, `@tanstack/react-query`, `lucide-react`.
- Twitch GQL: `POST https://gql.twitch.tv/gql`, заголовок
  `Client-ID: kimne78kx3ncx6brgo4mv6wki5h1ko`, без токена.
- Поля GQL: `stream.createdAt`, `stream.viewersCount`, `stream.game.name`,
  `stream.title`; `stream == null` означает offline.
- Активные зрители: анонимный read-only IRC (`justinfan...`), только `PRIVMSG`,
  окно 5 минут. Никаких сообщений в чат.
- Пассивные = `max(0, viewers_total - active_viewers)`.
- Presence simulation: реальные IRC-подключения/отключения по расписанию плюс
  короткие приветствие/прощание. Twitch не показывает join/leave других
  зрителей в чате, поэтому «заход/выход» виден в нашем UI и через сообщения.
- Плеер: `https://player.twitch.tv/?channel=<login>&parent=<hostname>&muted=true`.
- В самом Twitch чате цвет сообщения задаёт Twitch; наша подсветка ботов
  работает только в чат-фиде панели.
- Язык интерфейса — русский.

## 6. Текущее состояние и найденные дефекты

На момент дизайна `go build ./...`, `go test ./...` и `npm run build` зелёные.
Речь ниже о runtime/UX-дефектах, а не о падающей сборке.

### UI

- `App.tsx` использует Dock, PixelSnow, TargetCursor, BGPattern и анимации
  страниц. Визуально тяжело, не соответствует панели управления.
- `StatusBar` и `FloatingProviderStatus` дублируют статус LLM.
- `Settings.tsx` — 799 строк, `styles/main.css` — 890 строк, много
  одноразовых стилей и `alert()`.
- `AudioTestModal` жёстко зашивает `ws://localhost:8080`, ключ
  `secret-api-key-12345` и `http://localhost:8080/api/settings/test-llm`.
- `api/client.ts` логирует в консоль все запросы и ответы.
- `Bots.tsx` логирует тело создания бота, включая OAuth-пароль, и использует
  `alert()`/`confirm()`.
- `Navigation.tsx` не используется.
- Ошибки API показываются сырым английским текстом.

### Backend

- `/api/stats` всегда возвращает `messages_per_min: 0`; счётчик не реализован.
- **`startBot` не запускает `runBotLoop`.** Бот подключается к IRC, но цикл
  генерации и отправки сообщений не стартует; `stopBot` затем ждёт
  `br.stopped` до 10-секундного таймаута. Это критический дефект.
- Нет endpoint для статуса стрима.
- Helix-клиент требует Client ID/Secret, которых у владельца нет.
- IRC-клиент читает строки, но не разбирает `PRIVMSG`, поэтому активных
  чаттеров сейчас посчитать нельзя.

## 7. Рассмотренные подходы

**A. Инкрементальный рефреш.** Оставить текущие страницы и модалки, убрать
эффекты, добавить светлый shell и раздел Stream. Быстро, но старые стили и
компоненты продолжат конфликтовать, а `Settings.tsx` останется перегруженным.

**B. Чистая переделка (выбран).** Новый shell, четыре страницы, общие примитивы
(Card, Stat, Table, Modal, Button, Input, Badge, Alert). API-клиент и backend
переиспользуются. Старые визуальные компоненты удаляются. Дороже, но результат
цельный и его можно дальше поддерживать.

**C. Минимум.** Только тема и sidebar, мониторинг карточкой на Dashboard.
Самый быстрый, но Stream-раздел будет тесным, а старый код останется.

Решение: **B**.

## 8. Дизайн UI

### 8.1 Визуальный язык

- Фон `#efefec`, поверхность `#f7f7f4`, карточки `#fbfbf9`.
- Текст `#181816`, вторичный `#6f6f68`, границы `#deded6`.
- Акцент — лаймовый `#c9f24d`, тёмный текст на нём.
- Успех `#2f855a`, предупреждение `#a86b12`, ошибка `#c43d3d`.
- Радиусы 16–24 px, мягкие тени, без glow и градиентов.
- Шрифт системный/Inter; заголовки обычным регистром, не ALL CAPS.
- Иконки `lucide-react`, размер 18–20.
- Никаких анимаций кроме короткого fade/transition 120–160 ms.

### 8.2 Shell

```
┌──────────────┬─────────────────────────────────────────────┐
│ Twitch SMM   │ Dashboard            [OpenRouter] [STT] [WS]│
│              ├─────────────────────────────────────────────┤
│ ▣ Dashboard  │  Добрый день                                │
│ ◉ Stream     │  4 карточки: боты, запущено, msgs/min,      │
│ 🤖 Bots      │  зрители                                    │
│ ⚙ Settings   │                                             │
│              │  [ превью стрима + uptime ]                 │
│              │  [ таблица активных ботов ]                 │
└──────────────┴─────────────────────────────────────────────┘
```

- Sidebar 232 px, светлая, скруглённая, активный пункт — лаймовая пилюля.
- Внизу sidebar — версия и индикатор соединения.
- Topbar: название страницы, чипы `OpenRouter`, `STT`, `WS`, время обновления.
  `OpenRouter` берётся из `/api/llm/status`; `STT` — это состояние настройки
  `audio_enabled` (без отдельного health-check); `WS` — из `useWebSocket`.
- На ширине < 900 px sidebar превращается в верхнее меню/шторку.
- Плеер и таблицы не выходят за контейнер max-width 1280 px.

### 8.3 Dashboard

- Заголовок и краткое описание.
- Баннеры конфигурации (не ошибки):
  - «Канал не задан» → кнопка в Settings.
  - «OpenRouter не настроен» → кнопка в Settings.
- Карточки: всего ботов, запущено, сообщений/мин, зрителей сейчас.
- Блок стрима: плеер 16:9 (или заглушка), название, игра, uptime,
  `всего / активные / пассивные`.
- Таблица «Активные боты»: имя, логин, статус, личность, действия
  start/stop. Пустое состояние с CTA «Добавить бота».

### 8.4 Stream

- Селектор канала (если в Settings несколько).
- Большой плеер 16:9 с `allowfullscreen`.
- Строка метрик: uptime, всего зрителей, активные, пассивные, игра.
- Список активных чаттеров за 5 минут (логин, время последнего сообщения).
- **Чат стрима**: последние 50 сообщений, обновление с тем же poll-интервалом.
  Сообщения наших ботов выделены лаймовым фоном/рамкой; presence-события
  «вошёл/вышел» показываются системными строками. Цвет в самом Twitch-чате
  не контролируется.
- Если стрим offline: спокойная заглушка «Стрим не идёт», метрики — прочерки.
- Если GQL недоступен: «Не удалось получить данные Twitch» + время последней
  успешной попытки; плеер остаётся.
- Если канал не задан: CTA в Settings.

### 8.5 Bots

- Таблица: логин, имя, личность, статус, задержки, лимит, действия.
- Статус presence: `в эфире`, `пауза до HH:MM`, `остановлен`, `ошибка`.
- «Добавить бота» открывает модалку.
- Модалка: ID, Twitch login, OAuth-токен, имя, личность, min/max задержка,
  лимит; OAuth-токен не показывается при редактировании.
- Действия: запустить, остановить, редактировать, удалить, логи.
- Удаление — confirm-модалка, не `window.confirm`.
- Ошибки — inline в модалке/строке, не `alert()`.
- Логи бота — выдвижная панель с последними 100 записями.

### 8.6 Settings

Секции в двух колонках, сохранение одной кнопкой внизу:

1. **Каналы** — список каналов, добавить/удалить.
2. **Twitch-мониторинг** — объяснение, что используется публичный GraphQL и
   read-only IRC; поля Client ID/Secret остаются как будущий fallback.
3. **OpenRouter** — бесплатная модель, платная модель, лимит, ключ, статус,
   источник ключа (`env`/`database`/`none`).
4. **Аудио** — включение, язык, тест аудио (модалка).
5. **Поведение** — debug mode, debug channel, prompts, max tokens.
6. **Presence simulation** — включение, длительность сессии (min/max),
   пауза (min/max), приветствие/прощание. Ротация по каналам удалена.
7. **Журнал настроек** — свернутый блок (если решим оставить).

`0` или меньше в дневном лимите = без лимита.

## 9. Frontend-архитектура

### 9.1 Новые/переписанные файлы

- `frontend/src/App.tsx` — shell, sidebar, topbar, страницы.
- `frontend/src/styles/app.css` — токены и общие стили; заменить `main.css`.
- `frontend/src/index.css` — минимальный reset.
- `frontend/src/components/layout/Sidebar.tsx`
- `frontend/src/components/layout/Topbar.tsx`
- `frontend/src/components/layout/PageHeader.tsx`
- `frontend/src/components/ui/Card.tsx`
- `frontend/src/components/ui/StatCard.tsx`
- `frontend/src/components/ui/Button.tsx`
- `frontend/src/components/ui/Input.tsx`
- `frontend/src/components/ui/Select.tsx`
- `frontend/src/components/ui/Badge.tsx`
- `frontend/src/components/ui/Alert.tsx`
- `frontend/src/components/ui/Modal.tsx`
- `frontend/src/components/ui/EmptyState.tsx`
- `frontend/src/components/StreamCard.tsx` — плеер + метрики.
- `frontend/src/components/ChatFeed.tsx` — сообщения чата, подсветка ботов,
  системные presence-события.
- `frontend/src/components/BotsTable.tsx`
- `frontend/src/components/BotLogsDrawer.tsx`
- `frontend/src/pages/Dashboard.tsx`
- `frontend/src/pages/Stream.tsx`
- `frontend/src/pages/Bots.tsx`
- `frontend/src/pages/Settings.tsx`
- `frontend/src/hooks/useStreamStatus.ts`
- `frontend/src/hooks/useStats.ts` (опционально)

Сохраняем и переписываем под новые примитивы:

- `frontend/src/components/BotForm.tsx` — поля формы бота без старых CSS.
- `frontend/src/components/BotFormField.tsx` — обёртка label/input/error.
- `frontend/src/components/ui/switch.tsx` — используется в Settings.

### 9.2 Изменяемые файлы

- `frontend/src/api/client.ts` — типы `StreamStatus`, `Chatter`, `ChatMessage`,
  поля presence в `Bot`; метод `getStreamStatus(channel?)`; удалить логирующие
  interceptors; добавить `getErrorMessage`.
- `frontend/src/hooks/useWebSocket.ts` — exponential backoff, без console.log,
  обработка `bot_presence_changed` для системных строк в чат-фиде.
- `frontend/src/config.ts` — без изменений.
- `frontend/src/hooks/useBotForm.ts` — переиспользовать валидацию.
- `frontend/src/components/AudioTestModal.tsx` — использовать `config` и
  `api`, без хардкода localhost/ключа.

### 9.3 Удаляемые файлы

- `frontend/src/components/Navigation.tsx`
- `frontend/src/components/StatusBar.tsx`
- `frontend/src/components/FloatingProviderStatus.tsx`
- `frontend/src/components/Stats.tsx`
- `frontend/src/components/BotList.tsx`
- `frontend/src/components/BotCard.tsx`
- `frontend/src/components/BotLogsModal.tsx`
- `frontend/src/components/LaunchBotModal.tsx`
- `frontend/src/components/BotLogPanel.tsx`
- `frontend/src/components/DebugMessagesPanel.tsx`
- `frontend/src/components/DebugModeNotification.tsx`
- `frontend/src/components/SettingsLogPanel.tsx`
- `frontend/src/components/FloatingPanel.tsx` + `FloatingPanel.css`
- `frontend/src/components/BotFormModal.tsx` + `BotFormModal.css`
- `frontend/src/components/BotForm.css`, `BotFormField.css`
- `frontend/src/components/ui/PixelSnow.*`, `TargetCursor.*`, `GooeyNav.*`,
  `dock.tsx`, `dock-tabs.tsx`, `grid-beam.tsx`, `bg-pattern.tsx`,
  `tilt-card.tsx`, `interactive-hover-button.tsx`
- `frontend/src/pages/BGPatternDemo.tsx`
- `frontend/src/styles/main.css`
- `frontend/src/App.css` (если не используется после переписывания shell)
- неиспользуемые `assets/react.svg`, `assets/vite.svg` (проверить импорты)

`BotForm.tsx` и `BotFormField.tsx` остаются, но переписываются под новые
примитивы и `app.css`. `AudioTestModal.tsx` остаётся и чинится.

Пакеты `framer-motion`, `gsap`, `three` можно оставить в `package.json` до
отдельной чистки, чтобы не ломать сборку; UI их больше не использует.

## 10. Backend: публичный Twitch-клиент

Файл `internal/twitch/public_stream.go`.

```go
type PublicStreamInfo struct {
    Live        bool
    Title       string
    GameName    string
    ViewerCount int
    StartedAt   time.Time
}

type PublicStreamClient struct { /* http.Client, cache, mutex */ }

func NewPublicStreamClient() *PublicStreamClient
func (c *PublicStreamClient) GetStream(ctx context.Context, channelLogin string) (*PublicStreamInfo, error)
```

- Endpoint `https://gql.twitch.tv/gql`.
- Заголовки: `Client-ID: kimne78kx3ncx6brgo4mv6wki5h1ko`,
  `Content-Type: application/json`.
- Запрос:
  ```graphql
  query($login:String!) {
    user(login:$login) {
      login
      displayName
      stream {
        id
        title
        createdAt
        viewersCount
        game { name }
        type
      }
    }
  }
  ```
- `user == null` → ошибка «channel not found».
- `stream == null` → `Live=false`.
- Таймаут 5 s, кеш 15 s на канал, защита мьютексом.
- Ошибки GQL не валят приложение: вызывающий получает error и показывает
  состояние «нет данных».

## 11. Backend: анонимный IRC-монитор чата

Файл `internal/twitch/chat_monitor.go`.

```go
type Chatter struct {
    Login         string    `json:"login"`
    LastMessageAt time.Time `json:"last_message_at"`
}

type ChatMessage struct {
    Login string    `json:"login"`
    Text  string    `json:"text"`
    At    time.Time `json:"at"`
    IsBot bool      `json:"is_bot"`
}

type ChatMonitor struct { /* conn, map[login]time.Time, mutex, cancel */ }

func NewChatMonitor() *ChatMonitor
func (m *ChatMonitor) SetChannel(channel string) // перезапускает подключение
func (m *ChatMonitor) ActiveChatters(window time.Duration) []Chatter
func (m *ChatMonitor) RecentMessages(limit int) []ChatMessage
func (m *ChatMonitor) Connected() bool
func (m *ChatMonitor) Close()
```

- Подключение: `irc.chat.twitch.tv:6697`, TLS.
- `PASS SCHMOOPIIE`, `NICK justinfan<random>`, `JOIN #channel`.
- Никаких `PRIVMSG` наружу и никакого OAuth.
- Парсинг строки `:login!login@login.tmi.twitch.tv PRIVMSG #channel :text`.
- Хранить `login -> lastMessageAt`, чистить старше 5 минут.
- Хранить последние 100 сообщений (не старше 10 минут) для чат-фида.
- Reconnect с backoff 5 s → максимум 60 s, jitter.
- `SetChannel("")` останавливает монитор.
- Ошибки подключения сохраняются в состоянии (`lastError`), но не логируются
  как шум на каждую попытку.

## 11.1. Presence simulation вместо ротации

Ротация по каналам удаляется: проект для одного стримера. Вместо неё каждый
запущенный бот живёт сессиями.

### Настройки

Новые поля `storage.Settings` (миграция `ALTER TABLE ... ADD COLUMN IF NOT EXISTS`):

| Поле | JSON | Default | Смысл |
|---|---|---|---|
| `presence_enabled` | `presence_enabled` | `TRUE` | включить сессии |
| `presence_session_min_minutes` | `presence_session_min_minutes` | `20` | минимум в эфире |
| `presence_session_max_minutes` | `presence_session_max_minutes` | `40` | максимум в эфире |
| `presence_cooldown_min_minutes` | `presence_cooldown_min_minutes` | `10` | минимум паузы |
| `presence_cooldown_max_minutes` | `presence_cooldown_max_minutes` | `20` | максимум паузы |
| `presence_greeting_enabled` | `presence_greeting_enabled` | `TRUE` | приветствие при заходе |
| `presence_farewell_enabled` | `presence_farewell_enabled` | `TRUE` | прощание при выходе |

Валидация: `1 <= min <= max`, верхняя граница — 240 минут. Некорректные
значения отклоняются API с понятной ошибкой.

`rotation_enabled` и `rotation_interval` остаются в БД для обратной
совместимости, но больше не читаются и не показываются в UI. Код
`startRotation`, `rotationEnabled`, `rotationCancel` удаляется.

### Runtime

`botRuntime` получает:

```go
presenceState  string    // online | cooldown | stopped | error
sessionEndsAt  time.Time
cooldownUntil  time.Time
presenceTimer  *time.Timer
autoPaused     bool
stoppedByError bool
```

Поведение:

1. `StartBot` выбирает случайную длительность сессии в диапазоне
   `[session_min, session_max]`, подключается к IRC, ставит `online`,
   **запускает `go bm.runBotLoop(ctx, br)`** (сейчас этот вызов отсутствует —
   критический баг) и планирует конец сессии.
2. Если `presence_greeting_enabled` и это не debug-режим и не `disableLLM`,
   отправляет короткое приветствие из фиксированного пула фраз.
3. По истечении сессии отправляет прощание (если включено и разрешены
   сообщения), отключается от IRC, ставит `cooldown` и планирует возврат через
   случайную паузу `[cooldown_min, cooldown_max]`. Это делает `pauseBot`:
   отменяет контекст, закрывает IRC и не пишет `enabled=false` в БД.
4. По таймеру паузы бот снова подключается и начинает новую сессию.
5. `StopBot` вручную отменяет таймеры, останавливает loop, пишет
   `enabled=false` и ставит `stopped`; авто-возврата нет.
6. Остановка после `maxConsecutiveErrors` ставит `error`; авто-возврата нет.
7. `StopAll` отменяет все таймеры.
8. Если `presence_enabled` выключили в Settings: отменить запланированные
   возвраты; боты в `cooldown` остаются `stopped` до ручного запуска; боты
   `online` продолжают работать до ручной остановки. Новые значения диапазонов
   применяются к следующей сессии.
9. Presence simulation не влияет на `viewer_count` и не является накруткой:
   это только IRC-подключение и сообщения.
10. Исправление отсутствующего `runBotLoop` входит в эту же задачу и
    проверяется тестом: после `StartBot` бот хотя бы один раз проходит цикл
    генерации; `StopBot` завершается без 10-секундного таймаута.

Приветствия/прощания — фиксированные фразы на русском, по 3–4 варианта,
без LLM. Отправляются через существующий `SendMessage` и учитываются в
`messages_per_min`.

### Состояние в API и UI

`api.BotInfo` получает:

```go
PresenceState string     `json:"presence_state"` // online | cooldown | stopped | error
SessionEndsAt *time.Time `json:"session_ends_at,omitempty"`
CooldownUntil *time.Time `json:"cooldown_until,omitempty"`
```

`Hub.BroadcastBotPresence(botID, login, state, at)` отправляет WebSocket
событие `bot_presence_changed`. UI:
- Bots table показывает `в эфире`, `пауза до HH:MM`, `остановлен`, `ошибка`;
- Stream-чат добавляет системную строку «@login зашёл на стрим» /
  «@login вышел со стрима».

## 12. Backend: StreamMonitor и API

Файл `internal/twitch/stream_monitor.go`.

```go
type StreamStatus struct {
    Configured          bool          `json:"configured"`
    Channel             string        `json:"channel"`
    Live                bool          `json:"live"`
    Title               string        `json:"title"`
    Game                string        `json:"game"`
    StartedAt           *time.Time    `json:"started_at,omitempty"`
    UptimeSeconds       int64         `json:"uptime_seconds"`
    ViewersTotal        int           `json:"viewers_total"`
    ActiveViewers       int           `json:"active_viewers"`
    PassiveViewers      int           `json:"passive_viewers"`
    ActiveWindowSeconds int           `json:"active_window_seconds"`
    ChatConnected       bool          `json:"chat_connected"`
    Chatters            []Chatter     `json:"chatters"`
    Messages            []ChatMessage `json:"messages"`
    UpdatedAt           time.Time     `json:"updated_at"`
    Error               string        `json:"error,omitempty"`
}

func NewStreamMonitor() *StreamMonitor
func (m *StreamMonitor) SetChannels(channels []string) // нормализует, дедупит, запускает/останавливает IRC-мониторы
func (m *StreamMonitor) Channels() []string
func (m *StreamMonitor) Snapshot(ctx context.Context, channel string, excludeLogins map[string]struct{}) StreamStatus
func (m *StreamMonitor) Close()
```

- `SetChannels` поддерживает один read-only IRC-монитор на каждый настроенный канал
  (практический предел — 5 каналов; лишние игнорируются с предупреждением в лог).
- `Snapshot` выбирает канал: явный аргумент, иначе первый настроенный.
- `Snapshot` всегда возвращает объект, ошибка — в поле `Error`.
- `UptimeSeconds = now - StartedAt` для live.
- `ActiveViewers` = уникальные чаттеры за 300 s, кроме `excludeLogins`.
- `PassiveViewers = max(0, ViewersTotal - ActiveViewers)`.
- `Chatters` сортируются по `LastMessageAt` убыв., максимум 20.
- `Messages` — последние 50 сообщений из `ChatMonitor`; `IsBot` выставляется
  по множеству логинов ботов, переданному handler-ом.
- Если канал не задан: `Configured=false`, остальные поля нулевые/пустые.
- Если IRC-монитор канала не подключён: `ChatConnected=false`, active=0,
  passive=ViewersTotal; UI показывает предупреждение.
- Кеш GQL 15 s; IRC-мониторы работают постоянно для выбранных каналов.

### API

`GET /api/twitch/stream?channel=<login>`

- Нормализация: lower-case, срезать `#` и префикс `https://www.twitch.tv/`.
- Без `channel` — первый канал из Settings.
- Если `channel` не входит в настроенные каналы — 400
  `{"error":"channel is not configured"}`.
- Ответ — `StreamStatus` JSON.
- Handler получает список ботов через `manager.ListBots()` и использует их логины
  как `excludeLogins`: они не входят в active viewers и получают `is_bot:true`
  в `messages`.
- 200 даже при offline/ошибке; UI различает состояния по полям.

`GET /api/stats`

- `messages_per_min` становится реальным.
- В `botManager` добавить ring buffer времени успешных отправок (последние
  60 s, максимум 512 записей).
- `MessagesPerMin()` считает записи за последние 60 s.
- Добавить метод в `BotManager` и `MockBotManager`.

## 13. Проводка backend

- `main`: создать `streamMonitor := twitch.NewStreamMonitor()`, `defer Close()`.
- После загрузки Settings вызвать `streamMonitor.SetChannels(settings.Channels)`.
- `api.NewServer`/`NewHandler` принимают `*twitch.StreamMonitor`.
- `botManager` получает тот же указатель; `ApplySettings` обновляет набор каналов
  монитора при изменении `channels`.
- `GetStats` использует `manager.MessagesPerMin()`.
- Регистрация route: `api.HandleFunc("/twitch/stream", handler.GetStreamStatus).Methods("GET")`.

### Файлы backend

Создать:

- `internal/twitch/public_stream.go`
- `internal/twitch/chat_monitor.go`
- `internal/twitch/stream_monitor.go`
- `internal/twitch/public_stream_test.go`
- `internal/twitch/chat_monitor_test.go`
- `internal/twitch/stream_monitor_test.go`

Изменить:

- `logic.go` — создать монитор, обновлять каналы в `ApplySettings`, считать
  успешные отправки сообщений, presence-сессии, удалить ротацию.
- `internal/storage/settings.go` + `settings_migration_test.go` — новые
  presence-поля и миграция.
- `internal/api/handlers.go` — `GetStreamStatus`, `MessagesPerMin` в статистике,
  поля presence в `BotInfo`, потокобезопасное чтение списка ботов.
- `internal/api/websocket.go` — `bot_presence_changed`.
- `internal/api/server.go` — новый route и параметр монитора.
- `internal/api/handlers_test.go` — `MockBotManager.MessagesPerMin`, тесты
  `/api/twitch/stream`, обновление сигнатуры `NewHandler`.
- `README.md` — раздел про Stream, presence simulation, публичный GQL и лимиты.

### Счётчик сообщений

- В `botManager` добавить `messageTimes []time.Time` под существующим мьютексом.
- После успешного `br.client.SendMessage(msg)` (только не в debug mode) вызывать
  `bm.recordMessageSent()`.
- `MessagesPerMin()` возвращает количество записей за последние 60 s.
- Добавить `MessagesPerMin() int` в интерфейс `BotManager` и в
  `MockBotManager`.

## 14. Состояния ошибок и конфигурации

- Нет канала → info-баннер, не ошибка.
- Нет ключа OpenRouter → info-баннер; `/api/llm/status` остаётся источником
  причины, но UI показывает «Ключ не задан», а не «ошибка подключения».
- Нет ботов → empty state с кнопкой.
- GQL/Twitch недоступен → warning в блоке стрима, плеер остаётся.
- Стрим offline → нейтральная заглушка.
- 401 API → «Неверный API_KEY. Проверьте `.env` и пересоберите frontend».
- Сетевые ошибки мутаций → inline alert с retry.
- Все ошибки — на русском, без сырых стеков и без `alert()`.
- ErrorBoundary показывает русский экран с кнопкой перезагрузки.

## 15. Безопасность и приватность

- Удалить console-логирование запросов/ответов и тел ботов.
- OAuth-токены ботов не показывать после сохранения и не логировать.
- Публичный GraphQL Client-ID не является секретом, но путь неофициальный;
  риск задокументирован в README.
- IRC-монитор только читает публичный чат, ничего не отправляет.
- Список чаттеров — публичные логины; хранить только в памяти, не в БД.

## 16. Проверка

1. `go build ./...`, `go test ./...`, `npm run build` — зелёные.
2. `docker compose up -d --build --wait` — все контейнеры healthy.
3. `/api/twitch/stream`:
   - live-канал → uptime > 0, viewers > 0, total = active + passive;
   - offline → `live:false`, метрики пустые, без 500;
   - без канала → `configured:false`.
4. Активные чаттеры считаются при **остановленных** ботах.
5. `/api/stats` показывает `messages_per_min > 0` после отправки сообщений.
6. Presence simulation:
   - сессия короче тестового порога → бот уходит в `cooldown`, затем снова
     `online`;
   - приветствие и прощание появляются в чат-фиде и в Twitch;
   - ручной `StopBot` не возвращает бота;
   - остановка по ошибке не возвращает бота;
   - `presence_enabled=false` отменяет запланированные возвраты.
7. Чат-фид: сообщения ботов подсвечены, обычные — нет; presence-события
   отображаются системными строками.
8. Ротации нет: в UI и логах нет `rotation_enabled`/`rotation_interval`.
9. UI: 4 раздела, светлая тема, плеер играет, нет console-логов с payload,
   нет `alert()`, ошибки конфигурации — не красные.
10. Регрессия: создание/запуск/остановка бота, сохранение Settings, OpenRouter
    статус, аудио-тест.

## 17. Риски

- **Публичный GQL может измениться/заблокироваться.** Клиент изолирован;
  при сбое UI показывает «нет данных». Позже заменяется на Helix с Client ID.
- **Активные зрители приблизительны.** Учитываются только пишущие в чат;
  Twitch не отдаёт полный список зрителей.
- **Twitch embed требует `parent`.** Для `localhost` работает; для другого
  домена фронт подставляет `window.location.hostname`.
- **Публичный Client-ID может нарушать ожидания Twitch.** Владелец выбрал этот
  путь осознанно; код не отправляет секреты и не пишет в чат.
- **Twitch не показывает join/leave в чате.** «Заход/выход» виден в нашем UI,
  а в Twitch — только приветствие/прощание как обычные сообщения.
- **Presence simulation может выглядеть неестественно.** Смягчаем случайными
  диапазонами и пулом фраз; это не накрутка зрителей и не влияет на счётчик
  просмотров.

## 18. Решения, оставленные строителю

- Точные размеры/отступы токенов и сетки.
- Использовать ли CSS-модули или один `app.css`.
- Оставить ли журнал настроек в UI (по умолчанию — свернутый блок).
- Нужен ли график активности чата (по умолчанию — нет).
- Формат времени uptime (`2ч 14м` vs `02:14:33`).
- Точные фразы приветствия/прощания (3–4 варианта на личность).

## 19. Порядок работ

1. Backend: GQL-клиент, IRC-монитор, StreamMonitor, endpoint, messages/min.
2. Backend: presence-поля, миграция, сессии, приветствия/прощания, удаление ротации.
3. Frontend: API-типы и hooks.
4. Frontend: shell, примитивы, четыре страницы, чат-фид.
5. Удаление старых компонентов и стилей.
6. Исправление перечисленных дефектов и ошибок.
7. Сквозная проверка.
