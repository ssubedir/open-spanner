CREATE INDEX IF NOT EXISTS idx_usage_events_subject_meter_time_quantity
	ON usage_events (subject, meter_name, event_time)
	INCLUDE (quantity, metadata);

CREATE INDEX IF NOT EXISTS idx_usage_events_prune_meter_time_id
	ON usage_events (meter_name, event_time ASC, id ASC);
