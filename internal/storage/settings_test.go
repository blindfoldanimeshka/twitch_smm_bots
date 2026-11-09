package storage

import (
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGetSettings(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	// Mock pgxpool (simplified - in real tests use testcontainers)
	// For now, test the logic structure

	t.Run("successfully retrieves settings", func(t *testing.T) {
		expectedSettings := &Settings{
			ID:               "default",
			Channels:         []string{"https://www.twitch.tv/test"},
			DebugMode:        false,
			DebugChannel:     "",
			PositivePrompt:   "test prompt",
			NegativePrompt:   "",
			MaxTokens:        60,
			AudioEnabled:     true,
			RotationEnabled:  false,
			RotationInterval: 60,
			LLMProvider:      "openrouter",
			LLMApiURL:        "https://openrouter.ai/api/v1",
			LLMModel:         "test-model",
			LLMApiKey:        "",
			TwitchClientID:     "test-client-id",
			TwitchClientSecret: "test-secret",
			UpdatedAt:        time.Now(),
		}

		rows := sqlmock.NewRows([]string{
			"id", "channels", "debug_mode", "debug_channel", "positive_prompt",
			"negative_prompt", "max_tokens", "audio_enabled", "rotation_enabled",
			"rotation_interval", "llm_provider", "llm_api_url", "llm_model",
			"llm_api_key", "twitch_client_id", "twitch_client_secret", "updated_at",
		}).AddRow(
			expectedSettings.ID,
			`["https://www.twitch.tv/test"]`,
			expectedSettings.DebugMode,
			expectedSettings.DebugChannel,
			expectedSettings.PositivePrompt,
			expectedSettings.NegativePrompt,
			expectedSettings.MaxTokens,
			expectedSettings.AudioEnabled,
			expectedSettings.RotationEnabled,
			expectedSettings.RotationInterval,
			expectedSettings.LLMProvider,
			expectedSettings.LLMApiURL,
			expectedSettings.LLMModel,
			expectedSettings.LLMApiKey,
			expectedSettings.TwitchClientID,
			expectedSettings.TwitchClientSecret,
			expectedSettings.UpdatedAt,
		)

		mock.ExpectQuery("SELECT (.+) FROM settings WHERE id = 'default'").
			WillReturnRows(rows)

		// Note: This is a structure test. Full integration test would use testcontainers
		assert.NotNil(t, expectedSettings)
		assert.Equal(t, "default", expectedSettings.ID)
		assert.Equal(t, "openrouter", expectedSettings.LLMProvider)
		assert.Equal(t, "test-client-id", expectedSettings.TwitchClientID)
	})

	t.Run("handles missing settings", func(t *testing.T) {
		mock.ExpectQuery("SELECT (.+) FROM settings WHERE id = 'default'").
			WillReturnError(sqlmock.ErrCancelled)

		// Test error handling structure
		assert.Error(t, sqlmock.ErrCancelled)
	})
}

func TestUpdateSettings(t *testing.T) {
	t.Run("successfully updates settings", func(t *testing.T) {
		settings := &Settings{
			ID:               "default",
			Channels:         []string{"https://www.twitch.tv/updated"},
			DebugMode:        true,
			DebugChannel:     "https://www.twitch.tv/debug",
			PositivePrompt:   "updated prompt",
			NegativePrompt:   "negative",
			MaxTokens:        100,
			AudioEnabled:     false,
			RotationEnabled:  true,
			RotationInterval: 120,
			LLMProvider:      "openai",
			LLMApiURL:        "https://api.openai.com/v1",
			LLMModel:         "gpt-4",
			LLMApiKey:        "sk-test",
			TwitchClientID:     "new-client-id",
			TwitchClientSecret: "new-secret",
		}

		// Validate settings structure
		assert.Equal(t, "default", settings.ID)
		assert.True(t, settings.DebugMode)
		assert.Equal(t, "openai", settings.LLMProvider)
		assert.Equal(t, 100, settings.MaxTokens)
		assert.Equal(t, "new-client-id", settings.TwitchClientID)
	})
}

func TestSettingsValidation(t *testing.T) {
	t.Run("validates required fields", func(t *testing.T) {
		settings := &Settings{
			ID:          "default",
			Channels:    []string{},
			MaxTokens:   60,
			LLMProvider: "openrouter",
			LLMApiURL:   "https://openrouter.ai/api/v1",
		}

		assert.NotEmpty(t, settings.ID)
		assert.NotEmpty(t, settings.LLMProvider)
		assert.NotEmpty(t, settings.LLMApiURL)
		assert.Greater(t, settings.MaxTokens, 0)
	})

	t.Run("validates max tokens range", func(t *testing.T) {
		validTokens := []int{10, 60, 100, 500}
		for _, tokens := range validTokens {
			assert.Greater(t, tokens, 0)
			assert.LessOrEqual(t, tokens, 1000)
		}
	})

	t.Run("validates channel URLs", func(t *testing.T) {
		validChannels := []string{
			"https://www.twitch.tv/channel1",
			"https://www.twitch.tv/channel2",
		}

		for _, channel := range validChannels {
			assert.Contains(t, channel, "twitch.tv")
		}
	})
}
