DROP TABLE IF EXISTS usage_export_cleanup_runs;
DROP INDEX IF EXISTS idx_usage_export_jobs_artifact_retention;
ALTER TABLE usage_export_jobs DROP COLUMN IF EXISTS expired_at;
