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

-- name: SaveBulkUsageIngestion :execrows
INSERT INTO bulk_usage_ingestions (workspace_id, idempotency_key, response, created_at)
VALUES (?, ?, ?, ?)
ON CONFLICT DO NOTHING;

-- name: FindBulkUsageIngestion :one
SELECT response
FROM bulk_usage_ingestions
WHERE workspace_id = sqlc.arg('workspace_id')
	AND idempotency_key = sqlc.arg('idempotency_key');
