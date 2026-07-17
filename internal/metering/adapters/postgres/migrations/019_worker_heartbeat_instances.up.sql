ALTER TABLE system_worker_heartbeats
	ADD COLUMN instance_id TEXT NOT NULL DEFAULT 'legacy';

DELETE FROM system_worker_heartbeats;

ALTER TABLE system_worker_heartbeats
	DROP CONSTRAINT system_worker_heartbeats_pkey;

ALTER TABLE system_worker_heartbeats
	ADD PRIMARY KEY (worker_name, instance_id);
