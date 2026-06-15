DROP INDEX IF EXISTS idx_usage_saved_queries_user_pinned_position;

ALTER TABLE usage_saved_queries DROP COLUMN IF EXISTS position;
ALTER TABLE usage_saved_queries DROP COLUMN IF EXISTS pinned;
