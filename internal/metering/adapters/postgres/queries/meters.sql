-- name: SaveMeter :exec
INSERT INTO meters (id, name, description, unit, aggregation, metadata_schema, dimensions, event_retention_days, created_at)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
ON CONFLICT(id) DO UPDATE SET
	description = excluded.description,
	unit = excluded.unit,
	aggregation = excluded.aggregation,
	metadata_schema = excluded.metadata_schema,
	dimensions = excluded.dimensions,
	event_retention_days = excluded.event_retention_days;

-- name: ListMeters :many
SELECT id, name, description, unit, aggregation, metadata_schema, dimensions, event_retention_days, created_at
FROM meters
WHERE (sqlc.narg('id')::text IS NULL OR id = sqlc.narg('id')::text)
	AND (sqlc.narg('name')::text IS NULL OR name = sqlc.narg('name')::text)
	AND (sqlc.narg('cursor')::text IS NULL OR name > sqlc.narg('cursor')::text)
ORDER BY name
LIMIT sqlc.arg('limit')::int;

-- name: CountMeters :one
SELECT COUNT(*)
FROM meters;

-- name: CountUsageEventsForMeter :one
SELECT COUNT(*)
FROM usage_events
WHERE meter_name = $1;

-- name: DeleteMeter :exec
DELETE FROM meters
WHERE id = $1;
