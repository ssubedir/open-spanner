-- name: EnqueueUsageEventOutbox :exec
INSERT INTO usage_event_outbox (
	public_id, workspace_id, event_id, subject, meter_name, quantity, metadata,
	status, attempts, next_attempt_at, created_at, updated_at
) VALUES (
	sqlc.arg('public_id'), sqlc.arg('workspace_id'), sqlc.arg('event_id'),
	sqlc.arg('subject'), sqlc.arg('meter_name'), sqlc.arg('quantity'), sqlc.arg('metadata'),
	'pending', 0, sqlc.arg('now'), sqlc.arg('now'), sqlc.arg('now')
)
ON CONFLICT (workspace_id, event_id) DO NOTHING;

-- name: ClaimUsageEventOutbox :one
UPDATE usage_event_outbox
SET status = 'running', attempts = usage_event_outbox.attempts + 1,
	locked_until = sqlc.arg('locked_until'), claim_token = sqlc.arg('claim_token'), updated_at = sqlc.arg('now')
WHERE id = (
	SELECT id FROM usage_event_outbox
	WHERE usage_event_outbox.attempts < sqlc.arg('max_attempts')
		AND julianday(next_attempt_at) <= julianday(sqlc.arg('now'))
		AND (status = 'pending' OR (status = 'running' AND julianday(locked_until) <= julianday(sqlc.arg('now'))))
	ORDER BY next_attempt_at, id LIMIT 1
)
RETURNING public_id, workspace_id, event_id, subject, meter_name,
	quantity, metadata, attempts, claim_token, created_at;

-- name: CompleteUsageEventOutbox :execrows
DELETE FROM usage_event_outbox
WHERE public_id = sqlc.arg('public_id') AND claim_token = sqlc.arg('claim_token') AND status = 'running';

-- name: RetryUsageEventOutbox :execrows
UPDATE usage_event_outbox
SET status = CASE WHEN attempts >= sqlc.arg('max_attempts') THEN 'dead_letter' ELSE 'pending' END,
	next_attempt_at = sqlc.arg('next_attempt_at'), locked_until = NULL, claim_token = NULL,
	last_error = sqlc.arg('last_error'), updated_at = sqlc.arg('now')
WHERE public_id = sqlc.arg('public_id') AND claim_token = sqlc.arg('claim_token') AND status = 'running';
