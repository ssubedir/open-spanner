ALTER TABLE alert_rules ADD COLUMN trigger_type TEXT NOT NULL DEFAULT 'webhook';
ALTER TABLE alert_rules ADD COLUMN webhook_url TEXT NOT NULL DEFAULT '';
