ALTER TABLE devices
    ADD COLUMN model_code VARCHAR(64) NULL AFTER secret,
    ADD COLUMN current_version VARCHAR(128) NULL AFTER model_code;
