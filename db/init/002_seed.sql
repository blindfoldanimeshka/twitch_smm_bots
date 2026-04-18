INSERT INTO bots (id, twitch_login, oauth_token, name, personality, min_delay_sec, max_delay_sec, rate_limit_per_min, enabled)
VALUES
    ('bot1', 'your_bot_login_1', 'oauth:replace_me_1', 'Alex_Gamer', 'веселый_фанат', 45, 180, 10, FALSE),
    ('bot2', 'your_bot_login_2', 'oauth:replace_me_2', 'Troll_99', 'токсик', 60, 200, 8, FALSE),
    ('bot3', 'your_bot_login_3', 'oauth:replace_me_3', 'Viewer_X', 'нейтрал', 40, 150, 12, FALSE)
ON CONFLICT (id) DO NOTHING;
