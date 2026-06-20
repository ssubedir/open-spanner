CREATE TABLE IF NOT EXISTS auth_workspaces (
	id TEXT PRIMARY KEY,
	name TEXT NOT NULL,
	created_at TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS auth_workspace_memberships (
	workspace_id TEXT NOT NULL,
	user_id TEXT NOT NULL,
	role TEXT NOT NULL,
	created_at TEXT NOT NULL,
	PRIMARY KEY (workspace_id, user_id),
	FOREIGN KEY (workspace_id) REFERENCES auth_workspaces(id) ON DELETE CASCADE,
	FOREIGN KEY (user_id) REFERENCES auth_users(id) ON DELETE CASCADE
);

INSERT OR IGNORE INTO auth_workspaces (id, name, created_at)
VALUES ('default', 'Default workspace', '1970-01-01T00:00:00Z');

ALTER TABLE auth_sessions ADD COLUMN workspace_id TEXT NOT NULL DEFAULT 'default';
ALTER TABLE auth_api_keys ADD COLUMN workspace_id TEXT NOT NULL DEFAULT 'default';
ALTER TABLE meters ADD COLUMN workspace_id TEXT NOT NULL DEFAULT 'default';
ALTER TABLE usage_events ADD COLUMN workspace_id TEXT NOT NULL DEFAULT 'default';
ALTER TABLE bulk_usage_ingestions ADD COLUMN workspace_id TEXT NOT NULL DEFAULT 'default';
ALTER TABLE usage_prune_runs ADD COLUMN workspace_id TEXT NOT NULL DEFAULT 'default';
ALTER TABLE usage_ingestions ADD COLUMN workspace_id TEXT NOT NULL DEFAULT 'default';
ALTER TABLE usage_export_jobs ADD COLUMN workspace_id TEXT NOT NULL DEFAULT 'default';
ALTER TABLE alert_destinations ADD COLUMN workspace_id TEXT NOT NULL DEFAULT 'default';
ALTER TABLE alert_rules ADD COLUMN workspace_id TEXT NOT NULL DEFAULT 'default';

PRAGMA foreign_keys=OFF;

DROP TABLE IF EXISTS meters_workspace_next;
CREATE TABLE meters_workspace_next (
	id TEXT PRIMARY KEY,
	workspace_id TEXT NOT NULL DEFAULT 'default',
	name TEXT NOT NULL,
	description TEXT NOT NULL DEFAULT '',
	unit TEXT NOT NULL,
	aggregation TEXT NOT NULL,
	dimensions TEXT NOT NULL DEFAULT '[]',
	event_retention_days INTEGER NOT NULL DEFAULT 90,
	created_at TEXT NOT NULL,
	UNIQUE (workspace_id, name),
	FOREIGN KEY (workspace_id) REFERENCES auth_workspaces(id) ON DELETE CASCADE
);
INSERT INTO meters_workspace_next (id, workspace_id, name, description, unit, aggregation, dimensions, event_retention_days, created_at)
SELECT id, workspace_id, name, description, unit, aggregation, dimensions, event_retention_days, created_at
FROM meters;
DROP TABLE meters;
ALTER TABLE meters_workspace_next RENAME TO meters;

DROP TABLE IF EXISTS usage_events_workspace_next;
CREATE TABLE usage_events_workspace_next (
	id TEXT PRIMARY KEY,
	workspace_id TEXT NOT NULL DEFAULT 'default',
	idempotency_key TEXT,
	subject TEXT NOT NULL,
	meter_name TEXT NOT NULL,
	quantity REAL NOT NULL,
	event_time TEXT NOT NULL,
	received_at TEXT NOT NULL,
	metadata TEXT NOT NULL DEFAULT '{}',
	UNIQUE (workspace_id, idempotency_key),
	FOREIGN KEY (workspace_id) REFERENCES auth_workspaces(id) ON DELETE CASCADE,
	FOREIGN KEY (workspace_id, meter_name) REFERENCES meters(workspace_id, name)
);
INSERT INTO usage_events_workspace_next (id, workspace_id, idempotency_key, subject, meter_name, quantity, event_time, received_at, metadata)
SELECT id, workspace_id, idempotency_key, subject, meter_name, quantity, event_time, received_at, metadata
FROM usage_events;
DROP TABLE usage_events;
ALTER TABLE usage_events_workspace_next RENAME TO usage_events;

DROP TABLE IF EXISTS bulk_usage_ingestions_workspace_next;
CREATE TABLE bulk_usage_ingestions_workspace_next (
	workspace_id TEXT NOT NULL DEFAULT 'default',
	idempotency_key TEXT NOT NULL,
	response TEXT NOT NULL,
	created_at TEXT NOT NULL,
	PRIMARY KEY (workspace_id, idempotency_key),
	FOREIGN KEY (workspace_id) REFERENCES auth_workspaces(id) ON DELETE CASCADE
);
INSERT INTO bulk_usage_ingestions_workspace_next (workspace_id, idempotency_key, response, created_at)
SELECT workspace_id, idempotency_key, response, created_at
FROM bulk_usage_ingestions;
DROP TABLE bulk_usage_ingestions;
ALTER TABLE bulk_usage_ingestions_workspace_next RENAME TO bulk_usage_ingestions;

DROP TABLE IF EXISTS alert_rules_workspace_next;
CREATE TABLE alert_rules_workspace_next (
	id TEXT PRIMARY KEY,
	workspace_id TEXT NOT NULL DEFAULT 'default',
	name TEXT NOT NULL,
	meter_name TEXT NOT NULL,
	enabled INTEGER NOT NULL,
	subject TEXT NOT NULL DEFAULT '',
	metadata TEXT NOT NULL DEFAULT '{}',
	window_seconds INTEGER NOT NULL,
	comparator TEXT NOT NULL,
	threshold REAL NOT NULL,
	evaluation_interval_seconds INTEGER NOT NULL,
	group_by TEXT NOT NULL DEFAULT '',
	destination_id TEXT NOT NULL DEFAULT '',
	next_evaluate_at TEXT NOT NULL,
	created_at TEXT NOT NULL,
	updated_at TEXT NOT NULL,
	FOREIGN KEY (workspace_id) REFERENCES auth_workspaces(id) ON DELETE CASCADE,
	FOREIGN KEY (workspace_id, meter_name) REFERENCES meters(workspace_id, name)
);
INSERT INTO alert_rules_workspace_next (
	id,
	workspace_id,
	name,
	meter_name,
	enabled,
	subject,
	metadata,
	window_seconds,
	comparator,
	threshold,
	evaluation_interval_seconds,
	group_by,
	destination_id,
	next_evaluate_at,
	created_at,
	updated_at
)
SELECT
	id,
	workspace_id,
	name,
	meter_name,
	enabled,
	subject,
	metadata,
	window_seconds,
	comparator,
	threshold,
	evaluation_interval_seconds,
	group_by,
	destination_id,
	next_evaluate_at,
	created_at,
	updated_at
FROM alert_rules;
DROP TABLE alert_rules;
ALTER TABLE alert_rules_workspace_next RENAME TO alert_rules;

PRAGMA foreign_keys=ON;

CREATE INDEX IF NOT EXISTS idx_auth_workspace_memberships_user_id
	ON auth_workspace_memberships (user_id, workspace_id);

CREATE INDEX IF NOT EXISTS idx_auth_sessions_workspace_user
	ON auth_sessions (workspace_id, user_id);

CREATE INDEX IF NOT EXISTS idx_auth_api_keys_workspace_user
	ON auth_api_keys (workspace_id, user_id, created_at DESC, id DESC);

CREATE INDEX IF NOT EXISTS idx_meters_workspace_name
	ON meters (workspace_id, name);

CREATE INDEX IF NOT EXISTS idx_usage_events_workspace_meter_time_id
	ON usage_events (workspace_id, meter_name, event_time DESC, id DESC);

CREATE INDEX IF NOT EXISTS idx_usage_events_workspace_subject_meter_time_id
	ON usage_events (workspace_id, subject, meter_name, event_time DESC, id DESC);

CREATE INDEX IF NOT EXISTS idx_usage_ingestions_workspace_created
	ON usage_ingestions (workspace_id, created_at DESC);

CREATE INDEX IF NOT EXISTS idx_usage_prune_runs_workspace_created
	ON usage_prune_runs (workspace_id, created_at DESC);

CREATE INDEX IF NOT EXISTS idx_usage_export_jobs_workspace_created
	ON usage_export_jobs (workspace_id, created_at DESC, id DESC);

CREATE INDEX IF NOT EXISTS idx_alert_destinations_workspace_created
	ON alert_destinations (workspace_id, created_at DESC, id DESC);

CREATE INDEX IF NOT EXISTS idx_alert_rules_workspace_meter_enabled
	ON alert_rules (workspace_id, meter_name, enabled);

CREATE INDEX IF NOT EXISTS idx_alert_rules_due
	ON alert_rules (enabled, next_evaluate_at);
