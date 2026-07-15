CREATE TABLE auth_api_key_events (
	id TEXT PRIMARY KEY,
	workspace_id TEXT NOT NULL,
	user_id TEXT NOT NULL,
	api_key_id TEXT NOT NULL,
	key_name TEXT NOT NULL,
	key_prefix TEXT NOT NULL,
	event_type TEXT NOT NULL CHECK (event_type IN ('created', 'rotated', 'revoked')),
	related_api_key_id TEXT,
	effective_at TIMESTAMPTZ,
	created_at TIMESTAMPTZ NOT NULL
);

CREATE INDEX idx_auth_api_key_events_workspace_user
	ON auth_api_key_events (workspace_id, user_id, created_at DESC, id DESC);
