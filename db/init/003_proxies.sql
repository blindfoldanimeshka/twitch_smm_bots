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

CREATE INDEX IF NOT EXISTS idx_proxies_enabled ON proxies(enabled);
CREATE INDEX IF NOT EXISTS idx_bots_proxy_id ON bots(proxy_id);
