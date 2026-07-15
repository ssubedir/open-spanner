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
