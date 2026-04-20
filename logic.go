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

var personalities = map[string]promptPair{
	"веселый_фанат": {
		system: "Ты весёлый фанат стримера. Пишешь короткие сообщения в чат Twitch (макс 60 символов). Без эмодзи. Естественный тон.",
		user:   "Напиши позитивный комментарий стримеру или в чат. Максимально коротко.",
	},
	"токсик": {
		system: "Ты провокатор в чате. Пишешь короткие (макс 50 символов) провокационные, но не оскорбительные сообщения.",
		user:   "Напиши что-то чтобы спровоцировать дискуссию в чате. Коротко.",
	},
	"нейтрал": {
		system: "Ты обычный зритель Twitch. Пишешь короткие вопросы или комментарии (макс 55 символов). Естественно.",
		user:   "Напиши вопрос стримеру или комментарий к происходящему. Коротко.",
	},
	"хайпер": {
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
	mu            sync.Mutex
	store         *storage.Store
	llmClient     *llm.Client
	cacheClient   *cache.Client
	channel       string
	globalLimiter *globalLimiter
	bots          map[string]*botRuntime
	apiHub        *api.Hub
}

func newBotManager(store *storage.Store, llmClient *llm.Client, cacheClient *cache.Client, channel string, globalLimit int) *botManager {
	return &botManager{
		store:         store,
		llmClient:     llmClient,
		cacheClient:   cacheClient,
		channel:       channel,
		globalLimiter: newGlobalLimiter(globalLimit, cacheClient),
		bots:          make(map[string]*botRuntime),
	}
}

func (bm *botManager) setAPIHub(hub *api.Hub) {
	bm.apiHub = hub
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
		bm.bots[cfg.ID] = &botRuntime{
			cfg:    cfg,
			client: twitch.NewIRCClient(cfg.TwitchLogin, cfg.OAuthToken, bm.channel),
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
	defer log.Printf("[%s] stopped", br.cfg.ID)

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
			consecutiveErrors = 0
		}

		if err := br.client.Connect(); err != nil {
			log.Printf("[%s] twitch connect error: %v", br.cfg.ID, err)
			consecutiveErrors++
			if consecutiveErrors >= maxConsecutiveErrors {
				log.Printf("[%s] too many errors, stopping bot", br.cfg.ID)
				return
			}
			time.Sleep(errorRetryDelay)
			continue
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
			continue
		}

		msg, err := bm.generateMessage(ctx, br.cfg.Personality)
		if err != nil {
			log.Printf("[%s] llm error: %v", br.cfg.ID, err)
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
		p = personalities["нейтрал"]
	}

	llmCtx, cancel := context.WithTimeout(ctx, llmTimeout)
	defer cancel()

	msg, err := bm.llmClient.Generate(llmCtx, p.system, p.user, llmMaxTokens)
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
	llmProvider := env("LLM_PROVIDER", "lmstudio") // lmstudio, omniroute, openai, custom

	// Database & Cache
	databaseURL := env("DATABASE_URL", "postgres://botfarm:botfarm@localhost:5432/twitch_bot_farm?sslmode=disable")
	redisURL := env("REDIS_URL", "redis://localhost:6379")

	// Twitch & API
	channel := env("TWITCH_CHANNEL", "")
	apiKey := env("API_KEY", "")
	apiPortStr := env("API_PORT", "8080")
	corsOrigins := strings.Split(env("API_CORS_ORIGINS", "http://localhost:5173"), ",")
	enableConsole := env("ENABLE_CONSOLE", "true") == "true"
	globalLimit := 30

	apiPort, err := strconv.Atoi(apiPortStr)
	if err != nil {
		apiPort = 8080
	}

	if strings.TrimSpace(channel) == "" {
		log.Fatal("TWITCH_CHANNEL is required (example: test_channel)")
	}

	llmClient := llm.NewClient(llmURL, llmModel, llmAPIKey, llmProvider)
	healthCtx, healthCancel := context.WithTimeout(context.Background(), 8*time.Second)
	if err := llmClient.HealthCheck(healthCtx); err != nil {
		healthCancel()
		log.Fatalf("LLM API (%s) is not reachable: %v", llmProvider, err)
	}
	healthCancel()
	log.Printf("LLM API OK (provider: %s, url: %s, model: %s)", llmProvider, llmURL, llmModel)

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

	manager := newBotManager(store, llmClient, cacheClient, channel, globalLimit)
	if err := manager.loadBots(ctx); err != nil {
		log.Fatalf("load bots error: %v", err)
	}

	if len(manager.listIDs(false)) == 0 {
		log.Printf("no bots found in database; add rows into table bots first")
	}

	apiServer := api.NewServer(manager, store, apiKey, apiPort, corsOrigins)
	manager.setAPIHub(apiServer.Hub())

	go func() {
		if err := apiServer.Start(); err != nil {
			log.Printf("API server error: %v", err)
		}
	}()

	manager.startEnabled(ctx)

	if enableConsole {
		runConsole(ctx, manager)
	} else {
		<-ctx.Done()
	}

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutdownCancel()
	apiServer.Shutdown(shutdownCtx)

	manager.stopAll()
}
