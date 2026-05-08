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
	disableLLM bool // if true, skip LLM/STT generation
	stopped    chan struct{} // P0-1: Channel to signal when bot fully stopped
}

type botManager struct {
	mu              sync.RWMutex // P0-5: RWMutex for read-heavy operations
	runtimeCtx      context.Context
	store           *storage.Store
	llmClient       *llm.Client
	cacheClient     *cache.Client
	helixClient     *twitch.HelixClient
	channel         string
	audioConfig     audio.Config
	audioService    *audio.Service
	audioCancel     context.CancelFunc
	globalLimiter   *globalLimiter
	bots            map[string]*botRuntime
	apiHub          *api.Hub
	rotationEnabled bool
	rotationCancel  context.CancelFunc
	debugMode       bool
	positivePrompt  string
	negativePrompt  string
	maxTokens       int
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
		channel:       channel,
		audioConfig:   audioCfg,
		globalLimiter: newGlobalLimiter(globalLimit, cacheClient),
		bots:          make(map[string]*botRuntime),
		debugMode:     debugMode,
		maxTokens:     llmMaxTokens,
	}
}

func (bm *botManager) setAPIHub(hub *api.Hub) {
	bm.apiHub = hub
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
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		_ = bm.store.CreateBotLog(ctx, log)
	}()

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
			client: twitch.NewIRCClient(cfg.TwitchLogin, cfg.Password, channel),
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

func (bm *botManager) ListBots() []api.BotInfo {
	bm.mu.RLock() // P0-5: RLock for read-only
	defer bm.mu.RUnlock()

	infos := make([]api.BotInfo, 0, len(bm.bots))
	for _, br := range bm.bots {
		infos = append(infos, api.BotInfo{
			Config:  br.cfg,
			Running: br.running,
		})
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

	return &api.BotInfo{
		Config:  br.cfg,
		Running: br.running,
	}, nil
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
		client: twitch.NewIRCClient(cfg.TwitchLogin, cfg.Password, channel),
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
	br.client = twitch.NewIRCClient(br.cfg.TwitchLogin, br.cfg.Password, targetChannel)

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

	if persist {
		if err := bm.store.SetBotEnabled(context.Background(), botID, true); err != nil {
			bm.stopBot(botID, false)
			return err
		}
	}

	return nil
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

	// P0-1: Graceful shutdown - set stopping flag and wait for loop to finish
	br.stopping = true

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

	// P0-1: Wait for graceful shutdown with timeout
	gracefulTimeout := 10 * time.Second
	select {
	case <-br.stopped:
		slog.Info("graceful shutdown completed", "bot_id", botID)
	case <-time.After(gracefulTimeout):
		slog.Warn("graceful shutdown timeout, forcing stop", "bot_id", botID)
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
	slog.Info("bot started", "bot_id", br.cfg.ID)
	bm.logBotEvent(br.cfg.ID, "lifecycle", "info", "Bot started", nil)

	// P0-1: Graceful shutdown - ensure we signal when done
	defer func() {
		// P0-1: Wait for any pending message to be sent (up to configured delay)
		if br.cfg.MinDelaySec > 0 {
			slog.Info("waiting for pending operations before graceful stop", "bot_id", br.cfg.ID)
			time.Sleep(time.Duration(br.cfg.MinDelaySec) * time.Second)
		}

		// P0-1: Log reason for stopping
		reason := "stopped"
		if br.stopping {
			reason = "stopped gracefully (user request)"
		}

		slog.Info("bot stopped", "bot_id", br.cfg.ID, "reason", reason)
		bm.logBotEvent(br.cfg.ID, "lifecycle", "info", reason, nil)

		// P0-1: Signal that bot has fully stopped
		if br.stopped != nil {
			close(br.stopped)
			br.stopped = nil
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
			time.Sleep(time.Duration(rand.Intn(br.cfg.MaxDelaySec-br.cfg.MinDelaySec)+br.cfg.MinDelaySec) * time.Second)
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
					return
				}
				continue
			}
		} else {
			slog.Info("DEBUG MODE: message not sent to Twitch", "bot_id", br.cfg.ID)
		}
	}
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

// startRotation запускает горутину для периодической ротации ботов между каналами
func (bm *botManager) startRotation(ctx context.Context, channels []string, intervalMinutes int) {
	if len(channels) < 2 || intervalMinutes <= 0 {
		slog.Warn("Rotation disabled: need at least 2 channels and positive interval")
		return
	}

	rotationCtx, cancel := context.WithCancel(ctx)
	bm.mu.Lock()
	bm.rotationEnabled = true
	bm.rotationCancel = cancel
	bm.mu.Unlock()

	slog.Info("Starting bot rotation", "channels", len(channels), "interval_minutes", intervalMinutes)

	go func() {
		ticker := time.NewTicker(time.Duration(intervalMinutes) * time.Minute)
		defer ticker.Stop()

		for {
			select {
			case <-rotationCtx.Done():
				slog.Info("Bot rotation stopped")
				return
			case <-ticker.C:
				bm.rotateBots(ctx, channels)
			}
		}
	}()
}

// stopRotation останавливает ротацию ботов
func (bm *botManager) stopRotation() {
	bm.mu.Lock()
	defer bm.mu.Unlock()

	if bm.rotationCancel != nil {
		bm.rotationCancel()
		bm.rotationCancel = nil
	}
	bm.rotationEnabled = false
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

	// Пересоздать LLM клиент если настройки изменились
	if settings.LLMProvider != "" && settings.LLMApiURL != "" {
		slog.Info("Initializing LLM client", "provider", settings.LLMProvider, "model", settings.LLMModel, "url", settings.LLMApiURL)

		newClient := llm.NewClient(
			settings.LLMApiURL,
			settings.LLMModel,
			settings.LLMApiKey,
			settings.LLMProvider,
		)

		// Проверка доступности нового клиента
		healthCtx, cancel := context.WithTimeout(baseCtx, 10*time.Second)
		if err := newClient.HealthCheck(healthCtx); err != nil {
			cancel()
			slog.Error("LLM Health check failed", "error", err)
			if settings.LLMProvider == "lmstudio" {
				slog.Warn("Make sure LM Studio is running with a model loaded", "url", settings.LLMApiURL)
			}
		} else {
			cancel()
			slog.Info("LLM Client ready", "provider", settings.LLMProvider, "model", settings.LLMModel)
		}

		bm.mu.Lock()
		bm.llmClient = newClient
		bm.mu.Unlock()
	}

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
	audioWasRunning := bm.audioService != nil
	bm.mu.Unlock()

	if primaryChanged {
		slog.Info("Primary Twitch channel set from UI settings", "channel", primaryChannel)
	}

	if settings.RotationEnabled && len(settings.Channels) > 1 {
		bm.stopRotation()
		bm.startRotation(baseCtx, settings.Channels, settings.RotationInterval)
	} else {
		bm.stopRotation()
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

// rotateBots переключает ботов на случайные каналы
func (bm *botManager) rotateBots(ctx context.Context, channels []string) {
	if len(channels) < 2 {
		return
	}

	bm.mu.Lock()
	runningBots := make([]*botRuntime, 0)
	for _, br := range bm.bots {
		if br.running {
			runningBots = append(runningBots, br)
		}
	}
	bm.mu.Unlock()

	if len(runningBots) == 0 {
		return
	}

	slog.Info("Rotating bots", "bots", len(runningBots), "channels", len(channels))

	for _, br := range runningBots {
		// Выбираем случайный канал
		newChannel := channels[rand.Intn(len(channels))]

		// Останавливаем бота
		if err := bm.stopBot(br.cfg.ID, false); err != nil {
			slog.Error("Failed to stop bot for rotation", "bot_id", br.cfg.ID, "error", err)
			continue
		}

		// Обновляем канал в БД
		if err := bm.store.UpdateBotChannel(ctx, br.cfg.ID, newChannel); err != nil {
			slog.Error("Failed to update channel", "bot_id", br.cfg.ID, "error", err)
			continue
		}

		// Обновляем конфигурацию и создаем новый IRC клиент
		bm.mu.Lock()
		br.cfg.CurrentChannel = newChannel
		br.client = twitch.NewIRCClient(br.cfg.TwitchLogin, br.cfg.Password, newChannel)
		bm.mu.Unlock()

		// Запускаем бота на новом канале
		if err := bm.startBot(ctx, br.cfg.ID, false, false); err != nil {
			slog.Error("Failed to restart bot on channel", "bot_id", br.cfg.ID, "channel", newChannel, "error", err)
			continue
		}

		slog.Info("Bot rotated to channel", "bot_id", br.cfg.ID, "channel", newChannel)
	}
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

func main() {
	// LLM Configuration
	llmURL := env("LLM_API_URL", env("LMSTUDIO_URL", "http://127.0.0.1:1234/v1"))
	llmModel := env("LLM_MODEL", env("LMSTUDIO_MODEL", "local-model"))
	llmAPIKey := env("LLM_API_KEY", "")
	llmProvider := env("LLM_PROVIDER", "lmstudio") // lmstudio (единственный поддерживаемый провайдер)

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

	llmClient := llm.NewClient(llmURL, llmModel, llmAPIKey, llmProvider)
	healthCtx, healthCancel := context.WithTimeout(context.Background(), 8*time.Second)
	if err := llmClient.HealthCheck(healthCtx); err != nil {
		healthCancel()
		slog.Warn("LLM API is not reachable", "provider", llmProvider, "error", err)
		slog.Info("Backend will start anyway. Configure LLM settings in UI.")
	} else {
		healthCancel()
		slog.Info("LLM API OK", "provider", llmProvider, "url", llmURL, "model", llmModel)
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

	channel = primaryChannelFromSettings(settings, channel)
	audioCfg.Channel = channel
	if channel == "" {
		slog.Warn("No default channel configured yet. Set Settings -> channels in UI before starting bots.")
	}

	debugMode := false
	if settings != nil {
		debugMode = settings.DebugMode
	}

	manager := newBotManager(store, llmClient, cacheClient, nil, ctx, channel, globalLimit, audioCfg, debugMode)
	if err := manager.loadBots(ctx); err != nil {
		slog.Error("load bots error", "error", err)
		os.Exit(1)
	}

	if len(manager.listIDs(false)) == 0 {
		slog.Warn("no bots found in database; add rows into table bots first")
	}

	apiServer := api.NewServer(manager, store, llmClient, cacheClient, apiKey, apiPort, corsOrigins)
	manager.setAPIHub(apiServer.Hub())

	go func() {
		if err := apiServer.Start(); err != nil {
			slog.Error("API server error", "error", err)
		}
	}()

	// Загружаем настройки и применяем runtime-флаги (ротация, аудио-контекст)
	if err := manager.ApplySettings(ctx, settings); err != nil {
		slog.Error("Failed to apply settings", "error", err)
	}

	manager.startEnabled(ctx)

	if enableConsole {
		runConsole(ctx, manager)
	} else {
		<-ctx.Done()
	}

	manager.stopRotation()
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
