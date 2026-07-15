DROP TABLE IF EXISTS reconciliation_notification_attempts;
DROP TABLE IF EXISTS reconciliation_notifications;
ALTER TABLE reconciliation_schedules DROP COLUMN IF EXISTS last_failure_fingerprint;
