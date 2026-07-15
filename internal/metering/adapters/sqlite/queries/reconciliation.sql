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
RETURNING workspace_id, last_fingerprint, last_notified_fingerprint;

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
	updated_at = sqlc.arg('updated_at')
WHERE workspace_id = sqlc.arg('workspace_id');

-- name: FailReconciliationSchedule :exec
UPDATE reconciliation_schedules
SET next_run_at = sqlc.arg('next_run_at'), locked_until = NULL, updated_at = sqlc.arg('updated_at')
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
