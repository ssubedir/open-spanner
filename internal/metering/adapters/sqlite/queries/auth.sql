-- name: CountUsers :one
SELECT COUNT(*)
FROM auth_users;

-- name: SaveUser :exec
INSERT INTO auth_users (id, email, password_hash, created_at)
VALUES (?, ?, ?, ?);

-- name: SaveWorkspace :exec
INSERT INTO auth_workspaces (id, name, created_at)
VALUES (?, ?, ?);

-- name: SaveWorkspaceMembership :exec
INSERT INTO auth_workspace_memberships (workspace_id, user_id, role, created_at)
VALUES (?, ?, ?, ?);

-- name: FindDefaultWorkspaceByUserID :one
SELECT auth_workspaces.id, auth_workspaces.name, auth_workspaces.created_at
FROM auth_workspaces
JOIN auth_workspace_memberships
	ON auth_workspace_memberships.workspace_id = auth_workspaces.id
WHERE auth_workspace_memberships.user_id = ?
ORDER BY
	CASE auth_workspace_memberships.role
		WHEN 'owner' THEN 0
		WHEN 'admin' THEN 1
		ELSE 2
	END,
	auth_workspace_memberships.created_at ASC,
	auth_workspaces.id ASC
LIMIT 1;

-- name: FindUserByID :one
SELECT id, email, password_hash, created_at
FROM auth_users
WHERE id = ?;

-- name: FindUserByEmail :one
SELECT id, email, password_hash, created_at
FROM auth_users
WHERE email = ?;

-- name: SaveIdentity :exec
INSERT INTO auth_identities (id, user_id, provider, subject, email, email_verified, created_at, updated_at)
VALUES (?, ?, ?, ?, ?, ?, ?, ?);

-- name: FindIdentityByProviderSubject :one
SELECT id, user_id, provider, subject, email, email_verified, created_at, updated_at
FROM auth_identities
WHERE provider = ? AND subject = ?;

-- name: SaveSession :exec
INSERT INTO auth_sessions (id, user_id, workspace_id, token_hash, kind, expires_at, created_at)
VALUES (?, ?, ?, ?, ?, ?, ?);

-- name: FindSessionByTokenHash :one
SELECT id, user_id, workspace_id, token_hash, kind, expires_at, created_at
FROM auth_sessions
WHERE token_hash = ? AND kind = ? AND expires_at > ?;

-- name: DeleteSessionByTokenHash :exec
DELETE FROM auth_sessions
WHERE token_hash = ?;

-- name: SaveAPIKey :exec
INSERT INTO auth_api_keys (id, user_id, workspace_id, name, token_hash, prefix, scopes, allowed_meters, expires_at, revoked_at, created_at, last_used_at)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?);

-- name: ListAPIKeys :many
SELECT id, user_id, workspace_id, name, token_hash, prefix, scopes, allowed_meters, expires_at, revoked_at, created_at, last_used_at
FROM auth_api_keys
WHERE user_id = ?
	AND workspace_id = ?
ORDER BY created_at DESC, id DESC;

-- name: FindAPIKeyByTokenHash :one
SELECT id, user_id, workspace_id, name, token_hash, prefix, scopes, allowed_meters, expires_at, revoked_at, created_at, last_used_at
FROM auth_api_keys
WHERE token_hash = ?;

-- name: UpdateAPIKeyLastUsed :exec
UPDATE auth_api_keys
SET last_used_at = ?
WHERE id = ?;

-- name: DeleteAPIKey :execrows
DELETE FROM auth_api_keys
WHERE id = ? AND user_id = ? AND workspace_id = ?;
