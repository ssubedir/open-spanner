-- name: PruneUsageEvents :execrows
DELETE FROM usage_events
WHERE workspace_id = sqlc.arg('workspace_id')
	AND meter_name = sqlc.arg('meter_name')
	AND event_time < sqlc.arg('event_time');

-- name: CountPrunableUsageEvents :one
SELECT COUNT(*)
FROM usage_events
WHERE workspace_id = sqlc.arg('workspace_id')
	AND meter_name = sqlc.arg('meter_name')
	AND event_time < sqlc.arg('event_time');

-- name: SaveUsagePruneRun :exec
INSERT INTO usage_prune_runs (id, workspace_id, dry_run, deleted, meters, created_at)
VALUES (?, ?, ?, ?, ?, ?);

-- name: ListUsagePruneRuns :many
SELECT id, dry_run, deleted, meters, created_at
FROM usage_prune_runs
WHERE workspace_id = sqlc.arg('workspace_id')
	AND (CAST(sqlc.narg('cursor_created_at') AS TEXT) IS NULL
	OR (created_at < CAST(sqlc.narg('cursor_created_at') AS TEXT)
		OR (created_at = CAST(sqlc.narg('cursor_created_at') AS TEXT) AND id < CAST(sqlc.narg('cursor_id') AS TEXT))))
ORDER BY created_at DESC, id DESC
LIMIT sqlc.arg('limit');

-- name: CountUsagePruneRuns :one
SELECT COUNT(*)
FROM usage_prune_runs
WHERE workspace_id = sqlc.arg('workspace_id');

-- name: SaveUsageIngestionRun :exec
INSERT INTO usage_ingestions (id, workspace_id, kind, accepted, duplicates, failed, created_at)
VALUES (?, ?, ?, ?, ?, ?, ?);

-- name: ListUsageIngestionRuns :many
SELECT id, kind, accepted, duplicates, failed, created_at
FROM usage_ingestions
WHERE workspace_id = sqlc.arg('workspace_id')
	AND (CAST(sqlc.narg('cursor_created_at') AS TEXT) IS NULL
	OR (created_at < CAST(sqlc.narg('cursor_created_at') AS TEXT)
		OR (created_at = CAST(sqlc.narg('cursor_created_at') AS TEXT) AND id < CAST(sqlc.narg('cursor_id') AS TEXT))))
ORDER BY created_at DESC, id DESC
LIMIT sqlc.arg('limit');

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
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?);

-- name: FindUsageExportJob :one
SELECT id, workspace_id, kind, status, format, query_json, error, attempts, locked_until, claim_token, artifact_path, artifact_size, created_at, updated_at, completed_at, expired_at
FROM usage_export_jobs
WHERE workspace_id = sqlc.arg('workspace_id')
	AND id = sqlc.arg('id');

-- name: ListUsageExportJobs :many
SELECT id, workspace_id, kind, status, format, query_json, error, attempts, locked_until, claim_token, artifact_path, artifact_size, created_at, updated_at, completed_at, expired_at
FROM usage_export_jobs
WHERE workspace_id = sqlc.arg('workspace_id')
	AND (CAST(sqlc.narg('cursor_created_at') AS TEXT) IS NULL
	OR (created_at < CAST(sqlc.narg('cursor_created_at') AS TEXT)
		OR (created_at = CAST(sqlc.narg('cursor_created_at') AS TEXT) AND id < CAST(sqlc.narg('cursor_id') AS TEXT))))
ORDER BY created_at DESC, id DESC
LIMIT sqlc.arg('limit');

-- name: ClaimUsageExportJob :one
UPDATE usage_export_jobs
SET status = 'running',
	attempts = usage_export_jobs.attempts + 1,
	locked_until = sqlc.arg('locked_until'),
	claim_token = sqlc.arg('claim_token'),
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
RETURNING id, workspace_id, kind, status, format, query_json, error, attempts, locked_until, claim_token, artifact_path, artifact_size, created_at, updated_at, completed_at, expired_at;

-- name: RenewUsageExportJobLease :execrows
UPDATE usage_export_jobs
SET locked_until = sqlc.arg('locked_until'),
	updated_at = sqlc.arg('now')
WHERE id = sqlc.arg('id')
	AND workspace_id = sqlc.arg('workspace_id')
	AND status = 'running'
	AND claim_token = sqlc.arg('claim_token');

-- name: CompleteUsageExportJob :one
UPDATE usage_export_jobs
SET status = 'completed',
	artifact_path = sqlc.arg('artifact_path'),
	artifact_size = sqlc.arg('artifact_size'),
	locked_until = NULL,
	claim_token = NULL,
	error = '',
	updated_at = sqlc.arg('completed_at'),
	completed_at = sqlc.arg('completed_at')
WHERE id = sqlc.arg('id')
	AND workspace_id = sqlc.arg('workspace_id')
	AND status = 'running'
	AND claim_token = sqlc.arg('claim_token')
RETURNING id, workspace_id, kind, status, format, query_json, error, attempts, locked_until, claim_token, artifact_path, artifact_size, created_at, updated_at, completed_at, expired_at;

-- name: FailUsageExportJob :one
UPDATE usage_export_jobs
SET status = 'failed',
	error = sqlc.arg('error'),
	locked_until = NULL,
	claim_token = NULL,
	updated_at = sqlc.arg('failed_at'),
	completed_at = sqlc.arg('failed_at')
WHERE id = sqlc.arg('id')
	AND workspace_id = sqlc.arg('workspace_id')
	AND status = 'running'
	AND claim_token = sqlc.arg('claim_token')
RETURNING id, workspace_id, kind, status, format, query_json, error, attempts, locked_until, claim_token, artifact_path, artifact_size, created_at, updated_at, completed_at, expired_at;

-- name: CancelUsageExportJob :one
UPDATE usage_export_jobs
SET status = 'canceled',
	error = 'canceled by user',
	locked_until = NULL,
	claim_token = NULL,
	updated_at = sqlc.arg('canceled_at'),
	completed_at = sqlc.arg('canceled_at')
WHERE id = sqlc.arg('id')
	AND workspace_id = sqlc.arg('workspace_id')
	AND status IN ('queued', 'running')
RETURNING id, workspace_id, kind, status, format, query_json, error, attempts, locked_until, claim_token, artifact_path, artifact_size, created_at, updated_at, completed_at, expired_at;

-- name: RetryUsageExportJob :one
UPDATE usage_export_jobs
SET status = 'queued',
	error = '',
	attempts = 0,
	locked_until = NULL,
	claim_token = NULL,
	artifact_path = '',
	artifact_size = 0,
	updated_at = sqlc.arg('retried_at'),
	completed_at = NULL
WHERE id = sqlc.arg('id')
	AND workspace_id = sqlc.arg('workspace_id')
	AND status IN ('failed', 'canceled')
RETURNING id, workspace_id, kind, status, format, query_json, error, attempts, locked_until, claim_token, artifact_path, artifact_size, created_at, updated_at, completed_at, expired_at;

-- name: ListExpiredUsageExportJobs :many
SELECT id, workspace_id, kind, status, format, query_json, error, attempts, locked_until, claim_token, artifact_path, artifact_size, created_at, updated_at, completed_at, expired_at
FROM usage_export_jobs
WHERE status = 'completed'
	AND expired_at IS NULL
	AND completed_at < sqlc.arg('expired_before')
ORDER BY completed_at ASC, id ASC
LIMIT sqlc.arg('limit');

-- name: ExpireUsageExportJob :execrows
UPDATE usage_export_jobs
SET expired_at = sqlc.arg('expired_at'), updated_at = sqlc.arg('expired_at')
WHERE id = sqlc.arg('id')
	AND workspace_id = sqlc.arg('workspace_id')
	AND status = 'completed'
	AND expired_at IS NULL;

-- name: SaveUsageExportCleanupRun :exec
INSERT INTO usage_export_cleanup_runs (public_id, workspace_id, expired_before, files_deleted, bytes_reclaimed, failures, created_at)
VALUES (?, ?, ?, ?, ?, ?, ?);

-- name: CountUsageExportCleanupRuns :one
SELECT COUNT(*) FROM usage_export_cleanup_runs WHERE workspace_id = sqlc.arg('workspace_id');

-- name: FindLatestUsageExportCleanupRun :one
SELECT public_id, expired_before, files_deleted, bytes_reclaimed, failures, created_at
FROM usage_export_cleanup_runs
WHERE workspace_id = sqlc.arg('workspace_id')
ORDER BY created_at DESC, id DESC
LIMIT 1;
