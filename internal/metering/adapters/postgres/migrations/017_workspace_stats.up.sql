CREATE TABLE IF NOT EXISTS workspace_stats (
	workspace_id TEXT PRIMARY KEY REFERENCES auth_workspaces(id) ON DELETE CASCADE,
	meters BIGINT NOT NULL DEFAULT 0,
	usage_events BIGINT NOT NULL DEFAULT 0,
	prune_runs BIGINT NOT NULL DEFAULT 0,
	updated_at TEXT NOT NULL
);

INSERT INTO workspace_stats (workspace_id, meters, usage_events, prune_runs, updated_at)
SELECT
	auth_workspaces.id,
	COALESCE(meter_counts.meters, 0),
	COALESCE(event_counts.usage_events, 0),
	COALESCE(prune_counts.prune_runs, 0),
	CURRENT_TIMESTAMP::text
FROM auth_workspaces
LEFT JOIN (
	SELECT workspace_id, COUNT(*) AS meters
	FROM meters
	GROUP BY workspace_id
) AS meter_counts ON meter_counts.workspace_id = auth_workspaces.id
LEFT JOIN (
	SELECT workspace_id, COUNT(*) AS usage_events
	FROM usage_events
	GROUP BY workspace_id
) AS event_counts ON event_counts.workspace_id = auth_workspaces.id
LEFT JOIN (
	SELECT workspace_id, COUNT(*) AS prune_runs
	FROM usage_prune_runs
	GROUP BY workspace_id
) AS prune_counts ON prune_counts.workspace_id = auth_workspaces.id
ON CONFLICT (workspace_id) DO UPDATE SET
	meters = excluded.meters,
	usage_events = excluded.usage_events,
	prune_runs = excluded.prune_runs,
	updated_at = excluded.updated_at;
