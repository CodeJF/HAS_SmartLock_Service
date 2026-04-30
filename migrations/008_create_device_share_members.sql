CREATE TABLE IF NOT EXISTS device_share_members (
    id BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY,
    device_id BIGINT UNSIGNED NOT NULL,
    user_id BIGINT UNSIGNED NOT NULL,
    role INT NOT NULL DEFAULT 2,
    granted_by_user_id BIGINT UNSIGNED NOT NULL,
    created_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
    updated_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
    deleted_at DATETIME(3) NULL,
    INDEX idx_device_share_members_device_id (device_id),
    INDEX idx_device_share_members_user_id (user_id),
    INDEX idx_device_share_members_granted_by_user_id (granted_by_user_id),
    INDEX idx_device_share_members_deleted_at (deleted_at),
    CONSTRAINT fk_device_share_members_device_id FOREIGN KEY (device_id) REFERENCES devices(id),
    CONSTRAINT fk_device_share_members_user_id FOREIGN KEY (user_id) REFERENCES users(id),
    CONSTRAINT fk_device_share_members_granted_by_user_id FOREIGN KEY (granted_by_user_id) REFERENCES users(id)
);
