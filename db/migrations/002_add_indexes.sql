-- Миграция для добавления индексов на часто используемые поля

-- Индексы для таблицы bots
CREATE INDEX IF NOT EXISTS idx_bots_enabled ON bots(enabled);
CREATE INDEX IF NOT EXISTS idx_bots_twitch_login ON bots(twitch_login);
CREATE INDEX IF NOT EXISTS idx_bots_current_channel ON bots(current_channel) WHERE current_channel != '';
CREATE INDEX IF NOT EXISTS idx_bots_created_at ON bots(created_at DESC);
CREATE INDEX IF NOT EXISTS idx_bots_updated_at ON bots(updated_at DESC);

-- Индексы для таблицы proxies (если существует)
CREATE INDEX IF NOT EXISTS idx_proxies_enabled ON proxies(enabled);
CREATE INDEX IF NOT EXISTS idx_proxies_fail_count ON proxies(fail_count);
CREATE INDEX IF NOT EXISTS idx_proxies_last_used_at ON proxies(last_used_at DESC NULLS LAST);
CREATE INDEX IF NOT EXISTS idx_proxies_created_at ON proxies(created_at DESC);

-- Составной индекс для выборки активных прокси с низким fail_count
CREATE INDEX IF NOT EXISTS idx_proxies_enabled_fail_count ON proxies(enabled, fail_count) WHERE enabled = true;

-- Комментарии к индексам
COMMENT ON INDEX idx_bots_enabled IS 'Быстрая фильтрация активных/неактивных ботов';
COMMENT ON INDEX idx_bots_twitch_login IS 'Поиск бота по Twitch логину';
COMMENT ON INDEX idx_bots_current_channel IS 'Фильтрация ботов по текущему каналу';
COMMENT ON INDEX idx_proxies_enabled_fail_count IS 'Выборка работающих прокси с минимальным количеством ошибок';
