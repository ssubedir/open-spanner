CREATE TABLE system_worker_heartbeats_v2 (
	worker_name TEXT NOT NULL,
	instance_id TEXT NOT NULL,
	started_at TEXT NOT NULL,
	last_heartbeat_at TEXT NOT NULL,
	PRIMARY KEY (worker_name, instance_id)
);

DROP TABLE system_worker_heartbeats;
ALTER TABLE system_worker_heartbeats_v2 RENAME TO system_worker_heartbeats;
