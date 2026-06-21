-- name: ListUsageBuckets :many
WITH filtered AS (
	SELECT
		(CASE sqlc.arg('bucket_size')::text
			WHEN 'hour' THEN date_trunc('hour', event_time::timestamptz)
			WHEN 'month' THEN date_trunc('month', event_time::timestamptz)
			ELSE date_trunc('day', event_time::timestamptz)
		END)::timestamptz AS bucket_start,
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
	bucket_start,
	(CASE sqlc.arg('aggregation')::text
		WHEN 'count' THEN COUNT(*)::double precision
		WHEN 'avg' THEN AVG(quantity)
		WHEN 'min' THEN MIN(quantity)
		WHEN 'max' THEN MAX(quantity)
		WHEN 'first' THEN (array_agg(quantity ORDER BY event_at ASC))[1]
		WHEN 'last' THEN (array_agg(quantity ORDER BY event_at DESC))[1]
		WHEN 'rate' THEN COUNT(*)::double precision / (CASE sqlc.arg('bucket_size')::text
			WHEN 'hour' THEN 3600::double precision
			WHEN 'month' THEN EXTRACT(EPOCH FROM (bucket_start + INTERVAL '1 month' - bucket_start))
			ELSE 86400::double precision
		END)
		ELSE SUM(quantity)
	END)::double precision AS quantity
FROM filtered
GROUP BY bucket_start
ORDER BY bucket_start ASC
LIMIT sqlc.arg('limit')::int;
