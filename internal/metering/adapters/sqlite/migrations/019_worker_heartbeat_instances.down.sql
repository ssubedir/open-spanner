CREATE TABLE system_worker_heartbeats_v1 (
	worker_name TEXT PRIMARY KEY,
	started_at TEXT NOT NULL,
	last_heartbeat_at TEXT NOT NULL
);

INSERT INTO system_worker_heartbeats_v1 (worker_name, started_at, last_heartbeat_at)
SELECT h.worker_name, h.started_at, h.last_heartbeat_at
FROM system_worker_heartbeats h
WHERE NOT EXISTS (
	SELECT 1
	FROM system_worker_heartbeats newer
	WHERE newer.worker_name = h.worker_name
		AND (newer.last_heartbeat_at > h.last_heartbeat_at
			OR (newer.last_heartbeat_at = h.last_heartbeat_at AND newer.instance_id > h.instance_id))
);

DROP TABLE system_worker_heartbeats;
ALTER TABLE system_worker_heartbeats_v1 RENAME TO system_worker_heartbeats;
