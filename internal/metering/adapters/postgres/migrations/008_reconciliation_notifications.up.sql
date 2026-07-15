ALTER TABLE reconciliation_schedules
	ADD COLUMN last_failure_fingerprint TEXT NOT NULL DEFAULT '';

CREATE TABLE reconciliation_notifications (
	id TEXT PRIMARY KEY,
	workspace_id TEXT NOT NULL REFERENCES auth_workspaces(id) ON DELETE CASCADE,
	event_type TEXT NOT NULL CHECK (event_type IN ('drift_detected', 'scan_failed')),
	fingerprint TEXT NOT NULL,
	payload JSONB NOT NULL,
	status TEXT NOT NULL CHECK (status IN ('pending', 'delivered', 'dead_letter')),
	attempts INTEGER NOT NULL DEFAULT 0,
	next_attempt_at TIMESTAMPTZ NOT NULL,
	locked_until TIMESTAMPTZ,
	last_error TEXT NOT NULL DEFAULT '',
	created_at TIMESTAMPTZ NOT NULL,
	delivered_at TIMESTAMPTZ
);

CREATE INDEX reconciliation_notifications_due_idx
	ON reconciliation_notifications (status, next_attempt_at, id);
CREATE INDEX reconciliation_notifications_workspace_created_idx
	ON reconciliation_notifications (workspace_id, created_at DESC, id DESC);

CREATE TABLE reconciliation_notification_attempts (
	id TEXT PRIMARY KEY,
	notification_id TEXT NOT NULL REFERENCES reconciliation_notifications(id) ON DELETE CASCADE,
	attempt INTEGER NOT NULL,
	status TEXT NOT NULL CHECK (status IN ('delivered', 'failed')),
	error TEXT NOT NULL DEFAULT '',
	created_at TIMESTAMPTZ NOT NULL
);

CREATE INDEX reconciliation_notification_attempts_notification_idx
	ON reconciliation_notification_attempts (notification_id, created_at, id);
