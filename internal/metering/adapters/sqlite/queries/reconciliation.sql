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

-- name: ListActiveEntitlementCounters :many
SELECT subject, meter_name, period, period_start, period_end,
	event_count, quantity_sum, quantity_min, quantity_max, updated_at
FROM entitlement_usage_counters
WHERE workspace_id = sqlc.arg('workspace_id')
	AND julianday(period_start) <= julianday(sqlc.arg('now'))
	AND julianday(period_end) > julianday(sqlc.arg('now'))
ORDER BY updated_at DESC, subject, meter_name, period
LIMIT sqlc.arg('limit');

-- name: ListCounterReconciliationEvents :many
SELECT id, quantity, event_time
FROM usage_events
WHERE workspace_id = sqlc.arg('workspace_id')
	AND subject = sqlc.arg('subject')
	AND meter_name = sqlc.arg('meter_name')
	AND julianday(event_time) >= julianday(sqlc.arg('period_start'))
	AND julianday(event_time) < julianday(sqlc.arg('period_end'))
ORDER BY event_time, id;

-- name: ListCounterReconciliationAssignments :many
SELECT id, assigned_at, period_anchor_at, unassigned_at
FROM plan_subject_assignments
WHERE workspace_id = sqlc.arg('workspace_id')
	AND subject = sqlc.arg('subject')
	AND julianday(assigned_at) < julianday(sqlc.arg('window_end'))
	AND (unassigned_at IS NULL OR julianday(unassigned_at) > julianday(sqlc.arg('window_start')))
ORDER BY assigned_at DESC, id DESC;
