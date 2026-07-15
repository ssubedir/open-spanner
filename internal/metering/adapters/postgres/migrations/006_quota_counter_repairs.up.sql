CREATE TABLE quota_counter_repair_runs (
	id TEXT PRIMARY KEY,
	workspace_id TEXT NOT NULL REFERENCES auth_workspaces(id) ON DELETE CASCADE,
	subject TEXT NOT NULL,
	meter_name TEXT NOT NULL,
	period TEXT NOT NULL,
	period_start TEXT NOT NULL,
	period_end TEXT NOT NULL,
	dry_run BOOLEAN NOT NULL,
	applied BOOLEAN NOT NULL,
	before_snapshot JSONB NOT NULL,
	after_snapshot JSONB NOT NULL,
	counter_updated_at TEXT NOT NULL,
	created_at TIMESTAMPTZ NOT NULL,
	FOREIGN KEY (workspace_id, meter_name) REFERENCES meters(workspace_id, name)
);

CREATE INDEX idx_quota_counter_repair_runs_workspace_created
	ON quota_counter_repair_runs (workspace_id, created_at DESC, id DESC);
