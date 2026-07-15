ALTER TABLE usage_export_jobs ADD COLUMN expired_at TEXT;

CREATE INDEX idx_usage_export_jobs_artifact_retention
	ON usage_export_jobs (completed_at, id)
	WHERE status = 'completed' AND expired_at IS NULL;

CREATE TABLE usage_export_cleanup_runs (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	public_id TEXT NOT NULL UNIQUE,
	workspace_id TEXT NOT NULL REFERENCES auth_workspaces(id) ON DELETE CASCADE,
	expired_before TEXT NOT NULL,
	files_deleted INTEGER NOT NULL,
	bytes_reclaimed INTEGER NOT NULL,
	failures INTEGER NOT NULL,
	created_at TEXT NOT NULL
);

CREATE INDEX idx_usage_export_cleanup_runs_workspace_created
	ON usage_export_cleanup_runs (workspace_id, created_at DESC, id DESC);
