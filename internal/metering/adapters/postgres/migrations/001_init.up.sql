CREATE TABLE auth_workspaces (
	id TEXT PRIMARY KEY,
	name TEXT NOT NULL,
	created_at TEXT NOT NULL
);

CREATE TABLE auth_users (
	id TEXT PRIMARY KEY,
	email TEXT NOT NULL UNIQUE,
	password_hash TEXT NOT NULL,
	created_at TEXT NOT NULL
);

CREATE TABLE auth_workspace_memberships (
	workspace_id TEXT NOT NULL REFERENCES auth_workspaces(id) ON DELETE CASCADE,
	user_id TEXT NOT NULL REFERENCES auth_users(id) ON DELETE CASCADE,
	role TEXT NOT NULL,
	created_at TEXT NOT NULL,
	PRIMARY KEY (workspace_id, user_id)
);

CREATE INDEX idx_auth_workspace_memberships_user_id
	ON auth_workspace_memberships (user_id, workspace_id);

CREATE TABLE auth_identities (
	id TEXT PRIMARY KEY,
	user_id TEXT NOT NULL REFERENCES auth_users(id) ON DELETE CASCADE,
	provider TEXT NOT NULL,
	subject TEXT NOT NULL,
	email TEXT NOT NULL,
	email_verified BOOLEAN NOT NULL DEFAULT false,
	created_at TEXT NOT NULL,
	updated_at TEXT NOT NULL,
	UNIQUE (provider, subject)
);

CREATE INDEX idx_auth_identities_user_id
	ON auth_identities (user_id);

CREATE TABLE auth_sessions (
	id TEXT PRIMARY KEY,
	user_id TEXT NOT NULL REFERENCES auth_users(id) ON DELETE CASCADE,
	workspace_id TEXT NOT NULL REFERENCES auth_workspaces(id) ON DELETE CASCADE,
	token_hash TEXT NOT NULL UNIQUE,
	kind TEXT NOT NULL CHECK (kind IN ('access', 'refresh')),
	expires_at TEXT NOT NULL,
	created_at TEXT NOT NULL
);

CREATE INDEX idx_auth_sessions_user_id
	ON auth_sessions (user_id);

CREATE INDEX idx_auth_sessions_expires_at
	ON auth_sessions (expires_at);

CREATE INDEX idx_auth_sessions_workspace_user
	ON auth_sessions (workspace_id, user_id);

CREATE TABLE auth_api_keys (
	id TEXT PRIMARY KEY,
	user_id TEXT NOT NULL REFERENCES auth_users(id) ON DELETE CASCADE,
	workspace_id TEXT NOT NULL REFERENCES auth_workspaces(id) ON DELETE CASCADE,
	name TEXT NOT NULL,
	token_hash TEXT NOT NULL UNIQUE,
	prefix TEXT NOT NULL,
	scopes TEXT NOT NULL,
	allowed_meters TEXT NOT NULL,
	expires_at TEXT,
	revoked_at TEXT,
	created_at TEXT NOT NULL,
	last_used_at TEXT
);

CREATE INDEX idx_auth_api_keys_user_id
	ON auth_api_keys (user_id);

CREATE INDEX idx_auth_api_keys_workspace_user
	ON auth_api_keys (workspace_id, user_id, created_at DESC, id DESC);

CREATE TABLE meters (
	id TEXT PRIMARY KEY,
	workspace_id TEXT NOT NULL REFERENCES auth_workspaces(id) ON DELETE CASCADE,
	name TEXT NOT NULL,
	description TEXT NOT NULL DEFAULT '',
	unit TEXT NOT NULL,
	aggregation TEXT NOT NULL,
	dimensions TEXT NOT NULL DEFAULT '[]',
	event_retention_days INTEGER NOT NULL DEFAULT 90,
	created_at TEXT NOT NULL,
	UNIQUE (workspace_id, name)
);

CREATE INDEX idx_meters_workspace_name
	ON meters (workspace_id, name);

CREATE TABLE plans (
	id TEXT PRIMARY KEY,
	workspace_id TEXT NOT NULL REFERENCES auth_workspaces(id) ON DELETE CASCADE,
	name TEXT NOT NULL,
	description TEXT NOT NULL DEFAULT '',
	version INTEGER NOT NULL DEFAULT 1,
	parent_plan_id TEXT REFERENCES plans(id) ON DELETE SET NULL,
	is_current BOOLEAN NOT NULL DEFAULT TRUE,
	created_at TEXT NOT NULL,
	updated_at TEXT NOT NULL,
	UNIQUE (workspace_id, name, version)
);

CREATE INDEX idx_plans_workspace_name
	ON plans (workspace_id, name, is_current);

CREATE UNIQUE INDEX idx_plans_workspace_current_name
	ON plans (workspace_id, name)
	WHERE is_current;

CREATE TABLE plan_limits (
	id TEXT PRIMARY KEY,
	workspace_id TEXT NOT NULL REFERENCES auth_workspaces(id) ON DELETE CASCADE,
	plan_id TEXT NOT NULL REFERENCES plans(id) ON DELETE CASCADE,
	meter_name TEXT NOT NULL,
	period TEXT NOT NULL,
	limit_value DOUBLE PRECISION NOT NULL,
	warning_percent DOUBLE PRECISION NOT NULL DEFAULT 80,
	created_at TEXT NOT NULL,
	updated_at TEXT NOT NULL,
	UNIQUE (workspace_id, plan_id, meter_name, period),
	FOREIGN KEY (workspace_id, meter_name) REFERENCES meters(workspace_id, name)
);

CREATE INDEX idx_plan_limits_workspace_plan
	ON plan_limits (workspace_id, plan_id);

CREATE TABLE plan_subject_assignments (
	id TEXT PRIMARY KEY,
	workspace_id TEXT NOT NULL REFERENCES auth_workspaces(id) ON DELETE CASCADE,
	subject TEXT NOT NULL,
	plan_id TEXT NOT NULL REFERENCES plans(id) ON DELETE CASCADE,
	assigned_at TEXT NOT NULL,
	unassigned_at TEXT,
	updated_at TEXT NOT NULL,
	UNIQUE (workspace_id, subject, assigned_at)
);

CREATE UNIQUE INDEX idx_plan_subject_assignments_active_subject
	ON plan_subject_assignments (workspace_id, subject)
	WHERE unassigned_at IS NULL;

CREATE INDEX idx_plan_subject_assignments_workspace_plan
	ON plan_subject_assignments (workspace_id, plan_id, subject, unassigned_at);

CREATE TABLE entitlement_states (
	workspace_id TEXT NOT NULL REFERENCES auth_workspaces(id) ON DELETE CASCADE,
	subject TEXT NOT NULL,
	meter_name TEXT NOT NULL,
	plan_id TEXT NOT NULL REFERENCES plans(id) ON DELETE CASCADE,
	plan_name TEXT NOT NULL,
	period TEXT NOT NULL,
	state TEXT NOT NULL CHECK (state IN ('ok', 'warning', 'exceeded')),
	current_value DOUBLE PRECISION NOT NULL,
	limit_value DOUBLE PRECISION NOT NULL,
	remaining_value DOUBLE PRECISION NOT NULL,
	warning_percent DOUBLE PRECISION NOT NULL,
	message TEXT NOT NULL,
	evaluated_at TEXT NOT NULL,
	updated_at TEXT NOT NULL,
	PRIMARY KEY (workspace_id, subject, meter_name, plan_id, period),
	FOREIGN KEY (workspace_id, meter_name) REFERENCES meters(workspace_id, name)
);

CREATE INDEX idx_entitlement_states_workspace_state
	ON entitlement_states (workspace_id, state, updated_at DESC);

CREATE TABLE entitlement_events (
	id TEXT PRIMARY KEY,
	workspace_id TEXT NOT NULL REFERENCES auth_workspaces(id) ON DELETE CASCADE,
	subject TEXT NOT NULL,
	meter_name TEXT NOT NULL,
	plan_id TEXT NOT NULL REFERENCES plans(id) ON DELETE CASCADE,
	plan_name TEXT NOT NULL,
	period TEXT NOT NULL,
	previous_state TEXT,
	state TEXT NOT NULL CHECK (state IN ('ok', 'warning', 'exceeded')),
	type TEXT NOT NULL CHECK (type IN ('warning', 'exceeded', 'recovered')),
	current_value DOUBLE PRECISION NOT NULL,
	limit_value DOUBLE PRECISION NOT NULL,
	remaining_value DOUBLE PRECISION NOT NULL,
	warning_percent DOUBLE PRECISION NOT NULL,
	message TEXT NOT NULL,
	created_at TEXT NOT NULL,
	FOREIGN KEY (workspace_id, meter_name) REFERENCES meters(workspace_id, name)
);

CREATE INDEX idx_entitlement_events_workspace_created
	ON entitlement_events (workspace_id, created_at DESC, id DESC);

CREATE INDEX idx_entitlement_events_workspace_subject_meter_created
	ON entitlement_events (workspace_id, subject, meter_name, created_at DESC, id DESC);

CREATE TABLE entitlement_check_jobs (
	workspace_id TEXT NOT NULL REFERENCES auth_workspaces(id) ON DELETE CASCADE,
	subject TEXT NOT NULL,
	meter_name TEXT NOT NULL,
	run_after TEXT NOT NULL,
	locked_until TEXT,
	attempts INTEGER NOT NULL DEFAULT 0,
	created_at TEXT NOT NULL,
	updated_at TEXT NOT NULL,
	PRIMARY KEY (workspace_id, subject, meter_name),
	FOREIGN KEY (workspace_id, meter_name) REFERENCES meters(workspace_id, name)
);

CREATE INDEX idx_entitlement_check_jobs_claim
	ON entitlement_check_jobs (run_after, locked_until, created_at);

CREATE TABLE entitlement_usage_counters (
	workspace_id TEXT NOT NULL REFERENCES auth_workspaces(id) ON DELETE CASCADE,
	subject TEXT NOT NULL,
	meter_name TEXT NOT NULL,
	period TEXT NOT NULL,
	period_start TEXT NOT NULL,
	period_end TEXT NOT NULL,
	event_count BIGINT NOT NULL,
	quantity_sum DOUBLE PRECISION NOT NULL,
	quantity_min DOUBLE PRECISION NOT NULL,
	quantity_max DOUBLE PRECISION NOT NULL,
	first_quantity DOUBLE PRECISION NOT NULL,
	first_event_time TEXT NOT NULL,
	last_quantity DOUBLE PRECISION NOT NULL,
	last_event_time TEXT NOT NULL,
	updated_at TEXT NOT NULL,
	PRIMARY KEY (workspace_id, subject, meter_name, period, period_start),
	FOREIGN KEY (workspace_id, meter_name) REFERENCES meters(workspace_id, name)
);

CREATE INDEX idx_entitlement_usage_counters_workspace_meter_period
	ON entitlement_usage_counters (workspace_id, meter_name, period, period_start);

CREATE TABLE usage_events (
	id TEXT PRIMARY KEY,
	workspace_id TEXT NOT NULL REFERENCES auth_workspaces(id) ON DELETE CASCADE,
	idempotency_key TEXT,
	subject TEXT NOT NULL,
	meter_name TEXT NOT NULL,
	quantity DOUBLE PRECISION NOT NULL,
	event_time TEXT NOT NULL,
	received_at TEXT NOT NULL,
	metadata JSONB NOT NULL DEFAULT '{}'::jsonb,
	FOREIGN KEY (workspace_id, meter_name) REFERENCES meters(workspace_id, name)
);

CREATE UNIQUE INDEX idx_usage_events_workspace_idempotency_key
	ON usage_events (workspace_id, idempotency_key)
	WHERE idempotency_key IS NOT NULL;

CREATE INDEX idx_usage_events_workspace_event_time_id
	ON usage_events (workspace_id, event_time DESC, id DESC);

CREATE INDEX idx_usage_events_workspace_subject_time_id
	ON usage_events (workspace_id, subject, event_time DESC, id DESC);

CREATE INDEX idx_usage_events_workspace_meter_time_id
	ON usage_events (workspace_id, meter_name, event_time DESC, id DESC);

CREATE INDEX idx_usage_events_workspace_subject_meter_time_id
	ON usage_events (workspace_id, subject, meter_name, event_time DESC, id DESC);

CREATE INDEX idx_usage_events_workspace_subject_meter_time_quantity
	ON usage_events (workspace_id, subject, meter_name, event_time)
	INCLUDE (quantity, metadata);

CREATE INDEX idx_usage_events_workspace_prune_meter_time_id
	ON usage_events (workspace_id, meter_name, event_time ASC, id ASC);

CREATE INDEX idx_usage_events_metadata_gin
	ON usage_events USING GIN (metadata);

CREATE TABLE bulk_usage_ingestions (
	workspace_id TEXT NOT NULL REFERENCES auth_workspaces(id) ON DELETE CASCADE,
	idempotency_key TEXT NOT NULL,
	response TEXT NOT NULL,
	created_at TEXT NOT NULL,
	PRIMARY KEY (workspace_id, idempotency_key)
);

CREATE TABLE usage_prune_runs (
	id TEXT PRIMARY KEY,
	workspace_id TEXT NOT NULL REFERENCES auth_workspaces(id) ON DELETE CASCADE,
	dry_run INTEGER NOT NULL,
	deleted INTEGER NOT NULL,
	meters TEXT NOT NULL,
	created_at TEXT NOT NULL
);

CREATE INDEX idx_usage_prune_runs_workspace_created
	ON usage_prune_runs (workspace_id, created_at DESC);

CREATE TABLE usage_ingestions (
	id TEXT PRIMARY KEY,
	workspace_id TEXT NOT NULL REFERENCES auth_workspaces(id) ON DELETE CASCADE,
	kind TEXT NOT NULL,
	accepted INTEGER NOT NULL,
	duplicates INTEGER NOT NULL,
	failed INTEGER NOT NULL,
	created_at TEXT NOT NULL
);

CREATE INDEX idx_usage_ingestions_workspace_created
	ON usage_ingestions (workspace_id, created_at DESC);

CREATE TABLE usage_saved_queries (
	id TEXT PRIMARY KEY,
	user_id TEXT NOT NULL REFERENCES auth_users(id) ON DELETE CASCADE,
	name TEXT NOT NULL,
	query_json JSONB NOT NULL,
	group_by JSONB NOT NULL DEFAULT '[]'::jsonb,
	bucket_size TEXT NOT NULL DEFAULT 'day',
	result_limit INTEGER NOT NULL DEFAULT 500,
	pinned BOOLEAN NOT NULL DEFAULT false,
	position INTEGER NOT NULL DEFAULT 0,
	created_at TEXT NOT NULL,
	updated_at TEXT NOT NULL,
	UNIQUE (user_id, name)
);

CREATE INDEX idx_usage_saved_queries_user_updated
	ON usage_saved_queries (user_id, updated_at DESC, id DESC);

CREATE INDEX idx_usage_saved_queries_user_pinned_position
	ON usage_saved_queries (user_id, pinned DESC, position ASC, updated_at DESC, id DESC);

CREATE TABLE usage_export_jobs (
	id TEXT PRIMARY KEY,
	workspace_id TEXT NOT NULL REFERENCES auth_workspaces(id) ON DELETE CASCADE,
	kind TEXT NOT NULL,
	status TEXT NOT NULL,
	format TEXT NOT NULL,
	query_json TEXT NOT NULL,
	error TEXT NOT NULL DEFAULT '',
	attempts INTEGER NOT NULL DEFAULT 0,
	locked_until TEXT,
	artifact_path TEXT NOT NULL DEFAULT '',
	artifact_size BIGINT NOT NULL DEFAULT 0,
	created_at TEXT NOT NULL,
	updated_at TEXT NOT NULL,
	completed_at TEXT
);

CREATE INDEX idx_usage_export_jobs_workspace_created
	ON usage_export_jobs (workspace_id, created_at DESC, id DESC);

CREATE INDEX idx_usage_export_jobs_status
	ON usage_export_jobs (status, created_at DESC, id DESC);

CREATE INDEX idx_usage_export_jobs_claim
	ON usage_export_jobs (status, locked_until, created_at, id);

CREATE TABLE alert_destinations (
	id TEXT PRIMARY KEY,
	workspace_id TEXT NOT NULL REFERENCES auth_workspaces(id) ON DELETE CASCADE,
	name TEXT NOT NULL,
	type TEXT NOT NULL DEFAULT 'webhook',
	enabled BOOLEAN NOT NULL DEFAULT true,
	webhook_url TEXT NOT NULL DEFAULT '',
	webhook_secret TEXT NOT NULL DEFAULT '',
	created_at TEXT NOT NULL,
	updated_at TEXT NOT NULL
);

CREATE INDEX idx_alert_destinations_workspace_created
	ON alert_destinations (workspace_id, created_at DESC, id DESC);

CREATE INDEX idx_alert_destinations_type_enabled
	ON alert_destinations (type, enabled);

CREATE TABLE alert_rules (
	id TEXT PRIMARY KEY,
	workspace_id TEXT NOT NULL REFERENCES auth_workspaces(id) ON DELETE CASCADE,
	name TEXT NOT NULL,
	meter_name TEXT NOT NULL,
	enabled BOOLEAN NOT NULL,
	subject TEXT NOT NULL DEFAULT '',
	metadata JSONB NOT NULL DEFAULT '{}'::jsonb,
	window_seconds INTEGER NOT NULL,
	comparator TEXT NOT NULL,
	threshold DOUBLE PRECISION NOT NULL,
	evaluation_interval_seconds INTEGER NOT NULL,
	group_by TEXT NOT NULL DEFAULT '',
	destination_id TEXT NOT NULL DEFAULT '',
	next_evaluate_at TEXT NOT NULL,
	created_at TEXT NOT NULL,
	updated_at TEXT NOT NULL,
	FOREIGN KEY (workspace_id, meter_name) REFERENCES meters(workspace_id, name)
);

CREATE INDEX idx_alert_rules_workspace_meter_enabled
	ON alert_rules (workspace_id, meter_name, enabled);

CREATE INDEX idx_alert_rules_due
	ON alert_rules (enabled, next_evaluate_at);

CREATE TABLE alert_states (
	rule_id TEXT NOT NULL REFERENCES alert_rules(id) ON DELETE CASCADE,
	group_key TEXT NOT NULL DEFAULT '',
	group_value TEXT NOT NULL DEFAULT '',
	status TEXT NOT NULL,
	value DOUBLE PRECISION NOT NULL DEFAULT 0,
	message TEXT NOT NULL DEFAULT '',
	evaluated_at TEXT,
	updated_at TEXT NOT NULL,
	PRIMARY KEY (rule_id, group_key, group_value)
);

CREATE TABLE alert_events (
	id TEXT PRIMARY KEY,
	rule_id TEXT NOT NULL REFERENCES alert_rules(id) ON DELETE CASCADE,
	type TEXT NOT NULL,
	value DOUBLE PRECISION NOT NULL DEFAULT 0,
	message TEXT NOT NULL DEFAULT '',
	group_key TEXT NOT NULL DEFAULT '',
	group_value TEXT NOT NULL DEFAULT '',
	created_at TEXT NOT NULL
);

CREATE INDEX idx_alert_events_rule_created
	ON alert_events (rule_id, created_at DESC, id DESC);

CREATE TABLE alert_deliveries (
	id TEXT PRIMARY KEY,
	event_id TEXT NOT NULL REFERENCES alert_events(id) ON DELETE CASCADE,
	trigger_type TEXT NOT NULL,
	status TEXT NOT NULL,
	status_code INTEGER,
	error TEXT NOT NULL DEFAULT '',
	duration_ms INTEGER NOT NULL DEFAULT 0,
	attempted_at TEXT NOT NULL,
	created_at TEXT NOT NULL
);

CREATE INDEX idx_alert_deliveries_event_attempted
	ON alert_deliveries (event_id, attempted_at DESC, id DESC);

CREATE INDEX idx_alert_deliveries_status_attempted
	ON alert_deliveries (status, attempted_at DESC);

CREATE TABLE alert_evaluation_jobs (
	rule_id TEXT PRIMARY KEY REFERENCES alert_rules(id) ON DELETE CASCADE,
	run_after TEXT NOT NULL,
	locked_until TEXT,
	attempts INTEGER NOT NULL DEFAULT 0,
	created_at TEXT NOT NULL,
	updated_at TEXT NOT NULL
);

CREATE INDEX idx_alert_evaluation_jobs_claim
	ON alert_evaluation_jobs (run_after, locked_until, created_at);

CREATE TABLE workspace_stats (
	workspace_id TEXT PRIMARY KEY REFERENCES auth_workspaces(id) ON DELETE CASCADE,
	meters BIGINT NOT NULL DEFAULT 0,
	usage_events BIGINT NOT NULL DEFAULT 0,
	prune_runs BIGINT NOT NULL DEFAULT 0,
	updated_at TEXT NOT NULL
);
