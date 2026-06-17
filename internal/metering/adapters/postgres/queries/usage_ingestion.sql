-- name: SaveUsageEvent :exec
INSERT INTO usage_events (
	id,
	idempotency_key,
	subject,
	meter_name,
	quantity,
	event_time,
	received_at,
	metadata
) VALUES (
	sqlc.arg('id'),
	NULLIF(sqlc.arg('idempotency_key')::text, ''),
	sqlc.arg('subject'),
	sqlc.arg('meter_name'),
	sqlc.arg('quantity'),
	sqlc.arg('event_time'),
	sqlc.arg('received_at'),
	sqlc.arg('metadata')::jsonb
);

-- name: FindUsageEventByID :one
SELECT id, idempotency_key, subject, meter_name, quantity, event_time, received_at, metadata
FROM usage_events
WHERE id = $1;

-- name: FindUsageEventByIdempotencyKey :one
SELECT id, idempotency_key, subject, meter_name, quantity, event_time, received_at, metadata
FROM usage_events
WHERE idempotency_key = $1;

-- name: SaveBulkUsageIngestion :exec
INSERT INTO bulk_usage_ingestions (idempotency_key, response, created_at)
VALUES ($1, $2, $3);

-- name: FindBulkUsageIngestion :one
SELECT response
FROM bulk_usage_ingestions
WHERE idempotency_key = $1;
