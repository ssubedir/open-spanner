CREATE TABLE consumption_decisions (
	workspace_id TEXT NOT NULL,
	idempotency_key TEXT NOT NULL,
	response TEXT NOT NULL CHECK (json_valid(response)),
	created_at TEXT NOT NULL,
	PRIMARY KEY (workspace_id, idempotency_key)
) STRICT;
