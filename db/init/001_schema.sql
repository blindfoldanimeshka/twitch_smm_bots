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
