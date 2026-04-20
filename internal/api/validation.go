package api

import (
	"fmt"
	"regexp"
	"strings"
)

var (
	// OAuth токен должен начинаться с "oauth:" и содержать 30 символов после префикса
	oauthTokenRegex = regexp.MustCompile(`^oauth:[a-z0-9]{30}$`)
	// Twitch логин: 4-25 символов, буквы, цифры, подчеркивания
	twitchLoginRegex = regexp.MustCompile(`^[a-zA-Z0-9_]{4,25}$`)
)

// ValidateOAuthToken проверяет формат OAuth токена
func ValidateOAuthToken(token string) error {
	token = strings.TrimSpace(token)
	if token == "" {
		return fmt.Errorf("oauth token is empty")
	}

	// Автоматически добавляем префикс если его нет
	if !strings.HasPrefix(strings.ToLower(token), "oauth:") {
		token = "oauth:" + token
	}

	if !oauthTokenRegex.MatchString(strings.ToLower(token)) {
		return fmt.Errorf("invalid oauth token format (expected: oauth:xxxxx... with 30 chars)")
	}

	return nil
}

// ValidateTwitchLogin проверяет формат Twitch логина
func ValidateTwitchLogin(login string) error {
	login = strings.TrimSpace(login)
	if login == "" {
		return fmt.Errorf("twitch login is empty")
	}

	if !twitchLoginRegex.MatchString(login) {
		return fmt.Errorf("invalid twitch login (4-25 chars, alphanumeric and underscore only)")
	}

	return nil
}

// ValidatePersonality проверяет что личность существует
func ValidatePersonality(personality string) error {
	validPersonalities := map[string]bool{
		"веселый_фанат": true,
		"токсик":        true,
		"нейтрал":       true,
		"хайпер":        true,
	}

	if !validPersonalities[personality] {
		return fmt.Errorf("invalid personality (must be: веселый_фанат, токсик, нейтрал, хайпер)")
	}

	return nil
}

// ValidateBotConfig проверяет конфигурацию бота
func ValidateBotConfig(id, twitchLogin, oauthToken, personality string, minDelay, maxDelay, rateLimit int) error {
	if id == "" {
		return fmt.Errorf("bot id is required")
	}

	if err := ValidateTwitchLogin(twitchLogin); err != nil {
		return fmt.Errorf("twitch_login: %w", err)
	}

	if err := ValidateOAuthToken(oauthToken); err != nil {
		return fmt.Errorf("oauth_token: %w", err)
	}

	if err := ValidatePersonality(personality); err != nil {
		return fmt.Errorf("personality: %w", err)
	}

	if minDelay <= 0 {
		return fmt.Errorf("min_delay_sec must be > 0")
	}

	if maxDelay < minDelay {
		return fmt.Errorf("max_delay_sec must be >= min_delay_sec")
	}

	if rateLimit <= 0 {
		return fmt.Errorf("rate_limit_per_min must be > 0")
	}

	return nil
}
