package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"reflect"
	"strconv"
	"strings"
	"time"

	"github.com/gorilla/mux"
	"twitch-smm-bot/internal/llm"
	"twitch-smm-bot/internal/storage"
	"twitch-smm-bot/internal/twitch"
)

type BotManager interface {
	ListBots() []BotInfo
	GetBot(id string) (*BotInfo, error)
	CreateBot(ctx context.Context, cfg storage.BotConfig) error
	UpdateBot(ctx context.Context, id string, updates map[string]interface{}) error
	DeleteBot(ctx context.Context, id string) error
	StartBot(ctx context.Context, id string) error
	StartBotWithOptions(ctx context.Context, id string, disableLLM bool) error
	StopBot(id string) error
	StartAll(ctx context.Context) error
	StopAll() error
	MessagesPerMin() int
}

type SettingsManager interface {
	ApplySettings(ctx context.Context, settings *storage.Settings) error
	GetAudioContext() string
	TestLLMGeneration(ctx context.Context, personality string) (string, error)
}

type StoreInterface interface {
	GetSettings(ctx context.Context) (*storage.Settings, error)
	UpdateSettings(ctx context.Context, settings *storage.Settings) error
	CreateSettingsLog(ctx context.Context, log storage.SettingsLog) error
	GetSettingsLogs(ctx context.Context, limit int) ([]storage.SettingsLog, error)
	GetBotLogs(ctx context.Context, botID string, limit int) ([]storage.BotLog, error)
	ListProxies(ctx context.Context) ([]storage.Proxy, error)
	CreateProxy(ctx context.Context, proxy storage.Proxy) error
	DeleteProxy(ctx context.Context, id string) error
	Ping(ctx context.Context) error
}

// StreamStatusProvider serves public stream state for the monitored channels;
// *twitch.StreamMonitor is the production implementation.
type StreamStatusProvider interface {
	Channels() []string
	Snapshot(ctx context.Context, channel string, excludeLogins map[string]struct{}) twitch.StreamStatus
}

type BotInfo struct {
	Config  storage.BotConfig
	Running bool

	// Presence simulation state. The time pointers are set only when a
	// session end or cooldown is actually scheduled.
	PresenceState string
	SessionEndsAt *time.Time
	CooldownUntil *time.Time
}

type Handler struct {
	manager         BotManager
	settingsManager SettingsManager
	store           StoreInterface
	hub             *Hub
	llmClient       *llm.Client
	cacheClient     CacheClient
	streamMonitor   StreamStatusProvider
	envLookup       func(string) string
}

type CacheClient interface {
	GetBotLogs(ctx context.Context, botID string, limit int) ([]storage.BotLog, error)
	GetSettingsLogs(ctx context.Context, limit int) ([]storage.SettingsLog, error)
	AddSettingsLog(ctx context.Context, log storage.SettingsLog) error
	Ping(ctx context.Context) error
}

func NewHandler(manager BotManager, store StoreInterface, hub *Hub, llmClient *llm.Client, cacheClient CacheClient, streamMonitor StreamStatusProvider) *Handler {
	var settingsManager SettingsManager
	if sm, ok := manager.(SettingsManager); ok {
		settingsManager = sm
	}

	return &Handler{
		manager:         manager,
		settingsManager: settingsManager,
		store:           store,
		hub:             hub,
		llmClient:       llmClient,
		cacheClient:     cacheClient,
		streamMonitor:   streamMonitor,
		envLookup:       os.Getenv,
	}
}

// resolveSettings merges the stored settings with the process environment into a
// fully resolved llm.Config and reports where the API key came from.
func (h *Handler) resolveSettings(settings *storage.Settings) (llm.Config, string) {
	lookup := h.envLookup
	if lookup == nil {
		lookup = os.Getenv
	}
	return llm.ResolveConfig(llm.EnvFromLookup(lookup), llm.DBConfigFromSettings(settings))
}

func (h *Handler) HealthCheck(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	health := map[string]interface{}{
		"status":    "ok",
		"timestamp": time.Now().UTC().Format(time.RFC3339),
		"checks":    map[string]string{},
	}

	checks := health["checks"].(map[string]string)
	overallStatus := "ok"

	// PostgreSQL check
	if err := h.store.Ping(ctx); err != nil {
		checks["postgres"] = "error: " + err.Error()
		overallStatus = "degraded"
	} else {
		checks["postgres"] = "ok"
	}

	// Redis check
	if h.cacheClient != nil {
		if err := h.cacheClient.Ping(ctx); err != nil {
			checks["redis"] = "error: " + err.Error()
			overallStatus = "degraded"
		} else {
			checks["redis"] = "ok"
		}
	} else {
		checks["redis"] = "not configured"
	}

	// LLM API check
	if h.llmClient != nil {
		if err := h.llmClient.HealthCheck(ctx); err != nil {
			checks["llm"] = "error: " + err.Error()
			overallStatus = "degraded"
		} else {
			checks["llm"] = "ok"
		}
	} else {
		checks["llm"] = "not configured"
	}

	health["status"] = overallStatus
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(health)
}

// botDTOFromInfo maps a BotInfo to its API representation. Presence time
// pointers are copied so the DTO never aliases manager-owned time values.
func botDTOFromInfo(bot BotInfo) BotDTO {
	status := "stopped"
	if bot.Running {
		status = "running"
	}

	dto := BotDTO{
		ID:              bot.Config.ID,
		TwitchLogin:     bot.Config.TwitchLogin,
		Name:            bot.Config.Name,
		Personality:     bot.Config.Personality,
		MinDelaySec:     bot.Config.MinDelaySec,
		MaxDelaySec:     bot.Config.MaxDelaySec,
		RateLimitPerMin: bot.Config.RateLimitPerMin,
		Enabled:         bot.Config.Enabled,
		Status:          status,
		PresenceState:   bot.PresenceState,
	}
	if bot.SessionEndsAt != nil {
		endsAt := *bot.SessionEndsAt
		dto.SessionEndsAt = &endsAt
	}
	if bot.CooldownUntil != nil {
		until := *bot.CooldownUntil
		dto.CooldownUntil = &until
	}
	return dto
}

func (h *Handler) ListBots(w http.ResponseWriter, r *http.Request) {
	bots := h.manager.ListBots()
	dtos := make([]BotDTO, len(bots))

	for i, bot := range bots {
		dtos[i] = botDTOFromInfo(bot)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(dtos)
}

func (h *Handler) GetBot(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	botID := vars["id"]

	bot, err := h.manager.GetBot(botID)
	if err != nil {
		http.Error(w, `{"error":"bot not found"}`, http.StatusNotFound)
		return
	}

	dto := botDTOFromInfo(*bot)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(dto)
}

func (h *Handler) CreateBot(w http.ResponseWriter, r *http.Request) {
	var req CreateBotRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"invalid request"}`, http.StatusBadRequest)
		return
	}

	if req.ID == "" || req.TwitchLogin == "" || req.Password == "" {
		http.Error(w, `{"error":"missing required fields"}`, http.StatusBadRequest)
		return
	}

	cfg := storage.BotConfig{
		ID:              req.ID,
		TwitchLogin:     req.TwitchLogin,
		Password:        req.Password,
		Name:            req.Name,
		Personality:     req.Personality,
		MinDelaySec:     req.MinDelaySec,
		MaxDelaySec:     req.MaxDelaySec,
		RateLimitPerMin: req.RateLimitPerMin,
		Enabled:         false,
	}

	if cfg.MinDelaySec == 0 {
		cfg.MinDelaySec = 45
	}
	if cfg.MaxDelaySec == 0 {
		cfg.MaxDelaySec = 180
	}
	if cfg.RateLimitPerMin == 0 {
		cfg.RateLimitPerMin = 10
	}
	if cfg.Personality == "" {
		cfg.Personality = "нейтрал"
	}

	// Валидация конфигурации
	if err := ValidateBotConfig(cfg.ID, cfg.TwitchLogin, cfg.Password, cfg.Personality, cfg.MinDelaySec, cfg.MaxDelaySec, cfg.RateLimitPerMin); err != nil {
		http.Error(w, fmt.Sprintf(`{"error":"validation failed: %s"}`, err.Error()), http.StatusBadRequest)
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	if err := h.manager.CreateBot(ctx, cfg); err != nil {
		if strings.Contains(err.Error(), "duplicate") || strings.Contains(err.Error(), "unique") {
			http.Error(w, `{"error":"bot with this id already exists"}`, http.StatusConflict)
		} else {
			http.Error(w, `{"error":"failed to create bot"}`, http.StatusInternalServerError)
		}
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(map[string]string{"id": cfg.ID})
}

func (h *Handler) UpdateBot(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	botID := vars["id"]

	var req UpdateBotRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"invalid request"}`, http.StatusBadRequest)
		return
	}

	updates := make(map[string]interface{})
	if req.Name != nil {
		updates["name"] = *req.Name
	}
	if req.Personality != nil {
		updates["personality"] = *req.Personality
	}
	if req.MinDelaySec != nil {
		updates["min_delay_sec"] = *req.MinDelaySec
	}
	if req.MaxDelaySec != nil {
		updates["max_delay_sec"] = *req.MaxDelaySec
	}
	if req.RateLimitPerMin != nil {
		updates["rate_limit_per_min"] = *req.RateLimitPerMin
	}
	if req.ProxyID != nil {
		updates["proxy_id"] = *req.ProxyID
	}
	if req.AutoRotateProxy != nil {
		updates["auto_rotate_proxy"] = *req.AutoRotateProxy
	}

	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	if err := h.manager.UpdateBot(ctx, botID, updates); err != nil {
		if strings.Contains(err.Error(), "not found") {
			http.Error(w, `{"error":"bot not found"}`, http.StatusNotFound)
		} else {
			http.Error(w, `{"error":"failed to update bot"}`, http.StatusInternalServerError)
		}
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "updated"})
}

func (h *Handler) DeleteBot(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	botID := vars["id"]

	_ = h.manager.StopBot(botID)
	if h.hub != nil {
		h.hub.BroadcastBotStatus(botID, "stopped")
	}

	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	if err := h.manager.DeleteBot(ctx, botID); err != nil {
		if strings.Contains(err.Error(), "not found") {
			http.Error(w, `{"error":"bot not found"}`, http.StatusNotFound)
		} else {
			http.Error(w, `{"error":"failed to delete bot"}`, http.StatusInternalServerError)
		}
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "deleted"})
}

func (h *Handler) StartBot(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	botID := vars["id"]

	// Parse disable_llm query parameter
	disableLLM := r.URL.Query().Get("disable_llm") == "true"

	// Используем context.Background() для долгоживущего бота, а не контекст запроса
	var err error
	if disableLLM {
		err = h.manager.StartBotWithOptions(context.Background(), botID, disableLLM)
	} else {
		err = h.manager.StartBot(context.Background(), botID)
	}
	if err != nil {
		http.Error(w, `{"error":"failed to start bot"}`, http.StatusInternalServerError)
		return
	}

	if h.hub != nil {
		h.hub.BroadcastBotStatus(botID, "running")
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "started"})
}

func (h *Handler) StopBot(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	botID := vars["id"]

	if err := h.manager.StopBot(botID); err != nil {
		http.Error(w, `{"error":"failed to stop bot"}`, http.StatusInternalServerError)
		return
	}

	if h.hub != nil {
		h.hub.BroadcastBotStatus(botID, "stopped")
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "stopped"})
}

func (h *Handler) StartAll(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	if err := h.manager.StartAll(ctx); err != nil {
		http.Error(w, `{"error":"failed to start all bots"}`, http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "all started"})
}

func (h *Handler) StopAll(w http.ResponseWriter, r *http.Request) {
	if err := h.manager.StopAll(); err != nil {
		http.Error(w, `{"error":"failed to stop all bots"}`, http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "all stopped"})
}

func (h *Handler) GetStats(w http.ResponseWriter, r *http.Request) {
	bots := h.manager.ListBots()

	stats := StatsDTO{
		TotalBots:      len(bots),
		RunningBots:    0,
		MessagesPerMin: h.manager.MessagesPerMin(),
		TotalProxies:   0,
		EnabledProxies: 0,
	}

	for _, bot := range bots {
		if bot.Running {
			stats.RunningBots++
		}
	}

	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	proxies, err := h.store.ListProxies(ctx)
	if err == nil {
		stats.TotalProxies = len(proxies)
		for _, p := range proxies {
			if p.Enabled {
				stats.EnabledProxies++
			}
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(stats)
}

func (h *Handler) ListProxies(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	proxies, err := h.store.ListProxies(ctx)
	if err != nil {
		http.Error(w, `{"error":"failed to list proxies"}`, http.StatusInternalServerError)
		return
	}

	dtos := make([]ProxyDTO, len(proxies))
	for i, p := range proxies {
		dtos[i] = ProxyDTO{
			ID:         p.ID,
			Host:       p.Host,
			Port:       p.Port,
			Username:   p.Username,
			Protocol:   p.Protocol,
			Enabled:    p.Enabled,
			LastUsedAt: p.LastUsedAt,
			FailCount:  p.FailCount,
			CreatedAt:  p.CreatedAt,
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(dtos)
}

func (h *Handler) CreateProxy(w http.ResponseWriter, r *http.Request) {
	var req CreateProxyRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"invalid request"}`, http.StatusBadRequest)
		return
	}

	if req.ID == "" || req.Host == "" || req.Port == 0 {
		http.Error(w, `{"error":"missing required fields"}`, http.StatusBadRequest)
		return
	}

	if req.Protocol == "" {
		req.Protocol = "http"
	}

	proxy := storage.Proxy{
		ID:        req.ID,
		Host:      req.Host,
		Port:      req.Port,
		Username:  req.Username,
		Password:  req.Password,
		Protocol:  req.Protocol,
		Enabled:   true,
		FailCount: 0,
	}

	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	if err := h.store.CreateProxy(ctx, proxy); err != nil {
		http.Error(w, `{"error":"failed to create proxy"}`, http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(map[string]string{"id": proxy.ID})
}

func (h *Handler) DeleteProxy(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	proxyID := vars["id"]

	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	if err := h.store.DeleteProxy(ctx, proxyID); err != nil {
		if strings.Contains(err.Error(), "not found") {
			http.Error(w, `{"error":"proxy not found"}`, http.StatusNotFound)
		} else {
			http.Error(w, `{"error":"failed to delete proxy"}`, http.StatusInternalServerError)
		}
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "deleted"})
}

func (h *Handler) GetSettings(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	settings, err := h.store.GetSettings(ctx)
	if err != nil {
		http.Error(w, `{"error":"failed to get settings"}`, http.StatusInternalServerError)
		return
	}

	cfg, source := h.resolveSettings(settings)
	settings.LLMApiKey = ""
	settings.LLMApiKeySet = cfg.APIKey != ""
	settings.LLMApiKeySource = source
	settings.LLMPaidEnabledEffective = cfg.PaidEnabled
	settings.LLMPaidDailyCapUSDEffective = cfg.PaidDailyCapUSD

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(settings)
}

// presenceRangesValid reports whether the presence ranges in settings satisfy
// 1 <= min <= max <= 240 for both the session and the cooldown pair. Zeros
// are invalid: the database carries valid defaults (20/40/10/20), so a real
// GET->POST round-trip always sends ranges.
func presenceRangesValid(s *storage.Settings) bool {
	sessionMin, sessionMax := s.PresenceSessionMinMinutes, s.PresenceSessionMaxMinutes
	cooldownMin, cooldownMax := s.PresenceCooldownMinMinutes, s.PresenceCooldownMaxMinutes

	return sessionMin >= 1 && sessionMin <= sessionMax && sessionMax <= 240 &&
		cooldownMin >= 1 && cooldownMin <= cooldownMax && cooldownMax <= 240
}

func (h *Handler) UpdateSettings(w http.ResponseWriter, r *http.Request) {
	var settings storage.Settings
	if err := json.NewDecoder(r.Body).Decode(&settings); err != nil {
		h.logSettingsChange("error", "Invalid settings format", nil)
		http.Error(w, `{"error":"invalid request"}`, http.StatusBadRequest)
		return
	}

	// Presence ranges must be sane before anything is read, written or
	// applied: both pairs must satisfy 1 <= min <= max <= 240. Zeros are
	// rejected — the database carries valid defaults, so a real round-trip
	// always sends ranges.
	if !presenceRangesValid(&settings) {
		http.Error(w, `{"error":"presence ranges must satisfy 1 <= min <= max <= 240"}`, http.StatusBadRequest)
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	oldSettings, err := h.store.GetSettings(ctx)
	if err != nil {
		h.logSettingsChange("error", "Failed to load current settings", nil)
		http.Error(w, `{"error":"failed to load current settings"}`, http.StatusInternalServerError)
		return
	}

	// An empty incoming key means "keep the stored database key".
	if settings.LLMApiKey == "" {
		settings.LLMApiKey = oldSettings.LLMApiKey
	}

	// Overwrite incoming computed fields from resolution so the client cannot
	// fabricate effective values or key provenance.
	cfg, source := h.resolveSettings(&settings)
	settings.LLMApiKeySet = cfg.APIKey != ""
	settings.LLMApiKeySource = source
	settings.LLMPaidEnabledEffective = cfg.PaidEnabled
	settings.LLMPaidDailyCapUSDEffective = cfg.PaidDailyCapUSD

	if err := h.store.UpdateSettings(ctx, &settings); err != nil {
		h.logSettingsChange("error", "Failed to save settings", nil)
		http.Error(w, `{"error":"failed to update settings"}`, http.StatusInternalServerError)
		return
	}

	changes := h.calculateSettingsChanges(oldSettings, &settings)
	message := h.formatSettingsChangeMessage(changes)
	h.logSettingsChange("success", message, changes)

	if h.settingsManager != nil {
		if err := h.settingsManager.ApplySettings(ctx, &settings); err != nil {
			h.logSettingsChange("warning", "Settings saved but runtime apply failed: "+err.Error(), nil)
			http.Error(w, `{"error":"settings saved but failed to apply at runtime"}`, http.StatusInternalServerError)
			return
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "updated"})
}

func (h *Handler) logSettingsChange(level, message string, changes map[string]interface{}) {
	log := storage.SettingsLog{
		Level:     level,
		Message:   message,
		Changes:   changes,
		CreatedAt: time.Now(),
	}

	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		_ = h.store.CreateSettingsLog(ctx, log)
	}()

	if h.cacheClient != nil {
		go func() {
			ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
			defer cancel()
			_ = h.cacheClient.AddSettingsLog(ctx, log)
		}()
	}

	if h.hub != nil {
		h.hub.BroadcastSettingsLog(log)
	}
}

func (h *Handler) calculateSettingsChanges(old, new *storage.Settings) map[string]interface{} {
	changes := make(map[string]interface{})

	if old == nil {
		old = &storage.Settings{}
	}

	if !reflect.DeepEqual(old.Channels, new.Channels) {
		changes["channels"] = map[string]interface{}{
			"old": old.Channels,
			"new": new.Channels,
		}
	}

	if old.LLMApiURL != new.LLMApiURL {
		changes["llm_api_url"] = map[string]interface{}{
			"old": old.LLMApiURL,
			"new": new.LLMApiURL,
		}
	}

	if old.LLMModel != new.LLMModel {
		changes["llm_model"] = map[string]interface{}{
			"old": old.LLMModel,
			"new": new.LLMModel,
		}
	}

	if old.AudioEnabled != new.AudioEnabled {
		changes["audio_enabled"] = map[string]interface{}{
			"old": old.AudioEnabled,
			"new": new.AudioEnabled,
		}
	}

	// Presence simulation. Rotation fields are deliberately absent: rotation
	// was removed from the runtime; the DB columns stay for compatibility.
	if old.PresenceEnabled != new.PresenceEnabled {
		changes["presence_enabled"] = map[string]interface{}{
			"old": old.PresenceEnabled,
			"new": new.PresenceEnabled,
		}
	}

	if old.PresenceSessionMinMinutes != new.PresenceSessionMinMinutes {
		changes["presence_session_min_minutes"] = map[string]interface{}{
			"old": old.PresenceSessionMinMinutes,
			"new": new.PresenceSessionMinMinutes,
		}
	}

	if old.PresenceSessionMaxMinutes != new.PresenceSessionMaxMinutes {
		changes["presence_session_max_minutes"] = map[string]interface{}{
			"old": old.PresenceSessionMaxMinutes,
			"new": new.PresenceSessionMaxMinutes,
		}
	}

	if old.PresenceCooldownMinMinutes != new.PresenceCooldownMinMinutes {
		changes["presence_cooldown_min_minutes"] = map[string]interface{}{
			"old": old.PresenceCooldownMinMinutes,
			"new": new.PresenceCooldownMinMinutes,
		}
	}

	if old.PresenceCooldownMaxMinutes != new.PresenceCooldownMaxMinutes {
		changes["presence_cooldown_max_minutes"] = map[string]interface{}{
			"old": old.PresenceCooldownMaxMinutes,
			"new": new.PresenceCooldownMaxMinutes,
		}
	}

	if old.PresenceGreetingEnabled != new.PresenceGreetingEnabled {
		changes["presence_greeting_enabled"] = map[string]interface{}{
			"old": old.PresenceGreetingEnabled,
			"new": new.PresenceGreetingEnabled,
		}
	}

	if old.PresenceFarewellEnabled != new.PresenceFarewellEnabled {
		changes["presence_farewell_enabled"] = map[string]interface{}{
			"old": old.PresenceFarewellEnabled,
			"new": new.PresenceFarewellEnabled,
		}
	}

	if old.DebugMode != new.DebugMode {
		changes["debug_mode"] = map[string]interface{}{
			"old": old.DebugMode,
			"new": new.DebugMode,
		}
	}

	if old.PositivePrompt != new.PositivePrompt {
		changes["positive_prompt"] = map[string]interface{}{
			"old": old.PositivePrompt,
			"new": new.PositivePrompt,
		}
	}

	if old.NegativePrompt != new.NegativePrompt {
		changes["negative_prompt"] = map[string]interface{}{
			"old": old.NegativePrompt,
			"new": new.NegativePrompt,
		}
	}

	if old.MaxTokens != new.MaxTokens {
		changes["max_tokens"] = map[string]interface{}{
			"old": old.MaxTokens,
			"new": new.MaxTokens,
		}
	}

	if old.LLMPaidModel != new.LLMPaidModel {
		changes["llm_paid_model"] = map[string]interface{}{
			"old": old.LLMPaidModel,
			"new": new.LLMPaidModel,
		}
	}

	if old.LLMPaidEnabled != new.LLMPaidEnabled {
		changes["llm_paid_enabled"] = map[string]interface{}{
			"old": old.LLMPaidEnabled,
			"new": new.LLMPaidEnabled,
		}
	}

	if old.LLMPaidDailyCapUSD != new.LLMPaidDailyCapUSD {
		changes["llm_paid_daily_cap_usd"] = map[string]interface{}{
			"old": old.LLMPaidDailyCapUSD,
			"new": new.LLMPaidDailyCapUSD,
		}
	}

	if old.LLMApiKey != new.LLMApiKey {
		changes["llm_api_key"] = map[string]interface{}{
			"old": "***",
			"new": "***",
		}
	}

	return changes
}

func (h *Handler) formatSettingsChangeMessage(changes map[string]interface{}) string {
	parts := []string{}

	if _, ok := changes["channels"]; ok {
		parts = append(parts, "Channels updated")
	}
	if _, ok := changes["llm_api_url"]; ok {
		parts = append(parts, "LLM URL changed")
	}
	if _, ok := changes["llm_model"]; ok {
		parts = append(parts, "LLM model changed")
	}
	if _, ok := changes["audio_enabled"]; ok {
		parts = append(parts, "Audio setting changed")
	}
	if _, ok := changes["presence_enabled"]; ok {
		parts = append(parts, "Presence simulation changed")
	}
	if _, ok := changes["presence_session_min_minutes"]; ok {
		parts = append(parts, "Presence session min changed")
	}
	if _, ok := changes["presence_session_max_minutes"]; ok {
		parts = append(parts, "Presence session max changed")
	}
	if _, ok := changes["presence_cooldown_min_minutes"]; ok {
		parts = append(parts, "Presence cooldown min changed")
	}
	if _, ok := changes["presence_cooldown_max_minutes"]; ok {
		parts = append(parts, "Presence cooldown max changed")
	}
	if _, ok := changes["presence_greeting_enabled"]; ok {
		parts = append(parts, "Presence greeting changed")
	}
	if _, ok := changes["presence_farewell_enabled"]; ok {
		parts = append(parts, "Presence farewell changed")
	}
	if _, ok := changes["debug_mode"]; ok {
		parts = append(parts, "Debug mode changed")
	}
	if _, ok := changes["positive_prompt"]; ok {
		parts = append(parts, "Positive prompt updated")
	}
	if _, ok := changes["negative_prompt"]; ok {
		parts = append(parts, "Negative prompt updated")
	}
	if _, ok := changes["max_tokens"]; ok {
		parts = append(parts, "Max tokens changed")
	}
	if _, ok := changes["llm_paid_model"]; ok {
		parts = append(parts, "Paid model changed")
	}
	if _, ok := changes["llm_paid_enabled"]; ok {
		parts = append(parts, "Paid mode changed")
	}
	if _, ok := changes["llm_paid_daily_cap_usd"]; ok {
		parts = append(parts, "Paid daily cap changed")
	}
	if _, ok := changes["llm_api_key"]; ok {
		parts = append(parts, "API key changed")
	}

	if len(parts) == 0 {
		return "Settings saved (no changes detected)"
	}

	return "Settings saved: " + strings.Join(parts, ", ")
}

func (h *Handler) GetLLMStatus(w http.ResponseWriter, r *http.Request) {
	if h.llmClient == nil {
		respondJSON(w, map[string]interface{}{
			"available":            false,
			"generation_available": false,
			"error":                "LLM client not initialized",
			"provider":             "Unknown",
			"model":                "N/A",
			"url":                  "N/A",
		})
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	st := h.llmClient.Status(ctx)

	response := map[string]interface{}{
		"available":               st.Available,
		"generation_available":    st.GenerationAvailable,
		"provider":                st.Provider,
		"model":                   st.FreeModel,
		"free_model":              st.FreeModel,
		"paid_model":              st.PaidModel,
		"url":                     h.llmClient.GetURL(),
		"paid_enabled":            st.PaidEnabled,
		"paid_daily_cap_usd":      st.PaidDailyCapUSD,
		"paid_spent_today_usd":    st.PaidSpentTodayUSD,
		"free_requests_limit":     st.FreeRequestsLimit,
		"free_requests_remaining": st.FreeRequestsRemaining,
		"free_requests_used":      st.FreeRequestsUsed,
		"free_cooldown_until":     st.FreeCooldownUntil,
		"inflight_paid_requests":  st.InflightPaidRequests,
		"last_error":              st.LastError,
		"checked_at":              st.CheckedAt,
	}

	if st.LastError != "" {
		response["error"] = st.LastError
	}

	respondJSON(w, response)
}

// GetStreamStatus serves the public state of one monitored channel. Without a
// channel query it reports the first configured one; it always answers 200 and
// reports problems through the status Error field instead of failing.
func (h *Handler) GetStreamStatus(w http.ResponseWriter, r *http.Request) {
	if h.streamMonitor == nil {
		respondJSON(w, map[string]interface{}{"configured": false})
		return
	}

	channel := twitch.NormalizeChannel(r.URL.Query().Get("channel"))
	if channel != "" {
		configured := false
		for _, ch := range h.streamMonitor.Channels() {
			if ch == channel {
				configured = true
				break
			}
		}
		if !configured {
			respondError(w, "channel is not configured", http.StatusBadRequest)
			return
		}
	}

	exclude := make(map[string]struct{})
	for _, bot := range h.manager.ListBots() {
		login := strings.ToLower(strings.TrimSpace(bot.Config.TwitchLogin))
		if login != "" {
			exclude[login] = struct{}{}
		}
	}

	respondJSON(w, h.streamMonitor.Snapshot(r.Context(), channel, exclude))
}

func (h *Handler) GetBotLogs(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	botID := vars["id"]

	limitStr := r.URL.Query().Get("limit")
	limit := 100
	if limitStr != "" {
		if parsedLimit, err := strconv.Atoi(limitStr); err == nil && parsedLimit > 0 {
			limit = parsedLimit
		}
	}

	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	var logs []storage.BotLog
	var err error

	if h.cacheClient != nil {
		logs, err = h.cacheClient.GetBotLogs(ctx, botID, limit)
		if err == nil && len(logs) > 0 {
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(logs)
			return
		}
	}

	logs, err = h.store.GetBotLogs(ctx, botID, limit)
	if err != nil {
		w.Header().Set("X-Log-Status", "unavailable")
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode([]storage.BotLog{})
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(logs)
}

func (h *Handler) GetSettingsLogs(w http.ResponseWriter, r *http.Request) {
	limitStr := r.URL.Query().Get("limit")
	limit := 50
	if limitStr != "" {
		if parsedLimit, err := strconv.Atoi(limitStr); err == nil && parsedLimit > 0 {
			limit = parsedLimit
		}
	}

	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	var logs []storage.SettingsLog
	var err error

	if h.cacheClient != nil {
		logs, err = h.cacheClient.GetSettingsLogs(ctx, limit)
		if err == nil && len(logs) > 0 {
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(logs)
			return
		}
	}

	logs, err = h.store.GetSettingsLogs(ctx, limit)
	if err != nil {
		w.Header().Set("X-Log-Status", "unavailable")
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode([]storage.SettingsLog{})
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(logs)
}

func (h *Handler) TestAudio(w http.ResponseWriter, r *http.Request) {
	if h.settingsManager == nil {
		http.Error(w, `{"error":"settings manager not available"}`, http.StatusServiceUnavailable)
		return
	}

	audioContext := h.settingsManager.GetAudioContext()

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"context": audioContext,
		"status":  "ok",
	})
}

func (h *Handler) TestLLM(w http.ResponseWriter, r *http.Request) {
	if h.settingsManager == nil {
		http.Error(w, `{"error":"settings manager not available"}`, http.StatusServiceUnavailable)
		return
	}

	var req struct {
		Personality string `json:"personality"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"invalid request"}`, http.StatusBadRequest)
		return
	}

	if req.Personality == "" {
		req.Personality = "нейтрал"
	}

	ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
	defer cancel()

	message, err := h.settingsManager.TestLLMGeneration(ctx, req.Personality)
	if err != nil {
		http.Error(w, fmt.Sprintf(`{"error":"LLM generation failed: %s"}`, err.Error()), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"message": message,
		"status":  "ok",
	})
}
