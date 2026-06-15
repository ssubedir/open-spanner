DROP INDEX IF EXISTS idx_usage_events_metadata_gin;

ALTER TABLE usage_events
	ALTER COLUMN metadata DROP DEFAULT;

ALTER TABLE usage_events
	ALTER COLUMN metadata TYPE TEXT
	USING metadata::text;

ALTER TABLE usage_events
	ALTER COLUMN metadata SET DEFAULT '{}',
	ALTER COLUMN metadata SET NOT NULL;
