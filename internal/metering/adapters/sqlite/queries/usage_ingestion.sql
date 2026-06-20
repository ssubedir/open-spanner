-- name: SaveUsageEvent :exec
INSERT INTO usage_events (
	id,
	workspace_id,
	idempotency_key,
	subject,
	meter_name,
	quantity,
	event_time,
	received_at,
	metadata
) VALUES (
	sqlc.arg('id'),
	sqlc.arg('workspace_id'),
	NULLIF(CAST(sqlc.arg('idempotency_key') AS TEXT), ''),
	sqlc.arg('subject'),
	sqlc.arg('meter_name'),
	sqlc.arg('quantity'),
	sqlc.arg('event_time'),
	sqlc.arg('received_at'),
	sqlc.arg('metadata')
);

-- name: FindUsageEventByID :one
SELECT id, idempotency_key, subject, meter_name, quantity, event_time, received_at, metadata
FROM usage_events
WHERE workspace_id = sqlc.arg('workspace_id')
	AND id = sqlc.arg('id');

-- name: FindUsageEventByIdempotencyKey :one
SELECT id, idempotency_key, subject, meter_name, quantity, event_time, received_at, metadata
FROM usage_events
WHERE workspace_id = sqlc.arg('workspace_id')
	AND idempotency_key = sqlc.arg('idempotency_key');

-- name: SaveBulkUsageIngestion :exec
INSERT INTO bulk_usage_ingestions (workspace_id, idempotency_key, response, created_at)
VALUES (?, ?, ?, ?);

-- name: FindBulkUsageIngestion :one
SELECT response
FROM bulk_usage_ingestions
WHERE workspace_id = sqlc.arg('workspace_id')
	AND idempotency_key = sqlc.arg('idempotency_key');
