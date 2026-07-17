-- name: EnqueueUsageEventOutbox :exec
INSERT INTO usage_event_outbox (
	public_id, workspace_id, event_id, subject, meter_name, quantity, metadata,
	status, attempts, next_attempt_at, created_at, updated_at
) VALUES (
	sqlc.arg('public_id')::uuid, sqlc.arg('workspace_id'), sqlc.arg('event_id'),
	sqlc.arg('subject'), sqlc.arg('meter_name'), sqlc.arg('quantity'), sqlc.arg('metadata'),
	'pending', 0, sqlc.arg('now')::timestamptz, sqlc.arg('now')::timestamptz, sqlc.arg('now')::timestamptz
)
ON CONFLICT (workspace_id, event_id) DO NOTHING;

-- name: ClaimUsageEventOutbox :one
WITH due AS (
	SELECT id FROM usage_event_outbox
	WHERE attempts < sqlc.arg('max_attempts')::int
		AND next_attempt_at <= sqlc.arg('now')::timestamptz
		AND (status = 'pending' OR (status = 'running' AND locked_until <= sqlc.arg('now')::timestamptz))
	ORDER BY next_attempt_at, id
	FOR UPDATE SKIP LOCKED
	LIMIT 1
)
UPDATE usage_event_outbox o
SET status = 'running', attempts = o.attempts + 1,
	locked_until = sqlc.arg('locked_until')::timestamptz,
	claim_token = sqlc.arg('claim_token')::uuid, updated_at = sqlc.arg('now')::timestamptz
FROM due
WHERE o.id = due.id
RETURNING o.public_id, o.workspace_id, o.event_id, o.subject, o.meter_name,
	o.quantity, o.metadata, o.attempts, o.claim_token, o.created_at;

-- name: CompleteUsageEventOutbox :execrows
DELETE FROM usage_event_outbox
WHERE public_id = sqlc.arg('public_id')::uuid AND claim_token = sqlc.arg('claim_token')::uuid AND status = 'running';

-- name: RetryUsageEventOutbox :execrows
UPDATE usage_event_outbox
SET status = CASE WHEN attempts >= sqlc.arg('max_attempts')::int THEN 'dead_letter' ELSE 'pending' END,
	next_attempt_at = sqlc.arg('next_attempt_at')::timestamptz,
	locked_until = NULL, claim_token = NULL, last_error = sqlc.arg('last_error'), updated_at = sqlc.arg('now')::timestamptz
WHERE public_id = sqlc.arg('public_id')::uuid AND claim_token = sqlc.arg('claim_token')::uuid AND status = 'running';
