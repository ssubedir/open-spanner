DROP INDEX IF EXISTS idx_usage_saved_queries_user_pinned_position;

CREATE TABLE usage_saved_queries_next (
	id TEXT PRIMARY KEY,
	user_id TEXT NOT NULL,
	name TEXT NOT NULL,
	query_json TEXT NOT NULL,
	group_by TEXT NOT NULL DEFAULT '[]',
	bucket_size TEXT NOT NULL DEFAULT 'day',
	result_limit INTEGER NOT NULL DEFAULT 500,
	created_at TEXT NOT NULL,
	updated_at TEXT NOT NULL,
	FOREIGN KEY (user_id) REFERENCES auth_users(id) ON DELETE CASCADE,
	UNIQUE (user_id, name)
);

INSERT INTO usage_saved_queries_next (id, user_id, name, query_json, group_by, bucket_size, result_limit, created_at, updated_at)
SELECT id, user_id, name, query_json, group_by, bucket_size, result_limit, created_at, updated_at
FROM usage_saved_queries;

DROP TABLE usage_saved_queries;
ALTER TABLE usage_saved_queries_next RENAME TO usage_saved_queries;

CREATE INDEX IF NOT EXISTS idx_usage_saved_queries_user_updated
	ON usage_saved_queries (user_id, updated_at DESC, id DESC);
