-- name: PruneUsageEvents :execrows
DELETE FROM usage_events
WHERE meter_name = ?
	AND event_time < ?;

-- name: CountPrunableUsageEvents :one
SELECT COUNT(*)
FROM usage_events
WHERE meter_name = ?
	AND event_time < ?;

-- name: SaveUsagePruneRun :exec
INSERT INTO usage_prune_runs (id, dry_run, deleted, meters, created_at)
VALUES (?, ?, ?, ?, ?);

-- name: ListUsagePruneRuns :many
SELECT id, dry_run, deleted, meters, created_at
FROM usage_prune_runs
WHERE (CAST(sqlc.narg('cursor_created_at') AS TEXT) IS NULL
	OR (created_at < CAST(sqlc.narg('cursor_created_at') AS TEXT)
		OR (created_at = CAST(sqlc.narg('cursor_created_at') AS TEXT) AND id < CAST(sqlc.narg('cursor_id') AS TEXT))))
ORDER BY created_at DESC, id DESC
LIMIT sqlc.arg('limit');

-- name: CountUsagePruneRuns :one
SELECT COUNT(*)
FROM usage_prune_runs;

-- name: SaveUsageIngestionRun :exec
INSERT INTO usage_ingestions (id, kind, accepted, duplicates, failed, created_at)
VALUES (?, ?, ?, ?, ?, ?);

-- name: ListUsageIngestionRuns :many
SELECT id, kind, accepted, duplicates, failed, created_at
FROM usage_ingestions
WHERE (CAST(sqlc.narg('cursor_created_at') AS TEXT) IS NULL
	OR (created_at < CAST(sqlc.narg('cursor_created_at') AS TEXT)
		OR (created_at = CAST(sqlc.narg('cursor_created_at') AS TEXT) AND id < CAST(sqlc.narg('cursor_id') AS TEXT))))
ORDER BY created_at DESC, id DESC
LIMIT sqlc.arg('limit');
