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

ALTER TABLE alert_rules ADD COLUMN destination_id TEXT NOT NULL DEFAULT '';
