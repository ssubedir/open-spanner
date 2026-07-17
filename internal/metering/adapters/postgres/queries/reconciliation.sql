-- name: ListDecisionReconciliationRows :many
SELECT d.idempotency_key, d.subject, d.meter_name, d.accepted, d.created_at,
	e.id AS event_id, e.subject AS event_subject, e.meter_name AS event_meter_name
FROM consumption_decisions d
LEFT JOIN usage_events e
	ON e.workspace_id = d.workspace_id AND e.idempotency_key = d.idempotency_key
WHERE d.workspace_id = sqlc.arg('workspace_id')::text
	AND d.created_at >= sqlc.arg('since')::timestamptz
ORDER BY d.created_at DESC, d.idempotency_key DESC
LIMIT sqlc.arg('limit')::int;

-- name: CountReconciliationNotificationStates :one
SELECT
	COUNT(*) FILTER (WHERE status = 'pending') AS pending,
	COUNT(*) FILTER (WHERE status = 'dead_letter') AS dead_letter
FROM reconciliation_notifications
WHERE workspace_id = sqlc.arg('workspace_id');

-- name: EnsureReconciliationSchedules :exec
INSERT INTO reconciliation_schedules (workspace_id, next_run_at, updated_at)
SELECT id, sqlc.arg('now')::timestamptz, sqlc.arg('now')::timestamptz
FROM auth_workspaces
ON CONFLICT (workspace_id) DO NOTHING;

-- name: ClaimReconciliationSchedule :one
WITH due AS (
	SELECT workspace_id
	FROM reconciliation_schedules
	WHERE next_run_at <= sqlc.arg('now')::timestamptz
		AND (locked_until IS NULL OR locked_until <= sqlc.arg('now')::timestamptz)
	ORDER BY next_run_at, workspace_id
	FOR UPDATE SKIP LOCKED
	LIMIT 1
)
UPDATE reconciliation_schedules s
SET locked_until = sqlc.arg('locked_until')::timestamptz,
	claim_token = sqlc.arg('claim_token'),
	updated_at = sqlc.arg('now')::timestamptz
FROM due
WHERE s.workspace_id = due.workspace_id
RETURNING s.workspace_id, s.claim_token, s.last_fingerprint, s.last_notified_fingerprint, s.last_failure_fingerprint;

-- name: SaveReconciliationRun :exec
INSERT INTO reconciliation_runs (
	id, workspace_id, status, decisions_checked, counters_checked, issue_count,
	truncated, lookback_hours, duration_ms, fingerprint, issues, error, created_at
) VALUES (
	sqlc.arg('id'), sqlc.arg('workspace_id'), sqlc.arg('status'),
	sqlc.arg('decisions_checked'), sqlc.arg('counters_checked'), sqlc.arg('issue_count'),
	sqlc.arg('truncated'), sqlc.arg('lookback_hours'), sqlc.arg('duration_ms'),
	sqlc.arg('fingerprint'), sqlc.arg('issues'), sqlc.arg('error'), sqlc.arg('created_at')
);

-- name: CompleteReconciliationSchedule :execrows
UPDATE reconciliation_schedules
SET next_run_at = sqlc.arg('next_run_at')::timestamptz,
	locked_until = NULL,
	claim_token = NULL,
	last_fingerprint = sqlc.arg('fingerprint'),
	last_notified_fingerprint = CASE WHEN sqlc.arg('fingerprint')::text = '' THEN '' ELSE last_notified_fingerprint END,
	last_failure_fingerprint = '',
	updated_at = sqlc.arg('updated_at')::timestamptz
WHERE workspace_id = sqlc.arg('workspace_id') AND claim_token = sqlc.arg('claim_token');

-- name: FailReconciliationSchedule :execrows
UPDATE reconciliation_schedules
SET next_run_at = sqlc.arg('next_run_at')::timestamptz,
	locked_until = NULL,
	claim_token = NULL,
	last_failure_fingerprint = sqlc.arg('failure_fingerprint'),
	updated_at = sqlc.arg('updated_at')::timestamptz
WHERE workspace_id = sqlc.arg('workspace_id') AND claim_token = sqlc.arg('claim_token');

-- name: ClaimMaintenanceLease :one
INSERT INTO system_maintenance_leases (worker_name, claim_token, locked_until, updated_at)
VALUES (sqlc.arg('worker_name'), sqlc.arg('claim_token'), sqlc.arg('locked_until')::timestamptz, sqlc.arg('now')::timestamptz)
ON CONFLICT (worker_name) DO UPDATE
SET claim_token = EXCLUDED.claim_token, locked_until = EXCLUDED.locked_until, updated_at = EXCLUDED.updated_at
WHERE system_maintenance_leases.locked_until <= sqlc.arg('now')::timestamptz
RETURNING claim_token;

-- name: ReleaseMaintenanceLease :execrows
DELETE FROM system_maintenance_leases
WHERE worker_name = sqlc.arg('worker_name') AND claim_token = sqlc.arg('claim_token');

-- name: MarkReconciliationNotified :exec
UPDATE reconciliation_schedules
SET last_notified_fingerprint = sqlc.arg('fingerprint'), updated_at = sqlc.arg('updated_at')::timestamptz
WHERE workspace_id = sqlc.arg('workspace_id') AND last_fingerprint = sqlc.arg('fingerprint');

-- name: ListReconciliationRuns :many
SELECT id, status, decisions_checked, counters_checked, issue_count, truncated,
	lookback_hours, duration_ms, fingerprint, issues, error, created_at
FROM reconciliation_runs
WHERE workspace_id = sqlc.arg('workspace_id')
ORDER BY created_at DESC, id DESC
LIMIT sqlc.arg('limit')::int;

-- name: GetReconciliationSchedule :one
SELECT next_run_at, locked_until, updated_at
FROM reconciliation_schedules
WHERE workspace_id = sqlc.arg('workspace_id');

-- name: SaveReconciliationNotification :exec
INSERT INTO reconciliation_notifications (
	id, workspace_id, event_type, fingerprint, payload, status,
	attempts, next_attempt_at, created_at
) VALUES (
	sqlc.arg('id'), sqlc.arg('workspace_id'), sqlc.arg('event_type'), sqlc.arg('fingerprint'),
	sqlc.arg('payload'), 'pending', 0, sqlc.arg('next_attempt_at'), sqlc.arg('created_at')
);

-- name: ClaimReconciliationNotification :one
WITH due AS (
	SELECT id FROM reconciliation_notifications
	WHERE status = 'pending' AND next_attempt_at <= sqlc.arg('now')::timestamptz
		AND (locked_until IS NULL OR locked_until <= sqlc.arg('now')::timestamptz)
	ORDER BY next_attempt_at, id
	FOR UPDATE SKIP LOCKED
	LIMIT 1
)
UPDATE reconciliation_notifications n
SET locked_until = sqlc.arg('locked_until')::timestamptz, claim_token = sqlc.arg('claim_token')
FROM due
WHERE n.id = due.id
RETURNING n.id, n.workspace_id, n.event_type, n.fingerprint, n.payload,
	n.status, n.attempts, n.next_attempt_at, n.last_error, n.created_at, n.delivered_at, n.claim_token,
	(SELECT COUNT(*) FROM reconciliation_notification_attempts a WHERE a.notification_id = n.id) AS total_attempts;

-- name: CompleteReconciliationNotification :execrows
UPDATE reconciliation_notifications
SET status = 'delivered', attempts = attempts + 1, locked_until = NULL,
	last_error = '', delivered_at = sqlc.arg('delivered_at')::timestamptz, claim_token = NULL
WHERE id = sqlc.arg('id') AND claim_token = sqlc.arg('claim_token');

-- name: SaveReconciliationNotificationAttempt :exec
INSERT INTO reconciliation_notification_attempts (id, notification_id, attempt, status, error, created_at)
VALUES (sqlc.arg('id'), sqlc.arg('notification_id'), sqlc.arg('attempt'), sqlc.arg('status'), sqlc.arg('error'), sqlc.arg('created_at'));

-- name: ListReconciliationNotificationAttempts :many
SELECT id, attempt, status, error, created_at
FROM reconciliation_notification_attempts
WHERE notification_id = sqlc.arg('notification_id')
ORDER BY created_at, id;

-- name: RetryReconciliationNotification :execrows
UPDATE reconciliation_notifications
SET status = CASE WHEN attempts + 1 >= sqlc.arg('max_attempts')::int THEN 'dead_letter' ELSE 'pending' END,
	attempts = attempts + 1, next_attempt_at = sqlc.arg('next_attempt_at')::timestamptz,
	locked_until = NULL, last_error = sqlc.arg('last_error'), claim_token = NULL
WHERE id = sqlc.arg('id') AND claim_token = sqlc.arg('claim_token');

-- name: ListReconciliationNotifications :many
SELECT id, event_type, fingerprint, payload, status, attempts, next_attempt_at,
	locked_until, last_error, created_at, delivered_at
FROM reconciliation_notifications
WHERE workspace_id = sqlc.arg('workspace_id')
ORDER BY created_at DESC, id DESC
LIMIT sqlc.arg('limit')::int;

-- name: RequeueReconciliationNotification :execrows
UPDATE reconciliation_notifications
SET status = 'pending', attempts = 0, next_attempt_at = sqlc.arg('next_attempt_at')::timestamptz,
	locked_until = NULL, claim_token = NULL, last_error = '', delivered_at = NULL
WHERE id = sqlc.arg('id') AND workspace_id = sqlc.arg('workspace_id') AND status = 'dead_letter';

-- name: ListActiveEntitlementCounters :many
SELECT c.subject, c.meter_name, c.period, c.period_start, c.period_end,
	c.event_count, c.quantity_sum, c.quantity_min, c.quantity_max, c.updated_at,
	m.event_retention_days
FROM entitlement_usage_counters c
JOIN meters m ON m.workspace_id = c.workspace_id AND m.name = c.meter_name
WHERE c.workspace_id = sqlc.arg('workspace_id')::text
	AND c.period_start::timestamptz <= sqlc.arg('now')::timestamptz
	AND c.period_end::timestamptz > sqlc.arg('now')::timestamptz
ORDER BY c.updated_at DESC, c.subject, c.meter_name, c.period
LIMIT sqlc.arg('limit')::int;

-- name: ListCounterReconciliationEvents :many
SELECT id, quantity, event_time, received_at
FROM usage_events
WHERE workspace_id = sqlc.arg('workspace_id')::text
	AND subject = sqlc.arg('subject')::text
	AND meter_name = sqlc.arg('meter_name')::text
	AND event_time::timestamptz >= sqlc.arg('period_start')::timestamptz
	AND event_time::timestamptz < sqlc.arg('period_end')::timestamptz
ORDER BY received_at, id;

-- name: ListCounterReconciliationAssignments :many
SELECT id, assigned_at, period_anchor_at, unassigned_at
FROM plan_subject_assignments
WHERE workspace_id = sqlc.arg('workspace_id')::text
	AND subject = sqlc.arg('subject')::text
	AND assigned_at::timestamptz < sqlc.arg('window_end')::timestamptz
	AND (unassigned_at IS NULL OR unassigned_at::timestamptz > sqlc.arg('window_start')::timestamptz)
ORDER BY assigned_at DESC, id DESC;

-- name: GetEntitlementCounterForRepair :one
SELECT c.subject, c.meter_name, c.period, c.period_start, c.period_end,
	c.event_count, c.quantity_sum, c.quantity_min, c.quantity_max,
	c.first_quantity, c.first_event_time, c.last_quantity, c.last_event_time, c.updated_at,
	m.event_retention_days
FROM entitlement_usage_counters c
JOIN meters m ON m.workspace_id = c.workspace_id AND m.name = c.meter_name
WHERE c.workspace_id = sqlc.arg('workspace_id')::text
	AND c.subject = sqlc.arg('subject')::text
	AND c.meter_name = sqlc.arg('meter_name')::text
	AND c.period = sqlc.arg('period')::text
	AND c.period_start = sqlc.arg('period_start')::text
FOR UPDATE OF c;

-- name: UpdateEntitlementCounterForRepair :execrows
UPDATE entitlement_usage_counters SET
	event_count = sqlc.arg('event_count'), quantity_sum = sqlc.arg('quantity_sum'),
	quantity_min = sqlc.arg('quantity_min'), quantity_max = sqlc.arg('quantity_max'),
	first_quantity = sqlc.arg('first_quantity'), first_event_time = sqlc.arg('first_event_time'),
	last_quantity = sqlc.arg('last_quantity'), last_event_time = sqlc.arg('last_event_time'),
	updated_at = sqlc.arg('updated_at')
WHERE workspace_id = sqlc.arg('workspace_id')::text
	AND subject = sqlc.arg('subject')::text
	AND meter_name = sqlc.arg('meter_name')::text
	AND period = sqlc.arg('period')::text
	AND period_start = sqlc.arg('period_start')::text
	AND updated_at = sqlc.arg('expected_updated_at')::text;

-- name: DeleteEntitlementCounterForRepair :execrows
DELETE FROM entitlement_usage_counters
WHERE workspace_id = sqlc.arg('workspace_id')::text
	AND subject = sqlc.arg('subject')::text
	AND meter_name = sqlc.arg('meter_name')::text
	AND period = sqlc.arg('period')::text
	AND period_start = sqlc.arg('period_start')::text
	AND updated_at = sqlc.arg('expected_updated_at')::text;

-- name: SaveQuotaCounterRepairRun :exec
INSERT INTO quota_counter_repair_runs (
	id, workspace_id, subject, meter_name, period, period_start, period_end,
	dry_run, applied, before_snapshot, after_snapshot, counter_updated_at, created_at
) VALUES (
	sqlc.arg('id'), sqlc.arg('workspace_id'), sqlc.arg('subject'), sqlc.arg('meter_name'),
	sqlc.arg('period'), sqlc.arg('period_start'), sqlc.arg('period_end'), sqlc.arg('dry_run'),
	sqlc.arg('applied'), sqlc.arg('before_snapshot'), sqlc.arg('after_snapshot'),
	sqlc.arg('counter_updated_at'), sqlc.arg('created_at')
);

-- name: ListQuotaCounterRepairRuns :many
SELECT id, subject, meter_name, period, period_start, period_end, dry_run, applied,
	before_snapshot, after_snapshot, counter_updated_at, created_at
FROM quota_counter_repair_runs
WHERE workspace_id = sqlc.arg('workspace_id')::text
ORDER BY created_at DESC, id DESC
LIMIT sqlc.arg('limit')::int;
-- name: UpsertWorkerHeartbeat :exec
INSERT INTO system_worker_heartbeats (worker_name, instance_id, started_at, last_heartbeat_at)
VALUES ($1, $2, $3, $4)
ON CONFLICT(worker_name, instance_id) DO UPDATE SET started_at = EXCLUDED.started_at, last_heartbeat_at = EXCLUDED.last_heartbeat_at;

-- name: ListWorkerHeartbeats :many
SELECT worker_name, instance_id, started_at, last_heartbeat_at
FROM system_worker_heartbeats
ORDER BY worker_name ASC, instance_id ASC;

-- name: DeleteExpiredWorkerHeartbeats :exec
DELETE FROM system_worker_heartbeats
WHERE last_heartbeat_at < sqlc.arg('cutoff')::timestamptz;

-- name: DeleteWorkerHeartbeat :exec
DELETE FROM system_worker_heartbeats
WHERE worker_name = sqlc.arg('worker_name')
	AND instance_id = sqlc.arg('instance_id');

-- name: ListWorkerDiagnostics :many
SELECT 'export'::text AS worker_name,
	COUNT(*) FILTER (WHERE status = 'queued' OR (status = 'running' AND locked_until::timestamptz < sqlc.arg('now')::timestamptz)) AS pending_jobs,
	COUNT(*) FILTER (WHERE status = 'running' AND locked_until::timestamptz >= sqlc.arg('now')::timestamptz) AS running_jobs,
	COUNT(*) FILTER (WHERE status = 'failed') AS failed_jobs,
	COALESCE(MIN(created_at) FILTER (WHERE status = 'queued' OR (status = 'running' AND locked_until::timestamptz < sqlc.arg('now')::timestamptz)), '') AS oldest_pending_at,
	COALESCE(MAX(completed_at) FILTER (WHERE status = 'completed'), '') AS last_success_at,
	COALESCE(MAX(updated_at) FILTER (WHERE status = 'failed'), '') AS last_failure_at
FROM usage_export_jobs
UNION ALL
SELECT 'alert',
	(SELECT COUNT(*) FROM alert_evaluation_jobs j JOIN alert_rules r ON r.id = j.rule_id WHERE (j.locked_until IS NULL OR j.locked_until::timestamptz < sqlc.arg('now')::timestamptz) AND (sqlc.arg('workspace_id')::text = '' OR r.workspace_id = sqlc.arg('workspace_id')::text))
		+ (SELECT COUNT(*) FROM alert_delivery_jobs WHERE (status = 'pending' OR (status = 'running' AND (locked_until IS NULL OR locked_until < sqlc.arg('now')::timestamptz))) AND (sqlc.arg('workspace_id')::text = '' OR workspace_id = sqlc.arg('workspace_id')::text)),
	(SELECT COUNT(*) FROM alert_evaluation_jobs j JOIN alert_rules r ON r.id = j.rule_id WHERE j.locked_until::timestamptz >= sqlc.arg('now')::timestamptz AND (sqlc.arg('workspace_id')::text = '' OR r.workspace_id = sqlc.arg('workspace_id')::text))
		+ (SELECT COUNT(*) FROM alert_delivery_jobs WHERE status = 'running' AND locked_until >= sqlc.arg('now')::timestamptz AND (sqlc.arg('workspace_id')::text = '' OR workspace_id = sqlc.arg('workspace_id')::text)),
	(SELECT COUNT(*) FROM system_worker_dead_letters WHERE worker_name = 'alert' AND status = 'dead_letter' AND (sqlc.arg('workspace_id')::text = '' OR workspace_id = sqlc.arg('workspace_id')::text))
		+ (SELECT COUNT(*) FROM alert_delivery_jobs WHERE status = 'dead_letter' AND (sqlc.arg('workspace_id')::text = '' OR workspace_id = sqlc.arg('workspace_id')::text)),
	COALESCE((SELECT MIN(pending_at) FROM (
		SELECT j.created_at AS pending_at FROM alert_evaluation_jobs j JOIN alert_rules r ON r.id = j.rule_id WHERE (j.locked_until IS NULL OR j.locked_until::timestamptz < sqlc.arg('now')::timestamptz) AND (sqlc.arg('workspace_id')::text = '' OR r.workspace_id = sqlc.arg('workspace_id')::text)
		UNION ALL
		SELECT created_at::text FROM alert_delivery_jobs WHERE (status = 'pending' OR (status = 'running' AND (locked_until IS NULL OR locked_until < sqlc.arg('now')::timestamptz))) AND (sqlc.arg('workspace_id')::text = '' OR workspace_id = sqlc.arg('workspace_id')::text)
	) pending), ''),
	GREATEST(COALESCE((SELECT MAX(s.evaluated_at) FROM alert_states s JOIN alert_rules r ON r.id = s.rule_id WHERE sqlc.arg('workspace_id')::text = '' OR r.workspace_id = sqlc.arg('workspace_id')::text), ''), COALESCE((SELECT MAX(delivered_at)::text FROM alert_delivery_jobs WHERE sqlc.arg('workspace_id')::text = '' OR workspace_id = sqlc.arg('workspace_id')::text), '')),
	GREATEST(COALESCE((SELECT MAX(created_at)::text FROM system_worker_dead_letters WHERE worker_name = 'alert' AND status = 'dead_letter' AND (sqlc.arg('workspace_id')::text = '' OR workspace_id = sqlc.arg('workspace_id')::text)), ''), COALESCE((SELECT MAX(updated_at)::text FROM alert_delivery_jobs WHERE status = 'dead_letter' AND (sqlc.arg('workspace_id')::text = '' OR workspace_id = sqlc.arg('workspace_id')::text)), ''))
UNION ALL
SELECT 'entitlement',
	COUNT(*) FILTER (WHERE locked_until IS NULL OR locked_until::timestamptz < sqlc.arg('now')::timestamptz),
	COUNT(*) FILTER (WHERE locked_until::timestamptz >= sqlc.arg('now')::timestamptz),
	(SELECT COUNT(*) FROM system_worker_dead_letters WHERE worker_name = 'entitlement' AND status = 'dead_letter'),
	COALESCE(MIN(created_at) FILTER (WHERE locked_until IS NULL OR locked_until::timestamptz < sqlc.arg('now')::timestamptz), ''),
	COALESCE((SELECT MAX(evaluated_at) FROM entitlement_states), ''),
	COALESCE((SELECT MAX(created_at)::text FROM system_worker_dead_letters WHERE worker_name = 'entitlement' AND status = 'dead_letter'), '')
FROM entitlement_check_jobs
UNION ALL
SELECT 'retention', 0, 0, 0, '',
	GREATEST(COALESCE((SELECT MAX(created_at) FROM usage_prune_runs), ''), COALESCE((SELECT MAX(created_at)::text FROM consumption_decision_prune_runs), '')), ''
UNION ALL
SELECT 'reconciliation',
	COUNT(*) FILTER (WHERE status = 'pending' AND (locked_until IS NULL OR locked_until < sqlc.arg('now')::timestamptz))
		+ (SELECT COUNT(*) FROM reconciliation_schedules s WHERE s.next_run_at <= sqlc.arg('now')::timestamptz AND (s.locked_until IS NULL OR s.locked_until < sqlc.arg('now')::timestamptz)),
	COUNT(*) FILTER (WHERE status = 'pending' AND locked_until >= sqlc.arg('now')::timestamptz)
		+ (SELECT COUNT(*) FROM reconciliation_schedules s WHERE s.locked_until >= sqlc.arg('now')::timestamptz),
	COUNT(*) FILTER (WHERE status = 'dead_letter'),
	COALESCE((SELECT MIN(pending_at)::text FROM (
		SELECT created_at AS pending_at FROM reconciliation_notifications WHERE status = 'pending' AND (locked_until IS NULL OR locked_until < sqlc.arg('now')::timestamptz)
		UNION ALL
		SELECT next_run_at FROM reconciliation_schedules WHERE next_run_at <= sqlc.arg('now')::timestamptz AND (locked_until IS NULL OR locked_until < sqlc.arg('now')::timestamptz)
	) pending), ''),
	COALESCE((SELECT MAX(created_at)::text FROM reconciliation_runs WHERE status IN ('healthy', 'drift_detected')), ''),
	GREATEST(
		COALESCE((SELECT MAX(created_at)::text FROM reconciliation_runs WHERE status = 'failed'), ''),
		COALESCE((SELECT MAX(created_at)::text FROM reconciliation_notification_attempts WHERE status = 'failed'), '')
	)
FROM reconciliation_notifications;

-- name: ListWorkerDeadLetters :many
SELECT public_id, worker_name, job_key, rule_id, subject, meter_name, attempts, last_error, status, created_at, requeued_at
FROM system_worker_dead_letters
WHERE workspace_id = sqlc.arg('workspace_id')::text
ORDER BY created_at DESC, id DESC
LIMIT sqlc.arg('limit')::int;

-- name: GetWorkerDeadLetter :one
SELECT public_id, worker_name, job_key, rule_id, subject, meter_name, attempts, last_error, status, created_at, requeued_at
FROM system_worker_dead_letters
WHERE workspace_id = sqlc.arg('workspace_id')::text AND public_id = sqlc.arg('public_id')::uuid;

-- name: EnqueueAlertWorkerDeadLetter :execrows
INSERT INTO alert_evaluation_jobs (rule_id, run_after, locked_until, attempts, created_at, updated_at)
SELECT d.rule_id, sqlc.arg('now')::text, NULL, 0, sqlc.arg('now')::text, sqlc.arg('now')::text
FROM system_worker_dead_letters d
JOIN alert_rules r ON r.id = d.rule_id AND r.workspace_id = d.workspace_id
WHERE d.workspace_id = sqlc.arg('workspace_id')::text AND d.public_id = sqlc.arg('public_id')::uuid
	AND d.worker_name = 'alert' AND d.status = 'dead_letter'
ON CONFLICT(rule_id) DO UPDATE SET run_after = EXCLUDED.run_after, locked_until = NULL, attempts = 0, updated_at = EXCLUDED.updated_at;

-- name: EnqueueEntitlementWorkerDeadLetter :execrows
INSERT INTO entitlement_check_jobs (workspace_id, subject, meter_name, run_after, locked_until, attempts, created_at, updated_at)
SELECT d.workspace_id, d.subject, d.meter_name, sqlc.arg('now')::text, NULL, 0, sqlc.arg('now')::text, sqlc.arg('now')::text
FROM system_worker_dead_letters d
JOIN meters m ON m.workspace_id = d.workspace_id AND m.name = d.meter_name
WHERE d.workspace_id = sqlc.arg('workspace_id')::text AND d.public_id = sqlc.arg('public_id')::uuid
	AND d.worker_name = 'entitlement' AND d.status = 'dead_letter'
ON CONFLICT(workspace_id, subject, meter_name) DO UPDATE SET run_after = EXCLUDED.run_after, locked_until = NULL, attempts = 0, updated_at = EXCLUDED.updated_at;

-- name: MarkWorkerDeadLetterRequeued :execrows
UPDATE system_worker_dead_letters
SET status = 'requeued', requeued_at = sqlc.arg('now')::timestamptz
WHERE workspace_id = sqlc.arg('workspace_id')::text AND public_id = sqlc.arg('public_id')::uuid AND status = 'dead_letter';
