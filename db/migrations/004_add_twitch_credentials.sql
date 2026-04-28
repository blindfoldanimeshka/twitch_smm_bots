-- Add Twitch API credentials to settings table
ALTER TABLE settings
ADD COLUMN IF NOT EXISTS twitch_client_id TEXT NOT NULL DEFAULT '',
ADD COLUMN IF NOT EXISTS twitch_client_secret TEXT NOT NULL DEFAULT '';
