ALTER TABLE home_share_invites
    ADD COLUMN is_read INT NOT NULL DEFAULT 0 AFTER accept;
