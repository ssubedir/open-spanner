-- name: TryPruneLock :one
SELECT pg_try_advisory_xact_lock($1);

-- name: PruneUsageEventsBatch :execrows
WITH deleted AS (
	SELECT usage_events.id
	FROM usage_events
	WHERE usage_events.meter_name = $1
		AND usage_events.event_time < $2
	ORDER BY usage_events.event_time ASC, usage_events.id ASC
	LIMIT $3
)
DELETE FROM usage_events
WHERE usage_events.id IN (SELECT deleted.id FROM deleted);

-- name: CountPrunableUsageEvents :one
SELECT COUNT(*)
FROM usage_events
WHERE meter_name = $1
	AND event_time < $2;

-- name: SaveUsagePruneRun :exec
INSERT INTO usage_prune_runs (id, dry_run, deleted, meters, created_at)
VALUES ($1, $2, $3, $4, $5);

-- name: ListUsagePruneRuns :many
SELECT id, dry_run, deleted, meters, created_at
FROM usage_prune_runs
WHERE (sqlc.narg('cursor_created_at')::text IS NULL
	OR (created_at < sqlc.narg('cursor_created_at')::text
		OR (created_at = sqlc.narg('cursor_created_at')::text AND id < sqlc.narg('cursor_id')::text)))
ORDER BY created_at DESC, id DESC
LIMIT sqlc.arg('limit')::int;

-- name: CountUsagePruneRuns :one
SELECT COUNT(*)
FROM usage_prune_runs;

-- name: SaveUsageIngestionRun :exec
INSERT INTO usage_ingestions (id, kind, accepted, duplicates, failed, created_at)
VALUES ($1, $2, $3, $4, $5, $6);

-- name: ListUsageIngestionRuns :many
SELECT id, kind, accepted, duplicates, failed, created_at
FROM usage_ingestions
WHERE (sqlc.narg('cursor_created_at')::text IS NULL
	OR (created_at < sqlc.narg('cursor_created_at')::text
		OR (created_at = sqlc.narg('cursor_created_at')::text AND id < sqlc.narg('cursor_id')::text)))
ORDER BY created_at DESC, id DESC
LIMIT sqlc.arg('limit')::int;
