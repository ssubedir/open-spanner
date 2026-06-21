-- name: ListUsageEvents :many
SELECT id, idempotency_key, subject, meter_name, quantity, event_time, received_at, metadata
FROM usage_events
WHERE workspace_id = sqlc.arg('workspace_id')
	AND (CAST(sqlc.narg('subject') AS TEXT) IS NULL OR subject = CAST(sqlc.narg('subject') AS TEXT))
	AND (CAST(sqlc.narg('meter_name') AS TEXT) IS NULL OR meter_name = CAST(sqlc.narg('meter_name') AS TEXT))
	AND (CAST(sqlc.narg('from_time') AS TEXT) IS NULL OR event_time >= CAST(sqlc.narg('from_time') AS TEXT))
	AND (CAST(sqlc.narg('to_time') AS TEXT) IS NULL OR event_time < CAST(sqlc.narg('to_time') AS TEXT))
	AND (CAST(sqlc.narg('cursor_event_time') AS TEXT) IS NULL
		OR (event_time < CAST(sqlc.narg('cursor_event_time') AS TEXT)
			OR (event_time = CAST(sqlc.narg('cursor_event_time') AS TEXT) AND id < CAST(sqlc.narg('cursor_id') AS TEXT))))
ORDER BY event_time DESC, id DESC
LIMIT sqlc.arg('limit');
