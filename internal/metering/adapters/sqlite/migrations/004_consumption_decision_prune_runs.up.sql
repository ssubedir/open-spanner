CREATE TABLE consumption_decision_prune_runs (
	id TEXT PRIMARY KEY,
	workspace_id TEXT NOT NULL,
	before TEXT NOT NULL,
	dry_run INTEGER NOT NULL CHECK (dry_run IN (0, 1)),
	deleted INTEGER NOT NULL CHECK (deleted >= 0),
	created_at TEXT NOT NULL
) STRICT;

CREATE INDEX idx_consumption_decision_prune_runs_workspace_created
	ON consumption_decision_prune_runs (workspace_id, created_at DESC, id DESC);
