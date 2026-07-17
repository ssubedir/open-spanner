DELETE FROM system_worker_heartbeats older
USING system_worker_heartbeats newer
WHERE older.worker_name = newer.worker_name
	AND (older.last_heartbeat_at, older.instance_id) < (newer.last_heartbeat_at, newer.instance_id);

ALTER TABLE system_worker_heartbeats
	DROP CONSTRAINT system_worker_heartbeats_pkey;

ALTER TABLE system_worker_heartbeats
	DROP COLUMN instance_id;

ALTER TABLE system_worker_heartbeats
	ADD PRIMARY KEY (worker_name);
