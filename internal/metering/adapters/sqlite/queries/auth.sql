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

-- name: ListWorkspaceAccessByUserID :many
SELECT w.id AS workspace_id, w.name AS workspace_name, w.created_at AS workspace_created_at,
	m.user_id, m.role, m.created_at AS membership_created_at
FROM auth_workspace_memberships m
JOIN auth_workspaces w ON w.id = m.workspace_id
WHERE m.user_id = ?
ORDER BY w.name, w.id;

-- name: FindWorkspaceAccess :one
SELECT w.id AS workspace_id, w.name AS workspace_name, w.created_at AS workspace_created_at,
	m.user_id, m.role, m.created_at AS membership_created_at
FROM auth_workspace_memberships m
JOIN auth_workspaces w ON w.id = m.workspace_id
WHERE m.workspace_id = ? AND m.user_id = ?;

-- name: ListWorkspaceMembers :many
SELECT m.workspace_id, m.user_id, u.email, m.role, m.created_at
FROM auth_workspace_memberships m
JOIN auth_users u ON u.id = m.user_id
WHERE m.workspace_id = ?
ORDER BY CASE m.role WHEN 'owner' THEN 0 WHEN 'admin' THEN 1 ELSE 2 END, u.email, m.user_id;

-- name: CountWorkspaceOwners :one
SELECT COUNT(*) FROM auth_workspace_memberships
WHERE workspace_id = ? AND role = 'owner';

-- name: UpdateWorkspaceMembershipRole :execrows
UPDATE auth_workspace_memberships SET role = ?
WHERE workspace_id = ? AND user_id = ?;

-- name: DeleteWorkspaceMembership :execrows
DELETE FROM auth_workspace_memberships
WHERE workspace_id = ? AND user_id = ?;

-- name: SaveWorkspaceInvitation :exec
INSERT INTO auth_workspace_invitations
	(id, workspace_id, email, role, token_hash, invited_by_user_id, expires_at, accepted_at, accepted_by_user_id, revoked_at, created_at)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?);

-- name: ListWorkspaceInvitations :many
SELECT id, workspace_id, email, role, token_hash, invited_by_user_id, expires_at,
	accepted_at, accepted_by_user_id, revoked_at, created_at
FROM auth_workspace_invitations
WHERE workspace_id = ?
ORDER BY created_at DESC, id DESC;

-- name: FindWorkspaceInvitationByTokenHash :one
SELECT i.id, i.workspace_id, w.name AS workspace_name, i.email, i.role, i.token_hash,
	i.invited_by_user_id, i.expires_at, i.accepted_at, i.accepted_by_user_id, i.revoked_at, i.created_at
FROM auth_workspace_invitations i
JOIN auth_workspaces w ON w.id = i.workspace_id
WHERE i.token_hash = ?;

-- name: AcceptWorkspaceInvitation :execrows
UPDATE auth_workspace_invitations
SET accepted_at = ?, accepted_by_user_id = ?
WHERE id = ? AND accepted_at IS NULL AND revoked_at IS NULL AND expires_at > ?;

-- name: RevokeWorkspaceInvitation :execrows
UPDATE auth_workspace_invitations SET revoked_at = ?
WHERE id = ? AND workspace_id = ? AND accepted_at IS NULL AND revoked_at IS NULL;

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

-- name: FindAPIKeyPrincipalByTokenHash :one
SELECT k.id AS key_id, k.user_id AS key_user_id, k.workspace_id AS key_workspace_id,
	k.name AS key_name, k.token_hash AS key_token_hash, k.prefix AS key_prefix,
	k.scopes AS key_scopes, k.allowed_meters AS key_allowed_meters,
	k.expires_at AS key_expires_at, k.revoked_at AS key_revoked_at,
	k.created_at AS key_created_at, k.last_used_at AS key_last_used_at,
	u.id AS user_id, u.email AS user_email, u.created_at AS user_created_at
FROM auth_api_keys k
JOIN auth_users u ON u.id = k.user_id
WHERE k.token_hash = ?;

-- name: FindAPIKeyByID :one
SELECT id, user_id, workspace_id, name, token_hash, prefix, scopes, allowed_meters, expires_at, revoked_at, created_at, last_used_at
FROM auth_api_keys
WHERE id = ? AND user_id = ? AND workspace_id = ?;

-- name: UpdateAPIKeyLastUsed :exec
UPDATE auth_api_keys
SET last_used_at = ?
WHERE id = ?;

-- name: RevokeAPIKey :execrows
UPDATE auth_api_keys
SET revoked_at = ?
WHERE id = ? AND user_id = ? AND workspace_id = ?
	AND (revoked_at IS NULL OR revoked_at > ?);

-- name: ScheduleAPIKeyRevocation :execrows
UPDATE auth_api_keys
SET revoked_at = ?
WHERE id = ? AND user_id = ? AND workspace_id = ? AND revoked_at IS NULL;

-- name: SaveAPIKeyEvent :exec
INSERT INTO auth_api_key_events (id, workspace_id, user_id, api_key_id, key_name, key_prefix, event_type, related_api_key_id, effective_at, created_at)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?);

-- name: ListAPIKeyEvents :many
SELECT id, workspace_id, user_id, api_key_id, key_name, key_prefix, event_type, related_api_key_id, effective_at, created_at
FROM auth_api_key_events
WHERE workspace_id = ? AND user_id = ?
ORDER BY created_at DESC, id DESC
LIMIT ?;
