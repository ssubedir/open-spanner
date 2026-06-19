ALTER TABLE auth_api_keys DROP COLUMN revoked_at;
ALTER TABLE auth_api_keys DROP COLUMN expires_at;
ALTER TABLE auth_api_keys DROP COLUMN allowed_meters;
ALTER TABLE auth_api_keys DROP COLUMN scopes;
