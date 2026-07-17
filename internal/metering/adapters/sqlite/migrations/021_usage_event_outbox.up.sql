CREATE TABLE usage_event_outbox (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	public_id TEXT NOT NULL UNIQUE,
	workspace_id TEXT NOT NULL REFERENCES auth_workspaces(id) ON DELETE CASCADE,
	event_id TEXT NOT NULL,
	subject TEXT NOT NULL,
	meter_name TEXT NOT NULL,
	quantity REAL NOT NULL,
	metadata TEXT NOT NULL DEFAULT '{}',
	status TEXT NOT NULL DEFAULT 'pending' CHECK (status IN ('pending', 'running', 'dead_letter')),
	attempts INTEGER NOT NULL DEFAULT 0,
	next_attempt_at TEXT NOT NULL,
	locked_until TEXT,
	claim_token TEXT,
	last_error TEXT NOT NULL DEFAULT '',
	created_at TEXT NOT NULL,
	updated_at TEXT NOT NULL,
	UNIQUE (workspace_id, event_id)
);

CREATE INDEX usage_event_outbox_claim_idx
	ON usage_event_outbox (status, next_attempt_at, locked_until, id);
