DELETE FROM alert_states
WHERE group_key <> '' OR group_value <> '';

ALTER TABLE alert_states DROP CONSTRAINT IF EXISTS alert_states_pkey;
ALTER TABLE alert_states ADD PRIMARY KEY (rule_id);
ALTER TABLE alert_states DROP COLUMN IF EXISTS group_key;
ALTER TABLE alert_states DROP COLUMN IF EXISTS group_value;

ALTER TABLE alert_events DROP COLUMN IF EXISTS group_key;
ALTER TABLE alert_events DROP COLUMN IF EXISTS group_value;

ALTER TABLE alert_rules DROP COLUMN IF EXISTS group_by;
