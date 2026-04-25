-- Миграция для добавления таблиц логов и аудита с foreign key constraints

-- Таблица для логирования сообщений ботов
CREATE TABLE IF NOT EXISTS bot_messages (
    id BIGSERIAL PRIMARY KEY,
    bot_id TEXT NOT NULL,
    channel TEXT NOT NULL,
    message TEXT NOT NULL,
    response TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT fk_bot_messages_bot FOREIGN KEY (bot_id) REFERENCES bots(id) ON DELETE CASCADE
);

-- Таблица аудита изменений
CREATE TABLE IF NOT EXISTS audit_log (
    id BIGSERIAL PRIMARY KEY,
    table_name TEXT NOT NULL,
    record_id TEXT NOT NULL,
    action TEXT NOT NULL CHECK (action IN ('INSERT', 'UPDATE', 'DELETE')),
    old_values JSONB,
    new_values JSONB,
    changed_by TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Индексы для производительности
CREATE INDEX IF NOT EXISTS idx_bot_messages_bot_id ON bot_messages(bot_id);
CREATE INDEX IF NOT EXISTS idx_bot_messages_created_at ON bot_messages(created_at DESC);
CREATE INDEX IF NOT EXISTS idx_bot_messages_channel ON bot_messages(channel);
CREATE INDEX IF NOT EXISTS idx_audit_log_table_record ON audit_log(table_name, record_id);
CREATE INDEX IF NOT EXISTS idx_audit_log_created_at ON audit_log(created_at DESC);

-- Комментарии
COMMENT ON TABLE bot_messages IS 'История сообщений ботов с каскадным удалением при удалении бота';
COMMENT ON TABLE audit_log IS 'Журнал аудита всех изменений в системе';
COMMENT ON CONSTRAINT fk_bot_messages_bot ON bot_messages IS 'Каскадное удаление сообщений при удалении бота';
