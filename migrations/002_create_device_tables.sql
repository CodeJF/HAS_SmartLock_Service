CREATE TABLE IF NOT EXISTS devices (
    id BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY,
    uuid VARCHAR(64) NOT NULL UNIQUE,
    device_id VARCHAR(64) NOT NULL UNIQUE,
    uid VARCHAR(64) NOT NULL,
    bind_type INT NOT NULL DEFAULT 1,
    secret VARCHAR(191) NOT NULL,
    name VARCHAR(191) NOT NULL,
    first_bind_time BIGINT NOT NULL,
    bind_time BIGINT NOT NULL,
    created_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
    updated_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
    deleted_at DATETIME(3) NULL,
    INDEX idx_devices_deleted_at (deleted_at)
);

CREATE TABLE IF NOT EXISTS home_devices (
    id BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY,
    home_id BIGINT UNSIGNED NOT NULL,
    device_id BIGINT UNSIGNED NOT NULL,
    created_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
    updated_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
    deleted_at DATETIME(3) NULL,
    INDEX idx_home_devices_home_id (home_id),
    INDEX idx_home_devices_device_id (device_id),
    INDEX idx_home_devices_deleted_at (deleted_at),
    CONSTRAINT fk_home_devices_home_id FOREIGN KEY (home_id) REFERENCES homes(id),
    CONSTRAINT fk_home_devices_device_id FOREIGN KEY (device_id) REFERENCES devices(id)
);
