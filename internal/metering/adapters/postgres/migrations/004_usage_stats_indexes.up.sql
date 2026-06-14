CREATE INDEX IF NOT EXISTS idx_usage_events_meter_stats
	ON usage_events (meter_name)
	INCLUDE (event_time);

CREATE INDEX IF NOT EXISTS idx_usage_events_subject_stats
	ON usage_events (subject)
	INCLUDE (meter_name, event_time);
