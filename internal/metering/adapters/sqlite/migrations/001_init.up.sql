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
	workspace_id TEXT NOT NULL,
	user_id TEXT NOT NULL,
	role TEXT NOT NULL,
	created_at TEXT NOT NULL,
	PRIMARY KEY (workspace_id, user_id),
	FOREIGN KEY (workspace_id) REFERENCES auth_workspaces(id) ON DELETE CASCADE,
	FOREIGN KEY (user_id) REFERENCES auth_users(id) ON DELETE CASCADE
);

CREATE INDEX idx_auth_workspace_memberships_user_id
	ON auth_workspace_memberships (user_id, workspace_id);

CREATE TABLE auth_identities (
	id TEXT PRIMARY KEY,
	user_id TEXT NOT NULL,
	provider TEXT NOT NULL,
	subject TEXT NOT NULL,
	email TEXT NOT NULL,
	email_verified INTEGER NOT NULL DEFAULT 0,
	created_at TEXT NOT NULL,
	updated_at TEXT NOT NULL,
	FOREIGN KEY (user_id) REFERENCES auth_users(id) ON DELETE CASCADE,
	UNIQUE (provider, subject)
);

CREATE INDEX idx_auth_identities_user_id
	ON auth_identities (user_id);

CREATE TABLE auth_sessions (
	id TEXT PRIMARY KEY,
	user_id TEXT NOT NULL,
	workspace_id TEXT NOT NULL,
	token_hash TEXT NOT NULL UNIQUE,
	kind TEXT NOT NULL CHECK (kind IN ('access', 'refresh')),
	expires_at TEXT NOT NULL,
	created_at TEXT NOT NULL,
	FOREIGN KEY (user_id) REFERENCES auth_users(id) ON DELETE CASCADE,
	FOREIGN KEY (workspace_id) REFERENCES auth_workspaces(id) ON DELETE CASCADE
);

CREATE INDEX idx_auth_sessions_user_id
	ON auth_sessions (user_id);

CREATE INDEX idx_auth_sessions_expires_at
	ON auth_sessions (expires_at);

CREATE INDEX idx_auth_sessions_workspace_user
	ON auth_sessions (workspace_id, user_id);

CREATE TABLE auth_api_keys (
	id TEXT PRIMARY KEY,
	user_id TEXT NOT NULL,
	workspace_id TEXT NOT NULL,
	name TEXT NOT NULL,
	token_hash TEXT NOT NULL UNIQUE,
	prefix TEXT NOT NULL,
	scopes TEXT NOT NULL,
	allowed_meters TEXT NOT NULL,
	expires_at TEXT,
	revoked_at TEXT,
	created_at TEXT NOT NULL,
	last_used_at TEXT,
	FOREIGN KEY (user_id) REFERENCES auth_users(id) ON DELETE CASCADE,
	FOREIGN KEY (workspace_id) REFERENCES auth_workspaces(id) ON DELETE CASCADE
);

CREATE INDEX idx_auth_api_keys_user_id
	ON auth_api_keys (user_id);

CREATE INDEX idx_auth_api_keys_workspace_user
	ON auth_api_keys (workspace_id, user_id, created_at DESC, id DESC);

CREATE TABLE meters (
	id TEXT PRIMARY KEY,
	workspace_id TEXT NOT NULL,
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

CREATE INDEX idx_meters_workspace_name
	ON meters (workspace_id, name);

CREATE TABLE plans (
	id TEXT PRIMARY KEY,
	workspace_id TEXT NOT NULL,
	name TEXT NOT NULL,
	description TEXT NOT NULL DEFAULT '',
	created_at TEXT NOT NULL,
	updated_at TEXT NOT NULL,
	UNIQUE (workspace_id, name),
	FOREIGN KEY (workspace_id) REFERENCES auth_workspaces(id) ON DELETE CASCADE
);

CREATE INDEX idx_plans_workspace_name
	ON plans (workspace_id, name);

CREATE TABLE plan_limits (
	id TEXT PRIMARY KEY,
	workspace_id TEXT NOT NULL,
	plan_id TEXT NOT NULL,
	meter_name TEXT NOT NULL,
	period TEXT NOT NULL,
	limit_value REAL NOT NULL,
	warning_percent REAL NOT NULL DEFAULT 80,
	created_at TEXT NOT NULL,
	updated_at TEXT NOT NULL,
	UNIQUE (workspace_id, plan_id, meter_name, period),
	FOREIGN KEY (workspace_id) REFERENCES auth_workspaces(id) ON DELETE CASCADE,
	FOREIGN KEY (plan_id) REFERENCES plans(id) ON DELETE CASCADE,
	FOREIGN KEY (workspace_id, meter_name) REFERENCES meters(workspace_id, name)
);

CREATE INDEX idx_plan_limits_workspace_plan
	ON plan_limits (workspace_id, plan_id);

CREATE TABLE plan_subject_assignments (
	workspace_id TEXT NOT NULL,
	subject TEXT NOT NULL,
	plan_id TEXT NOT NULL,
	assigned_at TEXT NOT NULL,
	updated_at TEXT NOT NULL,
	PRIMARY KEY (workspace_id, subject),
	FOREIGN KEY (workspace_id) REFERENCES auth_workspaces(id) ON DELETE CASCADE,
	FOREIGN KEY (plan_id) REFERENCES plans(id) ON DELETE CASCADE
);

CREATE INDEX idx_plan_subject_assignments_workspace_plan
	ON plan_subject_assignments (workspace_id, plan_id, subject);

CREATE TABLE entitlement_states (
	workspace_id TEXT NOT NULL,
	subject TEXT NOT NULL,
	meter_name TEXT NOT NULL,
	plan_id TEXT NOT NULL,
	plan_name TEXT NOT NULL,
	period TEXT NOT NULL,
	state TEXT NOT NULL CHECK (state IN ('ok', 'warning', 'exceeded')),
	current_value REAL NOT NULL,
	limit_value REAL NOT NULL,
	remaining_value REAL NOT NULL,
	warning_percent REAL NOT NULL,
	message TEXT NOT NULL,
	evaluated_at TEXT NOT NULL,
	updated_at TEXT NOT NULL,
	PRIMARY KEY (workspace_id, subject, meter_name, plan_id, period),
	FOREIGN KEY (workspace_id) REFERENCES auth_workspaces(id) ON DELETE CASCADE,
	FOREIGN KEY (plan_id) REFERENCES plans(id) ON DELETE CASCADE,
	FOREIGN KEY (workspace_id, meter_name) REFERENCES meters(workspace_id, name)
);

CREATE INDEX idx_entitlement_states_workspace_state
	ON entitlement_states (workspace_id, state, updated_at DESC);

CREATE TABLE entitlement_events (
	id TEXT PRIMARY KEY,
	workspace_id TEXT NOT NULL,
	subject TEXT NOT NULL,
	meter_name TEXT NOT NULL,
	plan_id TEXT NOT NULL,
	plan_name TEXT NOT NULL,
	period TEXT NOT NULL,
	previous_state TEXT,
	state TEXT NOT NULL CHECK (state IN ('ok', 'warning', 'exceeded')),
	type TEXT NOT NULL CHECK (type IN ('warning', 'exceeded', 'recovered')),
	current_value REAL NOT NULL,
	limit_value REAL NOT NULL,
	remaining_value REAL NOT NULL,
	warning_percent REAL NOT NULL,
	message TEXT NOT NULL,
	created_at TEXT NOT NULL,
	FOREIGN KEY (workspace_id) REFERENCES auth_workspaces(id) ON DELETE CASCADE,
	FOREIGN KEY (plan_id) REFERENCES plans(id) ON DELETE CASCADE,
	FOREIGN KEY (workspace_id, meter_name) REFERENCES meters(workspace_id, name)
);

CREATE INDEX idx_entitlement_events_workspace_created
	ON entitlement_events (workspace_id, created_at DESC, id DESC);

CREATE INDEX idx_entitlement_events_workspace_subject_meter_created
	ON entitlement_events (workspace_id, subject, meter_name, created_at DESC, id DESC);

CREATE TABLE entitlement_check_jobs (
	workspace_id TEXT NOT NULL,
	subject TEXT NOT NULL,
	meter_name TEXT NOT NULL,
	run_after TEXT NOT NULL,
	locked_until TEXT,
	attempts INTEGER NOT NULL DEFAULT 0,
	created_at TEXT NOT NULL,
	updated_at TEXT NOT NULL,
	PRIMARY KEY (workspace_id, subject, meter_name),
	FOREIGN KEY (workspace_id) REFERENCES auth_workspaces(id) ON DELETE CASCADE,
	FOREIGN KEY (workspace_id, meter_name) REFERENCES meters(workspace_id, name)
);

CREATE INDEX idx_entitlement_check_jobs_claim
	ON entitlement_check_jobs (run_after, locked_until, created_at);

CREATE TABLE usage_events (
	id TEXT PRIMARY KEY,
	workspace_id TEXT NOT NULL,
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

CREATE INDEX idx_usage_events_workspace_event_time_id
	ON usage_events (workspace_id, event_time DESC, id DESC);

CREATE INDEX idx_usage_events_workspace_subject_time_id
	ON usage_events (workspace_id, subject, event_time DESC, id DESC);

CREATE INDEX idx_usage_events_workspace_meter_time_id
	ON usage_events (workspace_id, meter_name, event_time DESC, id DESC);

CREATE INDEX idx_usage_events_workspace_subject_meter_time_id
	ON usage_events (workspace_id, subject, meter_name, event_time DESC, id DESC);

CREATE INDEX idx_usage_events_workspace_prune_meter_time_id
	ON usage_events (workspace_id, meter_name, event_time ASC, id ASC);

CREATE TABLE bulk_usage_ingestions (
	workspace_id TEXT NOT NULL,
	idempotency_key TEXT NOT NULL,
	response TEXT NOT NULL,
	created_at TEXT NOT NULL,
	PRIMARY KEY (workspace_id, idempotency_key),
	FOREIGN KEY (workspace_id) REFERENCES auth_workspaces(id) ON DELETE CASCADE
);

CREATE TABLE usage_prune_runs (
	id TEXT PRIMARY KEY,
	workspace_id TEXT NOT NULL,
	dry_run INTEGER NOT NULL,
	deleted INTEGER NOT NULL,
	meters TEXT NOT NULL,
	created_at TEXT NOT NULL,
	FOREIGN KEY (workspace_id) REFERENCES auth_workspaces(id) ON DELETE CASCADE
);

CREATE INDEX idx_usage_prune_runs_workspace_created
	ON usage_prune_runs (workspace_id, created_at DESC);

CREATE TABLE usage_ingestions (
	id TEXT PRIMARY KEY,
	workspace_id TEXT NOT NULL,
	kind TEXT NOT NULL,
	accepted INTEGER NOT NULL,
	duplicates INTEGER NOT NULL,
	failed INTEGER NOT NULL,
	created_at TEXT NOT NULL,
	FOREIGN KEY (workspace_id) REFERENCES auth_workspaces(id) ON DELETE CASCADE
);

CREATE INDEX idx_usage_ingestions_workspace_created
	ON usage_ingestions (workspace_id, created_at DESC);

CREATE TABLE usage_saved_queries (
	id TEXT PRIMARY KEY,
	user_id TEXT NOT NULL,
	name TEXT NOT NULL,
	query_json TEXT NOT NULL,
	group_by TEXT NOT NULL DEFAULT '[]',
	bucket_size TEXT NOT NULL DEFAULT 'day',
	result_limit INTEGER NOT NULL DEFAULT 500,
	pinned INTEGER NOT NULL DEFAULT 0,
	position INTEGER NOT NULL DEFAULT 0,
	created_at TEXT NOT NULL,
	updated_at TEXT NOT NULL,
	FOREIGN KEY (user_id) REFERENCES auth_users(id) ON DELETE CASCADE,
	UNIQUE (user_id, name)
);

CREATE INDEX idx_usage_saved_queries_user_updated
	ON usage_saved_queries (user_id, updated_at DESC, id DESC);

CREATE INDEX idx_usage_saved_queries_user_pinned_position
	ON usage_saved_queries (user_id, pinned DESC, position ASC, updated_at DESC, id DESC);

CREATE TABLE usage_export_jobs (
	id TEXT PRIMARY KEY,
	workspace_id TEXT NOT NULL,
	kind TEXT NOT NULL,
	status TEXT NOT NULL,
	format TEXT NOT NULL,
	query_json TEXT NOT NULL,
	error TEXT NOT NULL DEFAULT '',
	attempts INTEGER NOT NULL DEFAULT 0,
	locked_until TEXT,
	artifact_path TEXT NOT NULL DEFAULT '',
	artifact_size INTEGER NOT NULL DEFAULT 0,
	created_at TEXT NOT NULL,
	updated_at TEXT NOT NULL,
	completed_at TEXT,
	FOREIGN KEY (workspace_id) REFERENCES auth_workspaces(id) ON DELETE CASCADE
);

CREATE INDEX idx_usage_export_jobs_workspace_created
	ON usage_export_jobs (workspace_id, created_at DESC, id DESC);

CREATE INDEX idx_usage_export_jobs_status
	ON usage_export_jobs (status, created_at DESC, id DESC);

CREATE INDEX idx_usage_export_jobs_claim
	ON usage_export_jobs (status, locked_until, created_at, id);

CREATE TABLE alert_destinations (
	id TEXT PRIMARY KEY,
	workspace_id TEXT NOT NULL,
	name TEXT NOT NULL,
	type TEXT NOT NULL DEFAULT 'webhook',
	enabled INTEGER NOT NULL DEFAULT 1,
	webhook_url TEXT NOT NULL DEFAULT '',
	webhook_secret TEXT NOT NULL DEFAULT '',
	created_at TEXT NOT NULL,
	updated_at TEXT NOT NULL,
	FOREIGN KEY (workspace_id) REFERENCES auth_workspaces(id) ON DELETE CASCADE
);

CREATE INDEX idx_alert_destinations_workspace_created
	ON alert_destinations (workspace_id, created_at DESC, id DESC);

CREATE INDEX idx_alert_destinations_type_enabled
	ON alert_destinations (type, enabled);

CREATE TABLE alert_rules (
	id TEXT PRIMARY KEY,
	workspace_id TEXT NOT NULL,
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

CREATE INDEX idx_alert_rules_workspace_meter_enabled
	ON alert_rules (workspace_id, meter_name, enabled);

CREATE INDEX idx_alert_rules_due
	ON alert_rules (enabled, next_evaluate_at);

CREATE TABLE alert_states (
	rule_id TEXT NOT NULL,
	group_key TEXT NOT NULL DEFAULT '',
	group_value TEXT NOT NULL DEFAULT '',
	status TEXT NOT NULL,
	value REAL NOT NULL DEFAULT 0,
	message TEXT NOT NULL DEFAULT '',
	evaluated_at TEXT,
	updated_at TEXT NOT NULL,
	PRIMARY KEY (rule_id, group_key, group_value),
	FOREIGN KEY (rule_id) REFERENCES alert_rules(id) ON DELETE CASCADE
);

CREATE TABLE alert_events (
	id TEXT PRIMARY KEY,
	rule_id TEXT NOT NULL,
	type TEXT NOT NULL,
	value REAL NOT NULL DEFAULT 0,
	message TEXT NOT NULL DEFAULT '',
	group_key TEXT NOT NULL DEFAULT '',
	group_value TEXT NOT NULL DEFAULT '',
	created_at TEXT NOT NULL,
	FOREIGN KEY (rule_id) REFERENCES alert_rules(id) ON DELETE CASCADE
);

CREATE INDEX idx_alert_events_rule_created
	ON alert_events (rule_id, created_at DESC, id DESC);

CREATE TABLE alert_deliveries (
	id TEXT PRIMARY KEY,
	event_id TEXT NOT NULL,
	trigger_type TEXT NOT NULL,
	status TEXT NOT NULL,
	status_code INTEGER,
	error TEXT NOT NULL DEFAULT '',
	duration_ms INTEGER NOT NULL DEFAULT 0,
	attempted_at TEXT NOT NULL,
	created_at TEXT NOT NULL,
	FOREIGN KEY (event_id) REFERENCES alert_events(id) ON DELETE CASCADE
);

CREATE INDEX idx_alert_deliveries_event_attempted
	ON alert_deliveries (event_id, attempted_at DESC, id DESC);

CREATE INDEX idx_alert_deliveries_status_attempted
	ON alert_deliveries (status, attempted_at DESC);

CREATE TABLE alert_evaluation_jobs (
	rule_id TEXT PRIMARY KEY,
	run_after TEXT NOT NULL,
	locked_until TEXT,
	attempts INTEGER NOT NULL DEFAULT 0,
	created_at TEXT NOT NULL,
	updated_at TEXT NOT NULL,
	FOREIGN KEY (rule_id) REFERENCES alert_rules(id) ON DELETE CASCADE
);

CREATE INDEX idx_alert_evaluation_jobs_claim
	ON alert_evaluation_jobs (run_after, locked_until, created_at);

CREATE TABLE workspace_stats (
	workspace_id TEXT PRIMARY KEY,
	meters INTEGER NOT NULL DEFAULT 0,
	usage_events INTEGER NOT NULL DEFAULT 0,
	prune_runs INTEGER NOT NULL DEFAULT 0,
	updated_at TEXT NOT NULL,
	FOREIGN KEY (workspace_id) REFERENCES auth_workspaces(id) ON DELETE CASCADE
);
