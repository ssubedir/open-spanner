ALTER TABLE usage_events
	ALTER COLUMN metadata DROP DEFAULT;

ALTER TABLE usage_events
	ALTER COLUMN metadata TYPE JSONB
	USING metadata::jsonb;

ALTER TABLE usage_events
	ALTER COLUMN metadata SET DEFAULT '{}'::jsonb,
	ALTER COLUMN metadata SET NOT NULL;

CREATE INDEX IF NOT EXISTS idx_usage_events_metadata_gin
	ON usage_events USING GIN (metadata);
