package api

import "time"

type BotDTO struct {
	ID              string `json:"id"`
	TwitchLogin     string `json:"twitch_login"`
	Name            string `json:"name"`
	Personality     string `json:"personality"`
	MinDelaySec     int    `json:"min_delay_sec"`
	MaxDelaySec     int    `json:"max_delay_sec"`
	RateLimitPerMin int    `json:"rate_limit_per_min"`
	Enabled         bool   `json:"enabled"`
	Status          string `json:"status"`
	ProxyID         string `json:"proxy_id,omitempty"`
	AutoRotateProxy bool   `json:"auto_rotate_proxy"`

	// Presence simulation state; pointers are set only when a value exists.
	PresenceState string     `json:"presence_state"`
	SessionEndsAt *time.Time `json:"session_ends_at,omitempty"`
	CooldownUntil *time.Time `json:"cooldown_until,omitempty"`

	// Imported Twitch token metadata (filled by the token-import flow).
	TokenExpiresAt  *time.Time `json:"token_expires_at"`
	TokenHasRefresh bool       `json:"token_has_refresh"`
	TokenScopes     []string   `json:"token_scopes"`
	TokenValid      bool       `json:"token_valid"`
}

type CreateBotRequest struct {
	ID              string `json:"id"`
	TwitchLogin     string `json:"twitch_login"`
	Password        string `json:"password"`
	Name            string `json:"name"`
	Personality     string `json:"personality"`
	MinDelaySec     int    `json:"min_delay_sec"`
	MaxDelaySec     int    `json:"max_delay_sec"`
	RateLimitPerMin int    `json:"rate_limit_per_min"`
}

type UpdateBotRequest struct {
	Name            *string `json:"name,omitempty"`
	Personality     *string `json:"personality,omitempty"`
	MinDelaySec     *int    `json:"min_delay_sec,omitempty"`
	MaxDelaySec     *int    `json:"max_delay_sec,omitempty"`
	RateLimitPerMin *int    `json:"rate_limit_per_min,omitempty"`
	ProxyID         *string `json:"proxy_id,omitempty"`
	AutoRotateProxy *bool   `json:"auto_rotate_proxy,omitempty"`
}

// ImportTokenRequest carries the raw tokens for POST /api/bots/{id}/token.
// The values are write-only: no response and no DTO ever echoes them back.
type ImportTokenRequest struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
}

type ProxyDTO struct {
	ID         string     `json:"id"`
	Host       string     `json:"host"`
	Port       int        `json:"port"`
	Username   string     `json:"username,omitempty"`
	Protocol   string     `json:"protocol"`
	Enabled    bool       `json:"enabled"`
	LastUsedAt *time.Time `json:"last_used_at,omitempty"`
	FailCount  int        `json:"fail_count"`
	CreatedAt  time.Time  `json:"created_at"`
}

type CreateProxyRequest struct {
	ID       string `json:"id"`
	Host     string `json:"host"`
	Port     int    `json:"port"`
	Username string `json:"username,omitempty"`
	Password string `json:"password,omitempty"`
	Protocol string `json:"protocol"`
}

type StatsDTO struct {
	TotalBots      int `json:"total_bots"`
	RunningBots    int `json:"running_bots"`
	MessagesPerMin int `json:"messages_per_min"`
	TotalProxies   int `json:"total_proxies"`
	EnabledProxies int `json:"enabled_proxies"`
}

type ErrorResponse struct {
	Error string `json:"error"`
}

type WSMessage struct {
	Type    string      `json:"type"`
	Payload interface{} `json:"payload"`
}

type BotStatusPayload struct {
	BotID  string `json:"bot_id"`
	Status string `json:"status"`
}

type BotPresencePayload struct {
	BotID string    `json:"bot_id"`
	Login string    `json:"login"`
	State string    `json:"state"`
	At    time.Time `json:"at"`
}

type BotMessagePayload struct {
	BotID       string    `json:"bot_id"`
	Message     string    `json:"message"`
	Personality string    `json:"personality"`
	Channel     string    `json:"channel,omitempty"`
	Timestamp   time.Time `json:"timestamp"`
}

type LLMStatusDTO struct {
	Provider  string `json:"provider"`
	Available bool   `json:"available"`
	Error     string `json:"error,omitempty"`
	Model     string `json:"model"`
	URL       string `json:"url"`
}

type BotLogPayload struct {
	BotID     string                 `json:"bot_id"`
	LogType   string                 `json:"log_type"`
	Level     string                 `json:"level"`
	Message   string                 `json:"message"`
	Context   map[string]interface{} `json:"context,omitempty"`
	Timestamp time.Time              `json:"timestamp"`
}

type SettingsLogPayload struct {
	Level     string                 `json:"level"`
	Message   string                 `json:"message"`
	Changes   map[string]interface{} `json:"changes,omitempty"`
	Timestamp time.Time              `json:"timestamp"`
}

// --- Temporary mail ---------------------------------------------------------

// CreateTempInboxRequest is the optional body of POST /api/temp-mail/inbox.
// An empty alias asks the provider for its own default address.
type CreateTempInboxRequest struct {
	Alias string `json:"alias,omitempty"`
}

// TempMailInbox is the public view of a temporary inbox. There is no sid
// field by design: the provider session token never leaves the backend.
type TempMailInbox struct {
	ID        string            `json:"id"`
	Email     string            `json:"email"`
	ExpiresAt time.Time         `json:"expires_at"`
	Messages  []TempMailMessage `json:"messages,omitempty"`
}

// TempMailMessage is one inbox entry without its body; bodies are fetched per
// message so a list response stays small.
type TempMailMessage struct {
	ID         string    `json:"id"`
	From       string    `json:"from"`
	Subject    string    `json:"subject"`
	Excerpt    string    `json:"excerpt"`
	ReceivedAt time.Time `json:"received_at"`
}

// TempMailMessageBody is a single message with its body converted to plain
// text by the backend.
type TempMailMessageBody struct {
	ID         string    `json:"id"`
	From       string    `json:"from"`
	Subject    string    `json:"subject"`
	Body       string    `json:"body"`
	ReceivedAt time.Time `json:"received_at"`
}
