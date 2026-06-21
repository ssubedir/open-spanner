-- name: TryPruneLock :one
SELECT pg_try_advisory_xact_lock($1);

-- name: PruneUsageEventsBatch :execrows
WITH deleted AS (
	SELECT usage_events.id
	FROM usage_events
	WHERE usage_events.workspace_id = sqlc.arg('workspace_id')::text
		AND usage_events.meter_name = sqlc.arg('meter_name')::text
		AND usage_events.event_time < sqlc.arg('event_time')::text
	ORDER BY usage_events.event_time ASC, usage_events.id ASC
	LIMIT sqlc.arg('limit')::int
)
DELETE FROM usage_events
WHERE usage_events.id IN (SELECT deleted.id FROM deleted);

-- name: CountPrunableUsageEvents :one
SELECT COUNT(*)
FROM usage_events
WHERE workspace_id = sqlc.arg('workspace_id')::text
	AND meter_name = sqlc.arg('meter_name')::text
	AND event_time < sqlc.arg('event_time')::text;

-- name: SaveUsagePruneRun :exec
INSERT INTO usage_prune_runs (id, workspace_id, dry_run, deleted, meters, created_at)
VALUES ($1, $2, $3, $4, $5, $6);

-- name: ListUsagePruneRuns :many
SELECT id, dry_run, deleted, meters, created_at
FROM usage_prune_runs
WHERE workspace_id = sqlc.arg('workspace_id')::text
	AND (sqlc.narg('cursor_created_at')::text IS NULL
	OR (created_at < sqlc.narg('cursor_created_at')::text
		OR (created_at = sqlc.narg('cursor_created_at')::text AND id < sqlc.narg('cursor_id')::text)))
ORDER BY created_at DESC, id DESC
LIMIT sqlc.arg('limit')::int;

-- name: CountUsagePruneRuns :one
SELECT COUNT(*)
FROM usage_prune_runs
WHERE workspace_id = sqlc.arg('workspace_id')::text;

-- name: SaveUsageIngestionRun :exec
INSERT INTO usage_ingestions (id, workspace_id, kind, accepted, duplicates, failed, created_at)
VALUES ($1, $2, $3, $4, $5, $6, $7);

-- name: ListUsageIngestionRuns :many
SELECT id, kind, accepted, duplicates, failed, created_at
FROM usage_ingestions
WHERE workspace_id = sqlc.arg('workspace_id')::text
	AND (sqlc.narg('cursor_created_at')::text IS NULL
	OR (created_at < sqlc.narg('cursor_created_at')::text
		OR (created_at = sqlc.narg('cursor_created_at')::text AND id < sqlc.narg('cursor_id')::text)))
ORDER BY created_at DESC, id DESC
LIMIT sqlc.arg('limit')::int;

-- name: SaveUsageExportJob :exec
INSERT INTO usage_export_jobs (
	id,
	workspace_id,
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
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14);

-- name: FindUsageExportJob :one
SELECT id, workspace_id, kind, status, format, query_json, error, attempts, locked_until, artifact_path, artifact_size, created_at, updated_at, completed_at
FROM usage_export_jobs
WHERE workspace_id = sqlc.arg('workspace_id')::text
	AND id = sqlc.arg('id')::text;

-- name: ListUsageExportJobs :many
SELECT id, workspace_id, kind, status, format, query_json, error, attempts, locked_until, artifact_path, artifact_size, created_at, updated_at, completed_at
FROM usage_export_jobs
WHERE workspace_id = sqlc.arg('workspace_id')::text
	AND (sqlc.narg('cursor_created_at')::text IS NULL
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
RETURNING id, workspace_id, kind, status, format, query_json, error, attempts, locked_until, artifact_path, artifact_size, created_at, updated_at, completed_at;

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
	AND workspace_id = sqlc.arg('workspace_id')::text
	AND status = 'running'
RETURNING id, workspace_id, kind, status, format, query_json, error, attempts, locked_until, artifact_path, artifact_size, created_at, updated_at, completed_at;

-- name: FailUsageExportJob :one
UPDATE usage_export_jobs
SET status = 'failed',
	error = sqlc.arg('error')::text,
	locked_until = NULL,
	updated_at = sqlc.arg('failed_at')::text,
	completed_at = sqlc.arg('failed_at')::text
WHERE id = sqlc.arg('id')::text
	AND workspace_id = sqlc.arg('workspace_id')::text
	AND status = 'running'
RETURNING id, workspace_id, kind, status, format, query_json, error, attempts, locked_until, artifact_path, artifact_size, created_at, updated_at, completed_at;

-- name: CancelUsageExportJob :one
UPDATE usage_export_jobs
SET status = 'canceled',
	error = 'canceled by user',
	locked_until = NULL,
	updated_at = sqlc.arg('canceled_at')::text,
	completed_at = sqlc.arg('canceled_at')::text
WHERE id = sqlc.arg('id')::text
	AND workspace_id = sqlc.arg('workspace_id')::text
	AND status IN ('queued', 'running')
RETURNING id, workspace_id, kind, status, format, query_json, error, attempts, locked_until, artifact_path, artifact_size, created_at, updated_at, completed_at;

-- name: RetryUsageExportJob :one
UPDATE usage_export_jobs
SET status = 'queued',
	error = '',
	attempts = 0,
	locked_until = NULL,
	artifact_path = '',
	artifact_size = 0,
	updated_at = sqlc.arg('retried_at')::text,
	completed_at = NULL
WHERE id = sqlc.arg('id')::text
	AND workspace_id = sqlc.arg('workspace_id')::text
	AND status IN ('failed', 'canceled')
RETURNING id, workspace_id, kind, status, format, query_json, error, attempts, locked_until, artifact_path, artifact_size, created_at, updated_at, completed_at;
