package storage

import (
	"context"
	"encoding/json"
	"fmt"
	"time"
)

// Settings represents global system settings
type Settings struct {
	ID               string   `json:"id"`
	Channels         []string `json:"channels"`
	DebugMode        bool     `json:"debug_mode"`
	DebugChannel     string   `json:"debug_channel"`
	PositivePrompt   string   `json:"positive_prompt"`
	NegativePrompt   string   `json:"negative_prompt"`
	MaxTokens        int      `json:"max_tokens"`
	AudioEnabled     bool     `json:"audio_enabled"`
	RotationEnabled  bool     `json:"rotation_enabled"`
	RotationInterval int      `json:"rotation_interval"` // minutes
	UpdatedAt        time.Time `json:"updated_at"`
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
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

INSERT INTO settings (id) VALUES ('default') ON CONFLICT (id) DO NOTHING;
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
		       max_tokens, audio_enabled, rotation_enabled, rotation_interval, updated_at
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
	)
	if err != nil {
		return fmt.Errorf("update settings: %w", err)
	}

	return nil
}
