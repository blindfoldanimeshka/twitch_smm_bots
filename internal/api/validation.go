package api

import (
	"fmt"
	"regexp"
	"strings"
)

var (
	// Twitch логин: 4-25 символов, буквы, цифры, подчеркивания
	twitchLoginRegex = regexp.MustCompile(`^[a-zA-Z0-9_]{4,25}$`)
)

// ValidatePassword проверяет что пароль не пустой
func ValidatePassword(password string) error {
	password = strings.TrimSpace(password)
	if password == "" {
		return fmt.Errorf("password is empty")
	}

	if len(password) < 3 {
		return fmt.Errorf("password is too short (minimum 3 characters)")
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
func ValidateBotConfig(id, twitchLogin, password, personality string, minDelay, maxDelay, rateLimit int) error {
	if id == "" {
		return fmt.Errorf("bot id is required")
	}

	if err := ValidateTwitchLogin(twitchLogin); err != nil {
		return fmt.Errorf("twitch_login: %w", err)
	}

	if err := ValidatePassword(password); err != nil {
		return fmt.Errorf("password: %w", err)
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
