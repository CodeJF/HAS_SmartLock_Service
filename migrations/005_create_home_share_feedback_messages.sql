CREATE TABLE IF NOT EXISTS home_share_feedback_messages (
    id BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY,
    msg_id VARCHAR(64) NOT NULL UNIQUE,
    home_id BIGINT UNSIGNED NOT NULL,
    from_user_id BIGINT UNSIGNED NOT NULL,
    to_user_id BIGINT UNSIGNED NOT NULL,
    status INT NOT NULL,
    is_read INT NOT NULL DEFAULT 0,
    created_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
    updated_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
    deleted_at DATETIME(3) NULL,
    INDEX idx_home_share_feedback_messages_home_id (home_id),
    INDEX idx_home_share_feedback_messages_from_user_id (from_user_id),
    INDEX idx_home_share_feedback_messages_to_user_id (to_user_id),
    INDEX idx_home_share_feedback_messages_deleted_at (deleted_at),
    CONSTRAINT fk_home_share_feedback_messages_home_id FOREIGN KEY (home_id) REFERENCES homes(id),
    CONSTRAINT fk_home_share_feedback_messages_from_user_id FOREIGN KEY (from_user_id) REFERENCES users(id),
    CONSTRAINT fk_home_share_feedback_messages_to_user_id FOREIGN KEY (to_user_id) REFERENCES users(id)
);
