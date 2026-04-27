package main

import (
	"bufio"
	"context"
	"fmt"
	"log"
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
	llmTimeout      = 20 * time.Second
	llmMaxTokens    = 60
	llmRetryAttempts = 3
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
	}
}

func (l *globalLimiter) allow(now time.Time) bool {
	if l.cacheClient != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()

		count, err := l.cacheClient.IncrementRateLimit(ctx, cache.GlobalRateLimitKey, l.window)
		if err != nil {
			log.Printf("redis rate limit error (fallback to memory): %v", err)
		} else {
			return count <= int64(l.limit)
		}
	}

	l.mu.Lock()
	defer l.mu.Unlock()

	cutoff := now.Add(-l.window)
	idx := 0
	for idx < len(l.events) && l.events[idx].Before(cutoff) {
		idx++
	}
	if idx > 0 {
		l.events = append([]time.Time(nil), l.events[idx:]...)
	}

	// Prevent unbounded growth
	const maxEvents = 10000
	if len(l.events) >= maxEvents {
		log.Printf("rate limiter memory limit reached, dropping oldest events")
		l.events = l.events[len(l.events)-maxEvents/2:]
	}

	if len(l.events) >= l.limit {
		return false
	}

	l.events = append(l.events, now)
	return true
}

type botRuntime struct {
	cfg      storage.BotConfig
	client   *twitch.IRCClient
	cancel   context.CancelFunc
	running  bool
	lastSent time.Time
}

type botManager struct {
	mu              sync.Mutex
	runtimeCtx      context.Context
	store           *storage.Store
	llmClient       *llm.Client
	cacheClient     *cache.Client
	channel         string
	audioConfig     audio.Config
	audioService    *audio.Service
	audioCancel     context.CancelFunc
	globalLimiter   *globalLimiter
	bots            map[string]*botRuntime
	apiHub          *api.Hub
	rotationEnabled bool
	rotationCancel  context.CancelFunc
}

func normalizeChannelName(raw string) string {
	ch := strings.TrimSpace(raw)
	ch = strings.TrimPrefix(ch, "#")
	return strings.ToLower(ch)
}

func primaryChannelFromSettings(settings *storage.Settings, fallback string) string {
	if settings != nil {
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
	runtimeCtx context.Context,
	channel string,
	globalLimit int,
	audioCfg audio.Config,
) *botManager {
	if runtimeCtx == nil {
		runtimeCtx = context.Background()
	}
	return &botManager{
		runtimeCtx:    runtimeCtx,
		store:         store,
		llmClient:     llmClient,
		cacheClient:   cacheClient,
		channel:       channel,
		audioConfig:   audioCfg,
		globalLimiter: newGlobalLimiter(globalLimit, cacheClient),
		bots:          make(map[string]*botRuntime),
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
		if err := bm.startBot(ctx, id, false); err != nil {
			log.Printf("start %s: %v", id, err)
		}
	}
}

func (bm *botManager) listIDs(enabledOnly bool) []string {
	bm.mu.Lock()
	defer bm.mu.Unlock()

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
	bm.mu.Lock()
	defer bm.mu.Unlock()

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
	bm.mu.Lock()
	defer bm.mu.Unlock()

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
	return bm.startBot(ctx, id, true)
}

func (bm *botManager) StopBot(id string) error {
	return bm.stopBot(id, true)
}

func (bm *botManager) StartAll(ctx context.Context) error {
	ids := bm.listIDs(false)
	for _, id := range ids {
		if err := bm.startBot(ctx, id, true); err != nil {
			log.Printf("start %s: %v", id, err)
		}
	}
	return nil
}

func (bm *botManager) StopAll() error {
	bm.stopAll()
	return nil
}

func (bm *botManager) startBot(parent context.Context, botID string, persist bool) error {
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

	ctx, cancel := context.WithCancel(parent)
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

	go bm.runBotLoop(ctx, br)
	return nil
}

func (bm *botManager) stopBot(botID string, persist bool) error {
	bm.mu.Lock()
	br, ok := bm.bots[botID]
	if !ok {
		bm.mu.Unlock()
		return fmt.Errorf("bot not found: %s", botID)
	}
	if br.cancel != nil {
		br.cancel()
	}
	br.running = false
	br.cancel = nil
	br.cfg.Enabled = false
	br.client.Close()
	bm.mu.Unlock()

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
	bm.mu.Lock()
	defer bm.mu.Unlock()

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
	log.Printf("[%s] started", br.cfg.ID)
	bm.logBotEvent(br.cfg.ID, "lifecycle", "info", "Bot started", nil)

	defer func() {
		log.Printf("[%s] stopped", br.cfg.ID)
		bm.logBotEvent(br.cfg.ID, "lifecycle", "info", "Bot stopped", nil)
	}()

	consecutiveErrors := 0

	for {
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
			log.Printf("[%s] IRC disconnected, reconnecting...", br.cfg.ID)
			bm.logBotEvent(br.cfg.ID, "error", "warning", "IRC disconnected, reconnecting...", nil)

			if err := br.client.Reconnect(); err != nil {
				log.Printf("[%s] reconnect failed: %v", br.cfg.ID, err)
				consecutiveErrors++
				if consecutiveErrors >= maxConsecutiveErrors {
					log.Printf("[%s] too many errors, stopping bot", br.cfg.ID)
					return
				}
				time.Sleep(errorRetryDelay)
				continue
			}
			log.Printf("[%s] reconnected successfully", br.cfg.ID)
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
			log.Printf("[%s] global limit reached, skip", br.cfg.ID)
			bm.logBotEvent(br.cfg.ID, "rate_limit", "warning", "Global rate limit reached", nil)
			continue
		}

		msg, err := bm.generateMessage(ctx, br.cfg.Personality)
		if err != nil {
			log.Printf("[%s] llm error: %v", br.cfg.ID, err)
			bm.logBotEvent(br.cfg.ID, "llm", "error", "LLM generation failed", map[string]interface{}{
				"error":       err.Error(),
				"personality": br.cfg.Personality,
			})
			consecutiveErrors++
			if consecutiveErrors >= maxConsecutiveErrors {
				log.Printf("[%s] too many LLM errors, stopping bot", br.cfg.ID)
				return
			}
			continue
		}

		if err := br.client.SendMessage(msg); err != nil {
			log.Printf("[%s] twitch send error: %v", br.cfg.ID, err)
			br.client.Close()
			consecutiveErrors++
			if consecutiveErrors >= maxConsecutiveErrors {
				log.Printf("[%s] too many send errors, stopping bot", br.cfg.ID)
				return
			}
			continue
		}

		// Успешная отправка - сбрасываем счетчик ошибок
		consecutiveErrors = 0
		br.lastSent = now
		log.Printf("[%s|%s] %s", br.cfg.ID, br.cfg.Personality, msg)

		bm.logBotEvent(br.cfg.ID, "message", "success", msg, map[string]interface{}{
			"personality": br.cfg.Personality,
			"channel":     normalizeChannelName(br.cfg.CurrentChannel),
		})

		if bm.cacheClient != nil {
			// Используем родительский контекст с таймаутом вместо Background
			cacheCtx, cancel := context.WithTimeout(ctx, cacheOperationTimeout)
			_ = bm.cacheClient.AddMessage(cacheCtx, br.cfg.ID, msg, messageRetentionTime)
			_ = bm.cacheClient.AddBotMessageLog(cacheCtx, br.cfg.ID, msg, br.cfg.Personality)
			_ = bm.cacheClient.IncrementBotStats(cacheCtx, br.cfg.ID)
			cancel()
		}

		if bm.apiHub != nil {
			bm.apiHub.BroadcastBotMessage(br.cfg.ID, msg, br.cfg.Personality)
		}
	}
}

func (bm *botManager) generateMessage(ctx context.Context, personality string) (string, error) {
	p, ok := personalities[personality]
	if !ok {
		p = personalities[PersonalityNeutral]
	}

	userPrompt := p.user
	bm.mu.Lock()
	audioService := bm.audioService
	bm.mu.Unlock()

	if audioService != nil {
		streamContext := strings.TrimSpace(audioService.GetRecentContext())
		if streamContext != "" {
			userPrompt = p.user + "\n\nКонтекст из недавней транскрипции стрима (может быть неточным):\n" +
				streamContext +
				"\n\nИспользуй это как подсказку по теме и напиши короткое сообщение."
		}
	}

	llmCtx, cancel := context.WithTimeout(ctx, llmTimeout)
	defer cancel()

	// Проверка доступности LLM перед генерацией
	if bm.llmClient == nil {
		return "", fmt.Errorf("LLM client not initialized")
	}

	msg, err := bm.llmClient.Generate(llmCtx, p.system, userPrompt, llmMaxTokens)
	if err != nil {
		return "", fmt.Errorf("generate message for %s: %w", personality, err)
	}
	return strings.TrimSpace(msg), nil
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
		log.Printf("Rotation disabled: need at least 2 channels and positive interval")
		return
	}

	rotationCtx, cancel := context.WithCancel(ctx)
	bm.mu.Lock()
	bm.rotationEnabled = true
	bm.rotationCancel = cancel
	bm.mu.Unlock()

	log.Printf("Starting bot rotation: %d channels, interval %d minutes", len(channels), intervalMinutes)

	go func() {
		ticker := time.NewTicker(time.Duration(intervalMinutes) * time.Minute)
		defer ticker.Stop()

		for {
			select {
			case <-rotationCtx.Done():
				log.Printf("Bot rotation stopped")
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
	bm.mu.Unlock()

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

	log.Printf("Audio context service enabled (channel: %s)", cfg.Channel)
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
		log.Printf("Audio context service disabled")
	}
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
		log.Printf("[LLM] Initializing client: provider=%s, model=%s, url=%s",
			settings.LLMProvider, settings.LLMModel, settings.LLMApiURL)

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
			log.Printf("[LLM] Health check failed: %v", err)
			if settings.LLMProvider == "lmstudio" {
				log.Printf("[LLM] Make sure LM Studio is running on %s with a model loaded", settings.LLMApiURL)
			}
		} else {
			cancel()
			log.Printf("[LLM] Client ready: provider=%s, model=%s",
				settings.LLMProvider, settings.LLMModel)
		}

		bm.mu.Lock()
		bm.llmClient = newClient
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
	audioWasRunning := bm.audioService != nil
	bm.mu.Unlock()

	if primaryChanged {
		log.Printf("Primary Twitch channel set from UI settings: %s", primaryChannel)
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

	log.Printf("Rotating %d bots across %d channels", len(runningBots), len(channels))

	for _, br := range runningBots {
		// Выбираем случайный канал
		newChannel := channels[rand.Intn(len(channels))]

		// Останавливаем бота
		if err := bm.stopBot(br.cfg.ID, false); err != nil {
			log.Printf("Failed to stop bot %s for rotation: %v", br.cfg.ID, err)
			continue
		}

		// Обновляем канал в БД
		if err := bm.store.UpdateBotChannel(ctx, br.cfg.ID, newChannel); err != nil {
			log.Printf("Failed to update channel for bot %s: %v", br.cfg.ID, err)
			continue
		}

		// Обновляем конфигурацию и создаем новый IRC клиент
		bm.mu.Lock()
		br.cfg.CurrentChannel = newChannel
		br.client = twitch.NewIRCClient(br.cfg.TwitchLogin, br.cfg.Password, newChannel)
		bm.mu.Unlock()

		// Запускаем бота на новом канале
		if err := bm.startBot(ctx, br.cfg.ID, false); err != nil {
			log.Printf("Failed to restart bot %s on channel %s: %v", br.cfg.ID, newChannel, err)
			continue
		}

		log.Printf("Bot %s rotated to channel %s", br.cfg.ID, newChannel)
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
			if err := bm.startBot(ctx, parts[1], true); err != nil {
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
				if err := bm.startBot(ctx, id, true); err != nil {
					fmt.Printf("start %s: %v\n", id, err)
				}
			}
		case "stop-all":
			for _, id := range bm.listIDs(false) {
				if err := bm.stopBot(id, true); err != nil {
					fmt.Printf("stop %s: %v\n", id, err)
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
	}

	apiPort, err := strconv.Atoi(apiPortStr)
	if err != nil {
		apiPort = 8080
	}

	llmClient := llm.NewClient(llmURL, llmModel, llmAPIKey, llmProvider)
	healthCtx, healthCancel := context.WithTimeout(context.Background(), 8*time.Second)
	if err := llmClient.HealthCheck(healthCtx); err != nil {
		healthCancel()
		log.Printf("Warning: LLM API (%s) is not reachable: %v", llmProvider, err)
		log.Printf("Backend will start anyway. Configure LLM settings in UI.")
	} else {
		healthCancel()
		log.Printf("LLM API OK (provider: %s, url: %s, model: %s)", llmProvider, llmURL, llmModel)
	}

	var cacheClient *cache.Client
	if redisURL != "" {
		var err error
		cacheClient, err = cache.NewClient(redisURL)
		if err != nil {
			log.Printf("Redis connection failed (fallback to in-memory): %v", err)
		} else {
			defer cacheClient.Close()
			log.Printf("Redis OK (%s)", redisURL)
		}
	}

	ctx, stopSignal := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stopSignal()

	store, err := storage.New(ctx, databaseURL)
	if err != nil {
		log.Fatalf("postgres connect error: %v", err)
	}
	defer store.Close()

	if err := store.EnsureSchema(ctx); err != nil {
		log.Fatalf("ensure schema error: %v", err)
	}

	if err := store.EnsureSettingsTable(ctx); err != nil {
		log.Fatalf("ensure settings table error: %v", err)
	}

	settings, err := store.GetSettings(ctx)
	if err != nil {
		log.Printf("Failed to load settings: %v", err)
	}

	channel = primaryChannelFromSettings(settings, channel)
	audioCfg.Channel = channel
	if channel == "" {
		log.Printf("No default channel configured yet. Set Settings -> channels in UI before starting bots.")
	}

	manager := newBotManager(store, llmClient, cacheClient, ctx, channel, globalLimit, audioCfg)
	if err := manager.loadBots(ctx); err != nil {
		log.Fatalf("load bots error: %v", err)
	}

	if len(manager.listIDs(false)) == 0 {
		log.Printf("no bots found in database; add rows into table bots first")
	}

	apiServer := api.NewServer(manager, store, llmClient, cacheClient, apiKey, apiPort, corsOrigins)
	manager.setAPIHub(apiServer.Hub())

	go func() {
		if err := apiServer.Start(); err != nil {
			log.Printf("API server error: %v", err)
		}
	}()

	// Загружаем настройки и применяем runtime-флаги (ротация, аудио-контекст)
	if err := manager.ApplySettings(ctx, settings); err != nil {
		log.Printf("Failed to apply settings: %v", err)
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
