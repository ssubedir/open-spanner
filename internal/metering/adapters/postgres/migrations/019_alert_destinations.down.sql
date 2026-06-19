ALTER TABLE alert_rules
	DROP COLUMN IF EXISTS destination_id;

DROP INDEX IF EXISTS idx_alert_destinations_type_enabled;
DROP TABLE IF EXISTS alert_destinations;
