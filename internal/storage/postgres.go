package storage

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

const (
	dbOperationTimeout = 5 * time.Second

	schemaSQL = `
CREATE TABLE IF NOT EXISTS bots (
    id TEXT PRIMARY KEY,
    twitch_login TEXT NOT NULL,
    password TEXT NOT NULL,
    name TEXT NOT NULL,
    personality TEXT NOT NULL,
    min_delay_sec INT NOT NULL DEFAULT 45,
    max_delay_sec INT NOT NULL DEFAULT 180,
    rate_limit_per_min INT NOT NULL DEFAULT 10,
    enabled BOOLEAN NOT NULL DEFAULT FALSE,
    current_channel TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT bots_min_max_delay_check CHECK (min_delay_sec > 0 AND max_delay_sec >= min_delay_sec),
    CONSTRAINT bots_rate_limit_check CHECK (rate_limit_per_min > 0)
);

-- Миграция: переименовываем oauth_token в password если колонка существует
DO $$
BEGIN
    IF EXISTS (
        SELECT 1 FROM information_schema.columns
        WHERE table_name = 'bots' AND column_name = 'oauth_token'
    ) THEN
        ALTER TABLE bots RENAME COLUMN oauth_token TO password;
    END IF;
END $$;

-- Миграция: добавляем current_channel если не существует
DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM information_schema.columns
        WHERE table_name = 'bots' AND column_name = 'current_channel'
    ) THEN
        ALTER TABLE bots ADD COLUMN current_channel TEXT NOT NULL DEFAULT '';
    END IF;
END $$;

CREATE OR REPLACE FUNCTION set_updated_at() RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = NOW();
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

DROP TRIGGER IF EXISTS bots_set_updated_at ON bots;
CREATE TRIGGER bots_set_updated_at
    BEFORE UPDATE ON bots
    FOR EACH ROW
    EXECUTE FUNCTION set_updated_at();

CREATE TABLE IF NOT EXISTS bot_logs (
    id BIGSERIAL PRIMARY KEY,
    bot_id TEXT NOT NULL,
    log_type TEXT NOT NULL,
    level TEXT NOT NULL,
    message TEXT NOT NULL,
    context JSONB,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_bot_logs_bot_id_created ON bot_logs(bot_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_bot_logs_created ON bot_logs(created_at DESC);

CREATE TABLE IF NOT EXISTS settings_logs (
    id BIGSERIAL PRIMARY KEY,
    level TEXT NOT NULL,
    message TEXT NOT NULL,
    changes JSONB,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_settings_logs_created ON settings_logs(created_at DESC);

CREATE TABLE IF NOT EXISTS proxies (
    id TEXT PRIMARY KEY,
    host TEXT NOT NULL,
    port INT NOT NULL,
    username TEXT,
    password TEXT,
    protocol TEXT NOT NULL DEFAULT 'http',
    enabled BOOLEAN NOT NULL DEFAULT TRUE,
    last_used_at TIMESTAMPTZ,
    fail_count INT NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT proxies_port_check CHECK (port > 0 AND port <= 65535),
    CONSTRAINT proxies_protocol_check CHECK (protocol IN ('http', 'https', 'socks5'))
);

ALTER TABLE bots ADD COLUMN IF NOT EXISTS proxy_id TEXT REFERENCES proxies(id) ON DELETE SET NULL;
ALTER TABLE bots ADD COLUMN IF NOT EXISTS auto_rotate_proxy BOOLEAN NOT NULL DEFAULT TRUE;

-- Миграция: метаданные импортированного Twitch-токена (refresh хранится только в зашифрованном виде)
ALTER TABLE bots ADD COLUMN IF NOT EXISTS token_refresh_enc TEXT NOT NULL DEFAULT '';
ALTER TABLE bots ADD COLUMN IF NOT EXISTS token_expires_at TIMESTAMPTZ;
ALTER TABLE bots ADD COLUMN IF NOT EXISTS token_client_id TEXT NOT NULL DEFAULT '';
ALTER TABLE bots ADD COLUMN IF NOT EXISTS token_scopes JSONB NOT NULL DEFAULT '[]';

CREATE INDEX IF NOT EXISTS idx_proxies_enabled ON proxies(enabled);
CREATE INDEX IF NOT EXISTS idx_bots_proxy_id ON bots(proxy_id);
`
)

// BotConfig is a runtime-ready bot configuration loaded from PostgreSQL.
type BotConfig struct {
	ID              string
	TwitchLogin     string
	Password        string
	Name            string
	Personality     string
	MinDelaySec     int
	MaxDelaySec     int
	RateLimitPerMin int
	Enabled         bool
	CurrentChannel  string

	// Imported Twitch token metadata. Password keeps the access token as "oauth:<token>";
	// TokenRefreshEnc holds only the ciphertext produced by internal/tokencrypt.
	TokenRefreshEnc string
	TokenExpiresAt  *time.Time
	TokenClientID   string
	TokenScopes     []string
}

type Store struct {
	pool *pgxpool.Pool
}

func New(ctx context.Context, databaseURL string) (*Store, error) {
	if strings.TrimSpace(databaseURL) == "" {
		return nil, errors.New("database url is empty")
	}

	pool, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		return nil, fmt.Errorf("create pg pool: %w", err)
	}

	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("ping postgres: %w", err)
	}

	return &Store{pool: pool}, nil
}

func (s *Store) Close() {
	if s.pool != nil {
		s.pool.Close()
	}
}

func (s *Store) EnsureSchema(ctx context.Context) error {
	_, err := s.pool.Exec(ctx, schemaSQL)
	if err != nil {
		return fmt.Errorf("ensure schema: %w", err)
	}
	return nil
}

func (s *Store) ListBots(ctx context.Context) ([]BotConfig, error) {
	ctx, cancel := context.WithTimeout(ctx, dbOperationTimeout)
	defer cancel()

	rows, err := s.pool.Query(ctx, `
		SELECT id, twitch_login, password, name, personality, min_delay_sec, max_delay_sec, rate_limit_per_min, enabled, current_channel,
		       token_refresh_enc, token_expires_at, token_client_id, token_scopes
		FROM bots
		ORDER BY id`)
	if err != nil {
		return nil, fmt.Errorf("query bots: %w", err)
	}
	defer rows.Close()

	result := make([]BotConfig, 0)
	for rows.Next() {
		var b BotConfig
		if err := rows.Scan(
			&b.ID,
			&b.TwitchLogin,
			&b.Password,
			&b.Name,
			&b.Personality,
			&b.MinDelaySec,
			&b.MaxDelaySec,
			&b.RateLimitPerMin,
			&b.Enabled,
			&b.CurrentChannel,
			&b.TokenRefreshEnc,
			&b.TokenExpiresAt,
			&b.TokenClientID,
			&b.TokenScopes,
		); err != nil {
			return nil, fmt.Errorf("scan bot: %w", err)
		}
		result = append(result, b)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate bots: %w", err)
	}

	return result, nil
}

func (s *Store) SetBotEnabled(ctx context.Context, botID string, enabled bool) error {
	ctx, cancel := context.WithTimeout(ctx, dbOperationTimeout)
	defer cancel()

	cmd, err := s.pool.Exec(ctx, `UPDATE bots SET enabled = $2 WHERE id = $1`, botID, enabled)
	if err != nil {
		return fmt.Errorf("update bot enabled: %w", err)
	}
	if cmd.RowsAffected() == 0 {
		return fmt.Errorf("bot not found: %s", botID)
	}
	return nil
}

func (s *Store) GetBot(ctx context.Context, botID string) (*BotConfig, error) {
	ctx, cancel := context.WithTimeout(ctx, dbOperationTimeout)
	defer cancel()

	var b BotConfig
	err := s.pool.QueryRow(ctx, `
		SELECT id, twitch_login, password, name, personality, min_delay_sec, max_delay_sec, rate_limit_per_min, enabled, current_channel,
		       token_refresh_enc, token_expires_at, token_client_id, token_scopes
		FROM bots WHERE id = $1`, botID).Scan(
		&b.ID, &b.TwitchLogin, &b.Password, &b.Name, &b.Personality,
		&b.MinDelaySec, &b.MaxDelaySec, &b.RateLimitPerMin, &b.Enabled, &b.CurrentChannel,
		&b.TokenRefreshEnc, &b.TokenExpiresAt, &b.TokenClientID, &b.TokenScopes,
	)
	if err != nil {
		return nil, fmt.Errorf("get bot: %w", err)
	}
	return &b, nil
}

func (s *Store) CreateBot(ctx context.Context, cfg BotConfig) error {
	ctx, cancel := context.WithTimeout(ctx, dbOperationTimeout)
	defer cancel()

	// JSONB NOT NULL: пустой срез вместо nil, чтобы не писать json-«null»
	scopes := cfg.TokenScopes
	if scopes == nil {
		scopes = []string{}
	}

	_, err := s.pool.Exec(ctx, `
		INSERT INTO bots (id, twitch_login, password, name, personality, min_delay_sec, max_delay_sec, rate_limit_per_min, enabled, current_channel,
		                  token_refresh_enc, token_expires_at, token_client_id, token_scopes)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14)`,
		cfg.ID, cfg.TwitchLogin, cfg.Password, cfg.Name, cfg.Personality,
		cfg.MinDelaySec, cfg.MaxDelaySec, cfg.RateLimitPerMin, cfg.Enabled, cfg.CurrentChannel,
		cfg.TokenRefreshEnc, cfg.TokenExpiresAt, cfg.TokenClientID, scopes,
	)
	if err != nil {
		return fmt.Errorf("create bot: %w", err)
	}
	return nil
}

func (s *Store) UpdateBot(ctx context.Context, botID string, updates map[string]interface{}) error {
	if len(updates) == 0 {
		return nil
	}

	// Whitelist разрешенных колонок для защиты от SQL injection
	allowedColumns := map[string]bool{
		"name":              true,
		"personality":       true,
		"min_delay_sec":     true,
		"max_delay_sec":     true,
		"rate_limit_per_min": true,
		"enabled":           true,
		"current_channel":   true,
		"proxy_id":          true,
		"auto_rotate_proxy": true,
	}

	ctx, cancel := context.WithTimeout(ctx, dbOperationTimeout)
	defer cancel()

	// P0-4: Use transaction for ACID guarantees
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	query := "UPDATE bots SET "
	args := make([]interface{}, 0, len(updates)+1)
	i := 1

	for key, val := range updates {
		// Проверяем, что колонка в whitelist
		if !allowedColumns[key] {
			tx.Rollback(ctx)
			return fmt.Errorf("invalid column name: %s", key)
		}

		if i > 1 {
			query += ", "
		}
		query += fmt.Sprintf("%s = $%d", key, i)
		args = append(args, val)
		i++
	}

	query += fmt.Sprintf(" WHERE id = $%d", i)
	args = append(args, botID)

	cmd, err := tx.Exec(ctx, query, args...)
	if err != nil {
		tx.Rollback(ctx)
		return fmt.Errorf("update bot: %w", err)
	}
	if cmd.RowsAffected() == 0 {
		tx.Rollback(ctx)
		return fmt.Errorf("bot not found: %s", botID)
	}

	// P0-4: Commit the transaction
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit transaction: %w", err)
	}

	return nil
}

func (s *Store) DeleteBot(ctx context.Context, botID string) error {
	ctx, cancel := context.WithTimeout(ctx, dbOperationTimeout)
	defer cancel()

	cmd, err := s.pool.Exec(ctx, `DELETE FROM bots WHERE id = $1`, botID)
	if err != nil {
		return fmt.Errorf("delete bot: %w", err)
	}
	if cmd.RowsAffected() == 0 {
		return fmt.Errorf("bot not found: %s", botID)
	}
	return nil
}

func (s *Store) UpdateBotChannel(ctx context.Context, botID string, channel string) error {
	ctx, cancel := context.WithTimeout(ctx, dbOperationTimeout)
	defer cancel()

	cmd, err := s.pool.Exec(ctx, `UPDATE bots SET current_channel = $2 WHERE id = $1`, botID, channel)
	if err != nil {
		return fmt.Errorf("update bot channel: %w", err)
	}
	if cmd.RowsAffected() == 0 {
		return fmt.Errorf("bot not found: %s", botID)
	}
	return nil
}

// SetBotToken writes an imported token pair for the bot. accessToken must already
// be in the "oauth:<token>" form (semantics of bots.password do not change) and
// refreshTokenEnc must be ciphertext — never pass a plaintext refresh token here.
func (s *Store) SetBotToken(ctx context.Context, id, accessToken, refreshTokenEnc string, expiresAt time.Time, clientID string, scopes []string) error {
	ctx, cancel := context.WithTimeout(ctx, dbOperationTimeout)
	defer cancel()

	// JSONB NOT NULL: пустой срез вместо nil, чтобы не писать json-«null»
	if scopes == nil {
		scopes = []string{}
	}

	cmd, err := s.pool.Exec(ctx, `
		UPDATE bots
		SET password = $2, token_refresh_enc = $3, token_expires_at = $4, token_client_id = $5, token_scopes = $6, updated_at = NOW()
		WHERE id = $1`,
		id, accessToken, refreshTokenEnc, expiresAt, clientID, scopes,
	)
	if err != nil {
		return fmt.Errorf("set bot token: %w", err)
	}
	if cmd.RowsAffected() == 0 {
		return fmt.Errorf("bot not found: %s", id)
	}
	return nil
}

type Proxy struct {
	ID         string
	Host       string
	Port       int
	Username   string
	Password   string
	Protocol   string
	Enabled    bool
	LastUsedAt *time.Time
	FailCount  int
	CreatedAt  time.Time
}

func (s *Store) ListProxies(ctx context.Context) ([]Proxy, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT id, host, port, COALESCE(username, ''), COALESCE(password, ''), protocol, enabled, last_used_at, fail_count, created_at
		FROM proxies
		ORDER BY created_at DESC`)
	if err != nil {
		return nil, fmt.Errorf("query proxies: %w", err)
	}
	defer rows.Close()

	result := make([]Proxy, 0)
	for rows.Next() {
		var p Proxy
		if err := rows.Scan(&p.ID, &p.Host, &p.Port, &p.Username, &p.Password, &p.Protocol, &p.Enabled, &p.LastUsedAt, &p.FailCount, &p.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan proxy: %w", err)
		}
		result = append(result, p)
	}

	return result, rows.Err()
}

func (s *Store) GetProxy(ctx context.Context, id string) (*Proxy, error) {
	var p Proxy
	err := s.pool.QueryRow(ctx, `
		SELECT id, host, port, COALESCE(username, ''), COALESCE(password, ''), protocol, enabled, last_used_at, fail_count, created_at
		FROM proxies WHERE id = $1`, id).Scan(
		&p.ID, &p.Host, &p.Port, &p.Username, &p.Password, &p.Protocol, &p.Enabled, &p.LastUsedAt, &p.FailCount, &p.CreatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("get proxy: %w", err)
	}
	return &p, nil
}

func (s *Store) CreateProxy(ctx context.Context, proxy Proxy) error {
	_, err := s.pool.Exec(ctx, `
		INSERT INTO proxies (id, host, port, username, password, protocol, enabled, fail_count)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)`,
		proxy.ID, proxy.Host, proxy.Port, proxy.Username, proxy.Password, proxy.Protocol, proxy.Enabled, proxy.FailCount,
	)
	if err != nil {
		return fmt.Errorf("create proxy: %w", err)
	}
	return nil
}

func (s *Store) UpdateProxyFailCount(ctx context.Context, id string, failCount int) error {
	_, err := s.pool.Exec(ctx, `UPDATE proxies SET fail_count = $2 WHERE id = $1`, id, failCount)
	return err
}

func (s *Store) UpdateProxyLastUsed(ctx context.Context, id string, lastUsed time.Time) error {
	_, err := s.pool.Exec(ctx, `UPDATE proxies SET last_used_at = $2 WHERE id = $1`, id, lastUsed)
	return err
}

func (s *Store) SetProxyEnabled(ctx context.Context, id string, enabled bool) error {
	_, err := s.pool.Exec(ctx, `UPDATE proxies SET enabled = $2 WHERE id = $1`, id, enabled)
	return err
}

func (s *Store) DeleteProxy(ctx context.Context, id string) error {
	cmd, err := s.pool.Exec(ctx, `DELETE FROM proxies WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("delete proxy: %w", err)
	}
	if cmd.RowsAffected() == 0 {
		return fmt.Errorf("proxy not found: %s", id)
	}
	return nil
}
