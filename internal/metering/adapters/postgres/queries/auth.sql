-- name: CountUsers :one
SELECT COUNT(*)
FROM auth_users;

-- name: SaveUser :exec
INSERT INTO auth_users (id, email, password_hash, created_at)
VALUES ($1, $2, $3, $4);

-- name: FindUserByID :one
SELECT id, email, password_hash, created_at
FROM auth_users
WHERE id = $1;

-- name: FindUserByEmail :one
SELECT id, email, password_hash, created_at
FROM auth_users
WHERE email = $1;

-- name: SaveSession :exec
INSERT INTO auth_sessions (id, user_id, token_hash, kind, expires_at, created_at)
VALUES ($1, $2, $3, $4, $5, $6);

-- name: FindSessionByTokenHash :one
SELECT id, user_id, token_hash, kind, expires_at, created_at
FROM auth_sessions
WHERE token_hash = $1 AND kind = $2 AND expires_at > $3;

-- name: DeleteSessionByTokenHash :exec
DELETE FROM auth_sessions
WHERE token_hash = $1;

-- name: SaveAPIKey :exec
INSERT INTO auth_api_keys (id, user_id, name, token_hash, prefix, created_at, last_used_at)
VALUES ($1, $2, $3, $4, $5, $6, $7);

-- name: ListAPIKeys :many
SELECT id, user_id, name, token_hash, prefix, created_at, last_used_at
FROM auth_api_keys
WHERE user_id = $1
ORDER BY created_at DESC, id DESC;

-- name: FindAPIKeyByTokenHash :one
SELECT id, user_id, name, token_hash, prefix, created_at, last_used_at
FROM auth_api_keys
WHERE token_hash = $1;

-- name: UpdateAPIKeyLastUsed :exec
UPDATE auth_api_keys
SET last_used_at = $1
WHERE id = $2;

-- name: DeleteAPIKey :execrows
DELETE FROM auth_api_keys
WHERE id = $1 AND user_id = $2;
