DROP TABLE IF EXISTS system_maintenance_leases;
ALTER TABLE reconciliation_schedules DROP COLUMN IF EXISTS claim_token;
ALTER TABLE reconciliation_notifications DROP COLUMN IF EXISTS claim_token;
