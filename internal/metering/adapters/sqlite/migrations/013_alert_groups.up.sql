ALTER TABLE alert_rules ADD COLUMN group_by TEXT NOT NULL DEFAULT '';

CREATE TABLE IF NOT EXISTS alert_states_next (
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

INSERT INTO alert_states_next (rule_id, group_key, group_value, status, value, message, evaluated_at, updated_at)
SELECT rule_id, '', '', status, value, message, evaluated_at, updated_at
FROM alert_states;

DROP TABLE alert_states;
ALTER TABLE alert_states_next RENAME TO alert_states;

ALTER TABLE alert_events ADD COLUMN group_key TEXT NOT NULL DEFAULT '';
ALTER TABLE alert_events ADD COLUMN group_value TEXT NOT NULL DEFAULT '';
