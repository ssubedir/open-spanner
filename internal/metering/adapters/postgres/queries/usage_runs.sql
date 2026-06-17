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
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13);

-- name: FindUsageExportJob :one
SELECT id, kind, status, format, query_json, error, attempts, locked_until, artifact_path, artifact_size, created_at, updated_at, completed_at
FROM usage_export_jobs
WHERE id = $1;

-- name: ListUsageExportJobs :many
SELECT id, kind, status, format, query_json, error, attempts, locked_until, artifact_path, artifact_size, created_at, updated_at, completed_at
FROM usage_export_jobs
WHERE (sqlc.narg('cursor_created_at')::text IS NULL
	OR (created_at < sqlc.narg('cursor_created_at')::text
		OR (created_at = sqlc.narg('cursor_created_at')::text AND id < sqlc.narg('cursor_id')::text)))
ORDER BY created_at DESC, id DESC
LIMIT sqlc.arg('limit')::int;

-- name: ClaimUsageExportJob :one
WITH next_job AS (
	SELECT id
	FROM usage_export_jobs
	WHERE (status = 'queued'
		OR (status = 'running' AND locked_until IS NOT NULL AND locked_until < sqlc.arg('now')::text))
		AND usage_export_jobs.attempts < sqlc.arg('max_attempts')::int
	ORDER BY created_at ASC, id ASC
	FOR UPDATE SKIP LOCKED
	LIMIT 1
)
UPDATE usage_export_jobs
SET status = 'running',
	attempts = usage_export_jobs.attempts + 1,
	locked_until = sqlc.arg('locked_until')::text,
	error = '',
	updated_at = sqlc.arg('now')::text
WHERE id = (SELECT id FROM next_job)
RETURNING id, kind, status, format, query_json, error, attempts, locked_until, artifact_path, artifact_size, created_at, updated_at, completed_at;

-- name: CompleteUsageExportJob :one
UPDATE usage_export_jobs
SET status = 'completed',
	artifact_path = sqlc.arg('artifact_path')::text,
	artifact_size = sqlc.arg('artifact_size')::bigint,
	locked_until = NULL,
	error = '',
	updated_at = sqlc.arg('completed_at')::text,
	completed_at = sqlc.arg('completed_at')::text
WHERE id = sqlc.arg('id')::text
RETURNING id, kind, status, format, query_json, error, attempts, locked_until, artifact_path, artifact_size, created_at, updated_at, completed_at;

-- name: FailUsageExportJob :one
UPDATE usage_export_jobs
SET status = 'failed',
	error = sqlc.arg('error')::text,
	locked_until = NULL,
	updated_at = sqlc.arg('failed_at')::text,
	completed_at = sqlc.arg('failed_at')::text
WHERE id = sqlc.arg('id')::text
RETURNING id, kind, status, format, query_json, error, attempts, locked_until, artifact_path, artifact_size, created_at, updated_at, completed_at;
