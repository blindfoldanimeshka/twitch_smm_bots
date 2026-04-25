package api

import (
	"crypto/rand"
	"encoding/base64"
	"net/http"

	"github.com/gorilla/csrf"
)

// generateCSRFKey генерирует случайный ключ для CSRF защиты
func generateCSRFKey() []byte {
	key := make([]byte, 32)
	if _, err := rand.Read(key); err != nil {
		panic("failed to generate CSRF key: " + err.Error())
	}
	return key
}

// CSRFProtection создает middleware для защиты от CSRF атак
func CSRFProtection(secure bool) func(http.Handler) http.Handler {
	csrfKey := generateCSRFKey()

	return csrf.Protect(
		csrfKey,
		csrf.Secure(secure),           // Требовать HTTPS в production
		csrf.Path("/"),                 // Применять ко всем путям
		csrf.HttpOnly(true),            // HttpOnly cookie
		csrf.SameSite(csrf.SameSiteStrictMode), // Strict SameSite
		csrf.ErrorHandler(http.HandlerFunc(csrfErrorHandler)),
	)
}

// csrfErrorHandler обрабатывает ошибки CSRF
func csrfErrorHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusForbidden)
	w.Write([]byte(`{"error":"CSRF token validation failed"}`))
}

// CSRFToken возвращает CSRF токен для текущего запроса
func CSRFToken(r *http.Request) string {
	return csrf.Token(r)
}

// CSRFTokenHeader добавляет CSRF токен в заголовок ответа
func CSRFTokenHeader(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Добавляем CSRF токен в заголовок для использования в JavaScript
		w.Header().Set("X-CSRF-Token", csrf.Token(r))
		next.ServeHTTP(w, r)
	})
}

// generateSecureToken генерирует безопасный случайный токен
func generateSecureToken(length int) (string, error) {
	bytes := make([]byte, length)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return base64.URLEncoding.EncodeToString(bytes), nil
}
