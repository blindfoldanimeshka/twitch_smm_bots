package api

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/gorilla/mux"
	"github.com/rs/cors"
	"twitch-smm-bot/internal/llm"
	"twitch-smm-bot/internal/storage"
)

type Server struct {
	handler    *Handler
	hub        *Hub
	httpServer *http.Server
	apiKey     string
	corsOrigins []string
}

func NewServer(manager BotManager, store *storage.Store, llmClient *llm.Client, cacheClient CacheClient, streamMonitor StreamStatusProvider, apiKey string, port int, corsOrigins []string) *Server {
	hub := NewHub()
	handler := NewHandler(manager, store, hub, llmClient, cacheClient, streamMonitor)

	s := &Server{
		handler:     handler,
		hub:         hub,
		apiKey:      apiKey,
		corsOrigins: corsOrigins,
	}

	router := mux.NewRouter()

	// Rate limiter: 100 запросов в минуту на клиента
	rateLimiter := NewRateLimiter(100, time.Minute)
	go rateLimiter.Cleanup()

	// WebSocket endpoint без CORS middleware
	router.HandleFunc("/api/ws", func(w http.ResponseWriter, r *http.Request) {
		// Проверка API ключа для WebSocket
		if r.URL.Query().Get("api_key") != apiKey {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}
		hub.ServeWS(w, r)
	}).Methods("GET")

	api := router.PathPrefix("/api").Subrouter()
	api.Use(SecurityHeadersMiddleware)
	api.Use(RateLimitMiddleware(rateLimiter))
	api.Use(AuthMiddleware(apiKey))
	api.Use(LoggingMiddleware)

	api.HandleFunc("/health", handler.HealthCheck).Methods("GET")
	api.HandleFunc("/llm/status", handler.GetLLMStatus).Methods("GET")
	api.HandleFunc("/twitch/stream", handler.GetStreamStatus).Methods("GET")
	api.HandleFunc("/bots", handler.ListBots).Methods("GET")
	api.HandleFunc("/bots", handler.CreateBot).Methods("POST")
	api.HandleFunc("/bots/{id}", handler.GetBot).Methods("GET")
	api.HandleFunc("/bots/{id}", handler.UpdateBot).Methods("PUT")
	api.HandleFunc("/bots/{id}", handler.DeleteBot).Methods("DELETE")
	api.HandleFunc("/bots/{id}/start", handler.StartBot).Methods("POST")
	api.HandleFunc("/bots/{id}/stop", handler.StopBot).Methods("POST")
	api.HandleFunc("/bots/{id}/logs", handler.GetBotLogs).Methods("GET")
	api.HandleFunc("/bots/{id}/token", handler.ImportBotToken).Methods("POST")
	api.HandleFunc("/bots/{id}/token", handler.GetBotToken).Methods("GET")
	api.HandleFunc("/bots/{id}/token/refresh", handler.RefreshBotToken).Methods("POST")
	api.HandleFunc("/bots/start-all", handler.StartAll).Methods("POST")
	api.HandleFunc("/bots/stop-all", handler.StopAll).Methods("POST")
	api.HandleFunc("/stats", handler.GetStats).Methods("GET")
	api.HandleFunc("/proxies", handler.ListProxies).Methods("GET")
	api.HandleFunc("/proxies", handler.CreateProxy).Methods("POST")
	api.HandleFunc("/proxies/{id}", handler.DeleteProxy).Methods("DELETE")
	api.HandleFunc("/settings", handler.GetSettings).Methods("GET")
	api.HandleFunc("/settings", handler.UpdateSettings).Methods("POST")
	api.HandleFunc("/settings/logs", handler.GetSettingsLogs).Methods("GET")
	api.HandleFunc("/settings/test-audio", handler.TestAudio).Methods("GET")
	api.HandleFunc("/settings/test-llm", handler.TestLLM).Methods("POST")

	api.HandleFunc("/temp-mail/inbox", handler.CreateTempInbox).Methods("POST")
	api.HandleFunc("/temp-mail/inbox/{id}", handler.GetTempInbox).Methods("GET")
	api.HandleFunc("/temp-mail/inbox/{id}/messages/{mailID}", handler.GetTempMessage).Methods("GET")
	api.HandleFunc("/temp-mail/inbox/{id}", handler.DeleteTempInbox).Methods("DELETE")

	c := cors.New(cors.Options{
		AllowedOrigins:   corsOrigins,
		AllowedMethods:   []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"Content-Type", "X-API-Key"},
		AllowCredentials: true,
		Debug:            false,
	})

	s.httpServer = &http.Server{
		Addr:         fmt.Sprintf(":%d", port),
		Handler:      c.Handler(router),
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	return s
}

func (s *Server) Start() error {
	go s.hub.Run()

	log.Printf("API server starting on %s", s.httpServer.Addr)
	if err := s.httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		return fmt.Errorf("api server error: %w", err)
	}
	return nil
}

func (s *Server) Shutdown(ctx context.Context) error {
	log.Println("Shutting down API server...")
	return s.httpServer.Shutdown(ctx)
}

func (s *Server) Hub() *Hub {
	return s.hub
}

// SetTokenService wires the token import service into the API handlers.
func (s *Server) SetTokenService(svc TokenService) {
	s.handler.SetTokenService(svc)
}

// SetTempMailService wires the Guerrilla temporary mail service into the API
// handlers.
func (s *Server) SetTempMailService(svc TempMailService) {
	s.handler.SetTempMailService(svc)
}
