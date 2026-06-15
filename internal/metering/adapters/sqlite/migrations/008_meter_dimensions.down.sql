CREATE TABLE meters_next (
	id TEXT PRIMARY KEY,
	name TEXT NOT NULL UNIQUE,
	description TEXT NOT NULL DEFAULT '',
	unit TEXT NOT NULL,
	aggregation TEXT NOT NULL,
	metadata_schema TEXT NOT NULL DEFAULT '{}',
	event_retention_days INTEGER NOT NULL DEFAULT 90,
	created_at TEXT NOT NULL
);

INSERT INTO meters_next (id, name, description, unit, aggregation, metadata_schema, event_retention_days, created_at)
SELECT id, name, description, unit, aggregation, metadata_schema, event_retention_days, created_at
FROM meters;

DROP TABLE meters;
ALTER TABLE meters_next RENAME TO meters;
