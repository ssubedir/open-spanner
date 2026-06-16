-- name: CountUsageEvents :one
SELECT COUNT(*)
FROM usage_events;

-- name: ListUsageMeterStats :many
SELECT meter_name, COUNT(*) AS usage_events, CAST(MAX(event_time) AS TEXT) AS last_event_at
FROM usage_events
GROUP BY meter_name
ORDER BY meter_name;

-- name: ListUsageSubjectStats :many
SELECT subject, usage_events, meters, last_event_at
FROM (
	SELECT subject, COUNT(*) AS usage_events, COUNT(DISTINCT meter_name) AS meters, CAST(MAX(event_time) AS TEXT) AS last_event_at
	FROM usage_events
	GROUP BY subject
)
WHERE (CAST(sqlc.narg('cursor_last_event_at') AS TEXT) IS NULL
	OR last_event_at < CAST(sqlc.narg('cursor_last_event_at') AS TEXT)
	OR (last_event_at = CAST(sqlc.narg('cursor_last_event_at') AS TEXT) AND subject > CAST(sqlc.narg('cursor_subject') AS TEXT)))
ORDER BY last_event_at DESC, subject ASC
LIMIT sqlc.arg('limit');
