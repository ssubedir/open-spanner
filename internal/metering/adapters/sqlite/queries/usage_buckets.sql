-- name: ListUsageBuckets :many
WITH filtered AS (
	SELECT
		id,
		CAST(CASE CAST(sqlc.arg('bucket_size') AS TEXT)
			WHEN 'hour' THEN substr(event_time, 1, 13) || ':00:00Z'
			WHEN 'month' THEN substr(event_time, 1, 7) || '-01T00:00:00Z'
			ELSE substr(event_time, 1, 10) || 'T00:00:00Z'
		END AS TEXT) AS bucket_start,
		quantity,
		event_time AS event_at
	FROM usage_events
	WHERE workspace_id = sqlc.arg('workspace_id')
		AND meter_name = sqlc.arg('meter_name')
		AND event_time >= CAST(sqlc.arg('from_time') AS TEXT)
		AND event_time < CAST(sqlc.arg('to_time') AS TEXT)
		AND (CAST(sqlc.narg('subject') AS TEXT) IS NULL OR subject = CAST(sqlc.narg('subject') AS TEXT))
),
ranked AS (
	SELECT
		bucket_start,
		quantity,
		ROW_NUMBER() OVER (PARTITION BY bucket_start ORDER BY event_at ASC, id ASC) AS first_rank,
		ROW_NUMBER() OVER (PARTITION BY bucket_start ORDER BY event_at DESC, id DESC) AS last_rank
	FROM filtered
)
SELECT
	bucket_start,
	CAST(CASE CAST(sqlc.arg('aggregation') AS TEXT)
		WHEN 'count' THEN CAST(COUNT(*) AS REAL)
		WHEN 'avg' THEN AVG(quantity)
		WHEN 'min' THEN MIN(quantity)
		WHEN 'max' THEN MAX(quantity)
		WHEN 'first' THEN MAX(CASE WHEN first_rank = 1 THEN quantity END)
		WHEN 'last' THEN MAX(CASE WHEN last_rank = 1 THEN quantity END)
		WHEN 'rate' THEN CAST(COUNT(*) AS REAL) / CASE CAST(sqlc.arg('bucket_size') AS TEXT)
			WHEN 'hour' THEN 3600.0
			WHEN 'month' THEN CAST(strftime('%s', datetime(bucket_start, '+1 month')) - strftime('%s', bucket_start) AS REAL)
			ELSE 86400.0
		END
		ELSE SUM(quantity)
	END AS REAL) AS quantity
FROM ranked
GROUP BY bucket_start
ORDER BY bucket_start ASC
LIMIT sqlc.arg('limit');
