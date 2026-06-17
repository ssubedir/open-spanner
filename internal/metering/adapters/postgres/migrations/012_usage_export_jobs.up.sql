CREATE TABLE IF NOT EXISTS usage_export_jobs (
	id TEXT PRIMARY KEY,
	kind TEXT NOT NULL,
	status TEXT NOT NULL,
	format TEXT NOT NULL,
	query_json TEXT NOT NULL,
	error TEXT NOT NULL DEFAULT '',
	created_at TEXT NOT NULL,
	updated_at TEXT NOT NULL,
	completed_at TEXT
);

CREATE INDEX IF NOT EXISTS idx_usage_export_jobs_created_at
	ON usage_export_jobs (created_at DESC, id DESC);

CREATE INDEX IF NOT EXISTS idx_usage_export_jobs_status
	ON usage_export_jobs (status, created_at DESC, id DESC);
