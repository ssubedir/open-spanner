CREATE TABLE IF NOT EXISTS meters (
	id TEXT PRIMARY KEY,
	name TEXT NOT NULL UNIQUE,
	description TEXT NOT NULL DEFAULT '',
	unit TEXT NOT NULL,
	aggregation TEXT NOT NULL,
	metadata_schema TEXT NOT NULL DEFAULT '{}',
	event_retention_days INTEGER NOT NULL DEFAULT 90,
	created_at TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS usage_events (
	id TEXT PRIMARY KEY,
	idempotency_key TEXT UNIQUE,
	subject TEXT NOT NULL,
	meter_name TEXT NOT NULL,
	quantity REAL NOT NULL,
	event_time TEXT NOT NULL,
	received_at TEXT NOT NULL,
	metadata TEXT NOT NULL DEFAULT '{}',
	FOREIGN KEY (meter_name) REFERENCES meters(name)
);

CREATE INDEX IF NOT EXISTS idx_usage_events_lookup
	ON usage_events (subject, meter_name, event_time);

CREATE TABLE IF NOT EXISTS bulk_usage_ingestions (
	idempotency_key TEXT PRIMARY KEY,
	response TEXT NOT NULL,
	created_at TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS usage_prune_runs (
	id TEXT PRIMARY KEY,
	dry_run INTEGER NOT NULL,
	deleted INTEGER NOT NULL,
	meters TEXT NOT NULL,
	created_at TEXT NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_usage_prune_runs_created_at
	ON usage_prune_runs (created_at DESC);

CREATE TABLE IF NOT EXISTS usage_ingestions (
	id TEXT PRIMARY KEY,
	kind TEXT NOT NULL,
	accepted INTEGER NOT NULL,
	duplicates INTEGER NOT NULL,
	failed INTEGER NOT NULL,
	created_at TEXT NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_usage_ingestions_created_at
	ON usage_ingestions (created_at DESC);
