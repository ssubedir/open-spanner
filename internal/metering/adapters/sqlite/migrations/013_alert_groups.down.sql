CREATE TABLE IF NOT EXISTS alert_rules_next (
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
	trigger_type TEXT NOT NULL DEFAULT 'webhook',
	webhook_url TEXT NOT NULL DEFAULT '',
	next_evaluate_at TEXT NOT NULL,
	created_at TEXT NOT NULL,
	updated_at TEXT NOT NULL,
	FOREIGN KEY (meter_name) REFERENCES meters(name)
);

INSERT INTO alert_rules_next (
	id,
	name,
	meter_name,
	enabled,
	subject,
	metadata,
	window_seconds,
	comparator,
	threshold,
	evaluation_interval_seconds,
	trigger_type,
	webhook_url,
	next_evaluate_at,
	created_at,
	updated_at
)
SELECT
	id,
	name,
	meter_name,
	enabled,
	subject,
	metadata,
	window_seconds,
	comparator,
	threshold,
	evaluation_interval_seconds,
	trigger_type,
	webhook_url,
	next_evaluate_at,
	created_at,
	updated_at
FROM alert_rules;

DROP TABLE alert_rules;
ALTER TABLE alert_rules_next RENAME TO alert_rules;

CREATE INDEX IF NOT EXISTS idx_alert_rules_meter_enabled
	ON alert_rules (meter_name, enabled);

CREATE INDEX IF NOT EXISTS idx_alert_rules_due
	ON alert_rules (enabled, next_evaluate_at);

CREATE TABLE IF NOT EXISTS alert_states_next (
	rule_id TEXT PRIMARY KEY,
	status TEXT NOT NULL,
	value REAL NOT NULL DEFAULT 0,
	message TEXT NOT NULL DEFAULT '',
	evaluated_at TEXT,
	updated_at TEXT NOT NULL,
	FOREIGN KEY (rule_id) REFERENCES alert_rules(id) ON DELETE CASCADE
);

INSERT INTO alert_states_next (rule_id, status, value, message, evaluated_at, updated_at)
SELECT rule_id, status, value, message, evaluated_at, updated_at
FROM alert_states
WHERE group_key = '' AND group_value = '';

DROP TABLE alert_states;
ALTER TABLE alert_states_next RENAME TO alert_states;

CREATE TABLE IF NOT EXISTS alert_events_next (
	id TEXT PRIMARY KEY,
	rule_id TEXT NOT NULL,
	type TEXT NOT NULL,
	value REAL NOT NULL DEFAULT 0,
	message TEXT NOT NULL DEFAULT '',
	created_at TEXT NOT NULL,
	FOREIGN KEY (rule_id) REFERENCES alert_rules(id) ON DELETE CASCADE
);

INSERT INTO alert_events_next (id, rule_id, type, value, message, created_at)
SELECT id, rule_id, type, value, message, created_at
FROM alert_events;

DROP TABLE alert_events;
ALTER TABLE alert_events_next RENAME TO alert_events;

CREATE INDEX IF NOT EXISTS idx_alert_events_rule_created
	ON alert_events (rule_id, created_at DESC, id DESC);
