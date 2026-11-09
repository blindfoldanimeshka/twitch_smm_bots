-- Создание базы данных (если не существует)
SELECT 'CREATE DATABASE botfarm'
WHERE NOT EXISTS (SELECT FROM pg_database WHERE datname = 'botfarm')\gexec

-- Подключаемся к базе
\c botfarm

-- Создание таблиц
CREATE TABLE IF NOT EXISTS bots (
    id TEXT PRIMARY KEY,
    username TEXT NOT NULL UNIQUE,
    oauth_token TEXT NOT NULL,
    personality TEXT NOT NULL DEFAULT 'neutral',
    status TEXT NOT NULL DEFAULT 'stopped',
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS settings (
    id TEXT PRIMARY KEY DEFAULT 'default',
    channels TEXT[] NOT NULL DEFAULT '{}',
    debug_mode BOOLEAN NOT NULL DEFAULT false,
    debug_channel TEXT NOT NULL DEFAULT '',
    positive_prompt TEXT NOT NULL DEFAULT '',
    negative_prompt TEXT NOT NULL DEFAULT '',
    max_tokens INTEGER NOT NULL DEFAULT 60,
    audio_enabled BOOLEAN NOT NULL DEFAULT false,
    rotation_enabled BOOLEAN NOT NULL DEFAULT false,
    rotation_interval INTEGER NOT NULL DEFAULT 60,
    llm_provider TEXT NOT NULL DEFAULT 'openrouter',
    llm_api_url TEXT NOT NULL DEFAULT 'https://openrouter.ai/api/v1',
    llm_model TEXT NOT NULL DEFAULT 'openrouter/free',
    llm_api_key TEXT NOT NULL DEFAULT '',
    llm_paid_model TEXT NOT NULL DEFAULT 'z-ai/glm-5.3-flash',
    llm_paid_enabled BOOLEAN NOT NULL DEFAULT TRUE,
    llm_paid_daily_cap_usd DOUBLE PRECISION NOT NULL DEFAULT 1.0,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

-- Вставка дефолтных настроек
INSERT INTO settings (id, channels, positive_prompt, negative_prompt, llm_model)
VALUES (
    'default',
    '{}',
    'Пиши короткие естественные сообщения, используй сленг, будь дружелюбным',
    'Не используй эмодзи, не оскорбляй, не спамь, не повторяйся',
    'openrouter/free'
)
ON CONFLICT (id) DO NOTHING;

-- Индексы
CREATE INDEX IF NOT EXISTS idx_bots_status ON bots(status);
CREATE INDEX IF NOT EXISTS idx_bots_username ON bots(username);
