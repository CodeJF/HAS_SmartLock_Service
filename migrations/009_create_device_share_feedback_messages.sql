CREATE TABLE IF NOT EXISTS device_share_feedback_messages (
    id BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY,
    msg_id VARCHAR(64) NOT NULL UNIQUE,
    device_id BIGINT UNSIGNED NOT NULL,
    from_user_id BIGINT UNSIGNED NOT NULL,
    to_user_id BIGINT UNSIGNED NOT NULL,
    status INT NOT NULL,
    is_read INT NOT NULL DEFAULT 0,
    created_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
    updated_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
    deleted_at DATETIME(3) NULL,
    INDEX idx_device_share_feedback_messages_device_id (device_id),
    INDEX idx_device_share_feedback_messages_from_user_id (from_user_id),
    INDEX idx_device_share_feedback_messages_to_user_id (to_user_id),
    INDEX idx_device_share_feedback_messages_deleted_at (deleted_at),
    CONSTRAINT fk_device_share_feedback_messages_device_id FOREIGN KEY (device_id) REFERENCES devices(id),
    CONSTRAINT fk_device_share_feedback_messages_from_user_id FOREIGN KEY (from_user_id) REFERENCES users(id),
    CONSTRAINT fk_device_share_feedback_messages_to_user_id FOREIGN KEY (to_user_id) REFERENCES users(id)
);
