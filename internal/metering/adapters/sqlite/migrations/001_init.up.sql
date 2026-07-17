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
	version INTEGER NOT NULL DEFAULT 1,
	parent_plan_id TEXT,
	is_current INTEGER NOT NULL DEFAULT 1,
	created_at TEXT NOT NULL,
	updated_at TEXT NOT NULL,
	UNIQUE (workspace_id, name, version),
	FOREIGN KEY (workspace_id) REFERENCES auth_workspaces(id) ON DELETE CASCADE,
	FOREIGN KEY (parent_plan_id) REFERENCES plans(id) ON DELETE SET NULL
);

CREATE INDEX idx_plans_workspace_name
	ON plans (workspace_id, name, is_current);

CREATE UNIQUE INDEX idx_plans_workspace_current_name
	ON plans (workspace_id, name)
	WHERE is_current = 1;

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
	id TEXT PRIMARY KEY,
	workspace_id TEXT NOT NULL,
	subject TEXT NOT NULL,
	plan_id TEXT NOT NULL,
	assigned_at TEXT NOT NULL,
	period_anchor_at TEXT NOT NULL,
	unassigned_at TEXT,
	updated_at TEXT NOT NULL,
	UNIQUE (workspace_id, subject, assigned_at),
	FOREIGN KEY (workspace_id) REFERENCES auth_workspaces(id) ON DELETE CASCADE,
	FOREIGN KEY (plan_id) REFERENCES plans(id) ON DELETE CASCADE
);

CREATE INDEX idx_plan_subject_assignments_workspace_plan
	ON plan_subject_assignments (workspace_id, plan_id, subject, unassigned_at);

CREATE INDEX idx_plan_subject_assignments_subject_window
	ON plan_subject_assignments (workspace_id, subject, assigned_at, unassigned_at);

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

CREATE TABLE entitlement_period_snapshots (
	workspace_id TEXT NOT NULL,
	subject TEXT NOT NULL,
	meter_name TEXT NOT NULL,
	plan_id TEXT NOT NULL,
	plan_name TEXT NOT NULL,
	plan_version INTEGER NOT NULL,
	period TEXT NOT NULL,
	period_start TEXT NOT NULL,
	period_end TEXT NOT NULL,
	state TEXT NOT NULL CHECK (state IN ('ok', 'warning', 'exceeded')),
	current_value REAL NOT NULL,
	limit_value REAL NOT NULL,
	included_value REAL NOT NULL,
	overage_value REAL NOT NULL,
	remaining_value REAL NOT NULL,
	warning_percent REAL NOT NULL,
	event_count INTEGER NOT NULL,
	updated_at TEXT NOT NULL,
	PRIMARY KEY (workspace_id, subject, meter_name, plan_id, period, period_start),
	FOREIGN KEY (workspace_id) REFERENCES auth_workspaces(id) ON DELETE CASCADE,
	FOREIGN KEY (plan_id) REFERENCES plans(id) ON DELETE CASCADE,
	FOREIGN KEY (workspace_id, meter_name) REFERENCES meters(workspace_id, name)
);

CREATE INDEX idx_entitlement_period_snapshots_workspace_period
	ON entitlement_period_snapshots (workspace_id, period_start DESC, subject, meter_name);

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

CREATE TABLE entitlement_usage_counters (
	workspace_id TEXT NOT NULL,
	subject TEXT NOT NULL,
	meter_name TEXT NOT NULL,
	period TEXT NOT NULL,
	period_start TEXT NOT NULL,
	period_end TEXT NOT NULL,
	event_count INTEGER NOT NULL,
	quantity_sum REAL NOT NULL,
	quantity_min REAL NOT NULL,
	quantity_max REAL NOT NULL,
	first_quantity REAL NOT NULL,
	first_event_time TEXT NOT NULL,
	last_quantity REAL NOT NULL,
	last_event_time TEXT NOT NULL,
	updated_at TEXT NOT NULL,
	PRIMARY KEY (workspace_id, subject, meter_name, period, period_start),
	FOREIGN KEY (workspace_id) REFERENCES auth_workspaces(id) ON DELETE CASCADE,
	FOREIGN KEY (workspace_id, meter_name) REFERENCES meters(workspace_id, name)
);

CREATE INDEX idx_entitlement_usage_counters_workspace_meter_period
	ON entitlement_usage_counters (workspace_id, meter_name, period, period_start);

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

ALTER TABLE plan_limits
	ADD COLUMN enforcement TEXT NOT NULL DEFAULT 'advisory'
		CHECK (enforcement IN ('advisory', 'hard'));

ALTER TABLE plan_limits
	ADD COLUMN failure_policy TEXT NOT NULL DEFAULT 'fail_open'
		CHECK (failure_policy IN ('fail_open', 'fail_closed'));

CREATE TABLE consumption_decisions (
	workspace_id TEXT NOT NULL,
	idempotency_key TEXT NOT NULL,
	response TEXT NOT NULL CHECK (json_valid(response)),
	created_at TEXT NOT NULL,
	PRIMARY KEY (workspace_id, idempotency_key)
) STRICT;

CREATE TABLE consumption_decision_prune_runs (
	id TEXT PRIMARY KEY,
	workspace_id TEXT NOT NULL,
	before TEXT NOT NULL,
	dry_run INTEGER NOT NULL CHECK (dry_run IN (0, 1)),
	deleted INTEGER NOT NULL CHECK (deleted >= 0),
	created_at TEXT NOT NULL
) STRICT;

CREATE INDEX idx_consumption_decision_prune_runs_workspace_created
	ON consumption_decision_prune_runs (workspace_id, created_at DESC, id DESC);

ALTER TABLE consumption_decisions ADD COLUMN subject TEXT NOT NULL DEFAULT '';
ALTER TABLE consumption_decisions ADD COLUMN meter_name TEXT NOT NULL DEFAULT '';
ALTER TABLE consumption_decisions ADD COLUMN accepted INTEGER NOT NULL DEFAULT 0 CHECK (accepted IN (0, 1));
ALTER TABLE consumption_decisions ADD COLUMN evaluation_failed INTEGER NOT NULL DEFAULT 0 CHECK (evaluation_failed IN (0, 1));
ALTER TABLE consumption_decisions ADD COLUMN enforcement TEXT NOT NULL DEFAULT '';
ALTER TABLE consumption_decisions ADD COLUMN state TEXT NOT NULL DEFAULT '';

UPDATE consumption_decisions SET
	subject = COALESCE(json_extract(response, '$.Quota.Subject'), ''),
	meter_name = COALESCE(json_extract(response, '$.Quota.MeterName'), ''),
	accepted = COALESCE(json_extract(response, '$.Accepted'), 0),
	evaluation_failed = COALESCE(json_extract(response, '$.EvaluationFailed'), 0),
	enforcement = COALESCE(json_extract(response, '$.Quota.Enforcement'), ''),
	state = COALESCE(json_extract(response, '$.Quota.State'), '');

CREATE INDEX idx_consumption_decisions_workspace_audit
	ON consumption_decisions (workspace_id, created_at DESC, idempotency_key DESC);
CREATE INDEX idx_consumption_decisions_workspace_subject
	ON consumption_decisions (workspace_id, subject, created_at DESC);
CREATE INDEX idx_consumption_decisions_workspace_meter
	ON consumption_decisions (workspace_id, meter_name, created_at DESC);

CREATE TABLE quota_counter_repair_runs (
	id TEXT PRIMARY KEY,
	workspace_id TEXT NOT NULL REFERENCES auth_workspaces(id) ON DELETE CASCADE,
	subject TEXT NOT NULL,
	meter_name TEXT NOT NULL,
	period TEXT NOT NULL,
	period_start TEXT NOT NULL,
	period_end TEXT NOT NULL,
	dry_run INTEGER NOT NULL CHECK (dry_run IN (0, 1)),
	applied INTEGER NOT NULL CHECK (applied IN (0, 1)),
	before_snapshot TEXT NOT NULL,
	after_snapshot TEXT NOT NULL,
	counter_updated_at TEXT NOT NULL,
	created_at TEXT NOT NULL,
	FOREIGN KEY (workspace_id, meter_name) REFERENCES meters(workspace_id, name)
);

CREATE INDEX idx_quota_counter_repair_runs_workspace_created
	ON quota_counter_repair_runs (workspace_id, created_at DESC, id DESC);

CREATE TABLE reconciliation_schedules (
	workspace_id TEXT PRIMARY KEY REFERENCES auth_workspaces(id) ON DELETE CASCADE,
	next_run_at TEXT NOT NULL,
	locked_until TEXT,
	last_fingerprint TEXT NOT NULL DEFAULT '',
	last_notified_fingerprint TEXT NOT NULL DEFAULT '',
	updated_at TEXT NOT NULL
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
	truncated INTEGER NOT NULL,
	lookback_hours INTEGER NOT NULL,
	duration_ms INTEGER NOT NULL,
	fingerprint TEXT NOT NULL DEFAULT '',
	issues TEXT NOT NULL DEFAULT '[]',
	error TEXT NOT NULL DEFAULT '',
	created_at TEXT NOT NULL
);

CREATE INDEX reconciliation_runs_workspace_created_idx
	ON reconciliation_runs (workspace_id, created_at DESC, id DESC);

ALTER TABLE reconciliation_schedules
	ADD COLUMN last_failure_fingerprint TEXT NOT NULL DEFAULT '';

CREATE TABLE reconciliation_notifications (
	id TEXT PRIMARY KEY,
	workspace_id TEXT NOT NULL REFERENCES auth_workspaces(id) ON DELETE CASCADE,
	event_type TEXT NOT NULL CHECK (event_type IN ('drift_detected', 'scan_failed')),
	fingerprint TEXT NOT NULL,
	payload TEXT NOT NULL,
	status TEXT NOT NULL CHECK (status IN ('pending', 'delivered', 'dead_letter')),
	attempts INTEGER NOT NULL DEFAULT 0,
	next_attempt_at TEXT NOT NULL,
	locked_until TEXT,
	last_error TEXT NOT NULL DEFAULT '',
	created_at TEXT NOT NULL,
	delivered_at TEXT
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
	created_at TEXT NOT NULL
);

CREATE INDEX reconciliation_notification_attempts_notification_idx
	ON reconciliation_notification_attempts (notification_id, created_at, id);

CREATE TABLE auth_api_key_events (
	id TEXT PRIMARY KEY,
	workspace_id TEXT NOT NULL,
	user_id TEXT NOT NULL,
	api_key_id TEXT NOT NULL,
	key_name TEXT NOT NULL,
	key_prefix TEXT NOT NULL,
	event_type TEXT NOT NULL CHECK (event_type IN ('created', 'rotated', 'revoked')),
	related_api_key_id TEXT,
	effective_at TEXT,
	created_at TEXT NOT NULL
);

CREATE INDEX idx_auth_api_key_events_workspace_user
	ON auth_api_key_events (workspace_id, user_id, created_at DESC, id DESC);

CREATE TABLE system_worker_heartbeats (
	worker_name TEXT PRIMARY KEY,
	started_at TEXT NOT NULL,
	last_heartbeat_at TEXT NOT NULL
);

CREATE TABLE system_worker_dead_letters (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	public_id TEXT NOT NULL UNIQUE,
	workspace_id TEXT NOT NULL REFERENCES auth_workspaces(id) ON DELETE CASCADE,
	worker_name TEXT NOT NULL CHECK (worker_name IN ('alert', 'entitlement')),
	job_key TEXT NOT NULL,
	rule_id TEXT NOT NULL DEFAULT '',
	subject TEXT NOT NULL DEFAULT '',
	meter_name TEXT NOT NULL DEFAULT '',
	attempts INTEGER NOT NULL,
	last_error TEXT NOT NULL,
	status TEXT NOT NULL CHECK (status IN ('dead_letter', 'requeued')),
	created_at TEXT NOT NULL,
	requeued_at TEXT
);

CREATE INDEX system_worker_dead_letters_workspace_created_idx
	ON system_worker_dead_letters (workspace_id, created_at DESC, id DESC);
CREATE INDEX system_worker_dead_letters_active_idx
	ON system_worker_dead_letters (worker_name, status, created_at DESC);

CREATE TABLE alert_delivery_jobs (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	public_id TEXT NOT NULL UNIQUE,
	workspace_id TEXT NOT NULL REFERENCES auth_workspaces(id) ON DELETE CASCADE,
	event_id TEXT NOT NULL UNIQUE REFERENCES alert_events(id) ON DELETE CASCADE,
	destination_id TEXT NOT NULL DEFAULT '',
	payload TEXT NOT NULL,
	status TEXT NOT NULL CHECK (status IN ('pending', 'running', 'delivered', 'dead_letter')),
	attempts INTEGER NOT NULL DEFAULT 0,
	next_attempt_at TEXT NOT NULL,
	locked_until TEXT,
	last_error TEXT NOT NULL DEFAULT '',
	created_at TEXT NOT NULL,
	updated_at TEXT NOT NULL,
	delivered_at TEXT
);

CREATE INDEX alert_delivery_jobs_claim_idx
	ON alert_delivery_jobs (status, next_attempt_at, locked_until, id);
CREATE INDEX alert_delivery_jobs_workspace_created_idx
	ON alert_delivery_jobs (workspace_id, created_at DESC, id DESC);

ALTER TABLE usage_export_jobs ADD COLUMN claim_token TEXT;

ALTER TABLE usage_export_jobs ADD COLUMN expired_at TEXT;

CREATE INDEX idx_usage_export_jobs_artifact_retention
	ON usage_export_jobs (completed_at, id)
	WHERE status = 'completed' AND expired_at IS NULL;

CREATE TABLE usage_export_cleanup_runs (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	public_id TEXT NOT NULL UNIQUE,
	workspace_id TEXT NOT NULL REFERENCES auth_workspaces(id) ON DELETE CASCADE,
	expired_before TEXT NOT NULL,
	files_deleted INTEGER NOT NULL,
	bytes_reclaimed INTEGER NOT NULL,
	failures INTEGER NOT NULL,
	created_at TEXT NOT NULL
);

CREATE INDEX idx_usage_export_cleanup_runs_workspace_created
	ON usage_export_cleanup_runs (workspace_id, created_at DESC, id DESC);

CREATE TABLE usage_hourly_rollups (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	workspace_id TEXT NOT NULL REFERENCES auth_workspaces(id) ON DELETE CASCADE,
	meter_name TEXT NOT NULL,
	subject TEXT NOT NULL,
	bucket_start TEXT NOT NULL,
	metadata TEXT NOT NULL DEFAULT '{}',
	event_count INTEGER NOT NULL,
	quantity_sum REAL NOT NULL,
	quantity_min REAL NOT NULL,
	quantity_max REAL NOT NULL,
	first_quantity REAL NOT NULL,
	first_event_time TEXT NOT NULL,
	first_event_id TEXT NOT NULL,
	last_quantity REAL NOT NULL,
	last_event_time TEXT NOT NULL,
	last_event_id TEXT NOT NULL,
	rolled_up_at TEXT NOT NULL,
	UNIQUE (workspace_id, meter_name, subject, bucket_start, metadata)
);

CREATE INDEX idx_usage_hourly_rollups_workspace_meter_bucket
	ON usage_hourly_rollups (workspace_id, meter_name, bucket_start);

CREATE VIEW usage_aggregation_fragments AS
SELECT workspace_id, subject, meter_name, quantity, quantity AS quantity_sum,
	quantity AS quantity_min, quantity AS quantity_max, 1 AS event_count,
	quantity AS first_quantity, event_time AS first_event_time, id AS first_event_id,
	quantity AS last_quantity, event_time AS last_event_time, id AS last_event_id,
	event_time, received_at, idempotency_key, metadata
FROM usage_events
UNION ALL
SELECT workspace_id, subject, meter_name, quantity_sum AS quantity, quantity_sum, quantity_min, quantity_max,
	event_count, first_quantity, first_event_time, first_event_id,
	last_quantity, last_event_time, last_event_id, bucket_start AS event_time,
	NULL AS received_at, NULL AS idempotency_key, metadata
FROM usage_hourly_rollups;

CREATE TABLE usage_rollup_runs (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	workspace_id TEXT NOT NULL REFERENCES auth_workspaces(id) ON DELETE CASCADE,
	meter_name TEXT NOT NULL,
	finalized_through TEXT NOT NULL,
	source_events INTEGER NOT NULL,
	rollup_rows INTEGER NOT NULL,
	created_at TEXT NOT NULL,
	UNIQUE (workspace_id, meter_name, finalized_through)
);

CREATE INDEX idx_usage_rollup_runs_workspace_meter_created
	ON usage_rollup_runs (workspace_id, meter_name, created_at DESC, id DESC);

ALTER TABLE workspace_stats ADD COLUMN ingestion_throttled INTEGER NOT NULL DEFAULT 0;

CREATE TABLE ingestion_rate_windows (
	workspace_id TEXT PRIMARY KEY REFERENCES auth_workspaces(id) ON DELETE CASCADE,
	window_start TEXT NOT NULL,
	permitted_events INTEGER NOT NULL DEFAULT 0,
	throttled_events INTEGER NOT NULL DEFAULT 0,
	updated_at TEXT NOT NULL
);

CREATE INDEX idx_usage_ingestions_cleanup ON usage_ingestions (created_at, id);
CREATE INDEX idx_usage_prune_runs_cleanup ON usage_prune_runs (created_at, id);
CREATE INDEX idx_consumption_decision_prune_runs_cleanup ON consumption_decision_prune_runs (created_at, id);
CREATE INDEX idx_usage_export_cleanup_runs_cleanup ON usage_export_cleanup_runs (created_at, id);
CREATE INDEX idx_usage_export_jobs_cleanup ON usage_export_jobs (completed_at, id)
	WHERE status = 'completed' AND expired_at IS NOT NULL;
CREATE INDEX idx_alert_delivery_jobs_cleanup ON alert_delivery_jobs (delivered_at, id)
	WHERE status = 'delivered';
CREATE INDEX idx_reconciliation_runs_cleanup ON reconciliation_runs (created_at, id);
CREATE INDEX idx_reconciliation_notifications_cleanup ON reconciliation_notifications (delivered_at, id)
	WHERE status = 'delivered';
CREATE INDEX idx_usage_rollup_runs_cleanup ON usage_rollup_runs (created_at, id);

CREATE TABLE system_worker_heartbeats_v2 (
	worker_name TEXT NOT NULL,
	instance_id TEXT NOT NULL,
	started_at TEXT NOT NULL,
	last_heartbeat_at TEXT NOT NULL,
	PRIMARY KEY (worker_name, instance_id)
);

DROP TABLE system_worker_heartbeats;
ALTER TABLE system_worker_heartbeats_v2 RENAME TO system_worker_heartbeats;

ALTER TABLE reconciliation_schedules
	ADD COLUMN claim_token TEXT;

ALTER TABLE reconciliation_notifications
	ADD COLUMN claim_token TEXT;

CREATE TABLE system_maintenance_leases (
	worker_name TEXT PRIMARY KEY,
	claim_token TEXT NOT NULL,
	locked_until TEXT NOT NULL,
	updated_at TEXT NOT NULL
);

CREATE TABLE usage_event_outbox (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	public_id TEXT NOT NULL UNIQUE,
	workspace_id TEXT NOT NULL REFERENCES auth_workspaces(id) ON DELETE CASCADE,
	event_id TEXT NOT NULL,
	subject TEXT NOT NULL,
	meter_name TEXT NOT NULL,
	quantity REAL NOT NULL,
	metadata TEXT NOT NULL DEFAULT '{}',
	status TEXT NOT NULL DEFAULT 'pending' CHECK (status IN ('pending', 'running', 'dead_letter')),
	attempts INTEGER NOT NULL DEFAULT 0,
	next_attempt_at TEXT NOT NULL,
	locked_until TEXT,
	claim_token TEXT,
	last_error TEXT NOT NULL DEFAULT '',
	created_at TEXT NOT NULL,
	updated_at TEXT NOT NULL,
	UNIQUE (workspace_id, event_id)
);

CREATE INDEX usage_event_outbox_claim_idx
	ON usage_event_outbox (status, next_attempt_at, locked_until, id);
