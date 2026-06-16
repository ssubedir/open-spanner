-- name: SaveMeter :exec
INSERT INTO meters (id, name, description, unit, aggregation, metadata_schema, dimensions, event_retention_days, created_at)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
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
WHERE (sqlc.narg('id') IS NULL OR id = sqlc.narg('id'))
	AND (sqlc.narg('name') IS NULL OR name = sqlc.narg('name'))
	AND (sqlc.narg('cursor') IS NULL OR name > sqlc.narg('cursor'))
ORDER BY name
LIMIT sqlc.arg('limit');

-- name: CountMeters :one
SELECT COUNT(*)
FROM meters;

-- name: CountUsageEventsForMeter :one
SELECT COUNT(*)
FROM usage_events
WHERE meter_name = ?;

-- name: DeleteMeter :exec
DELETE FROM meters
WHERE id = ?;
