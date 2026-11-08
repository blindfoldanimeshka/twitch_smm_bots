package storage

import (
	"context"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestEnsureSettingsTableMigratesLMStudioSettings(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, "postgres://botfarm:botfarm@localhost:5432/botfarm?sslmode=disable")
	if err != nil {
		t.Skipf("postgres not available: %v", err)
	}
	defer pool.Close()
	if err := pool.Ping(ctx); err != nil {
		t.Skipf("postgres not available: %v", err)
	}
	store := &Store{pool: pool}
	require.NoError(t, store.EnsureSettingsTable(ctx))

	_, err = pool.Exec(ctx, `UPDATE settings SET llm_provider='lmstudio',
		llm_api_url='http://127.0.0.1:1234/v1', llm_model='', llm_api_key='' WHERE id='default'`)
	require.NoError(t, err)
	require.NoError(t, store.EnsureSettingsTable(ctx))

	got, err := store.GetSettings(ctx)
	require.NoError(t, err)
	assert.Equal(t, "openrouter", got.LLMProvider)
	assert.Equal(t, "https://openrouter.ai/api/v1", got.LLMApiURL)
	assert.Equal(t, "openrouter/free", got.LLMModel)
	assert.Equal(t, "z-ai/glm-5.3-flash", got.LLMPaidModel)
	assert.True(t, got.LLMPaidEnabled)
	assert.Equal(t, 1.0, got.LLMPaidDailyCapUSD)
}
