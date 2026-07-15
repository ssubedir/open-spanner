CREATE TABLE system_worker_heartbeats (
	worker_name TEXT PRIMARY KEY,
	started_at TIMESTAMPTZ NOT NULL,
	last_heartbeat_at TIMESTAMPTZ NOT NULL
);
