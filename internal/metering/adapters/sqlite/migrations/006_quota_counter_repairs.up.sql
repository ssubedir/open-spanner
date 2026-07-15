CREATE TABLE quota_counter_repair_runs (
	id TEXT PRIMARY KEY,
	workspace_id TEXT NOT NULL REFERENCES auth_workspaces(id) ON DELETE CASCADE,
	subject TEXT NOT NULL,
	meter_name TEXT NOT NULL,
	period TEXT NOT NULL,
	period_start TEXT NOT NULL,
	period_end TEXT NOT NULL,
	dry_run INTEGER NOT NULL CHECK (dry_run IN (0, 1)),
	applied INTEGER NOT NULL CHECK (applied IN (0, 1)),
	before_snapshot TEXT NOT NULL,
	after_snapshot TEXT NOT NULL,
	counter_updated_at TEXT NOT NULL,
	created_at TEXT NOT NULL,
	FOREIGN KEY (workspace_id, meter_name) REFERENCES meters(workspace_id, name)
);

CREATE INDEX idx_quota_counter_repair_runs_workspace_created
	ON quota_counter_repair_runs (workspace_id, created_at DESC, id DESC);
