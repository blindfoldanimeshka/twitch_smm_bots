-- Add LLM provider settings to settings table
ALTER TABLE settings
ADD COLUMN IF NOT EXISTS llm_provider TEXT NOT NULL DEFAULT 'openrouter',
ADD COLUMN IF NOT EXISTS llm_api_url TEXT NOT NULL DEFAULT 'https://openrouter.ai/api/v1',
ADD COLUMN IF NOT EXISTS llm_model TEXT NOT NULL DEFAULT 'openrouter/free',
ADD COLUMN IF NOT EXISTS llm_api_key TEXT NOT NULL DEFAULT '',
ADD COLUMN IF NOT EXISTS llm_paid_model TEXT NOT NULL DEFAULT 'z-ai/glm-5.3-flash',
ADD COLUMN IF NOT EXISTS llm_paid_enabled BOOLEAN NOT NULL DEFAULT TRUE,
ADD COLUMN IF NOT EXISTS llm_paid_daily_cap_usd DOUBLE PRECISION NOT NULL DEFAULT 1.0;
