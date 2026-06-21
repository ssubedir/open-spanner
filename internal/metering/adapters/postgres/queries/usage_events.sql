-- name: ListUsageEvents :many
SELECT id, idempotency_key, subject, meter_name, quantity, event_time, received_at, metadata
FROM usage_events
WHERE workspace_id = sqlc.arg('workspace_id')::text
	AND (sqlc.narg('subject')::text IS NULL OR subject = sqlc.narg('subject')::text)
	AND (sqlc.narg('meter_name')::text IS NULL OR meter_name = sqlc.narg('meter_name')::text)
	AND (sqlc.narg('from_time')::text IS NULL OR event_time >= sqlc.narg('from_time')::text)
	AND (sqlc.narg('to_time')::text IS NULL OR event_time < sqlc.narg('to_time')::text)
	AND (sqlc.narg('cursor_event_time')::text IS NULL
		OR (event_time < sqlc.narg('cursor_event_time')::text
			OR (event_time = sqlc.narg('cursor_event_time')::text AND id < sqlc.narg('cursor_id')::text)))
ORDER BY event_time DESC, id DESC
LIMIT sqlc.arg('limit')::int;
