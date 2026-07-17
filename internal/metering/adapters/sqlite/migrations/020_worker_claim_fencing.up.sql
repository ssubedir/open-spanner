ALTER TABLE reconciliation_schedules
	ADD COLUMN claim_token TEXT;

ALTER TABLE reconciliation_notifications
	ADD COLUMN claim_token TEXT;

CREATE TABLE system_maintenance_leases (
	worker_name TEXT PRIMARY KEY,
	claim_token TEXT NOT NULL,
	locked_until TEXT NOT NULL,
	updated_at TEXT NOT NULL
);
