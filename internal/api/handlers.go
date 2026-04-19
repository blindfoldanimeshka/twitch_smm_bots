package api

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/gorilla/mux"
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

type BotInfo struct {
	Config  storage.BotConfig
	Running bool
}

type Handler struct {
	manager BotManager
	store   *storage.Store
	hub     *Hub
}

func NewHandler(manager BotManager, store *storage.Store, hub *Hub) *Handler {
	return &Handler{
		manager: manager,
		store:   store,
		hub:     hub,
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

	if req.ID == "" || req.TwitchLogin == "" || req.OAuthToken == "" {
		http.Error(w, `{"error":"missing required fields"}`, http.StatusBadRequest)
		return
	}

	cfg := storage.BotConfig{
		ID:              req.ID,
		TwitchLogin:     req.TwitchLogin,
		OAuthToken:      req.OAuthToken,
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

	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	if err := h.store.CreateBot(ctx, cfg); err != nil {
		http.Error(w, `{"error":"failed to create bot"}`, http.StatusInternalServerError)
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
