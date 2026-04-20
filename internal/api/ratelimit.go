package api

import (
	"net/http"
	"sync"
	"time"
)

// RateLimiter для защиты API от спама
type RateLimiter struct {
	mu       sync.Mutex
	requests map[string][]time.Time
	limit    int
	window   time.Duration
}

func NewRateLimiter(limit int, window time.Duration) *RateLimiter {
	return &RateLimiter{
		requests: make(map[string][]time.Time),
		limit:    limit,
		window:   window,
	}
}

func (rl *RateLimiter) Allow(clientID string) bool {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	now := time.Now()
	cutoff := now.Add(-rl.window)

	// Очистка старых запросов
	if times, exists := rl.requests[clientID]; exists {
		valid := make([]time.Time, 0, len(times))
		for _, t := range times {
			if t.After(cutoff) {
				valid = append(valid, t)
			}
		}
		rl.requests[clientID] = valid
	}

	// Проверка лимита
	if len(rl.requests[clientID]) >= rl.limit {
		return false
	}

	// Добавление нового запроса
	rl.requests[clientID] = append(rl.requests[clientID], now)
	return true
}

// Middleware для rate limiting
func RateLimitMiddleware(limiter *RateLimiter) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			clientID := r.RemoteAddr

			// Используем API key как идентификатор если есть
			if apiKey := r.Header.Get("X-API-Key"); apiKey != "" {
				clientID = apiKey
			}

			if !limiter.Allow(clientID) {
				http.Error(w, `{"error":"rate limit exceeded, try again later"}`, http.StatusTooManyRequests)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

// Cleanup периодически очищает старые записи
func (rl *RateLimiter) Cleanup() {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()

	for range ticker.C {
		rl.mu.Lock()
		now := time.Now()
		cutoff := now.Add(-rl.window)

		for clientID, times := range rl.requests {
			valid := make([]time.Time, 0)
			for _, t := range times {
				if t.After(cutoff) {
					valid = append(valid, t)
				}
			}
			if len(valid) == 0 {
				delete(rl.requests, clientID)
			} else {
				rl.requests[clientID] = valid
			}
		}
		rl.mu.Unlock()
	}
}
