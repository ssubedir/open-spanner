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
SELECT subject, meter_name, period, period_start, period_end,
	event_count, quantity_sum, quantity_min, quantity_max, updated_at
FROM entitlement_usage_counters
WHERE workspace_id = sqlc.arg('workspace_id')::text
	AND period_start::timestamptz <= sqlc.arg('now')::timestamptz
	AND period_end::timestamptz > sqlc.arg('now')::timestamptz
ORDER BY updated_at DESC, subject, meter_name, period
LIMIT sqlc.arg('limit')::int;

-- name: ListCounterReconciliationEvents :many
SELECT id, quantity, event_time
FROM usage_events
WHERE workspace_id = sqlc.arg('workspace_id')::text
	AND subject = sqlc.arg('subject')::text
	AND meter_name = sqlc.arg('meter_name')::text
	AND event_time::timestamptz >= sqlc.arg('period_start')::timestamptz
	AND event_time::timestamptz < sqlc.arg('period_end')::timestamptz
ORDER BY event_time, id;

-- name: ListCounterReconciliationAssignments :many
SELECT id, assigned_at, period_anchor_at, unassigned_at
FROM plan_subject_assignments
WHERE workspace_id = sqlc.arg('workspace_id')::text
	AND subject = sqlc.arg('subject')::text
	AND assigned_at::timestamptz < sqlc.arg('window_end')::timestamptz
	AND (unassigned_at IS NULL OR unassigned_at::timestamptz > sqlc.arg('window_start')::timestamptz)
ORDER BY assigned_at DESC, id DESC;
