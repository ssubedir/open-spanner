-- name: ListUsageDimensionValues :many
SELECT value, COUNT(*)::bigint AS usage_events
FROM (
	SELECT (metadata #>> string_to_array(sqlc.arg('field')::text, '.'))::text AS value
	FROM usage_events
	WHERE workspace_id = sqlc.arg('workspace_id')::text
		AND meter_name = sqlc.arg('meter_name')
		AND metadata #>> string_to_array(sqlc.arg('field')::text, '.') IS NOT NULL
		AND (sqlc.narg('subject')::text IS NULL OR subject = sqlc.narg('subject')::text)
		AND (sqlc.narg('from_time')::text IS NULL OR event_time >= sqlc.narg('from_time')::text)
		AND (sqlc.narg('to_time')::text IS NULL OR event_time < sqlc.narg('to_time')::text)
) discovered
WHERE value IS NOT NULL AND value != ''
GROUP BY value
ORDER BY usage_events DESC, value ASC
LIMIT sqlc.arg('limit')::int;
