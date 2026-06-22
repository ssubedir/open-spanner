-- name: SavePlan :exec
INSERT INTO plans (id, workspace_id, name, description, version, parent_plan_id, is_current, created_at, updated_at)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
ON CONFLICT(id) DO UPDATE SET
	name = excluded.name,
	description = excluded.description,
	version = excluded.version,
	parent_plan_id = excluded.parent_plan_id,
	is_current = excluded.is_current,
	updated_at = excluded.updated_at;

-- name: RetirePlan :execrows
UPDATE plans
SET is_current = 0,
	updated_at = sqlc.arg('updated_at')
WHERE workspace_id = sqlc.arg('workspace_id')
	AND id = sqlc.arg('id')
	AND is_current = 1;

-- name: ListPlans :many
SELECT id, name, description, version, parent_plan_id, is_current, created_at, updated_at
FROM plans
WHERE workspace_id = sqlc.arg('workspace_id')
	AND (sqlc.narg('id') IS NULL OR id = sqlc.narg('id'))
	AND (sqlc.narg('name') IS NULL OR name = sqlc.narg('name'))
	AND (sqlc.arg('current_only') = 0 OR is_current = 1)
ORDER BY name, version DESC
LIMIT sqlc.arg('limit');

-- name: DeletePlan :execrows
DELETE FROM plans
WHERE workspace_id = sqlc.arg('workspace_id')
	AND id = sqlc.arg('id');

-- name: SavePlanLimit :exec
INSERT INTO plan_limits (id, workspace_id, plan_id, meter_name, period, limit_value, warning_percent, created_at, updated_at)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
ON CONFLICT(id) DO UPDATE SET
	meter_name = excluded.meter_name,
	period = excluded.period,
	limit_value = excluded.limit_value,
	warning_percent = excluded.warning_percent,
	updated_at = excluded.updated_at;

-- name: DeletePlanLimits :exec
DELETE FROM plan_limits
WHERE workspace_id = sqlc.arg('workspace_id')
	AND plan_id = sqlc.arg('plan_id');

-- name: ListPlanLimits :many
SELECT id, plan_id, meter_name, period, limit_value, warning_percent, created_at, updated_at
FROM plan_limits
WHERE workspace_id = sqlc.arg('workspace_id')
	AND (sqlc.narg('plan_id') IS NULL OR plan_id = sqlc.narg('plan_id'))
ORDER BY meter_name, period;

-- name: CountPlanAssignments :one
SELECT COUNT(*)
FROM plan_subject_assignments
WHERE workspace_id = sqlc.arg('workspace_id')
	AND plan_id = sqlc.arg('plan_id');

-- name: SavePlanSubjectAssignment :exec
INSERT INTO plan_subject_assignments (id, workspace_id, subject, plan_id, assigned_at, period_anchor_at, unassigned_at, updated_at)
VALUES (?, ?, ?, ?, ?, ?, NULL, ?);

-- name: CancelPendingPlanSubjectAssignments :execrows
UPDATE plan_subject_assignments
SET unassigned_at = sqlc.arg('unassigned_at'),
	updated_at = sqlc.arg('updated_at')
WHERE workspace_id = sqlc.arg('workspace_id')
	AND subject = sqlc.arg('subject')
	AND assigned_at > sqlc.arg('now')
	AND (unassigned_at IS NULL OR unassigned_at > sqlc.arg('now'));

-- name: EndEffectivePlanSubjectAssignments :execrows
UPDATE plan_subject_assignments
SET unassigned_at = sqlc.arg('unassigned_at'),
	updated_at = sqlc.arg('updated_at')
WHERE workspace_id = sqlc.arg('workspace_id')
	AND subject = sqlc.arg('subject')
	AND assigned_at < sqlc.arg('unassigned_at')
	AND (unassigned_at IS NULL OR unassigned_at > sqlc.arg('unassigned_at'));

-- name: ListPlanSubjectAssignments :many
SELECT a.id, a.subject, a.plan_id, p.name AS plan_name, p.version AS plan_version, a.assigned_at, a.period_anchor_at, a.unassigned_at, a.updated_at
FROM plan_subject_assignments a
JOIN plans p ON p.workspace_id = a.workspace_id AND p.id = a.plan_id
WHERE a.workspace_id = sqlc.arg('workspace_id')
	AND (sqlc.narg('subject') IS NULL OR a.subject = sqlc.narg('subject'))
	AND (sqlc.narg('plan_id') IS NULL OR a.plan_id = sqlc.narg('plan_id'))
	AND (sqlc.arg('active_only') = 0 OR a.unassigned_at IS NULL OR a.unassigned_at > sqlc.arg('now'))
ORDER BY a.updated_at DESC, a.assigned_at DESC, a.subject ASC
LIMIT sqlc.arg('limit');

-- name: FindEffectivePlanSubjectAssignment :one
SELECT a.id, a.subject, a.plan_id, p.name AS plan_name, p.version AS plan_version, a.assigned_at, a.period_anchor_at, a.unassigned_at, a.updated_at
FROM plan_subject_assignments a
JOIN plans p ON p.workspace_id = a.workspace_id AND p.id = a.plan_id
WHERE a.workspace_id = sqlc.arg('workspace_id')
	AND a.subject = sqlc.arg('subject')
	AND a.assigned_at <= sqlc.arg('now')
	AND (a.unassigned_at IS NULL OR a.unassigned_at > sqlc.arg('now'))
ORDER BY a.assigned_at DESC, a.updated_at DESC
LIMIT 1;

-- name: FindActivePlanAssignmentAnchor :one
SELECT period_anchor_at
FROM plan_subject_assignments
WHERE workspace_id = sqlc.arg('workspace_id')
  AND subject = sqlc.arg('subject')
  AND assigned_at <= sqlc.arg('now')
  AND (unassigned_at IS NULL OR unassigned_at > sqlc.arg('now'))
ORDER BY assigned_at DESC
LIMIT 1;

-- name: DeletePlanSubjectAssignment :execrows
UPDATE plan_subject_assignments
SET unassigned_at = sqlc.arg('unassigned_at'),
	updated_at = sqlc.arg('updated_at')
WHERE workspace_id = sqlc.arg('workspace_id')
	AND subject = sqlc.arg('subject')
	AND (unassigned_at IS NULL OR unassigned_at > sqlc.arg('unassigned_at'));

-- name: GetEntitlementState :one
SELECT workspace_id, subject, meter_name, plan_id, plan_name, period, state, current_value, limit_value, remaining_value, warning_percent, message, evaluated_at, updated_at
FROM entitlement_states
WHERE workspace_id = sqlc.arg('workspace_id')
	AND subject = sqlc.arg('subject')
	AND meter_name = sqlc.arg('meter_name')
	AND plan_id = sqlc.arg('plan_id')
	AND period = sqlc.arg('period');

-- name: SaveEntitlementState :exec
INSERT INTO entitlement_states (
	workspace_id, subject, meter_name, plan_id, plan_name, period, state,
	current_value, limit_value, remaining_value, warning_percent, message, evaluated_at, updated_at
)
VALUES (
	sqlc.arg('workspace_id'), sqlc.arg('subject'), sqlc.arg('meter_name'), sqlc.arg('plan_id'), sqlc.arg('plan_name'), sqlc.arg('period'), sqlc.arg('state'),
	sqlc.arg('current_value'), sqlc.arg('limit_value'), sqlc.arg('remaining_value'), sqlc.arg('warning_percent'), sqlc.arg('message'), sqlc.arg('evaluated_at'), sqlc.arg('updated_at')
)
ON CONFLICT(workspace_id, subject, meter_name, plan_id, period) DO UPDATE SET
	plan_name = excluded.plan_name,
	state = excluded.state,
	current_value = excluded.current_value,
	limit_value = excluded.limit_value,
	remaining_value = excluded.remaining_value,
	warning_percent = excluded.warning_percent,
	message = excluded.message,
	evaluated_at = excluded.evaluated_at,
	updated_at = excluded.updated_at;

-- name: SaveEntitlementEvent :exec
INSERT INTO entitlement_events (
	id, workspace_id, subject, meter_name, plan_id, plan_name, period, previous_state, state, type,
	current_value, limit_value, remaining_value, warning_percent, message, created_at
)
VALUES (
	sqlc.arg('id'), sqlc.arg('workspace_id'), sqlc.arg('subject'), sqlc.arg('meter_name'), sqlc.arg('plan_id'), sqlc.arg('plan_name'), sqlc.arg('period'), sqlc.narg('previous_state'), sqlc.arg('state'), sqlc.arg('type'),
	sqlc.arg('current_value'), sqlc.arg('limit_value'), sqlc.arg('remaining_value'), sqlc.arg('warning_percent'), sqlc.arg('message'), sqlc.arg('created_at')
);

-- name: SaveEntitlementPeriodSnapshot :exec
INSERT INTO entitlement_period_snapshots (
	workspace_id, subject, meter_name, plan_id, plan_name, plan_version, period, period_start, period_end, state,
	current_value, limit_value, included_value, overage_value, remaining_value, warning_percent, event_count, updated_at
)
VALUES (
	sqlc.arg('workspace_id'), sqlc.arg('subject'), sqlc.arg('meter_name'), sqlc.arg('plan_id'), sqlc.arg('plan_name'), sqlc.arg('plan_version'), sqlc.arg('period'), sqlc.arg('period_start'), sqlc.arg('period_end'), sqlc.arg('state'),
	sqlc.arg('current_value'), sqlc.arg('limit_value'), sqlc.arg('included_value'), sqlc.arg('overage_value'), sqlc.arg('remaining_value'), sqlc.arg('warning_percent'), sqlc.arg('event_count'), sqlc.arg('updated_at')
)
ON CONFLICT(workspace_id, subject, meter_name, plan_id, period, period_start) DO UPDATE SET
	plan_name = excluded.plan_name,
	plan_version = excluded.plan_version,
	period_end = excluded.period_end,
	state = excluded.state,
	current_value = excluded.current_value,
	limit_value = excluded.limit_value,
	included_value = excluded.included_value,
	overage_value = excluded.overage_value,
	remaining_value = excluded.remaining_value,
	warning_percent = excluded.warning_percent,
	event_count = excluded.event_count,
	updated_at = excluded.updated_at;

-- name: ListEntitlementPeriodSnapshots :many
SELECT workspace_id, subject, meter_name, plan_id, plan_name, plan_version, period, period_start, period_end, state,
	current_value, limit_value, included_value, overage_value, remaining_value, warning_percent, event_count, updated_at
FROM entitlement_period_snapshots
WHERE workspace_id = sqlc.arg('workspace_id')
	AND (CAST(sqlc.narg('subject') AS TEXT) IS NULL OR subject = CAST(sqlc.narg('subject') AS TEXT))
	AND (CAST(sqlc.narg('meter_name') AS TEXT) IS NULL OR meter_name = CAST(sqlc.narg('meter_name') AS TEXT))
	AND (CAST(sqlc.narg('plan_id') AS TEXT) IS NULL OR plan_id = CAST(sqlc.narg('plan_id') AS TEXT))
	AND (CAST(sqlc.narg('state') AS TEXT) IS NULL OR state = CAST(sqlc.narg('state') AS TEXT))
ORDER BY period_start DESC, updated_at DESC, subject ASC, meter_name ASC, plan_id ASC
LIMIT sqlc.arg('limit');

-- name: EnqueueEntitlementCheckJob :exec
INSERT INTO entitlement_check_jobs (workspace_id, subject, meter_name, run_after, locked_until, attempts, created_at, updated_at)
VALUES (sqlc.arg('workspace_id'), sqlc.arg('subject'), sqlc.arg('meter_name'), sqlc.arg('run_after'), NULL, 0, sqlc.arg('now'), sqlc.arg('now'))
ON CONFLICT(workspace_id, subject, meter_name) DO UPDATE SET
	run_after = CASE
		WHEN entitlement_check_jobs.run_after > excluded.run_after THEN excluded.run_after
		ELSE entitlement_check_jobs.run_after
	END,
	updated_at = excluded.updated_at
WHERE entitlement_check_jobs.locked_until IS NULL
	OR entitlement_check_jobs.locked_until < excluded.updated_at;

-- name: ClaimEntitlementCheckJob :one
UPDATE entitlement_check_jobs
SET attempts = entitlement_check_jobs.attempts + 1,
	locked_until = sqlc.arg('locked_until'),
	updated_at = sqlc.arg('now')
WHERE (workspace_id, subject, meter_name) = (
	SELECT workspace_id, subject, meter_name
	FROM entitlement_check_jobs
	WHERE run_after <= sqlc.arg('now')
		AND (locked_until IS NULL OR locked_until < sqlc.arg('now'))
		AND entitlement_check_jobs.attempts < sqlc.arg('max_attempts')
	ORDER BY run_after ASC, created_at ASC, workspace_id ASC, subject ASC, meter_name ASC
	LIMIT 1
)
RETURNING workspace_id, subject, meter_name, run_after, locked_until, attempts, created_at, updated_at;

-- name: RequeueEntitlementCheckJob :execrows
UPDATE entitlement_check_jobs
SET run_after = sqlc.arg('run_after'),
	locked_until = NULL,
	updated_at = sqlc.arg('now')
WHERE workspace_id = sqlc.arg('workspace_id')
	AND subject = sqlc.arg('subject')
	AND meter_name = sqlc.arg('meter_name');

-- name: DeleteEntitlementCheckJob :execrows
DELETE FROM entitlement_check_jobs
WHERE workspace_id = sqlc.arg('workspace_id')
	AND subject = sqlc.arg('subject')
	AND meter_name = sqlc.arg('meter_name');

-- name: ListEntitlementStates :many
SELECT workspace_id, subject, meter_name, plan_id, plan_name, period, state, current_value, limit_value, remaining_value, warning_percent, message, evaluated_at, updated_at
FROM entitlement_states
WHERE workspace_id = sqlc.arg('workspace_id')
	AND (CAST(sqlc.narg('subject') AS TEXT) IS NULL OR subject = CAST(sqlc.narg('subject') AS TEXT))
	AND (CAST(sqlc.narg('meter_name') AS TEXT) IS NULL OR meter_name = CAST(sqlc.narg('meter_name') AS TEXT))
	AND (CAST(sqlc.narg('state') AS TEXT) IS NULL OR state = CAST(sqlc.narg('state') AS TEXT))
ORDER BY updated_at DESC, subject ASC, meter_name ASC, plan_id ASC, period ASC
LIMIT sqlc.arg('limit');

-- name: ListEntitlementEvents :many
SELECT id, workspace_id, subject, meter_name, plan_id, plan_name, period, previous_state, state, type,
	current_value, limit_value, remaining_value, warning_percent, message, created_at
FROM entitlement_events
WHERE workspace_id = sqlc.arg('workspace_id')
	AND (CAST(sqlc.narg('subject') AS TEXT) IS NULL OR subject = CAST(sqlc.narg('subject') AS TEXT))
	AND (CAST(sqlc.narg('meter_name') AS TEXT) IS NULL OR meter_name = CAST(sqlc.narg('meter_name') AS TEXT))
	AND (CAST(sqlc.narg('plan_id') AS TEXT) IS NULL OR plan_id = CAST(sqlc.narg('plan_id') AS TEXT))
	AND (CAST(sqlc.narg('state') AS TEXT) IS NULL OR state = CAST(sqlc.narg('state') AS TEXT))
	AND (CAST(sqlc.narg('type') AS TEXT) IS NULL OR type = CAST(sqlc.narg('type') AS TEXT))
	AND (CAST(sqlc.narg('cursor_created_at') AS TEXT) IS NULL
		OR (created_at < CAST(sqlc.narg('cursor_created_at') AS TEXT)
			OR (created_at = CAST(sqlc.narg('cursor_created_at') AS TEXT) AND id < CAST(sqlc.narg('cursor_id') AS TEXT))))
ORDER BY created_at DESC, id DESC
LIMIT sqlc.arg('limit');

-- name: IncrementEntitlementUsageCounter :exec
INSERT INTO entitlement_usage_counters (
	workspace_id, subject, meter_name, period, period_start, period_end,
	event_count, quantity_sum, quantity_min, quantity_max,
	first_quantity, first_event_time, last_quantity, last_event_time, updated_at
)
VALUES (
	sqlc.arg('workspace_id'), sqlc.arg('subject'), sqlc.arg('meter_name'), sqlc.arg('period'), sqlc.arg('period_start'), sqlc.arg('period_end'),
	1, sqlc.arg('quantity'), sqlc.arg('quantity'), sqlc.arg('quantity'),
	sqlc.arg('quantity'), sqlc.arg('event_time'), sqlc.arg('quantity'), sqlc.arg('event_time'), sqlc.arg('updated_at')
)
ON CONFLICT(workspace_id, subject, meter_name, period, period_start) DO UPDATE SET
	period_end = excluded.period_end,
	event_count = entitlement_usage_counters.event_count + excluded.event_count,
	quantity_sum = entitlement_usage_counters.quantity_sum + excluded.quantity_sum,
	quantity_min = MIN(entitlement_usage_counters.quantity_min, excluded.quantity_min),
	quantity_max = MAX(entitlement_usage_counters.quantity_max, excluded.quantity_max),
	first_quantity = CASE
		WHEN excluded.first_event_time < entitlement_usage_counters.first_event_time THEN excluded.first_quantity
		ELSE entitlement_usage_counters.first_quantity
	END,
	first_event_time = MIN(entitlement_usage_counters.first_event_time, excluded.first_event_time),
	last_quantity = CASE
		WHEN excluded.last_event_time >= entitlement_usage_counters.last_event_time THEN excluded.last_quantity
		ELSE entitlement_usage_counters.last_quantity
	END,
	last_event_time = MAX(entitlement_usage_counters.last_event_time, excluded.last_event_time),
	updated_at = excluded.updated_at;

-- name: GetEntitlementUsageCounter :one
SELECT workspace_id, subject, meter_name, period, period_start, period_end,
	event_count, quantity_sum, quantity_min, quantity_max,
	first_quantity, first_event_time, last_quantity, last_event_time, updated_at
FROM entitlement_usage_counters
WHERE workspace_id = sqlc.arg('workspace_id')
	AND subject = sqlc.arg('subject')
	AND meter_name = sqlc.arg('meter_name')
	AND period = sqlc.arg('period')
	AND period_start = sqlc.arg('period_start');
