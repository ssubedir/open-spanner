DROP TABLE IF EXISTS usage_event_outbox;

DROP TABLE IF EXISTS system_maintenance_leases;
ALTER TABLE reconciliation_schedules DROP COLUMN claim_token;
ALTER TABLE reconciliation_notifications DROP COLUMN claim_token;

CREATE TABLE system_worker_heartbeats_v1 (
	worker_name TEXT PRIMARY KEY,
	started_at TEXT NOT NULL,
	last_heartbeat_at TEXT NOT NULL
);

INSERT INTO system_worker_heartbeats_v1 (worker_name, started_at, last_heartbeat_at)
SELECT h.worker_name, h.started_at, h.last_heartbeat_at
FROM system_worker_heartbeats h
WHERE NOT EXISTS (
	SELECT 1
	FROM system_worker_heartbeats newer
	WHERE newer.worker_name = h.worker_name
		AND (newer.last_heartbeat_at > h.last_heartbeat_at
			OR (newer.last_heartbeat_at = h.last_heartbeat_at AND newer.instance_id > h.instance_id))
);

DROP TABLE system_worker_heartbeats;
ALTER TABLE system_worker_heartbeats_v1 RENAME TO system_worker_heartbeats;

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
ALTER TABLE usage_export_jobs DROP COLUMN expired_at;

ALTER TABLE usage_export_jobs DROP COLUMN claim_token;

DROP TABLE IF EXISTS alert_delivery_jobs;

DROP TABLE IF EXISTS system_worker_dead_letters;

DROP TABLE IF EXISTS system_worker_heartbeats;

DROP TABLE IF EXISTS auth_api_key_events;

DROP TABLE IF EXISTS reconciliation_notification_attempts;
DROP TABLE IF EXISTS reconciliation_notifications;
ALTER TABLE reconciliation_schedules DROP COLUMN last_failure_fingerprint;

DROP TABLE IF EXISTS reconciliation_runs;
DROP TABLE IF EXISTS reconciliation_schedules;

DROP TABLE quota_counter_repair_runs;

DROP INDEX idx_consumption_decisions_workspace_meter;
DROP INDEX idx_consumption_decisions_workspace_subject;
DROP INDEX idx_consumption_decisions_workspace_audit;
ALTER TABLE consumption_decisions DROP COLUMN state;
ALTER TABLE consumption_decisions DROP COLUMN enforcement;
ALTER TABLE consumption_decisions DROP COLUMN evaluation_failed;
ALTER TABLE consumption_decisions DROP COLUMN accepted;
ALTER TABLE consumption_decisions DROP COLUMN meter_name;
ALTER TABLE consumption_decisions DROP COLUMN subject;

DROP TABLE consumption_decision_prune_runs;

DROP TABLE consumption_decisions;

ALTER TABLE plan_limits DROP COLUMN failure_policy;
ALTER TABLE plan_limits DROP COLUMN enforcement;

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
DROP TABLE IF EXISTS auth_workspace_invitations;
DROP TABLE IF EXISTS auth_workspace_memberships;
DROP TABLE IF EXISTS auth_users;
DROP TABLE IF EXISTS auth_workspaces;
