CREATE TABLE alert_delivery_jobs (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	public_id TEXT NOT NULL UNIQUE,
	workspace_id TEXT NOT NULL REFERENCES auth_workspaces(id) ON DELETE CASCADE,
	event_id TEXT NOT NULL UNIQUE REFERENCES alert_events(id) ON DELETE CASCADE,
	destination_id TEXT NOT NULL DEFAULT '',
	payload TEXT NOT NULL,
	status TEXT NOT NULL CHECK (status IN ('pending', 'running', 'delivered', 'dead_letter')),
	attempts INTEGER NOT NULL DEFAULT 0,
	next_attempt_at TEXT NOT NULL,
	locked_until TEXT,
	last_error TEXT NOT NULL DEFAULT '',
	created_at TEXT NOT NULL,
	updated_at TEXT NOT NULL,
	delivered_at TEXT
);

CREATE INDEX alert_delivery_jobs_claim_idx
	ON alert_delivery_jobs (status, next_attempt_at, locked_until, id);
CREATE INDEX alert_delivery_jobs_workspace_created_idx
	ON alert_delivery_jobs (workspace_id, created_at DESC, id DESC);
