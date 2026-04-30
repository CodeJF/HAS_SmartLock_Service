CREATE TABLE IF NOT EXISTS device_events (
    id BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY,
    device_id BIGINT UNSIGNED NOT NULL,
    home_id BIGINT UNSIGNED NULL,
    event_type INT NOT NULL,
    event_time BIGINT NOT NULL,
    device_time BIGINT NOT NULL,
    thumbnail VARCHAR(1024) NULL,
    payload JSON NULL,
    created_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
    updated_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
    deleted_at DATETIME(3) NULL,
    INDEX idx_device_events_device_id (device_id),
    INDEX idx_device_events_home_id (home_id),
    INDEX idx_device_events_event_time (event_time),
    INDEX idx_device_events_deleted_at (deleted_at),
    CONSTRAINT fk_device_events_device_id FOREIGN KEY (device_id) REFERENCES devices(id),
    CONSTRAINT fk_device_events_home_id FOREIGN KEY (home_id) REFERENCES homes(id)
);

CREATE TABLE IF NOT EXISTS device_event_user_states (
    id BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY,
    event_id BIGINT UNSIGNED NOT NULL,
    user_id BIGINT UNSIGNED NOT NULL,
    is_read INT NOT NULL DEFAULT 0,
    created_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
    updated_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
    deleted_at DATETIME(3) NULL,
    UNIQUE KEY uk_device_event_user_states_event_user (event_id, user_id),
    INDEX idx_device_event_user_states_user_id (user_id),
    INDEX idx_device_event_user_states_deleted_at (deleted_at),
    CONSTRAINT fk_device_event_user_states_event_id FOREIGN KEY (event_id) REFERENCES device_events(id),
    CONSTRAINT fk_device_event_user_states_user_id FOREIGN KEY (user_id) REFERENCES users(id)
);
