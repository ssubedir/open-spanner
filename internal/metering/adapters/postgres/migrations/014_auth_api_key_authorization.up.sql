ALTER TABLE auth_api_keys
    ADD COLUMN scopes TEXT NOT NULL DEFAULT '["usage:write","usage:read","meters:read","meters:write"]',
    ADD COLUMN allowed_meters TEXT NOT NULL DEFAULT '[]',
    ADD COLUMN expires_at TEXT,
    ADD COLUMN revoked_at TEXT;
