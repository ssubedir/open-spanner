CREATE TABLE IF NOT EXISTS auth_workspaces (
	id TEXT PRIMARY KEY,
	name TEXT NOT NULL,
	created_at TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS auth_workspace_memberships (
	workspace_id TEXT NOT NULL REFERENCES auth_workspaces(id) ON DELETE CASCADE,
	user_id TEXT NOT NULL REFERENCES auth_users(id) ON DELETE CASCADE,
	role TEXT NOT NULL,
	created_at TEXT NOT NULL,
	PRIMARY KEY (workspace_id, user_id)
);

INSERT INTO auth_workspaces (id, name, created_at)
VALUES ('default', 'Default workspace', '1970-01-01T00:00:00Z')
ON CONFLICT (id) DO NOTHING;

ALTER TABLE auth_sessions
	ADD COLUMN IF NOT EXISTS workspace_id TEXT NOT NULL DEFAULT 'default' REFERENCES auth_workspaces(id) ON DELETE CASCADE;

ALTER TABLE auth_api_keys
	ADD COLUMN IF NOT EXISTS workspace_id TEXT NOT NULL DEFAULT 'default' REFERENCES auth_workspaces(id) ON DELETE CASCADE;

ALTER TABLE meters
	ADD COLUMN IF NOT EXISTS workspace_id TEXT NOT NULL DEFAULT 'default' REFERENCES auth_workspaces(id) ON DELETE CASCADE;

ALTER TABLE usage_events
	ADD COLUMN IF NOT EXISTS workspace_id TEXT NOT NULL DEFAULT 'default' REFERENCES auth_workspaces(id) ON DELETE CASCADE;

ALTER TABLE bulk_usage_ingestions
	ADD COLUMN IF NOT EXISTS workspace_id TEXT NOT NULL DEFAULT 'default' REFERENCES auth_workspaces(id) ON DELETE CASCADE;

ALTER TABLE usage_prune_runs
	ADD COLUMN IF NOT EXISTS workspace_id TEXT NOT NULL DEFAULT 'default' REFERENCES auth_workspaces(id) ON DELETE CASCADE;

ALTER TABLE usage_ingestions
	ADD COLUMN IF NOT EXISTS workspace_id TEXT NOT NULL DEFAULT 'default' REFERENCES auth_workspaces(id) ON DELETE CASCADE;

ALTER TABLE usage_export_jobs
	ADD COLUMN IF NOT EXISTS workspace_id TEXT NOT NULL DEFAULT 'default' REFERENCES auth_workspaces(id) ON DELETE CASCADE;

ALTER TABLE alert_destinations
	ADD COLUMN IF NOT EXISTS workspace_id TEXT NOT NULL DEFAULT 'default' REFERENCES auth_workspaces(id) ON DELETE CASCADE;

ALTER TABLE alert_rules
	ADD COLUMN IF NOT EXISTS workspace_id TEXT NOT NULL DEFAULT 'default' REFERENCES auth_workspaces(id) ON DELETE CASCADE;

ALTER TABLE usage_events DROP CONSTRAINT IF EXISTS usage_events_meter_name_fkey;
ALTER TABLE alert_rules DROP CONSTRAINT IF EXISTS alert_rules_meter_name_fkey;
ALTER TABLE meters DROP CONSTRAINT IF EXISTS meters_name_key;
ALTER TABLE usage_events DROP CONSTRAINT IF EXISTS usage_events_idempotency_key_key;
ALTER TABLE bulk_usage_ingestions DROP CONSTRAINT IF EXISTS bulk_usage_ingestions_pkey;
ALTER TABLE bulk_usage_ingestions ADD PRIMARY KEY (workspace_id, idempotency_key);

CREATE INDEX IF NOT EXISTS idx_auth_workspace_memberships_user_id
	ON auth_workspace_memberships (user_id, workspace_id);

CREATE INDEX IF NOT EXISTS idx_auth_sessions_workspace_user
	ON auth_sessions (workspace_id, user_id);

CREATE INDEX IF NOT EXISTS idx_auth_api_keys_workspace_user
	ON auth_api_keys (workspace_id, user_id, created_at DESC, id DESC);

CREATE UNIQUE INDEX IF NOT EXISTS idx_meters_workspace_name
	ON meters (workspace_id, name);

CREATE UNIQUE INDEX IF NOT EXISTS idx_usage_events_workspace_idempotency_key
	ON usage_events (workspace_id, idempotency_key)
	WHERE idempotency_key IS NOT NULL;

ALTER TABLE usage_events
	ADD CONSTRAINT usage_events_workspace_meter_name_fkey
	FOREIGN KEY (workspace_id, meter_name) REFERENCES meters(workspace_id, name);

ALTER TABLE alert_rules
	ADD CONSTRAINT alert_rules_workspace_meter_name_fkey
	FOREIGN KEY (workspace_id, meter_name) REFERENCES meters(workspace_id, name);

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
