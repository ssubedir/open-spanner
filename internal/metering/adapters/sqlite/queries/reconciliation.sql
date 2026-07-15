-- name: ListDecisionReconciliationRows :many
SELECT d.idempotency_key, d.subject, d.meter_name, d.accepted, d.created_at,
	e.id AS event_id, e.subject AS event_subject, e.meter_name AS event_meter_name
FROM consumption_decisions d
LEFT JOIN usage_events e
	ON e.workspace_id = d.workspace_id AND e.idempotency_key = d.idempotency_key
WHERE d.workspace_id = sqlc.arg('workspace_id')
	AND d.created_at >= sqlc.arg('since')
ORDER BY d.created_at DESC, d.idempotency_key DESC
LIMIT sqlc.arg('limit');

-- name: CountReconciliationNotificationStates :one
SELECT
	SUM(CASE WHEN status = 'pending' THEN 1 ELSE 0 END) AS pending,
	SUM(CASE WHEN status = 'dead_letter' THEN 1 ELSE 0 END) AS dead_letter
FROM reconciliation_notifications
WHERE workspace_id = sqlc.arg('workspace_id');

-- name: EnsureReconciliationSchedules :exec
INSERT INTO reconciliation_schedules (workspace_id, next_run_at, updated_at)
SELECT id, sqlc.arg('now'), sqlc.arg('now')
FROM auth_workspaces
WHERE 1
ON CONFLICT (workspace_id) DO NOTHING;

-- name: ClaimReconciliationSchedule :one
UPDATE reconciliation_schedules
SET locked_until = sqlc.arg('locked_until'), updated_at = sqlc.arg('now')
WHERE workspace_id = (
	SELECT workspace_id FROM reconciliation_schedules
	WHERE julianday(next_run_at) <= julianday(sqlc.arg('now'))
		AND (locked_until IS NULL OR julianday(locked_until) <= julianday(sqlc.arg('now')))
	ORDER BY next_run_at, workspace_id LIMIT 1
)
RETURNING workspace_id, last_fingerprint, last_notified_fingerprint, last_failure_fingerprint;

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

-- name: CompleteReconciliationSchedule :exec
UPDATE reconciliation_schedules
SET next_run_at = sqlc.arg('next_run_at'), locked_until = NULL,
	last_fingerprint = sqlc.arg('fingerprint'),
	last_notified_fingerprint = CASE WHEN sqlc.arg('fingerprint') = '' THEN '' ELSE last_notified_fingerprint END,
	last_failure_fingerprint = '',
	updated_at = sqlc.arg('updated_at')
WHERE workspace_id = sqlc.arg('workspace_id');

-- name: FailReconciliationSchedule :exec
UPDATE reconciliation_schedules
SET next_run_at = sqlc.arg('next_run_at'), locked_until = NULL,
	last_failure_fingerprint = sqlc.arg('failure_fingerprint'), updated_at = sqlc.arg('updated_at')
WHERE workspace_id = sqlc.arg('workspace_id');

-- name: MarkReconciliationNotified :exec
UPDATE reconciliation_schedules
SET last_notified_fingerprint = sqlc.arg('fingerprint'), updated_at = sqlc.arg('updated_at')
WHERE workspace_id = sqlc.arg('workspace_id') AND last_fingerprint = sqlc.arg('fingerprint');

-- name: ListReconciliationRuns :many
SELECT id, status, decisions_checked, counters_checked, issue_count, truncated,
	lookback_hours, duration_ms, fingerprint, issues, error, created_at
FROM reconciliation_runs
WHERE workspace_id = sqlc.arg('workspace_id')
ORDER BY created_at DESC, id DESC
LIMIT sqlc.arg('limit');

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
UPDATE reconciliation_notifications
SET locked_until = sqlc.arg('locked_until')
WHERE id = (
	SELECT id FROM reconciliation_notifications
	WHERE status = 'pending' AND julianday(next_attempt_at) <= julianday(sqlc.arg('now'))
		AND (locked_until IS NULL OR julianday(locked_until) <= julianday(sqlc.arg('now')))
	ORDER BY next_attempt_at, id LIMIT 1
)
RETURNING id, workspace_id, event_type, fingerprint, payload,
	status, attempts, next_attempt_at, last_error, created_at, delivered_at,
	(SELECT COUNT(*) FROM reconciliation_notification_attempts a WHERE a.notification_id = reconciliation_notifications.id) AS total_attempts;

-- name: CompleteReconciliationNotification :exec
UPDATE reconciliation_notifications
SET status = 'delivered', attempts = attempts + 1, locked_until = NULL,
	last_error = '', delivered_at = sqlc.arg('delivered_at')
WHERE id = sqlc.arg('id');

-- name: SaveReconciliationNotificationAttempt :exec
INSERT INTO reconciliation_notification_attempts (id, notification_id, attempt, status, error, created_at)
VALUES (sqlc.arg('id'), sqlc.arg('notification_id'), sqlc.arg('attempt'), sqlc.arg('status'), sqlc.arg('error'), sqlc.arg('created_at'));

-- name: ListReconciliationNotificationAttempts :many
SELECT id, attempt, status, error, created_at
FROM reconciliation_notification_attempts
WHERE notification_id = sqlc.arg('notification_id')
ORDER BY created_at, id;

-- name: RetryReconciliationNotification :exec
UPDATE reconciliation_notifications
SET status = CASE WHEN attempts + 1 >= sqlc.arg('max_attempts') THEN 'dead_letter' ELSE 'pending' END,
	attempts = attempts + 1, next_attempt_at = sqlc.arg('next_attempt_at'),
	locked_until = NULL, last_error = sqlc.arg('last_error')
WHERE id = sqlc.arg('id');

-- name: ListReconciliationNotifications :many
SELECT id, event_type, fingerprint, payload, status, attempts, next_attempt_at,
	locked_until, last_error, created_at, delivered_at
FROM reconciliation_notifications
WHERE workspace_id = sqlc.arg('workspace_id')
ORDER BY created_at DESC, id DESC
LIMIT sqlc.arg('limit');

-- name: RequeueReconciliationNotification :execrows
UPDATE reconciliation_notifications
SET status = 'pending', attempts = 0, next_attempt_at = sqlc.arg('next_attempt_at'),
	locked_until = NULL, last_error = '', delivered_at = NULL
WHERE id = sqlc.arg('id') AND workspace_id = sqlc.arg('workspace_id') AND status = 'dead_letter';

-- name: ListActiveEntitlementCounters :many
SELECT c.subject, c.meter_name, c.period, c.period_start, c.period_end,
	c.event_count, c.quantity_sum, c.quantity_min, c.quantity_max, c.updated_at,
	m.event_retention_days
FROM entitlement_usage_counters c
JOIN meters m ON m.workspace_id = c.workspace_id AND m.name = c.meter_name
WHERE c.workspace_id = sqlc.arg('workspace_id')
	AND julianday(c.period_start) <= julianday(sqlc.arg('now'))
	AND julianday(c.period_end) > julianday(sqlc.arg('now'))
ORDER BY c.updated_at DESC, c.subject, c.meter_name, c.period
LIMIT sqlc.arg('limit');

-- name: ListCounterReconciliationEvents :many
SELECT id, quantity, event_time, received_at
FROM usage_events
WHERE workspace_id = sqlc.arg('workspace_id')
	AND subject = sqlc.arg('subject')
	AND meter_name = sqlc.arg('meter_name')
	AND julianday(event_time) >= julianday(sqlc.arg('period_start'))
	AND julianday(event_time) < julianday(sqlc.arg('period_end'))
ORDER BY received_at, id;

-- name: ListCounterReconciliationAssignments :many
SELECT id, assigned_at, period_anchor_at, unassigned_at
FROM plan_subject_assignments
WHERE workspace_id = sqlc.arg('workspace_id')
	AND subject = sqlc.arg('subject')
	AND julianday(assigned_at) < julianday(sqlc.arg('window_end'))
	AND (unassigned_at IS NULL OR julianday(unassigned_at) > julianday(sqlc.arg('window_start')))
ORDER BY assigned_at DESC, id DESC;

-- name: GetEntitlementCounterForRepair :one
SELECT c.subject, c.meter_name, c.period, c.period_start, c.period_end,
	c.event_count, c.quantity_sum, c.quantity_min, c.quantity_max,
	c.first_quantity, c.first_event_time, c.last_quantity, c.last_event_time, c.updated_at,
	m.event_retention_days
FROM entitlement_usage_counters c
JOIN meters m ON m.workspace_id = c.workspace_id AND m.name = c.meter_name
WHERE c.workspace_id = sqlc.arg('workspace_id')
	AND c.subject = sqlc.arg('subject')
	AND c.meter_name = sqlc.arg('meter_name')
	AND c.period = sqlc.arg('period')
	AND c.period_start = sqlc.arg('period_start');

-- name: UpdateEntitlementCounterForRepair :execrows
UPDATE entitlement_usage_counters SET
	event_count = sqlc.arg('event_count'), quantity_sum = sqlc.arg('quantity_sum'),
	quantity_min = sqlc.arg('quantity_min'), quantity_max = sqlc.arg('quantity_max'),
	first_quantity = sqlc.arg('first_quantity'), first_event_time = sqlc.arg('first_event_time'),
	last_quantity = sqlc.arg('last_quantity'), last_event_time = sqlc.arg('last_event_time'),
	updated_at = sqlc.arg('updated_at')
WHERE workspace_id = sqlc.arg('workspace_id')
	AND subject = sqlc.arg('subject')
	AND meter_name = sqlc.arg('meter_name')
	AND period = sqlc.arg('period')
	AND period_start = sqlc.arg('period_start')
	AND updated_at = sqlc.arg('expected_updated_at');

-- name: DeleteEntitlementCounterForRepair :execrows
DELETE FROM entitlement_usage_counters
WHERE workspace_id = sqlc.arg('workspace_id')
	AND subject = sqlc.arg('subject')
	AND meter_name = sqlc.arg('meter_name')
	AND period = sqlc.arg('period')
	AND period_start = sqlc.arg('period_start')
	AND updated_at = sqlc.arg('expected_updated_at');

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
WHERE workspace_id = sqlc.arg('workspace_id')
ORDER BY created_at DESC, id DESC
LIMIT sqlc.arg('limit');
