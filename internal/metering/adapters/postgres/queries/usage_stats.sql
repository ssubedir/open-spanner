-- name: CountUsageEvents :one
SELECT COUNT(*)
FROM usage_events
WHERE workspace_id = sqlc.arg('workspace_id')::text;

-- name: ListUsageMeterStats :many
SELECT meter_name, COUNT(*)::bigint AS usage_events, MAX(event_time)::text AS last_event_at
FROM usage_events
WHERE workspace_id = sqlc.arg('workspace_id')::text
GROUP BY meter_name
ORDER BY meter_name;

-- name: ListUsageSubjectStats :many
SELECT subject, COUNT(*)::bigint AS usage_events, COUNT(DISTINCT meter_name)::bigint AS meters, MAX(event_time)::text AS last_event_at
FROM usage_events
WHERE workspace_id = sqlc.arg('workspace_id')::text
GROUP BY subject
HAVING (sqlc.narg('cursor_last_event_at')::text IS NULL
	OR MAX(event_time) < sqlc.narg('cursor_last_event_at')::text
	OR (MAX(event_time) = sqlc.narg('cursor_last_event_at')::text AND subject > sqlc.narg('cursor_subject')::text))
ORDER BY MAX(event_time) DESC, subject ASC
LIMIT sqlc.arg('limit')::int;
