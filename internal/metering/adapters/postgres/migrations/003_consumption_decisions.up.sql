CREATE TABLE consumption_decisions (
	workspace_id TEXT NOT NULL,
	idempotency_key TEXT NOT NULL,
	response JSONB NOT NULL,
	created_at TIMESTAMPTZ NOT NULL,
	PRIMARY KEY (workspace_id, idempotency_key)
);
