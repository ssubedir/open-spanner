-- name: ListUsageDimensionValues :many
SELECT value, COUNT(*) AS usage_events
FROM (
	SELECT CASE json_type(metadata, CAST(sqlc.arg('path') AS TEXT))
		WHEN 'true' THEN 'true'
		WHEN 'false' THEN 'false'
		ELSE CAST(json_extract(metadata, CAST(sqlc.arg('path') AS TEXT)) AS TEXT)
	END AS value
	FROM usage_events
	WHERE workspace_id = sqlc.arg('workspace_id')
		AND meter_name = sqlc.arg('meter_name')
		AND json_type(metadata, CAST(sqlc.arg('path') AS TEXT)) IS NOT NULL
		AND (CAST(sqlc.narg('subject') AS TEXT) IS NULL OR subject = CAST(sqlc.narg('subject') AS TEXT))
		AND (CAST(sqlc.narg('from_time') AS TEXT) IS NULL OR event_time >= CAST(sqlc.narg('from_time') AS TEXT))
		AND (CAST(sqlc.narg('to_time') AS TEXT) IS NULL OR event_time < CAST(sqlc.narg('to_time') AS TEXT))
) discovered
WHERE value IS NOT NULL AND value != ''
GROUP BY value
ORDER BY usage_events DESC, value ASC
LIMIT sqlc.arg('limit');
