DROP INDEX IF EXISTS idx_alert_rules_workspace_meter_enabled;
DROP INDEX IF EXISTS idx_alert_destinations_workspace_created;
DROP INDEX IF EXISTS idx_usage_export_jobs_workspace_created;
DROP INDEX IF EXISTS idx_usage_prune_runs_workspace_created;
DROP INDEX IF EXISTS idx_usage_ingestions_workspace_created;
DROP INDEX IF EXISTS idx_usage_events_workspace_subject_meter_time_id;
DROP INDEX IF EXISTS idx_usage_events_workspace_meter_time_id;
DROP INDEX IF EXISTS idx_meters_workspace_name;
DROP INDEX IF EXISTS idx_auth_api_keys_workspace_user;
DROP INDEX IF EXISTS idx_auth_sessions_workspace_user;
DROP INDEX IF EXISTS idx_auth_workspace_memberships_user_id;

ALTER TABLE alert_rules DROP COLUMN workspace_id;
ALTER TABLE alert_destinations DROP COLUMN workspace_id;
ALTER TABLE usage_export_jobs DROP COLUMN workspace_id;
ALTER TABLE usage_ingestions DROP COLUMN workspace_id;
ALTER TABLE usage_prune_runs DROP COLUMN workspace_id;
ALTER TABLE bulk_usage_ingestions DROP COLUMN workspace_id;
ALTER TABLE usage_events DROP COLUMN workspace_id;
ALTER TABLE meters DROP COLUMN workspace_id;
ALTER TABLE auth_api_keys DROP COLUMN workspace_id;
ALTER TABLE auth_sessions DROP COLUMN workspace_id;

DROP TABLE IF EXISTS auth_workspace_memberships;
DROP TABLE IF EXISTS auth_workspaces;
