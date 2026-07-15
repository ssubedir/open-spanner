CREATE TABLE system_worker_dead_letters (
	id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
	public_id UUID NOT NULL UNIQUE,
	workspace_id TEXT NOT NULL REFERENCES auth_workspaces(id) ON DELETE CASCADE,
	worker_name TEXT NOT NULL CHECK (worker_name IN ('alert', 'entitlement')),
	job_key TEXT NOT NULL,
	rule_id TEXT NOT NULL DEFAULT '',
	subject TEXT NOT NULL DEFAULT '',
	meter_name TEXT NOT NULL DEFAULT '',
	attempts INTEGER NOT NULL,
	last_error TEXT NOT NULL,
	status TEXT NOT NULL CHECK (status IN ('dead_letter', 'requeued')),
	created_at TIMESTAMPTZ NOT NULL,
	requeued_at TIMESTAMPTZ
);

CREATE INDEX system_worker_dead_letters_workspace_created_idx
	ON system_worker_dead_letters (workspace_id, created_at DESC, id DESC);
CREATE INDEX system_worker_dead_letters_active_idx
	ON system_worker_dead_letters (worker_name, status, created_at DESC);
