-- name: ListUsageBreakdown :many
WITH filtered AS (
	SELECT
		id,
		CASE
			WHEN CAST(sqlc.arg('field') AS TEXT) = 'subject' THEN subject
			WHEN json_type(metadata, CAST(sqlc.arg('path') AS TEXT)) = 'true' THEN 'true'
			WHEN json_type(metadata, CAST(sqlc.arg('path') AS TEXT)) = 'false' THEN 'false'
			ELSE CAST(json_extract(metadata, CAST(sqlc.arg('path') AS TEXT)) AS TEXT)
		END AS value,
		quantity,
		event_time AS event_at
	FROM usage_events
	WHERE meter_name = sqlc.arg('meter_name')
		AND event_time >= CAST(sqlc.arg('from_time') AS TEXT)
		AND event_time < CAST(sqlc.arg('to_time') AS TEXT)
		AND (CAST(sqlc.narg('subject') AS TEXT) IS NULL OR subject = CAST(sqlc.narg('subject') AS TEXT))
),
ranked AS (
	SELECT
		value,
		quantity,
		ROW_NUMBER() OVER (PARTITION BY value ORDER BY event_at ASC, id ASC) AS first_rank,
		ROW_NUMBER() OVER (PARTITION BY value ORDER BY event_at DESC, id DESC) AS last_rank
	FROM filtered
	WHERE value IS NOT NULL AND value != ''
)
SELECT
	value,
	CAST(CASE CAST(sqlc.arg('aggregation') AS TEXT)
		WHEN 'count' THEN CAST(COUNT(*) AS REAL)
		WHEN 'avg' THEN AVG(quantity)
		WHEN 'min' THEN MIN(quantity)
		WHEN 'max' THEN MAX(quantity)
		WHEN 'first' THEN MAX(CASE WHEN first_rank = 1 THEN quantity END)
		WHEN 'last' THEN MAX(CASE WHEN last_rank = 1 THEN quantity END)
		WHEN 'rate' THEN CAST(COUNT(*) AS REAL) / CAST(sqlc.arg('duration_seconds') AS REAL)
		ELSE SUM(quantity)
	END AS REAL) AS quantity,
	CAST(COUNT(*) AS INTEGER) AS usage_events
FROM ranked
GROUP BY value
ORDER BY quantity DESC, value ASC
LIMIT sqlc.arg('limit');
