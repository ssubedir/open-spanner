DROP TABLE IF EXISTS reconciliation_notification_attempts;
DROP TABLE IF EXISTS reconciliation_notifications;
ALTER TABLE reconciliation_schedules DROP COLUMN last_failure_fingerprint;
