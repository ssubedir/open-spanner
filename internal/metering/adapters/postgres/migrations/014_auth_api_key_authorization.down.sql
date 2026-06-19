ALTER TABLE auth_api_keys
    DROP COLUMN revoked_at,
    DROP COLUMN expires_at,
    DROP COLUMN allowed_meters,
    DROP COLUMN scopes;
