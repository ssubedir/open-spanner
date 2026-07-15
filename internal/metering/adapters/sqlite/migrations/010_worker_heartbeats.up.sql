CREATE TABLE system_worker_heartbeats (
	worker_name TEXT PRIMARY KEY,
	started_at TEXT NOT NULL,
	last_heartbeat_at TEXT NOT NULL
);
