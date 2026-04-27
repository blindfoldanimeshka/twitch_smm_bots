-- Миграция для добавления поддержки хешированных паролей
-- ВАЖНО: Существующие пароли в plain text нужно будет перехешировать вручную
-- или через скрипт миграции данных

-- Добавляем комментарий к колонке password
COMMENT ON COLUMN bots.password IS 'Bcrypt hashed password (cost=12)';

-- Создаем функцию для проверки, является ли пароль хешированным
CREATE OR REPLACE FUNCTION is_password_hashed(pwd TEXT) RETURNS BOOLEAN AS $$
BEGIN
    -- Bcrypt хеши начинаются с $2a$, $2b$ или $2y$
    RETURN pwd ~ '^\$2[aby]\$\d{2}\$';
END;
$$ LANGUAGE plpgsql IMMUTABLE;
