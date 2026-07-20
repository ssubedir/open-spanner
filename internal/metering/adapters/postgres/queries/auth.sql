-- name: CountUsers :one
SELECT COUNT(*)
FROM auth_users;

-- name: SaveUser :exec
INSERT INTO auth_users (id, email, password_hash, created_at)
VALUES ($1, $2, $3, $4);

-- name: SaveWorkspace :exec
INSERT INTO auth_workspaces (id, name, created_at)
VALUES ($1, $2, $3);

-- name: SaveWorkspaceMembership :exec
INSERT INTO auth_workspace_memberships (workspace_id, user_id, role, created_at)
VALUES ($1, $2, $3, $4);

-- name: FindDefaultWorkspaceByUserID :one
SELECT auth_workspaces.id, auth_workspaces.name, auth_workspaces.created_at
FROM auth_workspaces
JOIN auth_workspace_memberships
	ON auth_workspace_memberships.workspace_id = auth_workspaces.id
WHERE auth_workspace_memberships.user_id = $1
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
WHERE m.user_id = $1
ORDER BY w.name, w.id;

-- name: FindWorkspaceAccess :one
SELECT w.id AS workspace_id, w.name AS workspace_name, w.created_at AS workspace_created_at,
	m.user_id, m.role, m.created_at AS membership_created_at
FROM auth_workspace_memberships m
JOIN auth_workspaces w ON w.id = m.workspace_id
WHERE m.workspace_id = $1 AND m.user_id = $2;

-- name: ListWorkspaceMembers :many
SELECT m.workspace_id, m.user_id, u.email, m.role, m.created_at
FROM auth_workspace_memberships m
JOIN auth_users u ON u.id = m.user_id
WHERE m.workspace_id = $1
ORDER BY CASE m.role WHEN 'owner' THEN 0 WHEN 'admin' THEN 1 ELSE 2 END, u.email, m.user_id;

-- name: CountWorkspaceOwners :one
SELECT COUNT(*) FROM auth_workspace_memberships
WHERE workspace_id = $1 AND role = 'owner';

-- name: UpdateWorkspaceMembershipRole :execrows
UPDATE auth_workspace_memberships SET role = $1
WHERE workspace_id = $2 AND user_id = $3;

-- name: DeleteWorkspaceMembership :execrows
DELETE FROM auth_workspace_memberships
WHERE workspace_id = $1 AND user_id = $2;

-- name: SaveWorkspaceInvitation :exec
INSERT INTO auth_workspace_invitations
	(id, workspace_id, email, role, token_hash, invited_by_user_id, expires_at, accepted_at, accepted_by_user_id, revoked_at, created_at)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11);

-- name: ListWorkspaceInvitations :many
SELECT id, workspace_id, email, role, token_hash, invited_by_user_id, expires_at,
	accepted_at, accepted_by_user_id, revoked_at, created_at
FROM auth_workspace_invitations
WHERE workspace_id = $1
ORDER BY created_at DESC, id DESC;

-- name: FindWorkspaceInvitationByTokenHash :one
SELECT i.id, i.workspace_id, w.name AS workspace_name, i.email, i.role, i.token_hash,
	i.invited_by_user_id, i.expires_at, i.accepted_at, i.accepted_by_user_id, i.revoked_at, i.created_at
FROM auth_workspace_invitations i
JOIN auth_workspaces w ON w.id = i.workspace_id
WHERE i.token_hash = $1;

-- name: AcceptWorkspaceInvitation :execrows
UPDATE auth_workspace_invitations
SET accepted_at = $1, accepted_by_user_id = $2
WHERE id = $3 AND accepted_at IS NULL AND revoked_at IS NULL AND expires_at > $4;

-- name: RevokeWorkspaceInvitation :execrows
UPDATE auth_workspace_invitations SET revoked_at = $1
WHERE id = $2 AND workspace_id = $3 AND accepted_at IS NULL AND revoked_at IS NULL;

-- name: FindUserByID :one
SELECT id, email, password_hash, created_at
FROM auth_users
WHERE id = $1;

-- name: FindUserByEmail :one
SELECT id, email, password_hash, created_at
FROM auth_users
WHERE email = $1;

-- name: SaveIdentity :exec
INSERT INTO auth_identities (id, user_id, provider, subject, email, email_verified, created_at, updated_at)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8);

-- name: FindIdentityByProviderSubject :one
SELECT id, user_id, provider, subject, email, email_verified, created_at, updated_at
FROM auth_identities
WHERE provider = $1 AND subject = $2;

-- name: SaveSession :exec
INSERT INTO auth_sessions (id, user_id, workspace_id, token_hash, kind, expires_at, created_at)
VALUES ($1, $2, $3, $4, $5, $6, $7);

-- name: FindSessionByTokenHash :one
SELECT id, user_id, workspace_id, token_hash, kind, expires_at, created_at
FROM auth_sessions
WHERE token_hash = $1 AND kind = $2 AND expires_at > $3;

-- name: DeleteSessionByTokenHash :exec
DELETE FROM auth_sessions
WHERE token_hash = $1;

-- name: SaveAPIKey :exec
INSERT INTO auth_api_keys (id, user_id, workspace_id, name, token_hash, prefix, scopes, allowed_meters, expires_at, revoked_at, created_at, last_used_at)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12);

-- name: ListAPIKeys :many
SELECT id, user_id, workspace_id, name, token_hash, prefix, scopes, allowed_meters, expires_at, revoked_at, created_at, last_used_at
FROM auth_api_keys
WHERE user_id = $1
	AND workspace_id = $2
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
WHERE k.token_hash = $1;

-- name: FindAPIKeyByID :one
SELECT id, user_id, workspace_id, name, token_hash, prefix, scopes, allowed_meters, expires_at, revoked_at, created_at, last_used_at
FROM auth_api_keys
WHERE id = $1 AND user_id = $2 AND workspace_id = $3;

-- name: UpdateAPIKeyLastUsed :exec
UPDATE auth_api_keys
SET last_used_at = $1
WHERE id = $2;

-- name: RevokeAPIKey :execrows
UPDATE auth_api_keys
SET revoked_at = $1
WHERE id = $2 AND user_id = $3 AND workspace_id = $4
	AND (revoked_at IS NULL OR revoked_at > $5);

-- name: ScheduleAPIKeyRevocation :execrows
UPDATE auth_api_keys
SET revoked_at = $1
WHERE id = $2 AND user_id = $3 AND workspace_id = $4 AND revoked_at IS NULL;

-- name: SaveAPIKeyEvent :exec
INSERT INTO auth_api_key_events (id, workspace_id, user_id, api_key_id, key_name, key_prefix, event_type, related_api_key_id, effective_at, created_at)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10);

-- name: ListAPIKeyEvents :many
SELECT id, workspace_id, user_id, api_key_id, key_name, key_prefix, event_type, related_api_key_id, effective_at, created_at
FROM auth_api_key_events
WHERE workspace_id = $1 AND user_id = $2
ORDER BY created_at DESC, id DESC
LIMIT $3;
