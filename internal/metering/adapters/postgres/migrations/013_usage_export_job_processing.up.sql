ALTER TABLE usage_export_jobs
	ADD COLUMN attempts INTEGER NOT NULL DEFAULT 0,
	ADD COLUMN locked_until TEXT,
	ADD COLUMN artifact_path TEXT NOT NULL DEFAULT '',
	ADD COLUMN artifact_size BIGINT NOT NULL DEFAULT 0;

CREATE INDEX IF NOT EXISTS idx_usage_export_jobs_claim
	ON usage_export_jobs (status, locked_until, created_at, id);
