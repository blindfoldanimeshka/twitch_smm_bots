package api

import (
	"errors"
	"fmt"
	"regexp"
	"strings"
)

var (
	// Twitch логин: 4-25 символов, буквы, цифры, подчеркивания
	twitchLoginRegex = regexp.MustCompile(`^[a-zA-Z0-9_]{4,25}$`)

	// Validation errors
	ErrPasswordEmpty     = errors.New("password is empty")
	ErrPasswordTooShort  = errors.New("password is too short (minimum 3 characters)")
	ErrLoginEmpty        = errors.New("twitch login is empty")
	ErrLoginInvalid      = errors.New("invalid twitch login (4-25 chars, alphanumeric and underscore only)")
	ErrPersonalityInvalid = errors.New("invalid personality (must be: веселый_фанат, токсик, нейтрал, хайпер)")
	ErrBotIDRequired     = errors.New("bot id is required")
	ErrMinDelayInvalid   = errors.New("min_delay_sec must be > 0")
	ErrMaxDelayInvalid   = errors.New("max_delay_sec must be >= min_delay_sec")
	ErrRateLimitInvalid  = errors.New("rate_limit_per_min must be > 0")
)

// ValidatePassword проверяет что пароль не пустой
func ValidatePassword(password string) error {
	password = strings.TrimSpace(password)
	if password == "" {
		return ErrPasswordEmpty
	}

	if len(password) < 3 {
		return ErrPasswordTooShort
	}

	return nil
}

// ValidateTwitchLogin проверяет формат Twitch логина
func ValidateTwitchLogin(login string) error {
	login = strings.TrimSpace(login)
	if login == "" {
		return ErrLoginEmpty
	}

	if !twitchLoginRegex.MatchString(login) {
		return ErrLoginInvalid
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
		return ErrPersonalityInvalid
	}

	return nil
}

// ValidateBotConfig проверяет конфигурацию бота
func ValidateBotConfig(id, twitchLogin, password, personality string, minDelay, maxDelay, rateLimit int) error {
	if id == "" {
		return ErrBotIDRequired
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
		return ErrMinDelayInvalid
	}

	if maxDelay < minDelay {
		return ErrMaxDelayInvalid
	}

	if rateLimit <= 0 {
		return ErrRateLimitInvalid
	}

	return nil
}
