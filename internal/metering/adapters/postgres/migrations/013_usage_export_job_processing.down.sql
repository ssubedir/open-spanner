DROP INDEX IF EXISTS idx_usage_export_jobs_claim;

ALTER TABLE usage_export_jobs
	DROP COLUMN artifact_size,
	DROP COLUMN artifact_path,
	DROP COLUMN locked_until,
	DROP COLUMN attempts;
