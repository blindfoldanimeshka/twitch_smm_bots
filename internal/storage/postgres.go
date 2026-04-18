package storage

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"
)

const schemaSQL = `
CREATE TABLE IF NOT EXISTS bots (
    id TEXT PRIMARY KEY,
    twitch_login TEXT NOT NULL,
    oauth_token TEXT NOT NULL,
    name TEXT NOT NULL,
    personality TEXT NOT NULL,
    min_delay_sec INT NOT NULL DEFAULT 45,
    max_delay_sec INT NOT NULL DEFAULT 180,
    rate_limit_per_min INT NOT NULL DEFAULT 10,
    enabled BOOLEAN NOT NULL DEFAULT FALSE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT bots_min_max_delay_check CHECK (min_delay_sec > 0 AND max_delay_sec >= min_delay_sec),
    CONSTRAINT bots_rate_limit_check CHECK (rate_limit_per_min > 0)
);

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

// BotConfig is a runtime-ready bot configuration loaded from PostgreSQL.
type BotConfig struct {
	ID              string
	TwitchLogin     string
	OAuthToken      string
	Name            string
	Personality     string
	MinDelaySec     int
	MaxDelaySec     int
	RateLimitPerMin int
	Enabled         bool
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
	rows, err := s.pool.Query(ctx, `
		SELECT id, twitch_login, oauth_token, name, personality, min_delay_sec, max_delay_sec, rate_limit_per_min, enabled
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
			&b.OAuthToken,
			&b.Name,
			&b.Personality,
			&b.MinDelaySec,
			&b.MaxDelaySec,
			&b.RateLimitPerMin,
			&b.Enabled,
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
	cmd, err := s.pool.Exec(ctx, `UPDATE bots SET enabled = $2 WHERE id = $1`, botID, enabled)
	if err != nil {
		return fmt.Errorf("update bot enabled: %w", err)
	}
	if cmd.RowsAffected() == 0 {
		return fmt.Errorf("bot not found: %s", botID)
	}
	return nil
}
