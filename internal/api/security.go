package api

import (
	"net/http"
)

// SecurityHeadersMiddleware добавляет security headers ко всем ответам
func SecurityHeadersMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Content Security Policy
		w.Header().Set("Content-Security-Policy",
			"default-src 'self'; "+
			"script-src 'self' 'unsafe-inline'; "+
			"style-src 'self' 'unsafe-inline'; "+
			"img-src 'self' data: https:; "+
			"font-src 'self' data:; "+
			"connect-src 'self' ws: wss:; "+
			"frame-ancestors 'none'")

		// Защита от clickjacking
		w.Header().Set("X-Frame-Options", "DENY")

		// Предотвращение MIME-type sniffing
		w.Header().Set("X-Content-Type-Options", "nosniff")

		// XSS Protection (для старых браузеров)
		w.Header().Set("X-XSS-Protection", "1; mode=block")

		// Referrer Policy
		w.Header().Set("Referrer-Policy", "strict-origin-when-cross-origin")

		// Permissions Policy (ограничение доступа к API браузера)
		w.Header().Set("Permissions-Policy",
			"geolocation=(), microphone=(), camera=(), payment=()")

		next.ServeHTTP(w, r)
	})
}
