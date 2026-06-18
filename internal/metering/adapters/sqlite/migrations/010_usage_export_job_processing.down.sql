DROP INDEX IF EXISTS idx_usage_export_jobs_claim;

ALTER TABLE usage_export_jobs
	DROP COLUMN artifact_size;

ALTER TABLE usage_export_jobs
	DROP COLUMN artifact_path;

ALTER TABLE usage_export_jobs
	DROP COLUMN locked_until;

ALTER TABLE usage_export_jobs
	DROP COLUMN attempts;
