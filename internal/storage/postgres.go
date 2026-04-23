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
		SELECT id, twitch_login, password, name, personality, min_delay_sec, max_delay_sec, rate_limit_per_min, enabled, current_channel
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
		SELECT id, twitch_login, password, name, personality, min_delay_sec, max_delay_sec, rate_limit_per_min, enabled, current_channel
		FROM bots WHERE id = $1`, botID).Scan(
		&b.ID, &b.TwitchLogin, &b.Password, &b.Name, &b.Personality,
		&b.MinDelaySec, &b.MaxDelaySec, &b.RateLimitPerMin, &b.Enabled, &b.CurrentChannel,
	)
	if err != nil {
		return nil, fmt.Errorf("get bot: %w", err)
	}
	return &b, nil
}

func (s *Store) CreateBot(ctx context.Context, cfg BotConfig) error {
	ctx, cancel := context.WithTimeout(ctx, dbOperationTimeout)
	defer cancel()

	_, err := s.pool.Exec(ctx, `
		INSERT INTO bots (id, twitch_login, password, name, personality, min_delay_sec, max_delay_sec, rate_limit_per_min, enabled, current_channel)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)`,
		cfg.ID, cfg.TwitchLogin, cfg.Password, cfg.Name, cfg.Personality,
		cfg.MinDelaySec, cfg.MaxDelaySec, cfg.RateLimitPerMin, cfg.Enabled, cfg.CurrentChannel,
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

	ctx, cancel := context.WithTimeout(ctx, dbOperationTimeout)
	defer cancel()

	query := "UPDATE bots SET "
	args := make([]interface{}, 0, len(updates)+1)
	i := 1

	for key, val := range updates {
		if i > 1 {
			query += ", "
		}
		query += fmt.Sprintf("%s = $%d", key, i)
		args = append(args, val)
		i++
	}

	query += fmt.Sprintf(" WHERE id = $%d", i)
	args = append(args, botID)

	cmd, err := s.pool.Exec(ctx, query, args...)
	if err != nil {
		return fmt.Errorf("update bot: %w", err)
	}
	if cmd.RowsAffected() == 0 {
		return fmt.Errorf("bot not found: %s", botID)
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
