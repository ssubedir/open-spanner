-- name: ListUsageBreakdown :many
WITH filtered AS (
	SELECT
		(CASE
			WHEN sqlc.arg('field')::text = 'subject' THEN subject
			ELSE metadata #>> string_to_array(sqlc.arg('field')::text, '.')
		END)::text AS value,
		quantity,
		event_time::timestamptz AS event_at
	FROM usage_events
	WHERE workspace_id = sqlc.arg('workspace_id')::text
		AND meter_name = sqlc.arg('meter_name')
		AND event_time >= sqlc.arg('from_time')::text
		AND event_time < sqlc.arg('to_time')::text
		AND (sqlc.narg('subject')::text IS NULL OR subject = sqlc.narg('subject')::text)
)
SELECT
	value,
	(CASE sqlc.arg('aggregation')::text
		WHEN 'count' THEN COUNT(*)::double precision
		WHEN 'avg' THEN AVG(quantity)
		WHEN 'min' THEN MIN(quantity)
		WHEN 'max' THEN MAX(quantity)
		WHEN 'first' THEN (array_agg(quantity ORDER BY event_at ASC))[1]
		WHEN 'last' THEN (array_agg(quantity ORDER BY event_at DESC))[1]
		WHEN 'rate' THEN COUNT(*)::double precision / sqlc.arg('duration_seconds')::double precision
		ELSE SUM(quantity)
	END)::double precision AS quantity,
	COUNT(*)::bigint AS usage_events
FROM filtered
WHERE value IS NOT NULL AND value != ''
GROUP BY value
ORDER BY quantity DESC, value ASC
LIMIT sqlc.arg('limit')::int;
