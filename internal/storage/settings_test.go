package storage

import (
	"context"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const settingsTestDSN = "postgres://botfarm:botfarm@localhost:5432/botfarm?sslmode=disable"

// newSettingsStore connects to the real Postgres, mirroring settings_migration_test.go.
// sqlmock cannot intercept pgxpool, so the settings store is only exercised against a live database.
func newSettingsStore(t *testing.T) *Store {
	t.Helper()
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	ctx := context.Background()
	pool, err := pgxpool.New(ctx, settingsTestDSN)
	if err != nil {
		t.Skipf("postgres not available: %v", err)
	}
	t.Cleanup(pool.Close)
	if err := pool.Ping(ctx); err != nil {
		t.Skipf("postgres not available: %v", err)
	}

	return &Store{pool: pool}
}

func TestSettingsRoundTrip(t *testing.T) {
	store := newSettingsStore(t)
	ctx := context.Background()
	require.NoError(t, store.EnsureSettingsTable(ctx))

	original, err := store.GetSettings(ctx)
	require.NoError(t, err)
	require.Equal(t, "default", original.ID)

	// The settings row is global state: put it back exactly as we found it.
	t.Cleanup(func() {
		require.NoError(t, store.UpdateSettings(ctx, original))
	})

	updated := *original
	updated.PresenceEnabled = !original.PresenceEnabled
	updated.PresenceSessionMinMinutes = original.PresenceSessionMinMinutes + 7
	updated.PresenceSessionMaxMinutes = original.PresenceSessionMaxMinutes + 13
	updated.TwitchClientID = "roundtrip-client-id"
	updated.TwitchClientSecret = "roundtrip-client-secret"
	updated.LLMApiKey = "roundtrip-llm-api-key"
	// Computed flags, never persisted: they must not change what GetSettings returns.
	updated.TwitchClientSecretSet = true

	require.NoError(t, store.UpdateSettings(ctx, &updated))

	got, err := store.GetSettings(ctx)
	require.NoError(t, err)

	assert.Equal(t, updated.PresenceEnabled, got.PresenceEnabled)
	assert.NotEqual(t, original.PresenceEnabled, got.PresenceEnabled)
	assert.Equal(t, updated.PresenceSessionMinMinutes, got.PresenceSessionMinMinutes)
	assert.Equal(t, updated.PresenceSessionMaxMinutes, got.PresenceSessionMaxMinutes)
	assert.Equal(t, "roundtrip-client-id", got.TwitchClientID)
	assert.Equal(t, "roundtrip-client-secret", got.TwitchClientSecret)
	assert.Equal(t, "roundtrip-llm-api-key", got.LLMApiKey)
	assert.False(t, got.TwitchClientSecretSet, "TwitchClientSecretSet is computed in the API, it must not be stored")

	// Untouched columns must survive the write unchanged: GetSettings really reads the row back.
	assert.Equal(t, original.ID, got.ID)
	assert.Equal(t, original.Channels, got.Channels)
	assert.Equal(t, original.DebugMode, got.DebugMode)
	assert.Equal(t, original.DebugChannel, got.DebugChannel)
	assert.Equal(t, original.PositivePrompt, got.PositivePrompt)
	assert.Equal(t, original.NegativePrompt, got.NegativePrompt)
	assert.Equal(t, original.MaxTokens, got.MaxTokens)
	assert.Equal(t, original.AudioEnabled, got.AudioEnabled)
	assert.Equal(t, original.RotationEnabled, got.RotationEnabled)
	assert.Equal(t, original.RotationInterval, got.RotationInterval)
	assert.Equal(t, original.PresenceCooldownMinMinutes, got.PresenceCooldownMinMinutes)
	assert.Equal(t, original.PresenceCooldownMaxMinutes, got.PresenceCooldownMaxMinutes)
	assert.Equal(t, original.PresenceGreetingEnabled, got.PresenceGreetingEnabled)
	assert.Equal(t, original.PresenceFarewellEnabled, got.PresenceFarewellEnabled)
	assert.Equal(t, original.LLMProvider, got.LLMProvider)
	assert.Equal(t, original.LLMApiURL, got.LLMApiURL)
	assert.Equal(t, original.LLMModel, got.LLMModel)
	assert.Equal(t, original.LLMPaidModel, got.LLMPaidModel)
	assert.Equal(t, original.LLMPaidEnabled, got.LLMPaidEnabled)
	assert.Equal(t, original.LLMPaidDailyCapUSD, got.LLMPaidDailyCapUSD)

	assert.False(t, got.UpdatedAt.Before(updated.UpdatedAt), "UpdateSettings should stamp updated_at with the DB clock")
}

func TestUpdateSettingsMissingRowDoesNotPanic(t *testing.T) {
	store := newSettingsStore(t)
	ctx := context.Background()
	require.NoError(t, store.EnsureSettingsTable(ctx))

	original, err := store.GetSettings(ctx)
	require.NoError(t, err)

	// Temporarily move the only row away so UpdateSettings hits id='default' with 0 rows affected.
	const movedID = "settings-default-moved"
	_, err = store.pool.Exec(ctx, `DELETE FROM settings WHERE id = $1`, movedID)
	require.NoError(t, err)
	_, err = store.pool.Exec(ctx, `UPDATE settings SET id = $1 WHERE id = 'default'`, movedID)
	require.NoError(t, err)

	t.Cleanup(func() {
		_, err := store.pool.Exec(context.Background(), `UPDATE settings SET id = 'default' WHERE id = $1`, movedID)
		require.NoError(t, err)
	})

	var updateErr error
	require.NotPanics(t, func() { updateErr = store.UpdateSettings(ctx, original) })
	// RowsAffected == 0 is silently accepted today: the update is a no-op, not an error. Pin that behaviour.
	require.NoError(t, updateErr)

	// No row was inserted either: id='default' is still missing.
	_, err = store.GetSettings(ctx)
	require.Error(t, err)
}
