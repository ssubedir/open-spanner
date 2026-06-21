-- name: SaveMeter :exec
INSERT INTO meters (id, workspace_id, name, description, unit, aggregation, dimensions, event_retention_days, created_at)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
ON CONFLICT(id) DO UPDATE SET
	description = excluded.description,
	unit = excluded.unit,
	aggregation = excluded.aggregation,
	dimensions = excluded.dimensions,
	event_retention_days = excluded.event_retention_days;

-- name: ListMeters :many
SELECT id, name, description, unit, aggregation, dimensions, event_retention_days, created_at
FROM meters
WHERE workspace_id = sqlc.arg('workspace_id')::text
	AND (sqlc.narg('id')::text IS NULL OR id = sqlc.narg('id')::text)
	AND (sqlc.narg('name')::text IS NULL OR name = sqlc.narg('name')::text)
	AND (sqlc.narg('cursor')::text IS NULL OR name > sqlc.narg('cursor')::text)
ORDER BY name
LIMIT sqlc.arg('limit')::int;

-- name: CountMeters :one
SELECT COUNT(*)
FROM meters
WHERE workspace_id = sqlc.arg('workspace_id')::text;

-- name: CountUsageEventsForMeter :one
SELECT COUNT(*)
FROM usage_events
WHERE workspace_id = sqlc.arg('workspace_id')::text
	AND meter_name = sqlc.arg('meter_name')::text;

-- name: DeleteMeter :exec
DELETE FROM meters
WHERE workspace_id = sqlc.arg('workspace_id')::text
	AND id = sqlc.arg('id')::text;
