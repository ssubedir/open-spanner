ALTER TABLE workspace_stats ADD COLUMN ingestion_throttled BIGINT NOT NULL DEFAULT 0;

CREATE TABLE ingestion_rate_windows (
	workspace_id TEXT PRIMARY KEY REFERENCES auth_workspaces(id) ON DELETE CASCADE,
	window_start TEXT NOT NULL,
	permitted_events BIGINT NOT NULL DEFAULT 0,
	throttled_events BIGINT NOT NULL DEFAULT 0,
	updated_at TEXT NOT NULL
);
