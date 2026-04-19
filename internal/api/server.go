package api

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/gorilla/mux"
	"github.com/rs/cors"
	"twitch-smm-bot/internal/storage"
)

type Server struct {
	handler    *Handler
	hub        *Hub
	httpServer *http.Server
	apiKey     string
	corsOrigins []string
}

func NewServer(manager BotManager, store *storage.Store, apiKey string, port int, corsOrigins []string) *Server {
	hub := NewHub()
	handler := NewHandler(manager, store, hub)

	s := &Server{
		handler:     handler,
		hub:         hub,
		apiKey:      apiKey,
		corsOrigins: corsOrigins,
	}

	router := mux.NewRouter()

	api := router.PathPrefix("/api").Subrouter()
	api.Use(AuthMiddleware(apiKey))
	api.Use(LoggingMiddleware)

	api.HandleFunc("/health", handler.HealthCheck).Methods("GET")
	api.HandleFunc("/bots", handler.ListBots).Methods("GET")
	api.HandleFunc("/bots", handler.CreateBot).Methods("POST")
	api.HandleFunc("/bots/{id}", handler.GetBot).Methods("GET")
	api.HandleFunc("/bots/{id}", handler.UpdateBot).Methods("PUT")
	api.HandleFunc("/bots/{id}", handler.DeleteBot).Methods("DELETE")
	api.HandleFunc("/bots/{id}/start", handler.StartBot).Methods("POST")
	api.HandleFunc("/bots/{id}/stop", handler.StopBot).Methods("POST")
	api.HandleFunc("/bots/start-all", handler.StartAll).Methods("POST")
	api.HandleFunc("/bots/stop-all", handler.StopAll).Methods("POST")
	api.HandleFunc("/stats", handler.GetStats).Methods("GET")
	api.HandleFunc("/proxies", handler.ListProxies).Methods("GET")
	api.HandleFunc("/proxies", handler.CreateProxy).Methods("POST")
	api.HandleFunc("/proxies/{id}", handler.DeleteProxy).Methods("DELETE")
	api.HandleFunc("/ws", hub.ServeWS).Methods("GET")

	c := cors.New(cors.Options{
		AllowedOrigins:   corsOrigins,
		AllowedMethods:   []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"Content-Type", "X-API-Key"},
		AllowCredentials: true,
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
