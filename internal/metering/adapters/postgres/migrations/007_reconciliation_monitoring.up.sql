CREATE TABLE reconciliation_schedules (
	workspace_id TEXT PRIMARY KEY REFERENCES auth_workspaces(id) ON DELETE CASCADE,
	next_run_at TIMESTAMPTZ NOT NULL,
	locked_until TIMESTAMPTZ,
	last_fingerprint TEXT NOT NULL DEFAULT '',
	last_notified_fingerprint TEXT NOT NULL DEFAULT '',
	updated_at TIMESTAMPTZ NOT NULL
);

CREATE INDEX reconciliation_schedules_due_idx
	ON reconciliation_schedules (next_run_at, workspace_id);

CREATE TABLE reconciliation_runs (
	id TEXT PRIMARY KEY,
	workspace_id TEXT NOT NULL REFERENCES auth_workspaces(id) ON DELETE CASCADE,
	status TEXT NOT NULL CHECK (status IN ('healthy', 'drift_detected', 'failed')),
	decisions_checked INTEGER NOT NULL,
	counters_checked INTEGER NOT NULL,
	issue_count INTEGER NOT NULL,
	truncated BOOLEAN NOT NULL,
	lookback_hours INTEGER NOT NULL,
	duration_ms BIGINT NOT NULL,
	fingerprint TEXT NOT NULL DEFAULT '',
	issues JSONB NOT NULL DEFAULT '[]'::jsonb,
	error TEXT NOT NULL DEFAULT '',
	created_at TIMESTAMPTZ NOT NULL
);

CREATE INDEX reconciliation_runs_workspace_created_idx
	ON reconciliation_runs (workspace_id, created_at DESC, id DESC);
