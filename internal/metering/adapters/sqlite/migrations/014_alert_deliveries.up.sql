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
