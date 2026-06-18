ALTER TABLE alert_rules
	DROP COLUMN IF EXISTS webhook_url,
	DROP COLUMN IF EXISTS trigger_type;
