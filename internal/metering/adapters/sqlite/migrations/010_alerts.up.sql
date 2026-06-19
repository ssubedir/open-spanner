CREATE TABLE IF NOT EXISTS alert_destinations (
	id TEXT PRIMARY KEY,
	name TEXT NOT NULL,
	type TEXT NOT NULL DEFAULT 'webhook',
	enabled INTEGER NOT NULL DEFAULT 1,
	webhook_url TEXT NOT NULL DEFAULT '',
	webhook_secret TEXT NOT NULL DEFAULT '',
	created_at TEXT NOT NULL,
	updated_at TEXT NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_alert_destinations_type_enabled
	ON alert_destinations (type, enabled);

CREATE TABLE IF NOT EXISTS alert_rules (
	id TEXT PRIMARY KEY,
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
	FOREIGN KEY (meter_name) REFERENCES meters(name)
);

CREATE INDEX IF NOT EXISTS idx_alert_rules_meter_enabled
	ON alert_rules (meter_name, enabled);

CREATE INDEX IF NOT EXISTS idx_alert_rules_due
	ON alert_rules (enabled, next_evaluate_at);

CREATE TABLE IF NOT EXISTS alert_states (
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

CREATE TABLE IF NOT EXISTS alert_events (
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

CREATE INDEX IF NOT EXISTS idx_alert_events_rule_created
	ON alert_events (rule_id, created_at DESC, id DESC);

CREATE TABLE IF NOT EXISTS alert_deliveries (
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

CREATE INDEX IF NOT EXISTS idx_alert_deliveries_event_attempted
	ON alert_deliveries (event_id, attempted_at DESC, id DESC);

CREATE INDEX IF NOT EXISTS idx_alert_deliveries_status_attempted
	ON alert_deliveries (status, attempted_at DESC);

CREATE TABLE IF NOT EXISTS alert_evaluation_jobs (
	rule_id TEXT PRIMARY KEY,
	run_after TEXT NOT NULL,
	locked_until TEXT,
	attempts INTEGER NOT NULL DEFAULT 0,
	created_at TEXT NOT NULL,
	updated_at TEXT NOT NULL,
	FOREIGN KEY (rule_id) REFERENCES alert_rules(id) ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS idx_alert_evaluation_jobs_claim
	ON alert_evaluation_jobs (run_after, locked_until, created_at);
