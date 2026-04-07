INSERT INTO users (
    uid,
    username,
    country,
    password_hash,
    nickname,
    avatar,
    is_debug,
    register_time
) VALUES (
    'u_demo_dev',
    'demo@example.com',
    '86',
    'md5-password',
    'Demo User',
    NULL,
    0,
    1770000000
)
ON DUPLICATE KEY UPDATE
    country = VALUES(country),
    password_hash = VALUES(password_hash),
    nickname = VALUES(nickname),
    updated_at = CURRENT_TIMESTAMP(3);
