package main

import (
	"bufio"
	"context"
	"fmt"
	"log/slog"
	"math/rand"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"twitch-smm-bot/internal/api"
	"twitch-smm-bot/internal/audio"
	"twitch-smm-bot/internal/cache"
	"twitch-smm-bot/internal/llm"
	"twitch-smm-bot/internal/storage"
	"twitch-smm-bot/internal/twitch"
)

const (
	// Bot error handling
	maxConsecutiveErrors = 5
	errorRetryDelay      = 5 * time.Second

	// Cache timeouts
	cacheOperationTimeout = 2 * time.Second
	messageRetentionTime  = 5 * time.Minute

	// LLM settings
	llmTimeout   = 20 * time.Second
	llmMaxTokens = 60

	// Stats
	maxMessageTimes = 512 // cap for botManager.messageTimes

	// Presence simulation defaults
	defaultPresenceSessionMin  = 20 * time.Minute
	defaultPresenceSessionMax  = 40 * time.Minute
	defaultPresenceCooldownMin = 10 * time.Minute
	defaultPresenceCooldownMax = 20 * time.Minute

	// presenceStopWait bounds how long pauseBot waits for the old loop to
	// exit before giving up on scheduling the cooldown return.
	presenceStopWait = 5 * time.Second
)

// Presence session states for botRuntime.presenceState.
const (
	presenceStateOnline   = "online"
	presenceStateCooldown = "cooldown"
	presenceStateStopped  = "stopped"
	presenceStateError    = "error"
)

// Presence phrase kinds.
const (
	presenceGreeting = "greeting"
	presenceFarewell = "farewell"
)

type promptPair struct {
	system string
	user   string
}

// Personality constants
const (
	PersonalityCheerful = "веселый_фанат"
	PersonalityToxic    = "токсик"
	PersonalityNeutral  = "нейтрал"
	PersonalityHyper    = "хайпер"
)

var personalities = map[string]promptPair{
	PersonalityCheerful: {
		system: "Ты весёлый фанат стримера. Пишешь короткие сообщения в чат Twitch (макс 60 символов). Без эмодзи. Естественный тон.",
		user:   "Напиши позитивный комментарий стримеру или в чат. Максимально коротко.",
	},
	PersonalityToxic: {
		system: "Ты провокатор в чате. Пишешь короткие (макс 50 символов) провокационные, но не оскорбительные сообщения.",
		user:   "Напиши что-то чтобы спровоцировать дискуссию в чате. Коротко.",
	},
	PersonalityNeutral: {
		system: "Ты обычный зритель Twitch. Пишешь короткие вопросы или комментарии (макс 55 символов). Естественно.",
		user:   "Напиши вопрос стримеру или комментарий к происходящему. Коротко.",
	},
	PersonalityHyper: {
		system: "Ты хайповый подписчик. Пишешь энергичные короткие сообщения (макс 50 символов). Используешь сленг.",
		user:   "Напиши что-то энергичное в чат. Коротко, с энтузиазмом.",
	},
}

type globalLimiter struct {
	mu          sync.Mutex
	limit       int
	window      time.Duration
	cacheClient *cache.Client
	events      []time.Time
	maxEvents   int // P0-6: Maximum number of events to store
}

func newGlobalLimiter(limit int, cacheClient *cache.Client) *globalLimiter {
	if limit <= 0 {
		limit = 30
	}
	return &globalLimiter{
		limit:       limit,
		window:      time.Minute,
		cacheClient: cacheClient,
		events:      make([]time.Time, 0, limit),
		maxEvents:   10000, // P0-6: Default max events
	}
}

// P0-6: Cleanup old events to prevent memory leak
// Assumes lock is already held (called from allow() which holds the lock)
func (l *globalLimiter) cleanup() {
	now := time.Now()
	cutoff := now.Add(-l.window)

	// Remove old events
	newEvents := make([]time.Time, 0, len(l.events))
	for _, e := range l.events {
		if e.After(cutoff) {
			newEvents = append(newEvents, e)
		}
	}
	l.events = newEvents

	// P0-6: Also limit total size
	if len(l.events) > l.maxEvents {
		slog.Info("rate limiter: truncating events", "from", len(l.events), "to", l.maxEvents)
		l.events = l.events[len(l.events)-l.maxEvents:]
	}
}

func (l *globalLimiter) allow(now time.Time) bool {
	if l.cacheClient != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()

		count, err := l.cacheClient.IncrementRateLimit(ctx, cache.GlobalRateLimitKey, l.window)
		if err != nil {
			slog.Error("redis rate limit error (fallback to memory)", "error", err)
		} else {
			return count <= int64(l.limit)
		}
	}

	l.mu.Lock()
	defer l.mu.Unlock()

	// P0-6: Cleanup old events first
	l.cleanup()

	if len(l.events) >= l.limit {
		return false
	}

	l.events = append(l.events, now)
	return true
}

// presenceConfig holds the presence simulation settings. Presence makes bots
// behave like humans: they chat in sessions, then take a cooldown break and
// return afterwards. It only ever uses IRC SendMessage — it never touches
// viewer counts or anything else.
type presenceConfig struct {
	enabled     bool
	sessionMin  time.Duration
	sessionMax  time.Duration
	cooldownMin time.Duration
	cooldownMax time.Duration
	greeting    bool
	farewell    bool
}

// presenceTimer cancels a scheduled presence callback. *time.Timer satisfies
// it; tests replace it with a fake.
type presenceTimer interface{ Stop() bool }

// Fixed Russian presence phrases per personality (no LLM involved).
var presenceGreetings = map[string][]string{
	PersonalityCheerful: {
		"Всем привет! Отличный день для стрима!",
		"Привет, чат! Рад всех видеть!",
		"Здарова, банда! Как настроение?",
		"Ну что, погнали смотреть стрим?",
	},
	PersonalityToxic: {
		"Ну и что тут происходит? Посмотрим.",
		"Опять стрим. Ладно, я тут.",
		"Приветик. Постараюсь не уснуть.",
		"С чего начнем? С разочарования?",
	},
	PersonalityNeutral: {
		"Всем привет.",
		"Здарова, чат.",
		"Привет, что тут у нас?",
		"Вечер в хату.",
	},
	PersonalityHyper: {
		"ВВСЕМ ПРИВЕЕЕТ! ПОГНАЛИ!",
		"Я ТУТ! ЭТО БУДЕТ ЛУЧШИЙ СТРИМ!",
		"АААА, ПРИВЕТ ЧАТ! Я ЖДАЛ!",
		"НАКОНЕЦ-ТО! ВСЕМ ПРИВЕТ!",
	},
}

var presenceFarewells = map[string][]string{
	PersonalityCheerful: {
		"Мне пора, всем пока! До встречи!",
		"Побежал, хорошего стрима всем!",
		"Пока-пока! Возвращусь позже.",
		"Всем удачи, до скорого!",
	},
	PersonalityToxic: {
		"Скукота тут. Ухожу.",
		"Ладно, с меня хватит. Пока.",
		"Дальше смотреть нечего. Пока.",
		"Я такое не смотрю. Пока.",
	},
	PersonalityNeutral: {
		"Мне нужно отойти, пока.",
		"Ухожу, до встречи.",
		"Пока, всем хорошего дня.",
		"Отойду на время, вернусь позже.",
	},
	PersonalityHyper: {
		"ПОБЕЖАЛ! СКОРО ВЕРНУСЬ!",
		"ПОКА-ПОКА! ЕЩЕ ВЕРНУСЬ!",
		"МЕНЯ НЕДОЛГО НЕ БУДЕТ! ЖДИТЕ!",
		"ЭТО БЫЛО КРУТО! ПОКА!",
	},
}

// randomPresencePhrase picks a fixed phrase of the given kind ("greeting" or
// "farewell") for the personality, falling back to the neutral pool.
func randomPresencePhrase(personality, kind string) string {
	pool := presenceGreetings
	if kind == presenceFarewell {
		pool = presenceFarewells
	}
	phrases, ok := pool[personality]
	if !ok || len(phrases) == 0 {
		phrases = pool[PersonalityNeutral]
	}
	return phrases[rand.Intn(len(phrases))]
}

// pickPresenceDuration returns a random duration in [min, max] (minute
// granularity), clamping max to min and falling back to the default session
// length when min is not positive.
func pickPresenceDuration(minD, maxD time.Duration) time.Duration {
	if minD <= 0 {
		minD = defaultPresenceSessionMin
	}
	if maxD < minD {
		maxD = minD
	}
	if maxD == minD {
		return minD
	}
	return minD + time.Duration(rand.Intn(int((maxD-minD)/time.Minute)+1))*time.Minute
}

// presenceConfigFromSettings maps storage settings to presenceConfig using
// the built-in defaults for nil or zero values.
func presenceConfigFromSettings(settings *storage.Settings) presenceConfig {
	cfg := presenceConfig{
		enabled:     true,
		sessionMin:  defaultPresenceSessionMin,
		sessionMax:  defaultPresenceSessionMax,
		cooldownMin: defaultPresenceCooldownMin,
		cooldownMax: defaultPresenceCooldownMax,
		greeting:    true,
		farewell:    true,
	}
	if settings == nil {
		return cfg
	}

	cfg.enabled = settings.PresenceEnabled
	cfg.greeting = settings.PresenceGreetingEnabled
	cfg.farewell = settings.PresenceFarewellEnabled

	if minutes := settings.PresenceSessionMinMinutes; minutes > 0 {
		cfg.sessionMin = time.Duration(minutes) * time.Minute
	}
	if minutes := settings.PresenceSessionMaxMinutes; minutes > 0 {
		cfg.sessionMax = time.Duration(minutes) * time.Minute
	}
	if minutes := settings.PresenceCooldownMinMinutes; minutes > 0 {
		cfg.cooldownMin = time.Duration(minutes) * time.Minute
	}
	if minutes := settings.PresenceCooldownMaxMinutes; minutes > 0 {
		cfg.cooldownMax = time.Duration(minutes) * time.Minute
	}
	if cfg.sessionMax < cfg.sessionMin {
		cfg.sessionMax = cfg.sessionMin
	}
	if cfg.cooldownMax < cfg.cooldownMin {
		cfg.cooldownMax = cfg.cooldownMin
	}
	return cfg
}

// defaultPresenceAfter adapts time.AfterFunc to the presenceTimer interface.
func defaultPresenceAfter(d time.Duration, fn func()) presenceTimer {
	return time.AfterFunc(d, fn)
}

type TwitchClient interface {
	Connect() error
	IsConnected() bool
	SendMessage(text string) error
	Reconnect() error
	Close()
	WaitForRateLimit() // P0-2: Handle rate limits
}

type botRuntime struct {
	cfg        storage.BotConfig
	client     TwitchClient
	ctx        context.Context
	cancel     context.CancelFunc
	running    bool
	stopping   bool // P0-1: Graceful shutdown flag
	lastSent   time.Time
	disableLLM bool          // if true, skip LLM/STT generation
	stopped    chan struct{} // P0-1: Channel to signal when bot fully stopped

	// Presence simulation state.
	presenceState  string        // online | cooldown | stopped | error
	sessionEndsAt  time.Time     // end of the current presence session
	cooldownUntil  time.Time     // when a paused bot returns
	presenceTimer  presenceTimer // cancels the pending session end or return
	autoPaused     bool          // paused by the presence session end
	stoppedByError bool          // stopped by consecutive errors
}

type botManager struct {
	mu             sync.RWMutex // P0-5: RWMutex for read-heavy operations
	runtimeCtx     context.Context
	store          *storage.Store
	llmClient      *llm.Client
	cacheClient    *cache.Client
	helixClient    *twitch.HelixClient
	streamMonitor  *twitch.StreamMonitor
	channel        string
	audioConfig    audio.Config
	audioService   *audio.Service
	audioCancel    context.CancelFunc
	globalLimiter  *globalLimiter
	bots           map[string]*botRuntime
	apiHub         *api.Hub
	debugMode      bool
	positivePrompt string
	negativePrompt string
	maxTokens      int
	messageTimes   []time.Time // successful Twitch sends, for MessagesPerMin
	newIRCClient   func(login, password, channel string) TwitchClient

	// Presence simulation.
	presence      presenceConfig
	presenceAfter func(time.Duration, func()) presenceTimer // default: defaultPresenceAfter
}

func normalizeChannelName(raw string) string {
	ch := strings.TrimSpace(raw)

	// Если это URL, извлекаем имя канала
	if strings.Contains(ch, "twitch.tv/") {
		parts := strings.Split(ch, "twitch.tv/")
		if len(parts) > 1 {
			ch = parts[len(parts)-1]
		}
	}

	// Убираем всё что после следующего / (если есть путь)
	if idx := strings.Index(ch, "/"); idx != -1 {
		ch = ch[:idx]
	}

	ch = strings.TrimPrefix(ch, "#")
	return strings.ToLower(ch)
}

func primaryChannelFromSettings(settings *storage.Settings, fallback string) string {
	if settings != nil {
		// Если включен режим отладки и указан debug_channel - он приоритетнее
		if settings.DebugMode && settings.DebugChannel != "" {
			ch := normalizeChannelName(settings.DebugChannel)
			if ch != "" {
				return ch
			}
		}
		// Иначе берем первый канал из списка
		for _, candidate := range settings.Channels {
			ch := normalizeChannelName(candidate)
			if ch != "" {
				return ch
			}
		}
	}
	return normalizeChannelName(fallback)
}

func newBotManager(
	store *storage.Store,
	llmClient *llm.Client,
	cacheClient *cache.Client,
	helixClient *twitch.HelixClient,
	runtimeCtx context.Context,
	channel string,
	globalLimit int,
	audioCfg audio.Config,
	debugMode bool,
	streamMonitor *twitch.StreamMonitor,
) *botManager {
	if runtimeCtx == nil {
		runtimeCtx = context.Background()
	}
	return &botManager{
		runtimeCtx:    runtimeCtx,
		store:         store,
		llmClient:     llmClient,
		cacheClient:   cacheClient,
		helixClient:   helixClient,
		streamMonitor: streamMonitor,
		channel:       channel,
		audioConfig:   audioCfg,
		globalLimiter: newGlobalLimiter(globalLimit, cacheClient),
		bots:          make(map[string]*botRuntime),
		debugMode:     debugMode,
		maxTokens:     llmMaxTokens,
		newIRCClient: func(login, password, channel string) TwitchClient {
			return twitch.NewIRCClient(login, password, channel)
		},
		presence: presenceConfig{
			enabled:     true,
			sessionMin:  defaultPresenceSessionMin,
			sessionMax:  defaultPresenceSessionMax,
			cooldownMin: defaultPresenceCooldownMin,
			cooldownMax: defaultPresenceCooldownMax,
			greeting:    true,
			farewell:    true,
		},
		presenceAfter: defaultPresenceAfter,
	}
}

func (bm *botManager) setAPIHub(hub *api.Hub) {
	bm.apiHub = hub
}

// broadcastPresence publishes a bot's current presence state to the API
// WebSocket hub. The hub and the runtime snapshot are read under bm.mu, but
// the hub call happens outside the lock: broadcast fans out to client
// channels and must never run while bm.mu is held. The payload carries only
// public data (bot id, twitch login, state) and never secrets.
func (bm *botManager) broadcastPresence(br *botRuntime) {
	bm.mu.RLock()
	hub := bm.apiHub
	botID := br.cfg.ID
	login := br.cfg.TwitchLogin
	state := br.presenceState
	bm.mu.RUnlock()

	if hub == nil || state == "" {
		return
	}
	hub.BroadcastBotPresence(botID, login, state, time.Now())
}

func (bm *botManager) logBotEvent(botID, logType, level, message string, contextData map[string]interface{}) {
	log := storage.BotLog{
		BotID:     botID,
		LogType:   logType,
		Level:     level,
		Message:   message,
		Context:   contextData,
		CreatedAt: time.Now(),
	}

	// Write to PostgreSQL (async, don't block bot loop)
	if bm.store != nil {
		go func() {
			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
			defer cancel()
			_ = bm.store.CreateBotLog(ctx, log)
		}()
	}

	// Write to Redis cache (async)
	if bm.cacheClient != nil {
		go func() {
			ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
			defer cancel()
			_ = bm.cacheClient.AddBotLog(ctx, botID, log)
		}()
	}

	// Broadcast via WebSocket (immediate)
	if bm.apiHub != nil {
		bm.apiHub.BroadcastBotLog(botID, log)
	}
}

func (bm *botManager) loadBots(ctx context.Context) error {
	cfgs, err := bm.store.ListBots(ctx)
	if err != nil {
		return err
	}

	bm.mu.Lock()
	defer bm.mu.Unlock()

	bm.bots = make(map[string]*botRuntime, len(cfgs))
	for _, cfg := range cfgs {
		// Используем current_channel если задан, иначе дефолтный канал
		channel := normalizeChannelName(cfg.CurrentChannel)
		if channel == "" {
			channel = normalizeChannelName(bm.channel)
		}
		bm.bots[cfg.ID] = &botRuntime{
			cfg:    cfg,
			client: bm.newIRCClient(cfg.TwitchLogin, cfg.Password, channel),
		}
	}
	return nil
}

func (bm *botManager) startEnabled(ctx context.Context) {
	ids := bm.listIDs(true)
	for _, id := range ids {
		if err := bm.startBot(ctx, id, false, false); err != nil {
			slog.Error("start bot failed", "id", id, "error", err)
		}
	}
}

func (bm *botManager) listIDs(enabledOnly bool) []string {
	bm.mu.RLock() // P0-5: RLock for read-only
	defer bm.mu.RUnlock()

	ids := make([]string, 0, len(bm.bots))
	for id, br := range bm.bots {
		if enabledOnly && !br.cfg.Enabled {
			continue
		}
		ids = append(ids, id)
	}
	return ids
}

// botInfo snapshots the runtime into an api.BotInfo. The caller must hold
// bm.mu (read or write): the presence fields are guarded by it. A runtime
// that never started reports the stopped presence state.
func (br *botRuntime) botInfo() api.BotInfo {
	state := br.presenceState
	if state == "" {
		state = presenceStateStopped
	}

	info := api.BotInfo{
		Config:        br.cfg,
		Running:       br.running,
		PresenceState: state,
	}
	if !br.sessionEndsAt.IsZero() {
		endsAt := br.sessionEndsAt
		info.SessionEndsAt = &endsAt
	}
	if !br.cooldownUntil.IsZero() {
		until := br.cooldownUntil
		info.CooldownUntil = &until
	}
	return info
}

func (bm *botManager) ListBots() []api.BotInfo {
	bm.mu.RLock() // P0-5: RLock for read-only
	defer bm.mu.RUnlock()

	infos := make([]api.BotInfo, 0, len(bm.bots))
	for _, br := range bm.bots {
		infos = append(infos, br.botInfo())
	}
	return infos
}

func (bm *botManager) GetBot(id string) (*api.BotInfo, error) {
	bm.mu.RLock() // P0-5: RLock for read-only
	defer bm.mu.RUnlock()

	br, ok := bm.bots[id]
	if !ok {
		return nil, fmt.Errorf("bot not found: %s", id)
	}

	info := br.botInfo()
	return &info, nil
}

func (bm *botManager) StartBot(ctx context.Context, id string) error {
	return bm.startBot(ctx, id, true, false)
}

func (bm *botManager) StartBotWithOptions(ctx context.Context, id string, disableLLM bool) error {
	return bm.startBot(ctx, id, true, disableLLM)
}

func (bm *botManager) StopBot(id string) error {
	return bm.stopBot(id, true)
}

func (bm *botManager) CreateBot(ctx context.Context, cfg storage.BotConfig) error {
	// P0-3: Validate Twitch account before creating
	if bm.helixClient != nil {
		if err := bm.helixClient.ValidateAccount(ctx, cfg.TwitchLogin, cfg.Password); err != nil {
			return fmt.Errorf("account validation failed: %w", err)
		}
	}

	if err := bm.store.CreateBot(ctx, cfg); err != nil {
		return err
	}

	channel := normalizeChannelName(cfg.CurrentChannel)
	if channel == "" {
		channel = normalizeChannelName(bm.channel)
	}

	bm.mu.Lock()
	defer bm.mu.Unlock()
	bm.bots[cfg.ID] = &botRuntime{
		cfg:    cfg,
		client: bm.newIRCClient(cfg.TwitchLogin, cfg.Password, channel),
	}

	return nil
}

func (bm *botManager) UpdateBot(ctx context.Context, botID string, updates map[string]interface{}) error {
	// P0-3: Validate account if login or password is being updated
	if bm.helixClient != nil {
		var login, password string
		var needValidation bool

		if v, ok := updates["twitch_login"].(string); ok {
			login = v
			needValidation = true
		}
		if v, ok := updates["password"].(string); ok {
			password = v
			needValidation = true
		}

		if needValidation {
			if login == "" || password == "" {
				// Get current values for missing fields
				bm.mu.Lock()
				if br, ok := bm.bots[botID]; ok {
					if login == "" {
						login = br.cfg.TwitchLogin
					}
					if password == "" {
						password = br.cfg.Password
					}
				}
				bm.mu.Unlock()
			}

			if err := bm.helixClient.ValidateAccount(ctx, login, password); err != nil {
				return fmt.Errorf("account validation failed: %w", err)
			}
		}
	}

	if err := bm.store.UpdateBot(ctx, botID, updates); err != nil {
		return err
	}

	bm.mu.Lock()
	defer bm.mu.Unlock()

	br, ok := bm.bots[botID]
	if !ok {
		return nil
	}

	if v, ok := updates["name"].(string); ok {
		br.cfg.Name = v
	}
	if v, ok := updates["personality"].(string); ok {
		br.cfg.Personality = v
	}
	if v, ok := updates["min_delay_sec"].(int); ok {
		br.cfg.MinDelaySec = v
	}
	if v, ok := updates["max_delay_sec"].(int); ok {
		br.cfg.MaxDelaySec = v
	}
	if v, ok := updates["rate_limit_per_min"].(int); ok {
		br.cfg.RateLimitPerMin = v
	}
	if v, ok := updates["enabled"].(bool); ok {
		br.cfg.Enabled = v
	}
	if v, ok := updates["current_channel"].(string); ok {
		br.cfg.CurrentChannel = v
	}

	return nil
}

func (bm *botManager) DeleteBot(ctx context.Context, botID string) error {
	bm.mu.Lock()
	br, ok := bm.bots[botID]
	if ok && br.presenceTimer != nil {
		// Deleting cancels any pending presence session or cooldown return.
		br.presenceTimer.Stop()
		br.presenceTimer = nil
	}
	bm.mu.Unlock()
	if ok && br.running {
		if err := bm.stopBot(botID, false); err != nil {
			return err
		}
	}

	if err := bm.store.DeleteBot(ctx, botID); err != nil {
		return err
	}

	bm.mu.Lock()
	delete(bm.bots, botID)
	bm.mu.Unlock()

	return nil
}

func (bm *botManager) StartAll(ctx context.Context) error {
	ids := bm.listIDs(false)
	for _, id := range ids {
		if err := bm.startBot(ctx, id, true, false); err != nil {
			slog.Error("start bot failed", "id", id, "error", err)
		}
	}
	return nil
}

func (bm *botManager) StopAll() error {
	bm.stopAll()
	return nil
}

func (bm *botManager) startBot(parent context.Context, botID string, persist bool, disableLLM bool) error {
	bm.mu.Lock()
	br, ok := bm.bots[botID]
	if !ok {
		bm.mu.Unlock()
		return fmt.Errorf("bot not found: %s", botID)
	}
	if br.running {
		bm.mu.Unlock()
		return nil
	}

	// Set LLM disable flag
	br.disableLLM = disableLLM

	targetChannel := normalizeChannelName(br.cfg.CurrentChannel)
	if targetChannel == "" {
		targetChannel = normalizeChannelName(bm.channel)
	}
	if targetChannel == "" {
		bm.mu.Unlock()
		return fmt.Errorf("no twitch channel configured: set Settings -> channels in UI")
	}

	// Пересоздаем клиент на актуальный канал перед запуском.
	br.client = bm.newIRCClient(br.cfg.TwitchLogin, br.cfg.Password, targetChannel)

	// Подключаемся к Twitch IRC
	if err := br.client.Connect(); err != nil {
		bm.mu.Unlock()
		return fmt.Errorf("failed to connect to Twitch IRC: %w", err)
	}

	// P0-1: Reset graceful shutdown state
	br.stopping = false
	br.stopped = make(chan struct{}, 1)

	ctx, cancel := context.WithCancel(parent)
	br.ctx = ctx
	br.cancel = cancel
	br.running = true
	br.cfg.Enabled = true
	bm.mu.Unlock()

	go bm.runBotLoop(ctx, br)

	bm.beginPresenceSession(botID, disableLLM)
	bm.broadcastPresence(br)

	if persist {
		if err := bm.store.SetBotEnabled(context.Background(), botID, true); err != nil {
			bm.stopBot(botID, false)
			return err
		}
	}

	return nil
}

// beginPresenceSession marks a freshly started bot as online and schedules
// the end of its presence session. When presence is disabled the bot simply
// stays online with no timer. The greeting is sent outside the manager lock.
func (bm *botManager) beginPresenceSession(botID string, disableLLM bool) {
	bm.mu.Lock()
	br, ok := bm.bots[botID]
	if !ok {
		bm.mu.Unlock()
		return
	}

	// Re-check running under the lock: a concurrent stop may have landed
	// between the start and this call. A stopped bot must not go online or
	// register a session timer.
	if !br.running {
		bm.mu.Unlock()
		return
	}

	br.presenceState = presenceStateOnline
	br.autoPaused = false
	br.stoppedByError = false
	br.cooldownUntil = time.Time{}

	client := br.client
	personality := br.cfg.Personality
	sessionEnabled := bm.presence.enabled
	greetingEnabled := sessionEnabled && bm.presence.greeting && !bm.debugMode && !disableLLM
	debugMode := bm.debugMode
	var session time.Duration
	if sessionEnabled {
		session = pickPresenceDuration(bm.presence.sessionMin, bm.presence.sessionMax)
		br.sessionEndsAt = time.Now().Add(session)
	} else {
		br.sessionEndsAt = time.Time{}
	}

	// Register the session timer while holding the lock so a concurrent stop
	// can always observe (and cancel) it. The real adapter never runs the
	// callback synchronously, so this cannot deadlock.
	var timer presenceTimer
	if sessionEnabled && bm.presenceAfter != nil {
		timer = bm.presenceAfter(session, func() { bm.pauseBot(botID) })
		if br.presenceTimer != nil {
			br.presenceTimer.Stop()
		}
		br.presenceTimer = timer
	}
	bm.mu.Unlock()

	// Greeting message: presence only ever sends via IRC and never touches
	// viewer counts.
	if greetingEnabled {
		msg := randomPresencePhrase(personality, presenceGreeting)
		if err := client.SendMessage(msg); err != nil {
			slog.Warn("presence greeting send failed", "bot_id", botID, "debug_mode", debugMode, "error", err)
		} else {
			bm.recordMessageSent()
		}
	}
}

// pauseBot ends the current presence session: it sends the farewell, closes
// the client and schedules the bot's return after a cooldown. It is a no-op
// unless the bot is running inside an active (online) session. Only this
// completed-session path schedules a return; manual stops, error stops and
// StopAll cancel it instead.
func (bm *botManager) pauseBot(botID string) {
	bm.mu.Lock()
	br, ok := bm.bots[botID]
	if !ok || !br.running || br.presenceState != presenceStateOnline {
		bm.mu.Unlock()
		return
	}

	client := br.client
	personality := br.cfg.Personality
	stopped := br.stopped
	disableLLM := br.disableLLM
	farewellEnabled := bm.presence.farewell && !bm.debugMode && !disableLLM
	after := bm.presenceAfter
	cooldown := pickPresenceDuration(bm.presence.cooldownMin, bm.presence.cooldownMax)

	br.autoPaused = true
	br.running = false
	br.presenceState = presenceStateCooldown
	br.sessionEndsAt = time.Time{}
	br.cooldownUntil = time.Now().Add(cooldown)
	if br.presenceTimer != nil {
		br.presenceTimer.Stop()
		br.presenceTimer = nil
	}
	if br.cancel != nil {
		br.cancel()
	}
	bm.mu.Unlock()

	bm.broadcastPresence(br)

	// Farewell before the client is closed; network I/O must not happen
	// while bm.mu is held.
	if farewellEnabled {
		msg := randomPresencePhrase(personality, presenceFarewell)
		if err := client.SendMessage(msg); err != nil {
			slog.Warn("presence farewell send failed", "bot_id", botID, "error", err)
		} else {
			bm.recordMessageSent()
		}
	}
	client.Close()

	// Give the old loop time to fully exit so the return cannot produce a
	// second concurrent runtime. If it is still alive after the timeout, skip
	// the return entirely rather than risk a duplicate loop.
	if stopped != nil {
		select {
		case <-stopped:
		case <-time.After(presenceStopWait):
			slog.Warn("presence: previous bot loop still running, skipping cooldown return", "bot_id", botID)
			return
		}
	}

	bm.mu.Lock()
	ctx := bm.runtimeCtx
	if ctx == nil {
		ctx = context.Background()
	}
	returnEnabled := bm.presence.enabled
	bm.mu.Unlock()

	if after == nil || !returnEnabled {
		return
	}
	wasDisableLLM := disableLLM
	after(cooldown, func() { bm.resumeAfterCooldown(botID, wasDisableLLM) })
}

// resumeAfterCooldown returns a bot from its presence cooldown. Only a bot
// that is still parked in cooldown (nobody stopped, errored or manually
// restarted it in the meantime) comes back.
func (bm *botManager) resumeAfterCooldown(botID string, wasDisableLLM bool) {
	bm.mu.Lock()
	br, ok := bm.bots[botID]
	if !ok || br.presenceState != presenceStateCooldown || br.stoppedByError || !bm.presence.enabled || br.running {
		bm.mu.Unlock()
		return
	}
	ctx := bm.runtimeCtx
	if ctx == nil {
		ctx = context.Background()
	}
	bm.mu.Unlock()

	if err := bm.startBot(ctx, botID, false, wasDisableLLM); err != nil {
		slog.Error("presence: failed to resume bot after cooldown", "bot_id", botID, "error", err)
	}
}

// markBotError flags a runtime that stopped because of too many consecutive
// errors: no cooldown return may be scheduled afterwards. The runtime reports
// stopped (running=false) so the UI shows it and startBot may run again —
// only a manual restart brings an errored bot back. cfg.Enabled stays true:
// the bot is still desired, just errored.
func (bm *botManager) markBotError(br *botRuntime) {
	bm.mu.Lock()
	br.presenceState = presenceStateError
	br.stoppedByError = true
	br.running = false
	br.sessionEndsAt = time.Time{}
	if br.presenceTimer != nil {
		br.presenceTimer.Stop()
		br.presenceTimer = nil
	}
	bm.mu.Unlock()

	bm.broadcastPresence(br)
}

func (bm *botManager) stopAudio() {
	bm.mu.Lock()
	enabled := bm.audioService != nil
	if bm.audioCancel != nil {
		bm.audioCancel()
		bm.audioCancel = nil
	}
	bm.audioService = nil
	bm.mu.Unlock()

	if enabled {
		slog.Info("Audio context service disabled")
	}
}

func (bm *botManager) stopBot(botID string, persist bool) error {
	bm.mu.Lock()
	br, ok := bm.bots[botID]
	if !ok {
		bm.mu.Unlock()
		return fmt.Errorf("bot not found: %s", botID)
	}

	// Any stop cancels a pending presence session or cooldown return: only a
	// completed presence session may schedule a return.
	if br.presenceTimer != nil {
		br.presenceTimer.Stop()
		br.presenceTimer = nil
	}
	br.presenceState = presenceStateStopped
	br.sessionEndsAt = time.Time{}
	br.cooldownUntil = time.Time{}
	br.autoPaused = false
	br.stoppedByError = false

	// P0-1: Graceful shutdown - set stopping flag and wait for loop to finish
	br.stopping = true

	// Whether the loop was actually running: only then can it signal br.stopped.
	wasRunning := br.running

	// Create channel if not exists (for first stop)
	if br.stopped == nil {
		br.stopped = make(chan struct{}, 1)
	}

	// Cancel context to signal the loop to stop
	if br.cancel != nil {
		br.cancel()
	}
	br.running = false
	br.cancel = nil
	br.cfg.Enabled = false
	br.client.Close()
	bm.mu.Unlock()

	bm.broadcastPresence(br)

	// P0-1: Wait for graceful shutdown with timeout. Only when the loop was
	// actually running: a bot whose loop never started cannot signal br.stopped,
	// so waiting here would always block for the full timeout.
	if wasRunning && br.stopped != nil {
		gracefulTimeout := 10 * time.Second
		select {
		case <-br.stopped:
			slog.Info("graceful shutdown completed", "bot_id", botID)
		case <-time.After(gracefulTimeout):
			slog.Warn("graceful shutdown timeout, forcing stop", "bot_id", botID)
		}
	}

	if persist {
		return bm.store.SetBotEnabled(context.Background(), botID, false)
	}
	return nil
}

func (bm *botManager) stopAll() {
	ids := bm.listIDs(false)
	for _, id := range ids {
		_ = bm.stopBot(id, false)
	}
}

func (bm *botManager) statusLines() []string {
	bm.mu.RLock() // P0-5: RLock for read-only
	defer bm.mu.RUnlock()

	lines := make([]string, 0, len(bm.bots))
	for _, br := range bm.bots {
		state := "stopped"
		if br.running {
			state = "running"
		}
		lines = append(lines, fmt.Sprintf("%s (%s): %s, limit=%d/min", br.cfg.ID, br.cfg.TwitchLogin, state, br.cfg.RateLimitPerMin))
	}
	return lines
}

func (bm *botManager) runBotLoop(ctx context.Context, br *botRuntime) {
	// Capture this run's stop channel: startBot creates a fresh channel per
	// start, so closing the captured value can never double-close the channel
	// of a newer run that started after this loop timed out.
	stopped := br.stopped

	slog.Info("bot started", "bot_id", br.cfg.ID)
	bm.logBotEvent(br.cfg.ID, "lifecycle", "info", "Bot started", nil)

	// P0-1: Graceful shutdown - ensure we signal when done
	defer func() {
		// P0-1: Log reason for stopping
		reason := "stopped"
		if br.stopping {
			reason = "stopped gracefully (user request)"
		}

		slog.Info("bot stopped", "bot_id", br.cfg.ID, "reason", reason)
		bm.logBotEvent(br.cfg.ID, "lifecycle", "info", reason, nil)

		// P0-1: Signal that bot has fully stopped.
		// This is the only place the stop channel is closed.
		if stopped != nil {
			close(stopped)
		}
	}()

	consecutiveErrors := 0

	for {
		// P0-1: Check stopping flag for graceful shutdown
		bm.mu.Lock()
		shouldStop := br.stopping
		bm.mu.Unlock()

		if shouldStop {
			return
		}

		select {
		case <-ctx.Done():
			return
		default:
		}

		delay := randomDelay(br.cfg.MinDelaySec, br.cfg.MaxDelaySec)
		timer := time.NewTimer(delay)
		select {
		case <-ctx.Done():
			timer.Stop()
			return
		case <-timer.C:
		}

		// Проверка и переподключение IRC если нужно
		if !br.client.IsConnected() {
			slog.Warn("IRC disconnected, reconnecting...", "bot_id", br.cfg.ID)
			bm.logBotEvent(br.cfg.ID, "error", "warning", "IRC disconnected, reconnecting...", nil)

			if err := br.client.Reconnect(); err != nil {
				slog.Error("reconnect failed", "bot_id", br.cfg.ID, "error", err)
				consecutiveErrors++
				if consecutiveErrors >= maxConsecutiveErrors {
					slog.Error("too many errors, stopping bot", "bot_id", br.cfg.ID)
					bm.markBotError(br)
					return
				}
				time.Sleep(errorRetryDelay)
				continue
			}
			slog.Info("reconnected successfully", "bot_id", br.cfg.ID)
			bm.logBotEvent(br.cfg.ID, "lifecycle", "success", "Reconnected successfully", nil)
			consecutiveErrors = 0
		}

		interval := minInterval(br.cfg.RateLimitPerMin)
		if !br.lastSent.IsZero() {
			nextAllowed := br.lastSent.Add(interval)
			if now := time.Now(); nextAllowed.After(now) {
				sleep := time.NewTimer(nextAllowed.Sub(now))
				select {
				case <-ctx.Done():
					sleep.Stop()
					return
				case <-sleep.C:
				}
			}
		}

		now := time.Now()
		if !bm.globalLimiter.allow(now) {
			slog.Warn("global limit reached, skip", "bot_id", br.cfg.ID)
			bm.logBotEvent(br.cfg.ID, "rate_limit", "warning", "Global rate limit reached", nil)
			continue
		}

		// Если LLM отключен, пропускаем генерацию и отправку
		if br.disableLLM {
			slog.Info("LLM disabled, skipping message generation", "bot_id", br.cfg.ID)
			bm.logBotEvent(br.cfg.ID, "llm", "info", "LLM disabled, bot in monitor-only mode", nil)
			monitorDelay := time.Duration(br.cfg.MinDelaySec) * time.Second
			if span := br.cfg.MaxDelaySec - br.cfg.MinDelaySec; span > 0 {
				monitorDelay += time.Duration(rand.Intn(span)) * time.Second
			}
			if monitorDelay <= 0 {
				monitorDelay = time.Second
			}
			monitorTimer := time.NewTimer(monitorDelay)
			select {
			case <-ctx.Done():
				monitorTimer.Stop()
				return
			case <-monitorTimer.C:
			}
			continue
		}

		msg, sttContext, err := bm.generateMessage(ctx, br.cfg.Personality)
		if err != nil {
			slog.Error("llm error", "bot_id", br.cfg.ID, "error", err)
			bm.logBotEvent(br.cfg.ID, "llm", "error", "LLM generation failed", map[string]interface{}{
				"error":       err.Error(),
				"personality": br.cfg.Personality,
			})
			consecutiveErrors++
			if consecutiveErrors >= maxConsecutiveErrors {
				slog.Error("too many LLM errors, stopping bot", "bot_id", br.cfg.ID)
				bm.markBotError(br)
				return
			}
			continue
		}

		// Успешная генерация - сбрасываем счетчик ошибок и обновляем время
		consecutiveErrors = 0
		br.lastSent = now
		slog.Info("message sent", "bot_id", br.cfg.ID, "personality", br.cfg.Personality, "message", msg)

		// Логируем и отправляем через WebSocket ДО отправки в Twitch
		contextData := map[string]interface{}{
			"personality": br.cfg.Personality,
			"channel":     normalizeChannelName(br.cfg.CurrentChannel),
			"debug_mode":  bm.debugMode,
		}
		if sttContext != "" {
			contextData["stt_context"] = sttContext
		}
		bm.logBotEvent(br.cfg.ID, "message", "success", msg, contextData)

		if bm.cacheClient != nil {
			cacheCtx, cancel := context.WithTimeout(ctx, cacheOperationTimeout)
			_ = bm.cacheClient.AddMessage(cacheCtx, br.cfg.ID, msg, messageRetentionTime)
			_ = bm.cacheClient.AddBotMessageLog(cacheCtx, br.cfg.ID, msg, br.cfg.Personality)
			_ = bm.cacheClient.IncrementBotStats(cacheCtx, br.cfg.ID)
			cancel()
		}

		if bm.apiHub != nil {
			bm.apiHub.BroadcastBotMessage(br.cfg.ID, msg, br.cfg.Personality, normalizeChannelName(br.cfg.CurrentChannel))
		}

		// P0-2: Check and wait for rate limits before sending
		br.client.WaitForRateLimit()

		// В режиме отладки не отправляем сообщения в реальный чат
		if !bm.debugMode {
			if err := br.client.SendMessage(msg); err != nil {
				slog.Error("twitch send error", "bot_id", br.cfg.ID, "error", err)

				// P0-2: Check if rate limited
				if strings.Contains(err.Error(), "rate limited") {
					bm.logBotEvent(br.cfg.ID, "rate_limit", "warning", "Twitch rate limit reached", map[string]interface{}{
						"error": err.Error(),
					})
					// Don't count rate limit as error, just wait and continue
					time.Sleep(5 * time.Second)
					continue
				}

				br.client.Close()
				consecutiveErrors++
				if consecutiveErrors >= maxConsecutiveErrors {
					slog.Error("too many send errors, stopping bot", "bot_id", br.cfg.ID)
					bm.markBotError(br)
					return
				}
				continue
			}

			// Only successful real-chat sends count toward messages/min.
			bm.recordMessageSent()
		} else {
			slog.Info("DEBUG MODE: message not sent to Twitch", "bot_id", br.cfg.ID)
		}
	}
}

// recordMessageSent notes a successful Twitch send for the MessagesPerMin
// stat. Entries older than 60 seconds are pruned and the slice is capped.
func (bm *botManager) recordMessageSent() {
	bm.mu.Lock()
	defer bm.mu.Unlock()

	now := time.Now()
	bm.messageTimes = append(bm.messageTimes, now)

	cutoff := now.Add(-time.Minute)
	idx := 0
	for idx < len(bm.messageTimes) && bm.messageTimes[idx].Before(cutoff) {
		idx++
	}
	if idx > 0 {
		bm.messageTimes = bm.messageTimes[idx:]
	}

	if len(bm.messageTimes) > maxMessageTimes {
		bm.messageTimes = bm.messageTimes[len(bm.messageTimes)-maxMessageTimes:]
	}
}

// MessagesPerMin reports the number of successful Twitch sends in the last
// 60 seconds.
func (bm *botManager) MessagesPerMin() int {
	bm.mu.RLock()
	defer bm.mu.RUnlock()

	cutoff := time.Now().Add(-time.Minute)
	count := 0
	for _, ts := range bm.messageTimes {
		if ts.After(cutoff) {
			count++
		}
	}
	return count
}

func (bm *botManager) generateMessage(ctx context.Context, personality string) (string, string, error) {
	p, ok := personalities[personality]
	if !ok {
		p = personalities[PersonalityNeutral]
	}

	sttContext := ""
	bm.mu.Lock()
	audioService := bm.audioService
	helixClient := bm.helixClient
	channel := bm.channel
	positivePrompt := strings.TrimSpace(bm.positivePrompt)
	negativePrompt := strings.TrimSpace(bm.negativePrompt)
	maxTokens := bm.maxTokens
	bm.mu.Unlock()

	if maxTokens <= 0 {
		maxTokens = llmMaxTokens
	}

	// Получаем информацию о стриме (категория/игра)
	var streamInfo *twitch.StreamInfo
	if helixClient != nil {
		infoCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
		defer cancel()

		info, err := helixClient.GetStreamInfo(infoCtx, channel)
		if err != nil {
			slog.Error("Failed to get stream info", "error", err)
		} else {
			streamInfo = info
		}
	}

	systemPrompt := p.system
	if positivePrompt != "" {
		systemPrompt += "\n\nОбщие инструкции: " + positivePrompt
	}
	if negativePrompt != "" {
		systemPrompt += "\n\nОграничения: " + negativePrompt
	}

	userPrompt := p.user
	// Добавляем контекст из аудио транскрипции
	if audioService != nil {
		streamContext := strings.TrimSpace(audioService.GetRecentContext())
		if streamContext != "" {
			sttContext = streamContext
			userPrompt = p.user + "\n\nКонтекст из недавней транскрипции стрима (может быть неточным):\n" +
				streamContext +
				"\n\nИспользуй это как подсказку по теме и напиши короткое сообщение."
		}
	}

	// Добавляем информацию о категории/игре
	if streamInfo != nil {
		if streamInfo.IsLive && streamInfo.GameName != "" {
			userPrompt += fmt.Sprintf("\n\nСтример сейчас играет в: %s", streamInfo.GameName)
			if streamInfo.Title != "" {
				userPrompt += fmt.Sprintf("\nНазвание стрима: %s", streamInfo.Title)
			}
		} else if streamInfo.GameName != "" {
			userPrompt += fmt.Sprintf("\n\nКатегория канала: %s", streamInfo.GameName)
		}
	}

	llmCtx, cancel := context.WithTimeout(ctx, llmTimeout)
	defer cancel()

	// Проверка доступности LLM перед генерацией
	bm.mu.RLock() // P0-5: RLock for read-only
	llmClient := bm.llmClient
	bm.mu.RUnlock()

	if llmClient == nil {
		return "", "", fmt.Errorf("LLM client not initialized")
	}

	msg, err := llmClient.Generate(llmCtx, systemPrompt, userPrompt, maxTokens)
	if err != nil {
		return "", "", fmt.Errorf("generate message for %s: %w", personality, err)
	}
	return strings.TrimSpace(msg), sttContext, nil
}

func randomDelay(minSec, maxSec int) time.Duration {
	if minSec <= 0 {
		minSec = 45
	}
	if maxSec < minSec {
		maxSec = minSec
	}
	if maxSec == minSec {
		return time.Duration(minSec) * time.Second
	}
	span := maxSec - minSec + 1
	return time.Duration(minSec+rand.Intn(span)) * time.Second
}

// cancelPresenceBots cancels pending presence timers for every bot: cooldown
// bots are parked as stopped and running bots lose their session timer, so a
// disabled presence can never pause a bot into a return that would never
// happen.
func (bm *botManager) cancelPresenceBots() {
	bm.mu.Lock()
	defer bm.mu.Unlock()

	for _, br := range bm.bots {
		if br.presenceTimer != nil {
			br.presenceTimer.Stop()
			br.presenceTimer = nil
		}
		switch br.presenceState {
		case presenceStateCooldown:
			br.presenceState = presenceStateStopped
			br.cooldownUntil = time.Time{}
			br.autoPaused = false
		case presenceStateOnline:
			// The bot keeps chatting; no further session pause is scheduled.
			br.sessionEndsAt = time.Time{}
		}
	}
}

func (bm *botManager) startAudio(ctx context.Context) error {
	bm.mu.Lock()
	if bm.audioService != nil {
		bm.mu.Unlock()
		return nil
	}
	cfg := bm.audioConfig
	hub := bm.apiHub
	bm.mu.Unlock()

	// Добавляем callback для отправки контекста через WebSocket
	cfg.OnContextUpdate = func(context string) {
		slog.Info("Broadcasting context update", "chars", len(context))
		if hub != nil {
			hub.BroadcastAudioContext(context)
		} else {
			slog.Warn("Hub is nil, cannot broadcast")
		}
	}

	service := audio.NewService(cfg)
	audioCtx, cancel := context.WithCancel(ctx)
	if err := service.Start(audioCtx); err != nil {
		cancel()
		return err
	}

	bm.mu.Lock()
	bm.audioService = service
	bm.audioCancel = cancel
	bm.mu.Unlock()

	slog.Info("Audio context service enabled", "channel", cfg.Channel)
	return nil
}

func (bm *botManager) ApplySettings(_ context.Context, settings *storage.Settings) error {
	if settings == nil {
		return nil
	}

	baseCtx := bm.runtimeCtx
	if baseCtx == nil {
		baseCtx = context.Background()
	}

	// Reconfigure the single LLM client in place from env + database settings.
	cfg := mustResolve(llm.EnvFromLookup(os.Getenv), llm.DBConfigFromSettings(settings))

	bm.mu.Lock()
	llmClient := bm.llmClient
	bm.mu.Unlock()
	if llmClient != nil {
		llmClient.Reconfigure(cfg)
	}

	slog.Info("LLM client reconfigured",
		"provider", cfg.Provider,
		"free_model", cfg.FreeModel,
		"paid_model", cfg.PaidModel,
		"paid_enabled", cfg.PaidEnabled,
		"paid_daily_cap_usd", cfg.PaidDailyCapUSD,
		"api_key_set", cfg.APIKey != "",
	)

	// Update Twitch Helix client if credentials changed
	if settings.TwitchClientID != "" && settings.TwitchClientSecret != "" {
		bm.mu.Lock()
		if bm.helixClient == nil {
			bm.helixClient = twitch.NewHelixClient(settings.TwitchClientID, settings.TwitchClientSecret)
			if bm.helixClient != nil {
				slog.Info("Twitch Helix API client initialized from settings")
			}
		}
		bm.mu.Unlock()
	} else {
		bm.mu.Lock()
		if bm.helixClient != nil {
			slog.Info("Twitch API credentials removed, disabling Helix client")
		}
		bm.helixClient = nil
		bm.mu.Unlock()
	}

	primaryChannel := primaryChannelFromSettings(settings, bm.channel)
	primaryChanged := false

	bm.mu.Lock()
	if primaryChannel != "" && primaryChannel != bm.channel {
		bm.channel = primaryChannel
		primaryChanged = true
	}
	if primaryChannel != "" {
		bm.audioConfig.Channel = primaryChannel
	}
	// Обновляем режим отладки
	bm.debugMode = settings.DebugMode
	bm.positivePrompt = settings.PositivePrompt
	bm.negativePrompt = settings.NegativePrompt
	if settings.MaxTokens > 0 {
		bm.maxTokens = settings.MaxTokens
	} else {
		bm.maxTokens = llmMaxTokens
	}
	// Presence simulation configuration.
	bm.presence = presenceConfigFromSettings(settings)
	audioWasRunning := bm.audioService != nil
	bm.mu.Unlock()

	if primaryChanged {
		slog.Info("Primary Twitch channel set from UI settings", "channel", primaryChannel)
	}

	// Keep the public stream monitor on the configured channels (it
	// normalizes, dedupes and caps the list itself).
	if bm.streamMonitor != nil {
		bm.streamMonitor.SetChannels(settings.Channels)
	}

	// Presence disabled: cancel every pending presence session or cooldown
	// return so no bot comes back on its own.
	if !bm.presence.enabled {
		bm.cancelPresenceBots()
	}

	if settings.AudioEnabled {
		if primaryChanged && audioWasRunning {
			bm.stopAudio()
		}
		if err := bm.startAudio(baseCtx); err != nil {
			return fmt.Errorf("start audio service: %w", err)
		}
	} else {
		bm.stopAudio()
	}

	return nil
}

func runLogCleanupJob(ctx context.Context, store *storage.Store) {
	ticker := time.NewTicker(1 * time.Hour)
	defer ticker.Stop()

	slog.Info("Log cleanup job started (runs every 1 hour)")

	for {
		select {
		case <-ctx.Done():
			slog.Info("Log cleanup job stopped")
			return
		case <-ticker.C:
			cleanupCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)

			if err := store.CleanupOldLogs(cleanupCtx); err != nil {
				slog.Error("Log cleanup failed", "error", err)
			} else {
				slog.Info("Log cleanup completed successfully")
			}

			cancel()
		}
	}
}

func minInterval(ratePerMin int) time.Duration {
	if ratePerMin <= 0 {
		ratePerMin = 10
	}
	return time.Minute / time.Duration(ratePerMin)
}

func env(key, fallback string) string {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}
	return value
}

func envInt(key string, fallback int) int {
	raw := env(key, "")
	if raw == "" {
		return fallback
	}
	parsed, err := strconv.Atoi(raw)
	if err != nil {
		return fallback
	}
	return parsed
}

func runConsole(ctx context.Context, bm *botManager) {
	scanner := bufio.NewScanner(os.Stdin)
	fmt.Println("commands: list | start <id> | stop <id> | start-all | stop-all | help | quit")

	for {
		fmt.Print("> ")
		if !scanner.Scan() {
			return
		}

		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}

		parts := strings.Fields(line)
		cmd := strings.ToLower(parts[0])

		switch cmd {
		case "help":
			fmt.Println("list | start <id> | stop <id> | start-all | stop-all | quit")
		case "list":
			for _, ln := range bm.statusLines() {
				fmt.Println("-", ln)
			}
		case "start":
			if len(parts) < 2 {
				fmt.Println("usage: start <id>")
				continue
			}
			if err := bm.startBot(ctx, parts[1], true, false); err != nil {
				fmt.Println("error:", err)
				continue
			}
			fmt.Println("started", parts[1])
		case "stop":
			if len(parts) < 2 {
				fmt.Println("usage: stop <id>")
				continue
			}
			if err := bm.stopBot(parts[1], true); err != nil {
				fmt.Println("error:", err)
				continue
			}
			fmt.Println("stopped", parts[1])
		case "start-all":
			for _, id := range bm.listIDs(false) {
				if err := bm.startBot(ctx, id, true, false); err != nil {
					slog.Error("start bot failed", "id", id, "error", err)
				}
			}
		case "stop-all":
			for _, id := range bm.listIDs(false) {
				if err := bm.stopBot(id, true); err != nil {
					slog.Error("stop bot failed", "id", id, "error", err)
				}
			}
		case "quit", "exit":
			return
		default:
			fmt.Println("unknown command, type help")
		}
	}
}

func mustResolve(env llm.EnvConfig, db llm.DBConfig) llm.Config {
	cfg, _ := llm.ResolveConfig(env, db)
	return cfg
}

func main() {
	// LLM Configuration
	llmEnv := llm.EnvFromLookup(os.Getenv)
	llmClient := llm.NewClient(mustResolve(llmEnv, llm.DBConfig{}))

	// Database & Cache
	databaseURL := env("DATABASE_URL", "postgres://botfarm:botfarm@localhost:5432/botfarm?sslmode=disable")
	redisURL := env("REDIS_URL", "redis://localhost:6379")

	// Twitch & API
	channel := normalizeChannelName(env("TWITCH_CHANNEL", ""))
	apiKey := env("API_KEY", "")
	apiPortStr := env("API_PORT", "8080")
	corsOrigins := strings.Split(env("API_CORS_ORIGINS", "http://localhost:5173"), ",")
	enableConsole := env("ENABLE_CONSOLE", "true") == "true"
	globalLimit := 30

	// Local audio transcription (OpenAI-compatible STT endpoint)
	audioCfg := audio.Config{
		Channel:          channel,
		SourceURL:        env("AUDIO_SOURCE_URL", ""),
		Quality:          env("AUDIO_STREAM_QUALITY", "best"),
		ChunkSeconds:     envInt("AUDIO_CHUNK_SEC", 8),
		PollSeconds:      envInt("AUDIO_POLL_SEC", 15),
		ContextTTL:       time.Duration(envInt("AUDIO_CONTEXT_TTL_SEC", 120)) * time.Second,
		ContextMaxChars:  envInt("AUDIO_CONTEXT_MAX_CHARS", 320),
		STTURL:           env("AUDIO_STT_URL", "http://127.0.0.1:9000/v1/audio/transcriptions"),
		STTModel:         env("AUDIO_STT_MODEL", "whisper-1"),
		STTAPIKey:        env("AUDIO_STT_API_KEY", ""),
		STTLanguage:      env("AUDIO_STT_LANGUAGE", "ru"),
		StreamlinkBinary: env("AUDIO_STREAMLINK_CMD", "streamlink"),
		FFMPEGBinary:     env("AUDIO_FFMPEG_CMD", "ffmpeg"),
		TwitchOAuthToken: env("TWITCH_OAUTH_TOKEN", ""),
	}

	apiPort, err := strconv.Atoi(apiPortStr)
	if err != nil {
		apiPort = 8080
	}

	var cacheClient *cache.Client
	if redisURL != "" {
		var err error
		cacheClient, err = cache.NewClient(redisURL)
		if err != nil {
			slog.Error("Redis connection failed (fallback to in-memory)", "error", err)
		} else {
			defer cacheClient.Close()
			slog.Info("Redis OK", "url", redisURL)
		}
	}

	ctx, stopSignal := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stopSignal()

	store, err := storage.New(ctx, databaseURL)
	if err != nil {
		slog.Error("postgres connect error", "error", err)
		os.Exit(1)
	}
	defer store.Close()

	if err := store.EnsureSchema(ctx); err != nil {
		slog.Error("ensure schema error", "error", err)
		os.Exit(1)
	}

	if err := store.EnsureSettingsTable(ctx); err != nil {
		slog.Error("ensure settings table error", "error", err)
		os.Exit(1)
	}

	// Start background cleanup job for old logs
	go runLogCleanupJob(ctx, store)

	settings, err := store.GetSettings(ctx)
	if err != nil {
		slog.Error("Failed to load settings", "error", err)
	}

	llmClient.Reconfigure(mustResolve(llmEnv, llm.DBConfigFromSettings(settings)))

	// One public stream monitor for the app lifetime; the API serves it.
	monitor := twitch.NewStreamMonitor()
	defer monitor.Close()
	if settings != nil {
		monitor.SetChannels(settings.Channels)
	}

	healthCtx, healthCancel := context.WithTimeout(context.Background(), 8*time.Second)
	if err := llmClient.HealthCheck(healthCtx); err != nil {
		healthCancel()
		slog.Warn("OpenRouter API is not reachable", "error", err)
		slog.Info("Backend will start anyway. Configure LLM settings in UI.")
	} else {
		healthCancel()
		slog.Info("OpenRouter API OK")
	}

	channel = primaryChannelFromSettings(settings, channel)
	audioCfg.Channel = channel
	if channel == "" {
		slog.Warn("No default channel configured yet. Set Settings -> channels in UI before starting bots.")
	}

	debugMode := false
	if settings != nil {
		debugMode = settings.DebugMode
	}

	manager := newBotManager(store, llmClient, cacheClient, nil, ctx, channel, globalLimit, audioCfg, debugMode, monitor)
	if err := manager.loadBots(ctx); err != nil {
		slog.Error("load bots error", "error", err)
		os.Exit(1)
	}

	if len(manager.listIDs(false)) == 0 {
		slog.Warn("no bots found in database; add rows into table bots first")
	}

	apiServer := api.NewServer(manager, store, llmClient, cacheClient, monitor, apiKey, apiPort, corsOrigins)
	manager.setAPIHub(apiServer.Hub())

	go func() {
		if err := apiServer.Start(); err != nil {
			slog.Error("API server error", "error", err)
		}
	}()

	// Загружаем настройки и применяем runtime-флаги (presence, аудио-контекст)
	if err := manager.ApplySettings(ctx, settings); err != nil {
		slog.Error("Failed to apply settings", "error", err)
	}

	manager.startEnabled(ctx)

	if enableConsole {
		runConsole(ctx, manager)
	} else {
		<-ctx.Done()
	}

	manager.stopAudio()

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutdownCancel()
	apiServer.Shutdown(shutdownCtx)

	manager.stopAll()
}

func (bm *botManager) GetAudioContext() string {
	bm.mu.Lock()
	audioService := bm.audioService
	bm.mu.Unlock()

	if audioService == nil {
		return ""
	}

	return audioService.GetRecentContext()
}

func (bm *botManager) TestLLMGeneration(ctx context.Context, personality string) (string, error) {
	msg, _, err := bm.generateMessage(ctx, personality)
	return msg, err
}
