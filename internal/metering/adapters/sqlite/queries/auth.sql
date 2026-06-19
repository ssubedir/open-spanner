-- name: CountUsers :one
SELECT COUNT(*)
FROM auth_users;

-- name: SaveUser :exec
INSERT INTO auth_users (id, email, password_hash, created_at)
VALUES (?, ?, ?, ?);

-- name: FindUserByID :one
SELECT id, email, password_hash, created_at
FROM auth_users
WHERE id = ?;

-- name: FindUserByEmail :one
SELECT id, email, password_hash, created_at
FROM auth_users
WHERE email = ?;

-- name: SaveSession :exec
INSERT INTO auth_sessions (id, user_id, token_hash, kind, expires_at, created_at)
VALUES (?, ?, ?, ?, ?, ?);

-- name: FindSessionByTokenHash :one
SELECT id, user_id, token_hash, kind, expires_at, created_at
FROM auth_sessions
WHERE token_hash = ? AND kind = ? AND expires_at > ?;

-- name: DeleteSessionByTokenHash :exec
DELETE FROM auth_sessions
WHERE token_hash = ?;

-- name: SaveAPIKey :exec
INSERT INTO auth_api_keys (id, user_id, name, token_hash, prefix, scopes, allowed_meters, expires_at, revoked_at, created_at, last_used_at)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?);

-- name: ListAPIKeys :many
SELECT id, user_id, name, token_hash, prefix, scopes, allowed_meters, expires_at, revoked_at, created_at, last_used_at
FROM auth_api_keys
WHERE user_id = ?
ORDER BY created_at DESC, id DESC;

-- name: FindAPIKeyByTokenHash :one
SELECT id, user_id, name, token_hash, prefix, scopes, allowed_meters, expires_at, revoked_at, created_at, last_used_at
FROM auth_api_keys
WHERE token_hash = ?;

-- name: UpdateAPIKeyLastUsed :exec
UPDATE auth_api_keys
SET last_used_at = ?
WHERE id = ?;

-- name: DeleteAPIKey :execrows
DELETE FROM auth_api_keys
WHERE id = ? AND user_id = ?;
