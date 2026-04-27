package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"reflect"
	"strconv"
	"strings"
	"time"

	"github.com/gorilla/mux"
	"twitch-smm-bot/internal/llm"
	"twitch-smm-bot/internal/storage"
)

type BotManager interface {
	ListBots() []BotInfo
	GetBot(id string) (*BotInfo, error)
	StartBot(ctx context.Context, id string) error
	StopBot(id string) error
	StartAll(ctx context.Context) error
	StopAll() error
}

type SettingsManager interface {
	ApplySettings(ctx context.Context, settings *storage.Settings) error
}

type BotInfo struct {
	Config  storage.BotConfig
	Running bool
}

type Handler struct {
	manager         BotManager
	settingsManager SettingsManager
	store           *storage.Store
	hub             *Hub
	llmClient       *llm.Client
	cacheClient     CacheClient
}

type CacheClient interface {
	GetBotLogs(ctx context.Context, botID string, limit int) ([]storage.BotLog, error)
	GetSettingsLogs(ctx context.Context, limit int) ([]storage.SettingsLog, error)
	AddSettingsLog(ctx context.Context, log storage.SettingsLog) error
}

func NewHandler(manager BotManager, store *storage.Store, hub *Hub, llmClient *llm.Client, cacheClient CacheClient) *Handler {
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
	}
}

func (h *Handler) HealthCheck(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}

func (h *Handler) ListBots(w http.ResponseWriter, r *http.Request) {
	bots := h.manager.ListBots()
	dtos := make([]BotDTO, len(bots))

	for i, bot := range bots {
		status := "stopped"
		if bot.Running {
			status = "running"
		}

		dtos[i] = BotDTO{
			ID:              bot.Config.ID,
			TwitchLogin:     bot.Config.TwitchLogin,
			Name:            bot.Config.Name,
			Personality:     bot.Config.Personality,
			MinDelaySec:     bot.Config.MinDelaySec,
			MaxDelaySec:     bot.Config.MaxDelaySec,
			RateLimitPerMin: bot.Config.RateLimitPerMin,
			Enabled:         bot.Config.Enabled,
			Status:          status,
		}
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
	}

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

	if err := h.store.CreateBot(ctx, cfg); err != nil {
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

	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	if err := h.store.UpdateBot(ctx, botID, updates); err != nil {
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

	h.StopBot(w, r)

	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	if err := h.store.DeleteBot(ctx, botID); err != nil {
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

	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	if err := h.manager.StartBot(ctx, botID); err != nil {
		http.Error(w, `{"error":"failed to start bot"}`, http.StatusInternalServerError)
		return
	}

	h.hub.BroadcastBotStatus(botID, "running")

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

	h.hub.BroadcastBotStatus(botID, "stopped")

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
		TotalBots:   len(bots),
		RunningBots: 0,
	}

	for _, bot := range bots {
		if bot.Running {
			stats.RunningBots++
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

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(settings)
}

func (h *Handler) UpdateSettings(w http.ResponseWriter, r *http.Request) {
	var settings storage.Settings
	if err := json.NewDecoder(r.Body).Decode(&settings); err != nil {
		h.logSettingsChange("error", "Invalid settings format", nil)
		http.Error(w, `{"error":"invalid request"}`, http.StatusBadRequest)
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	oldSettings, _ := h.store.GetSettings(ctx)

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

	if old.RotationEnabled != new.RotationEnabled {
		changes["rotation_enabled"] = map[string]interface{}{
			"old": old.RotationEnabled,
			"new": new.RotationEnabled,
		}
	}

	if old.RotationInterval != new.RotationInterval {
		changes["rotation_interval"] = map[string]interface{}{
			"old": old.RotationInterval,
			"new": new.RotationInterval,
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
	if _, ok := changes["rotation_enabled"]; ok {
		parts = append(parts, "Rotation setting changed")
	}
	if _, ok := changes["rotation_interval"]; ok {
		parts = append(parts, "Rotation interval changed")
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

	if len(parts) == 0 {
		return "Settings saved (no changes detected)"
	}

	return "Settings saved: " + strings.Join(parts, ", ")
}

func (h *Handler) GetLLMStatus(w http.ResponseWriter, r *http.Request) {
	if h.llmClient == nil {
		respondJSON(w, map[string]interface{}{
			"available": false,
			"error":     "LLM client not initialized",
		})
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	err := h.llmClient.HealthCheck(ctx)
	available := err == nil
	provider := h.llmClient.GetProvider()

	response := map[string]interface{}{
		"available": available,
		"provider":  provider,
	}

	if err != nil {
		response["error"] = err.Error()
	}

	respondJSON(w, response)
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
