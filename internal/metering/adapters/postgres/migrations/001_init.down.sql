DROP TABLE IF EXISTS usage_event_outbox;

DROP TABLE IF EXISTS system_maintenance_leases;
ALTER TABLE reconciliation_schedules DROP COLUMN IF EXISTS claim_token;
ALTER TABLE reconciliation_notifications DROP COLUMN IF EXISTS claim_token;

DELETE FROM system_worker_heartbeats older
USING system_worker_heartbeats newer
WHERE older.worker_name = newer.worker_name
	AND (older.last_heartbeat_at, older.instance_id) < (newer.last_heartbeat_at, newer.instance_id);

ALTER TABLE system_worker_heartbeats
	DROP CONSTRAINT system_worker_heartbeats_pkey;

ALTER TABLE system_worker_heartbeats
	DROP COLUMN instance_id;

ALTER TABLE system_worker_heartbeats
	ADD PRIMARY KEY (worker_name);

DROP INDEX IF EXISTS idx_usage_rollup_runs_cleanup;
DROP INDEX IF EXISTS idx_reconciliation_notifications_cleanup;
DROP INDEX IF EXISTS idx_reconciliation_runs_cleanup;
DROP INDEX IF EXISTS idx_alert_delivery_jobs_cleanup;
DROP INDEX IF EXISTS idx_usage_export_jobs_cleanup;
DROP INDEX IF EXISTS idx_usage_export_cleanup_runs_cleanup;
DROP INDEX IF EXISTS idx_consumption_decision_prune_runs_cleanup;
DROP INDEX IF EXISTS idx_usage_prune_runs_cleanup;
DROP INDEX IF EXISTS idx_usage_ingestions_cleanup;

DROP TABLE IF EXISTS ingestion_rate_windows;
ALTER TABLE workspace_stats DROP COLUMN ingestion_throttled;

DROP TABLE IF EXISTS usage_rollup_runs;

DROP VIEW IF EXISTS usage_aggregation_fragments;
DROP TABLE IF EXISTS usage_hourly_rollups;

DROP TABLE IF EXISTS usage_export_cleanup_runs;
DROP INDEX IF EXISTS idx_usage_export_jobs_artifact_retention;
ALTER TABLE usage_export_jobs DROP COLUMN IF EXISTS expired_at;

ALTER TABLE usage_export_jobs DROP COLUMN IF EXISTS claim_token;

DROP TABLE IF EXISTS alert_delivery_jobs;

DROP TABLE IF EXISTS system_worker_dead_letters;

DROP TABLE IF EXISTS system_worker_heartbeats;

DROP TABLE IF EXISTS auth_api_key_events;

DROP TABLE IF EXISTS reconciliation_notification_attempts;
DROP TABLE IF EXISTS reconciliation_notifications;
ALTER TABLE reconciliation_schedules DROP COLUMN IF EXISTS last_failure_fingerprint;

DROP TABLE IF EXISTS reconciliation_runs;
DROP TABLE IF EXISTS reconciliation_schedules;

DROP TABLE quota_counter_repair_runs;

DROP INDEX idx_consumption_decisions_workspace_meter;
DROP INDEX idx_consumption_decisions_workspace_subject;
DROP INDEX idx_consumption_decisions_workspace_audit;
ALTER TABLE consumption_decisions
	DROP COLUMN state,
	DROP COLUMN enforcement,
	DROP COLUMN evaluation_failed,
	DROP COLUMN accepted,
	DROP COLUMN meter_name,
	DROP COLUMN subject;

DROP TABLE consumption_decision_prune_runs;

DROP TABLE consumption_decisions;

ALTER TABLE plan_limits
	DROP COLUMN failure_policy,
	DROP COLUMN enforcement;

DROP TABLE IF EXISTS workspace_stats;
DROP TABLE IF EXISTS alert_evaluation_jobs;
DROP TABLE IF EXISTS alert_deliveries;
DROP TABLE IF EXISTS alert_events;
DROP TABLE IF EXISTS alert_states;
DROP TABLE IF EXISTS alert_rules;
DROP TABLE IF EXISTS alert_destinations;
DROP TABLE IF EXISTS usage_export_jobs;
DROP TABLE IF EXISTS usage_saved_queries;
DROP TABLE IF EXISTS usage_ingestions;
DROP TABLE IF EXISTS usage_prune_runs;
DROP TABLE IF EXISTS bulk_usage_ingestions;
DROP TABLE IF EXISTS usage_events;
DROP TABLE IF EXISTS entitlement_usage_counters;
DROP TABLE IF EXISTS entitlement_check_jobs;
DROP TABLE IF EXISTS entitlement_period_snapshots;
DROP TABLE IF EXISTS entitlement_events;
DROP TABLE IF EXISTS entitlement_states;
DROP TABLE IF EXISTS plan_subject_assignments;
DROP TABLE IF EXISTS plan_limits;
DROP TABLE IF EXISTS plans;
DROP TABLE IF EXISTS meters;
DROP TABLE IF EXISTS auth_api_keys;
DROP TABLE IF EXISTS auth_sessions;
DROP TABLE IF EXISTS auth_identities;
DROP TABLE IF EXISTS auth_workspace_memberships;
DROP TABLE IF EXISTS auth_users;
DROP TABLE IF EXISTS auth_workspaces;
