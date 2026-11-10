package storage

import (
	"context"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestNewStore tests the New function
func TestNewStore(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	ctx := context.Background()
	pool, err := pgxpool.New(ctx, "postgres://botfarm:botfarm@localhost:5432/botfarm?sslmode=disable")
	if err != nil {
		t.Skipf("postgres not available: %v", err)
	}
	defer pool.Close()

	store := &Store{pool: pool}
	if store == nil {
		t.Error("expected store to be created")
	}
}

// TestEnsureSchema tests schema creation
func TestEnsureSchema(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	ctx := context.Background()
	store, cleanup := setupTestStore(t, ctx)
	defer cleanup()

	err := store.EnsureSchema(ctx)
	if err != nil {
		t.Errorf("EnsureSchema() error = %v", err)
	}
}

// TestCreateAndGetBot tests bot creation and retrieval
func TestCreateAndGetBot(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	ctx := context.Background()
	store, cleanup := setupTestStore(t, ctx)
	defer cleanup()

	// Create bot
	bot := BotConfig{
		ID:              "test-bot-1",
		TwitchLogin:     "testlogin",
		Password:        "oauth:test123",
		Name:            "Test Bot",
		Personality:     "веселый_фанат",
		MinDelaySec:     10,
		MaxDelaySec:     30,
		RateLimitPerMin: 5,
		Enabled:         false,
	}

	err := store.CreateBot(ctx, bot)
	if err != nil {
		t.Fatalf("CreateBot() error = %v", err)
	}

	// Get bot
	got, err := store.GetBot(ctx, "test-bot-1")
	if err != nil {
		t.Fatalf("GetBot() error = %v", err)
	}

	if got.ID != bot.ID {
		t.Errorf("expected ID %s, got %s", bot.ID, got.ID)
	}
	if got.TwitchLogin != bot.TwitchLogin {
		t.Errorf("expected TwitchLogin %s, got %s", bot.TwitchLogin, got.TwitchLogin)
	}
}

// TestUpdateBot tests bot updates with transaction
func TestUpdateBot(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	ctx := context.Background()
	store, cleanup := setupTestStore(t, ctx)
	defer cleanup()

	// Create bot first
	bot := BotConfig{
		ID:              "test-bot-2",
		TwitchLogin:     "testlogin2",
		Password:        "oauth:test456",
		Name:            "Test Bot 2",
		Personality:     "токсик",
		MinDelaySec:     15,
		MaxDelaySec:     45,
		RateLimitPerMin: 3,
		Enabled:         true,
	}

	err := store.CreateBot(ctx, bot)
	if err != nil {
		t.Fatalf("CreateBot() error = %v", err)
	}

	// Update bot
	updates := map[string]interface{}{
		"name":          "Updated Bot Name",
		"personality":   "нейтрал",
		"min_delay_sec": 20,
	}

	err = store.UpdateBot(ctx, "test-bot-2", updates)
	if err != nil {
		t.Errorf("UpdateBot() error = %v", err)
	}

	// Verify update
	got, err := store.GetBot(ctx, "test-bot-2")
	if err != nil {
		t.Fatalf("GetBot() error = %v", err)
	}

	if got.Name != "Updated Bot Name" {
		t.Errorf("expected Name 'Updated Bot Name', got '%s'", got.Name)
	}
	if got.Personality != "нейтрал" {
		t.Errorf("expected Personality 'нейтрал', got '%s'", got.Personality)
	}
	if got.MinDelaySec != 20 {
		t.Errorf("expected MinDelaySec 20, got %d", got.MinDelaySec)
	}
}

// TestUpdateBotInvalidColumn tests that invalid columns are rejected
func TestUpdateBotInvalidColumn(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	ctx := context.Background()
	store, cleanup := setupTestStore(t, ctx)
	defer cleanup()

	// Create bot first
	bot := BotConfig{
		ID:              "test-bot-3",
		TwitchLogin:     "testlogin3",
		Password:        "oauth:test789",
		Name:            "Test Bot 3",
		Personality:     "хайпер",
		MinDelaySec:     45,
		MaxDelaySec:     180,
		RateLimitPerMin: 10,
	}

	err := store.CreateBot(ctx, bot)
	if err != nil {
		t.Fatalf("CreateBot() error = %v", err)
	}

	// Try to update with invalid column
	updates := map[string]interface{}{
		"invalid_column": "should fail",
	}

	err = store.UpdateBot(ctx, "test-bot-3", updates)
	if err == nil {
		t.Error("expected error for invalid column, got nil")
	}
}

// TestDeleteBot tests bot deletion
func TestDeleteBot(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	ctx := context.Background()
	store, cleanup := setupTestStore(t, ctx)
	defer cleanup()

	// Create bot first
	bot := BotConfig{
		ID:              "test-bot-4",
		TwitchLogin:     "testlogin4",
		Password:        "oauth:test000",
		Name:            "Test Bot 4",
		MinDelaySec:     45,
		MaxDelaySec:     180,
		RateLimitPerMin: 10,
	}

	err := store.CreateBot(ctx, bot)
	if err != nil {
		t.Fatalf("CreateBot() error = %v", err)
	}

	// Delete bot
	err = store.DeleteBot(ctx, "test-bot-4")
	if err != nil {
		t.Errorf("DeleteBot() error = %v", err)
	}

	// Verify deletion
	_, err = store.GetBot(ctx, "test-bot-4")
	if err == nil {
		t.Error("expected error after deletion, got nil")
	}
}

// TestSetBotEnabled tests enabling/disabling bots
func TestSetBotEnabled(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	ctx := context.Background()
	store, cleanup := setupTestStore(t, ctx)
	defer cleanup()

	// Create bot first
	bot := BotConfig{
		ID:              "test-bot-5",
		TwitchLogin:     "testlogin5",
		Password:        "oauth:test111",
		Name:            "Test Bot 5",
		Enabled:         false,
		MinDelaySec:     45,
		MaxDelaySec:     180,
		RateLimitPerMin: 10,
	}

	err := store.CreateBot(ctx, bot)
	if err != nil {
		t.Fatalf("CreateBot() error = %v", err)
	}

	// Enable bot
	err = store.SetBotEnabled(ctx, "test-bot-5", true)
	if err != nil {
		t.Errorf("SetBotEnabled(true) error = %v", err)
	}

	// Verify
	got, _ := store.GetBot(ctx, "test-bot-5")
	if !got.Enabled {
		t.Error("expected bot to be enabled")
	}

	// Disable bot
	err = store.SetBotEnabled(ctx, "test-bot-5", false)
	if err != nil {
		t.Errorf("SetBotEnabled(false) error = %v", err)
	}

	// Verify
	got, _ = store.GetBot(ctx, "test-bot-5")
	if got.Enabled {
		t.Error("expected bot to be disabled")
	}
}

// TestUpdateBotChannel tests channel update
func TestUpdateBotChannel(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	ctx := context.Background()
	store, cleanup := setupTestStore(t, ctx)
	defer cleanup()

	// Create bot first
	bot := BotConfig{
		ID:              "test-bot-6",
		TwitchLogin:     "testlogin6",
		Password:        "oauth:test222",
		Name:            "Test Bot 6",
		CurrentChannel:  "channel1",
		MinDelaySec:     45,
		MaxDelaySec:     180,
		RateLimitPerMin: 10,
	}

	err := store.CreateBot(ctx, bot)
	if err != nil {
		t.Fatalf("CreateBot() error = %v", err)
	}

	// Update channel
	err = store.UpdateBotChannel(ctx, "test-bot-6", "new_channel")
	if err != nil {
		t.Errorf("UpdateBotChannel() error = %v", err)
	}

	// Verify
	got, _ := store.GetBot(ctx, "test-bot-6")
	if got.CurrentChannel != "new_channel" {
		t.Errorf("expected CurrentChannel 'new_channel', got '%s'", got.CurrentChannel)
	}
}

// TestListBots tests listing all bots
func TestListBots(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	ctx := context.Background()
	store, cleanup := setupTestStore(t, ctx)
	defer cleanup()

	// Create multiple bots
	bots := []BotConfig{
		{ID: "test-bot-7", TwitchLogin: "login7", Password: "pass7", Name: "Bot 7", MinDelaySec: 45, MaxDelaySec: 180, RateLimitPerMin: 10},
		{ID: "test-bot-8", TwitchLogin: "login8", Password: "pass8", Name: "Bot 8", MinDelaySec: 45, MaxDelaySec: 180, RateLimitPerMin: 10},
	}

	for _, bot := range bots {
		if err := store.CreateBot(ctx, bot); err != nil {
			t.Fatalf("CreateBot() error = %v", err)
		}
	}

	// List bots
	got, err := store.ListBots(ctx)
	if err != nil {
		t.Fatalf("ListBots() error = %v", err)
	}

	if len(got) < 2 {
		t.Errorf("expected at least 2 bots, got %d", len(got))
	}
}

// TestBotTokenRoundTrip проверяет, что импортированная пара токенов
// (access в password + зашифрованный refresh с метаданными) пишется и читается.
func TestBotTokenRoundTrip(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, "postgres://botfarm:botfarm@localhost:5432/botfarm?sslmode=disable")
	if err != nil {
		t.Skipf("postgres not available: %v", err)
	}
	// t.Cleanup — LIFO: pool.Close регистрируем первым, чтобы он отработал последним.
	t.Cleanup(pool.Close)
	if err := pool.Ping(ctx); err != nil {
		t.Skipf("postgres not available: %v", err)
	}
	store := &Store{pool: pool}
	require.NoError(t, store.EnsureSchema(ctx))

	const botID = "test-token-bot"
	t.Cleanup(func() {
		cctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_, _ = pool.Exec(cctx, "DELETE FROM bots WHERE id = $1", botID)
	})

	// Чистим следы прошлых прогонов, чтобы CreateBot не упал на duplicate key.
	_, err = pool.Exec(ctx, "DELETE FROM bots WHERE id = $1", botID)
	require.NoError(t, err)

	// После EnsureSchema новые колонки существуют.
	for _, col := range []string{"token_refresh_enc", "token_expires_at", "token_client_id", "token_scopes"} {
		var exists bool
		require.NoError(t, pool.QueryRow(ctx, `
			SELECT EXISTS (
				SELECT 1 FROM information_schema.columns
				WHERE table_name = 'bots' AND column_name = $1)`, col).Scan(&exists))
		require.True(t, exists, "column %s missing after EnsureSchema", col)
	}

	require.NoError(t, store.CreateBot(ctx, BotConfig{
		ID: botID, TwitchLogin: "tokenlogin", Password: "oauth:old",
		Name: "Token Bot", MinDelaySec: 45, MaxDelaySec: 180, RateLimitPerMin: 10,
	}))

	expiresAt := time.Now().Add(24 * time.Hour).UTC().Truncate(time.Second)
	require.NoError(t, store.SetBotToken(ctx, botID,
		"oauth:newtoken", "gcm:ciphertext:1", expiresAt, "client-id-123",
		[]string{"chat:read", "chat:edit"}))

	got, err := store.GetBot(ctx, botID)
	require.NoError(t, err)
	assert.Equal(t, "oauth:newtoken", got.Password)
	assert.Equal(t, "gcm:ciphertext:1", got.TokenRefreshEnc)
	require.NotNil(t, got.TokenExpiresAt)
	assert.True(t, expiresAt.Equal(*got.TokenExpiresAt), "expected %v, got %v", expiresAt, *got.TokenExpiresAt)
	assert.Equal(t, "client-id-123", got.TokenClientID)
	assert.Equal(t, []string{"chat:read", "chat:edit"}, got.TokenScopes)

	// Повторный вызов перезаписывает пару целиком.
	rotatedAt := time.Now().Add(48 * time.Hour).UTC().Truncate(time.Second)
	require.NoError(t, store.SetBotToken(ctx, botID,
		"oauth:rotated", "gcm:ciphertext:2", rotatedAt, "client-id-999",
		[]string{"user:read", "whispers:read"}))

	got, err = store.GetBot(ctx, botID)
	require.NoError(t, err)
	assert.Equal(t, "oauth:rotated", got.Password)
	assert.Equal(t, "gcm:ciphertext:2", got.TokenRefreshEnc)
	require.NotNil(t, got.TokenExpiresAt)
	assert.True(t, rotatedAt.Equal(*got.TokenExpiresAt), "expected %v, got %v", rotatedAt, *got.TokenExpiresAt)
	assert.Equal(t, "client-id-999", got.TokenClientID)
	assert.Equal(t, []string{"user:read", "whispers:read"}, got.TokenScopes)

	// Неизвестный id даёт ошибку (RowsAffected == 0).
	err = store.SetBotToken(ctx, "test-token-missing", "oauth:x", "gcm:enc", time.Now().UTC(), "cid", []string{"chat:read"})
	require.Error(t, err)
}

// Helper function to set up test store
func setupTestStore(t *testing.T, ctx context.Context) (*Store, func()) {
	t.Helper()

	pool, err := pgxpool.New(ctx, "postgres://botfarm:botfarm@localhost:5432/botfarm?sslmode=disable")
	if err != nil {
		t.Fatalf("failed to connect to postgres: %v", err)
	}

	store := &Store{pool: pool}

	// Ensure schema
	if err := store.EnsureSchema(ctx); err != nil {
		t.Fatalf("EnsureSchema() error = %v", err)
	}

	// Cleanup function
	cleanup := func() {
		// Clean up test data
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_, _ = pool.Exec(ctx, "DELETE FROM bots WHERE id LIKE 'test-bot-%'")
		pool.Close()
	}

	return store, cleanup
}
