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

-- name: SaveUsageExportJob :exec
INSERT INTO usage_export_jobs (
	id,
	kind,
	status,
	format,
	query_json,
	error,
	attempts,
	locked_until,
	artifact_path,
	artifact_size,
	created_at,
	updated_at,
	completed_at
)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?);

-- name: FindUsageExportJob :one
SELECT id, kind, status, format, query_json, error, attempts, locked_until, artifact_path, artifact_size, created_at, updated_at, completed_at
FROM usage_export_jobs
WHERE id = ?;

-- name: ListUsageExportJobs :many
SELECT id, kind, status, format, query_json, error, attempts, locked_until, artifact_path, artifact_size, created_at, updated_at, completed_at
FROM usage_export_jobs
WHERE (CAST(sqlc.narg('cursor_created_at') AS TEXT) IS NULL
	OR (created_at < CAST(sqlc.narg('cursor_created_at') AS TEXT)
		OR (created_at = CAST(sqlc.narg('cursor_created_at') AS TEXT) AND id < CAST(sqlc.narg('cursor_id') AS TEXT))))
ORDER BY created_at DESC, id DESC
LIMIT sqlc.arg('limit');

-- name: ClaimUsageExportJob :one
UPDATE usage_export_jobs
SET status = 'running',
	attempts = usage_export_jobs.attempts + 1,
	locked_until = sqlc.arg('locked_until'),
	error = '',
	updated_at = sqlc.arg('now')
WHERE id = (
	SELECT id
	FROM usage_export_jobs
	WHERE (status = 'queued'
		OR (status = 'running' AND locked_until IS NOT NULL AND locked_until < sqlc.arg('now')))
		AND usage_export_jobs.attempts < sqlc.arg('max_attempts')
	ORDER BY created_at ASC, id ASC
	LIMIT 1
)
RETURNING id, kind, status, format, query_json, error, attempts, locked_until, artifact_path, artifact_size, created_at, updated_at, completed_at;

-- name: CompleteUsageExportJob :one
UPDATE usage_export_jobs
SET status = 'completed',
	artifact_path = sqlc.arg('artifact_path'),
	artifact_size = sqlc.arg('artifact_size'),
	locked_until = NULL,
	error = '',
	updated_at = sqlc.arg('completed_at'),
	completed_at = sqlc.arg('completed_at')
WHERE id = sqlc.arg('id')
RETURNING id, kind, status, format, query_json, error, attempts, locked_until, artifact_path, artifact_size, created_at, updated_at, completed_at;

-- name: FailUsageExportJob :one
UPDATE usage_export_jobs
SET status = 'failed',
	error = sqlc.arg('error'),
	locked_until = NULL,
	updated_at = sqlc.arg('failed_at'),
	completed_at = sqlc.arg('failed_at')
WHERE id = sqlc.arg('id')
RETURNING id, kind, status, format, query_json, error, attempts, locked_until, artifact_path, artifact_size, created_at, updated_at, completed_at;
