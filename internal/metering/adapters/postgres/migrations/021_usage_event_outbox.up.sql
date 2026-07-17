CREATE TABLE usage_event_outbox (
	id BIGSERIAL PRIMARY KEY,
	public_id UUID NOT NULL UNIQUE,
	workspace_id TEXT NOT NULL REFERENCES auth_workspaces(id) ON DELETE CASCADE,
	event_id TEXT NOT NULL,
	subject TEXT NOT NULL,
	meter_name TEXT NOT NULL,
	quantity DOUBLE PRECISION NOT NULL,
	metadata JSONB NOT NULL DEFAULT '{}'::jsonb,
	status TEXT NOT NULL DEFAULT 'pending' CHECK (status IN ('pending', 'running', 'dead_letter')),
	attempts INTEGER NOT NULL DEFAULT 0,
	next_attempt_at TIMESTAMPTZ NOT NULL,
	locked_until TIMESTAMPTZ,
	claim_token UUID,
	last_error TEXT NOT NULL DEFAULT '',
	created_at TIMESTAMPTZ NOT NULL,
	updated_at TIMESTAMPTZ NOT NULL,
	UNIQUE (workspace_id, event_id)
);

CREATE INDEX usage_event_outbox_claim_idx
	ON usage_event_outbox (status, next_attempt_at, locked_until, id);
