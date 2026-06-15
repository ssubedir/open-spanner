CREATE TABLE IF NOT EXISTS usage_saved_queries (
	id TEXT PRIMARY KEY,
	user_id TEXT NOT NULL REFERENCES auth_users(id) ON DELETE CASCADE,
	name TEXT NOT NULL,
	query_json JSONB NOT NULL,
	group_by JSONB NOT NULL DEFAULT '[]'::jsonb,
	bucket_size TEXT NOT NULL DEFAULT 'day',
	result_limit INTEGER NOT NULL DEFAULT 500,
	created_at TEXT NOT NULL,
	updated_at TEXT NOT NULL,
	UNIQUE (user_id, name)
);

CREATE INDEX IF NOT EXISTS idx_usage_saved_queries_user_updated
	ON usage_saved_queries (user_id, updated_at DESC, id DESC);
