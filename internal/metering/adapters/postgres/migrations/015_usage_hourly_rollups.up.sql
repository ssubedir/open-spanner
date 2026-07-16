CREATE TABLE usage_hourly_rollups (
	id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
	workspace_id TEXT NOT NULL REFERENCES auth_workspaces(id) ON DELETE CASCADE,
	meter_name TEXT NOT NULL,
	subject TEXT NOT NULL,
	bucket_start TEXT NOT NULL,
	metadata JSONB NOT NULL DEFAULT '{}'::jsonb,
	event_count BIGINT NOT NULL,
	quantity_sum DOUBLE PRECISION NOT NULL,
	quantity_min DOUBLE PRECISION NOT NULL,
	quantity_max DOUBLE PRECISION NOT NULL,
	first_quantity DOUBLE PRECISION NOT NULL,
	first_event_time TEXT NOT NULL,
	first_event_id TEXT NOT NULL,
	last_quantity DOUBLE PRECISION NOT NULL,
	last_event_time TEXT NOT NULL,
	last_event_id TEXT NOT NULL,
	rolled_up_at TEXT NOT NULL,
	UNIQUE (workspace_id, meter_name, subject, bucket_start, metadata)
);

CREATE INDEX idx_usage_hourly_rollups_workspace_meter_bucket
	ON usage_hourly_rollups (workspace_id, meter_name, bucket_start);

CREATE VIEW usage_aggregation_fragments AS
SELECT workspace_id, subject, meter_name, quantity, quantity AS quantity_sum,
	quantity AS quantity_min, quantity AS quantity_max, 1::bigint AS event_count,
	quantity AS first_quantity, event_time AS first_event_time, id AS first_event_id,
	quantity AS last_quantity, event_time AS last_event_time, id AS last_event_id,
	event_time, received_at, idempotency_key, metadata
FROM usage_events
UNION ALL
SELECT workspace_id, subject, meter_name, quantity_sum AS quantity, quantity_sum, quantity_min, quantity_max,
	event_count, first_quantity, first_event_time, first_event_id,
	last_quantity, last_event_time, last_event_id, bucket_start AS event_time,
	NULL::text AS received_at, NULL::text AS idempotency_key, metadata
FROM usage_hourly_rollups;
