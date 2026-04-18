package main

import (
	"bufio"
	"context"
	"fmt"
	"log"
	"math/rand"
	"os"
	"os/signal"
	"strings"
	"sync"
	"syscall"
	"time"

	"twitch-smm-bot/internal/llm"
	"twitch-smm-bot/internal/storage"
	"twitch-smm-bot/internal/twitch"
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
	mu     sync.Mutex
	limit  int
	events []time.Time
	window time.Duration
}

func newGlobalLimiter(limit int) *globalLimiter {
	if limit <= 0 {
		limit = 30
	}
	return &globalLimiter{limit: limit, window: time.Minute, events: make([]time.Time, 0, limit)}
}

func (l *globalLimiter) allow(now time.Time) bool {
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
	channel       string
	globalLimiter *globalLimiter
	bots          map[string]*botRuntime
}

func newBotManager(store *storage.Store, llmClient *llm.Client, channel string, globalLimit int) *botManager {
	return &botManager{
		store:         store,
		llmClient:     llmClient,
		channel:       channel,
		globalLimiter: newGlobalLimiter(globalLimit),
		bots:          make(map[string]*botRuntime),
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

		if err := br.client.Connect(); err != nil {
			log.Printf("[%s] twitch connect error: %v", br.cfg.ID, err)
			time.Sleep(5 * time.Second)
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
			continue
		}

		if err := br.client.SendMessage(msg); err != nil {
			log.Printf("[%s] twitch send error: %v", br.cfg.ID, err)
			br.client.Close()
			continue
		}

		br.lastSent = now
		log.Printf("[%s|%s] %s", br.cfg.ID, br.cfg.Personality, msg)
	}
}

func (bm *botManager) generateMessage(ctx context.Context, personality string) (string, error) {
	p, ok := personalities[personality]
	if !ok {
		p = personalities["нейтрал"]
	}

	llmCtx, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()

	msg, err := bm.llmClient.Generate(llmCtx, p.system, p.user, 60)
	if err != nil {
		return "", err
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
	rand.Seed(time.Now().UnixNano())

	llmURL := env("LMSTUDIO_URL", "http://127.0.0.1:1234/v1")
	llmModel := env("LMSTUDIO_MODEL", "local-model")
	databaseURL := env("DATABASE_URL", "postgres://botfarm:botfarm@localhost:5432/twitch_bot_farm?sslmode=disable")
	channel := env("TWITCH_CHANNEL", "")
	globalLimit := 30

	if strings.TrimSpace(channel) == "" {
		log.Fatal("TWITCH_CHANNEL is required (example: test_channel)")
	}

	llmClient := llm.NewClient(llmURL, llmModel)
	healthCtx, healthCancel := context.WithTimeout(context.Background(), 8*time.Second)
	if err := llmClient.HealthCheck(healthCtx); err != nil {
		healthCancel()
		log.Fatalf("LM Studio is not reachable: %v", err)
	}
	healthCancel()
	log.Printf("LM Studio OK (%s)", llmURL)

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

	manager := newBotManager(store, llmClient, channel, globalLimit)
	if err := manager.loadBots(ctx); err != nil {
		log.Fatalf("load bots error: %v", err)
	}

	if len(manager.listIDs(false)) == 0 {
		log.Printf("no bots found in database; add rows into table bots first")
	}

	manager.startEnabled(ctx)
	runConsole(ctx, manager)
	manager.stopAll()
}
