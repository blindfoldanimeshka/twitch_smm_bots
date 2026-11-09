package storage

import (
	"context"
	"encoding/json"
	"fmt"
	"time"
)

// Settings represents global system settings
type Settings struct {
	ID                          string    `json:"id"`
	Channels                    []string  `json:"channels"`
	DebugMode                   bool      `json:"debug_mode"`
	DebugChannel                string    `json:"debug_channel"`
	PositivePrompt              string    `json:"positive_prompt"`
	NegativePrompt              string    `json:"negative_prompt"`
	MaxTokens                   int       `json:"max_tokens"`
	AudioEnabled                bool      `json:"audio_enabled"`
	RotationEnabled             bool      `json:"rotation_enabled"`
	RotationInterval            int       `json:"rotation_interval"` // minutes
	PresenceEnabled             bool      `json:"presence_enabled"`
	PresenceSessionMinMinutes   int       `json:"presence_session_min_minutes"`
	PresenceSessionMaxMinutes   int       `json:"presence_session_max_minutes"`
	PresenceCooldownMinMinutes  int       `json:"presence_cooldown_min_minutes"`
	PresenceCooldownMaxMinutes  int       `json:"presence_cooldown_max_minutes"`
	PresenceGreetingEnabled     bool      `json:"presence_greeting_enabled"`
	PresenceFarewellEnabled     bool      `json:"presence_farewell_enabled"`
	LLMProvider                 string    `json:"llm_provider"`
	LLMApiURL                   string    `json:"llm_api_url"`
	LLMModel                    string    `json:"llm_model"`
	LLMApiKey                   string    `json:"llm_api_key"`
	LLMPaidModel                string    `json:"llm_paid_model"`
	LLMPaidEnabled              bool      `json:"llm_paid_enabled"`
	LLMPaidDailyCapUSD          float64   `json:"llm_paid_daily_cap_usd"`
	LLMPaidEnabledEffective     bool      `json:"llm_paid_enabled_effective"`
	LLMPaidDailyCapUSDEffective float64   `json:"llm_paid_daily_cap_usd_effective"`
	LLMApiKeySet                bool      `json:"llm_api_key_set"`
	LLMApiKeySource             string    `json:"llm_api_key_source"`
	TwitchClientID              string    `json:"twitch_client_id"`
	TwitchClientSecret          string    `json:"twitch_client_secret"`
	TwitchClientSecretSet       bool      `json:"twitch_client_secret_set"`
	TwitchClientSecretClear     bool      `json:"twitch_client_secret_clear"` // request-only erase flag, never persisted
	UpdatedAt                   time.Time `json:"updated_at"`
}

const settingsTableSQL = `
CREATE TABLE IF NOT EXISTS settings (
    id TEXT PRIMARY KEY DEFAULT 'default',
    channels JSONB NOT NULL DEFAULT '[]',
    debug_mode BOOLEAN NOT NULL DEFAULT FALSE,
    debug_channel TEXT NOT NULL DEFAULT '',
    positive_prompt TEXT NOT NULL DEFAULT '',
    negative_prompt TEXT NOT NULL DEFAULT '',
    max_tokens INT NOT NULL DEFAULT 60,
    audio_enabled BOOLEAN NOT NULL DEFAULT FALSE,
    rotation_enabled BOOLEAN NOT NULL DEFAULT FALSE,
    rotation_interval INT NOT NULL DEFAULT 60,
    presence_enabled BOOLEAN NOT NULL DEFAULT TRUE,
    presence_session_min_minutes INT NOT NULL DEFAULT 20,
    presence_session_max_minutes INT NOT NULL DEFAULT 40,
    presence_cooldown_min_minutes INT NOT NULL DEFAULT 10,
    presence_cooldown_max_minutes INT NOT NULL DEFAULT 20,
    presence_greeting_enabled BOOLEAN NOT NULL DEFAULT TRUE,
    presence_farewell_enabled BOOLEAN NOT NULL DEFAULT TRUE,
    llm_provider TEXT NOT NULL DEFAULT 'openrouter',
    llm_api_url TEXT NOT NULL DEFAULT 'https://openrouter.ai/api/v1',
    llm_model TEXT NOT NULL DEFAULT 'openrouter/free',
    llm_api_key TEXT NOT NULL DEFAULT '',
    llm_paid_model TEXT NOT NULL DEFAULT 'z-ai/glm-5.3-flash',
    llm_paid_enabled BOOLEAN NOT NULL DEFAULT TRUE,
    llm_paid_daily_cap_usd DOUBLE PRECISION NOT NULL DEFAULT 1.0,
    twitch_client_id TEXT NOT NULL DEFAULT '',
    twitch_client_secret TEXT NOT NULL DEFAULT '',
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

INSERT INTO settings (id) VALUES ('default') ON CONFLICT (id) DO NOTHING;

ALTER TABLE settings ADD COLUMN IF NOT EXISTS llm_paid_model TEXT NOT NULL DEFAULT 'z-ai/glm-5.3-flash';
ALTER TABLE settings ADD COLUMN IF NOT EXISTS llm_paid_enabled BOOLEAN NOT NULL DEFAULT TRUE;
ALTER TABLE settings ADD COLUMN IF NOT EXISTS llm_paid_daily_cap_usd DOUBLE PRECISION NOT NULL DEFAULT 1.0;
ALTER TABLE settings ADD COLUMN IF NOT EXISTS presence_enabled BOOLEAN NOT NULL DEFAULT TRUE;
ALTER TABLE settings ADD COLUMN IF NOT EXISTS presence_session_min_minutes INT NOT NULL DEFAULT 20;
ALTER TABLE settings ADD COLUMN IF NOT EXISTS presence_session_max_minutes INT NOT NULL DEFAULT 40;
ALTER TABLE settings ADD COLUMN IF NOT EXISTS presence_cooldown_min_minutes INT NOT NULL DEFAULT 10;
ALTER TABLE settings ADD COLUMN IF NOT EXISTS presence_cooldown_max_minutes INT NOT NULL DEFAULT 20;
ALTER TABLE settings ADD COLUMN IF NOT EXISTS presence_greeting_enabled BOOLEAN NOT NULL DEFAULT TRUE;
ALTER TABLE settings ADD COLUMN IF NOT EXISTS presence_farewell_enabled BOOLEAN NOT NULL DEFAULT TRUE;

UPDATE settings
SET llm_provider = 'openrouter',
    llm_api_url = 'https://openrouter.ai/api/v1',
    llm_model = 'openrouter/free'
WHERE llm_provider = 'lmstudio'
   OR llm_api_url LIKE '%:1234%'
   OR llm_model = ''
   OR llm_model = 'local-model';
`

func (s *Store) EnsureSettingsTable(ctx context.Context) error {
	_, err := s.pool.Exec(ctx, settingsTableSQL)
	if err != nil {
		return fmt.Errorf("ensure settings table: %w", err)
	}
	return nil
}

func (s *Store) GetSettings(ctx context.Context) (*Settings, error) {
	ctx, cancel := context.WithTimeout(ctx, dbOperationTimeout)
	defer cancel()

	var settings Settings
	var channelsJSON []byte

	err := s.pool.QueryRow(ctx, `
		SELECT id, channels, debug_mode, debug_channel, positive_prompt, negative_prompt,
		       max_tokens, audio_enabled, rotation_enabled, rotation_interval,
		       presence_enabled, presence_session_min_minutes, presence_session_max_minutes,
		       presence_cooldown_min_minutes, presence_cooldown_max_minutes,
		       presence_greeting_enabled, presence_farewell_enabled,
		       llm_provider, llm_api_url, llm_model, llm_api_key,
		       llm_paid_model, llm_paid_enabled, llm_paid_daily_cap_usd,
		       twitch_client_id, twitch_client_secret, updated_at
		FROM settings WHERE id = 'default'
	`).Scan(
		&settings.ID,
		&channelsJSON,
		&settings.DebugMode,
		&settings.DebugChannel,
		&settings.PositivePrompt,
		&settings.NegativePrompt,
		&settings.MaxTokens,
		&settings.AudioEnabled,
		&settings.RotationEnabled,
		&settings.RotationInterval,
		&settings.PresenceEnabled,
		&settings.PresenceSessionMinMinutes,
		&settings.PresenceSessionMaxMinutes,
		&settings.PresenceCooldownMinMinutes,
		&settings.PresenceCooldownMaxMinutes,
		&settings.PresenceGreetingEnabled,
		&settings.PresenceFarewellEnabled,
		&settings.LLMProvider,
		&settings.LLMApiURL,
		&settings.LLMModel,
		&settings.LLMApiKey,
		&settings.LLMPaidModel,
		&settings.LLMPaidEnabled,
		&settings.LLMPaidDailyCapUSD,
		&settings.TwitchClientID,
		&settings.TwitchClientSecret,
		&settings.UpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("get settings: %w", err)
	}

	if err := json.Unmarshal(channelsJSON, &settings.Channels); err != nil {
		return nil, fmt.Errorf("unmarshal channels: %w", err)
	}

	return &settings, nil
}

func (s *Store) UpdateSettings(ctx context.Context, settings *Settings) error {
	ctx, cancel := context.WithTimeout(ctx, dbOperationTimeout)
	defer cancel()

	channelsJSON, err := json.Marshal(settings.Channels)
	if err != nil {
		return fmt.Errorf("marshal channels: %w", err)
	}

	_, err = s.pool.Exec(ctx, `
		UPDATE settings SET
			channels = $1,
			debug_mode = $2,
			debug_channel = $3,
			positive_prompt = $4,
			negative_prompt = $5,
			max_tokens = $6,
			audio_enabled = $7,
			rotation_enabled = $8,
			rotation_interval = $9,
			presence_enabled = $10,
			presence_session_min_minutes = $11,
			presence_session_max_minutes = $12,
			presence_cooldown_min_minutes = $13,
			presence_cooldown_max_minutes = $14,
			presence_greeting_enabled = $15,
			presence_farewell_enabled = $16,
			llm_provider = $17,
			llm_api_url = $18,
			llm_model = $19,
			llm_api_key = $20,
			llm_paid_model = $21,
			llm_paid_enabled = $22,
			llm_paid_daily_cap_usd = $23,
			twitch_client_id = $24,
			twitch_client_secret = $25,
			updated_at = NOW()
		WHERE id = 'default'
	`,
		channelsJSON,
		settings.DebugMode,
		settings.DebugChannel,
		settings.PositivePrompt,
		settings.NegativePrompt,
		settings.MaxTokens,
		settings.AudioEnabled,
		settings.RotationEnabled,
		settings.RotationInterval,
		settings.PresenceEnabled,
		settings.PresenceSessionMinMinutes,
		settings.PresenceSessionMaxMinutes,
		settings.PresenceCooldownMinMinutes,
		settings.PresenceCooldownMaxMinutes,
		settings.PresenceGreetingEnabled,
		settings.PresenceFarewellEnabled,
		settings.LLMProvider,
		settings.LLMApiURL,
		settings.LLMModel,
		settings.LLMApiKey,
		settings.LLMPaidModel,
		settings.LLMPaidEnabled,
		settings.LLMPaidDailyCapUSD,
		settings.TwitchClientID,
		settings.TwitchClientSecret,
	)
	if err != nil {
		return fmt.Errorf("update settings: %w", err)
	}

	return nil
}
