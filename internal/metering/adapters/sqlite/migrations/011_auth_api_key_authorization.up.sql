ALTER TABLE auth_api_keys ADD COLUMN scopes TEXT NOT NULL DEFAULT '["usage:write","usage:read","meters:read","meters:write"]';
ALTER TABLE auth_api_keys ADD COLUMN allowed_meters TEXT NOT NULL DEFAULT '[]';
ALTER TABLE auth_api_keys ADD COLUMN expires_at TEXT;
ALTER TABLE auth_api_keys ADD COLUMN revoked_at TEXT;
