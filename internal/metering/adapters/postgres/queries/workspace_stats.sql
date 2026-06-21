-- name: EnsureWorkspaceStats :exec
INSERT INTO workspace_stats (workspace_id, meters, usage_events, prune_runs, updated_at)
VALUES (sqlc.arg('workspace_id')::text, 0, 0, 0, sqlc.arg('updated_at')::text)
ON CONFLICT (workspace_id) DO NOTHING;

-- name: GetWorkspaceStats :one
SELECT meters, usage_events, prune_runs
FROM workspace_stats
WHERE workspace_id = sqlc.arg('workspace_id')::text;

-- name: IncrementWorkspaceMeters :exec
INSERT INTO workspace_stats (workspace_id, meters, usage_events, prune_runs, updated_at)
VALUES (sqlc.arg('workspace_id')::text, sqlc.arg('delta')::bigint, 0, 0, sqlc.arg('updated_at')::text)
ON CONFLICT (workspace_id) DO UPDATE SET
	meters = GREATEST(0, workspace_stats.meters + excluded.meters),
	updated_at = excluded.updated_at;

-- name: IncrementWorkspaceUsageEvents :exec
INSERT INTO workspace_stats (workspace_id, meters, usage_events, prune_runs, updated_at)
VALUES (sqlc.arg('workspace_id')::text, 0, sqlc.arg('delta')::bigint, 0, sqlc.arg('updated_at')::text)
ON CONFLICT (workspace_id) DO UPDATE SET
	usage_events = GREATEST(0, workspace_stats.usage_events + excluded.usage_events),
	updated_at = excluded.updated_at;

-- name: IncrementWorkspacePruneRuns :exec
INSERT INTO workspace_stats (workspace_id, meters, usage_events, prune_runs, updated_at)
VALUES (sqlc.arg('workspace_id')::text, 0, 0, sqlc.arg('delta')::bigint, sqlc.arg('updated_at')::text)
ON CONFLICT (workspace_id) DO UPDATE SET
	prune_runs = GREATEST(0, workspace_stats.prune_runs + excluded.prune_runs),
	updated_at = excluded.updated_at;
