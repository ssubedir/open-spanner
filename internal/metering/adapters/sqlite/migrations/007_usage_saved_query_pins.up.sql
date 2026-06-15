ALTER TABLE usage_saved_queries ADD COLUMN pinned INTEGER NOT NULL DEFAULT 0;
ALTER TABLE usage_saved_queries ADD COLUMN position INTEGER NOT NULL DEFAULT 0;

CREATE INDEX IF NOT EXISTS idx_usage_saved_queries_user_pinned_position
	ON usage_saved_queries (user_id, pinned DESC, position ASC, updated_at DESC, id DESC);
