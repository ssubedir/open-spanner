CREATE TABLE consumption_decision_prune_runs (
	id TEXT PRIMARY KEY,
	workspace_id TEXT NOT NULL,
	before TIMESTAMPTZ NOT NULL,
	dry_run BOOLEAN NOT NULL,
	deleted BIGINT NOT NULL CHECK (deleted >= 0),
	created_at TIMESTAMPTZ NOT NULL
);

CREATE INDEX idx_consumption_decision_prune_runs_workspace_created
	ON consumption_decision_prune_runs (workspace_id, created_at DESC, id DESC);
