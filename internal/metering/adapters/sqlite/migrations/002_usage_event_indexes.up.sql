CREATE INDEX IF NOT EXISTS idx_usage_events_event_time_id
	ON usage_events (event_time DESC, id DESC);

CREATE INDEX IF NOT EXISTS idx_usage_events_subject_event_time_id
	ON usage_events (subject, event_time DESC, id DESC);

CREATE INDEX IF NOT EXISTS idx_usage_events_meter_event_time_id
	ON usage_events (meter_name, event_time DESC, id DESC);

CREATE INDEX IF NOT EXISTS idx_usage_events_subject_meter_event_time_id
	ON usage_events (subject, meter_name, event_time DESC, id DESC);
