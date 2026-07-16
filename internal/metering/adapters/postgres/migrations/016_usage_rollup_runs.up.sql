CREATE TABLE usage_rollup_runs (
	id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
	workspace_id TEXT NOT NULL REFERENCES auth_workspaces(id) ON DELETE CASCADE,
	meter_name TEXT NOT NULL,
	finalized_through TEXT NOT NULL,
	source_events BIGINT NOT NULL,
	rollup_rows BIGINT NOT NULL,
	created_at TEXT NOT NULL,
	UNIQUE (workspace_id, meter_name, finalized_through)
);

CREATE INDEX idx_usage_rollup_runs_workspace_meter_created
	ON usage_rollup_runs (workspace_id, meter_name, created_at DESC, id DESC);
