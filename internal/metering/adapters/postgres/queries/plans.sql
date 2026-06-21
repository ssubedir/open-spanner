-- name: SavePlan :exec
INSERT INTO plans (id, workspace_id, name, description, version, parent_plan_id, is_current, created_at, updated_at)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
ON CONFLICT(id) DO UPDATE SET
	name = excluded.name,
	description = excluded.description,
	version = excluded.version,
	parent_plan_id = excluded.parent_plan_id,
	is_current = excluded.is_current,
	updated_at = excluded.updated_at;

-- name: RetirePlan :execrows
UPDATE plans
SET is_current = FALSE,
	updated_at = sqlc.arg('updated_at')
WHERE workspace_id = sqlc.arg('workspace_id')::text
	AND id = sqlc.arg('id')::text
	AND is_current = TRUE;

-- name: ListPlans :many
SELECT id, name, description, version, parent_plan_id, is_current, created_at, updated_at
FROM plans
WHERE workspace_id = sqlc.arg('workspace_id')::text
	AND (sqlc.narg('id')::text IS NULL OR id = sqlc.narg('id')::text)
	AND (sqlc.narg('name')::text IS NULL OR name = sqlc.narg('name')::text)
	AND (NOT sqlc.arg('current_only')::boolean OR is_current = TRUE)
ORDER BY name, version DESC
LIMIT sqlc.arg('limit')::int;

-- name: DeletePlan :execrows
DELETE FROM plans
WHERE workspace_id = sqlc.arg('workspace_id')::text
	AND id = sqlc.arg('id')::text;

-- name: SavePlanLimit :exec
INSERT INTO plan_limits (id, workspace_id, plan_id, meter_name, period, limit_value, warning_percent, created_at, updated_at)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
ON CONFLICT(id) DO UPDATE SET
	meter_name = excluded.meter_name,
	period = excluded.period,
	limit_value = excluded.limit_value,
	warning_percent = excluded.warning_percent,
	updated_at = excluded.updated_at;

-- name: DeletePlanLimits :exec
DELETE FROM plan_limits
WHERE workspace_id = sqlc.arg('workspace_id')::text
	AND plan_id = sqlc.arg('plan_id')::text;

-- name: ListPlanLimits :many
SELECT id, plan_id, meter_name, period, limit_value, warning_percent, created_at, updated_at
FROM plan_limits
WHERE workspace_id = sqlc.arg('workspace_id')::text
	AND (sqlc.narg('plan_id')::text IS NULL OR plan_id = sqlc.narg('plan_id')::text)
ORDER BY meter_name, period;

-- name: CountPlanAssignments :one
SELECT COUNT(*)
FROM plan_subject_assignments
WHERE workspace_id = sqlc.arg('workspace_id')::text
	AND plan_id = sqlc.arg('plan_id')::text;

-- name: SavePlanSubjectAssignment :exec
INSERT INTO plan_subject_assignments (id, workspace_id, subject, plan_id, assigned_at, period_anchor_at, unassigned_at, updated_at)
VALUES ($1, $2, $3, $4, $5, $6, NULL, $7);

-- name: EndCurrentPlanSubjectAssignment :execrows
UPDATE plan_subject_assignments
SET unassigned_at = sqlc.arg('unassigned_at'),
	updated_at = sqlc.arg('updated_at')
WHERE workspace_id = sqlc.arg('workspace_id')::text
	AND subject = sqlc.arg('subject')::text
	AND unassigned_at IS NULL;

-- name: ListPlanSubjectAssignments :many
SELECT a.id, a.subject, a.plan_id, p.name AS plan_name, p.version AS plan_version, a.assigned_at, a.period_anchor_at, a.unassigned_at, a.updated_at
FROM plan_subject_assignments a
JOIN plans p ON p.workspace_id = a.workspace_id AND p.id = a.plan_id
WHERE a.workspace_id = sqlc.arg('workspace_id')::text
	AND (sqlc.narg('subject')::text IS NULL OR a.subject = sqlc.narg('subject')::text)
	AND (sqlc.narg('plan_id')::text IS NULL OR a.plan_id = sqlc.narg('plan_id')::text)
	AND (NOT sqlc.arg('active_only')::boolean OR a.unassigned_at IS NULL)
ORDER BY a.updated_at DESC, a.assigned_at DESC, a.subject ASC
LIMIT sqlc.arg('limit')::int;

-- name: FindActivePlanAssignmentAnchor :one
SELECT period_anchor_at
FROM plan_subject_assignments
WHERE workspace_id = sqlc.arg('workspace_id')::text
  AND subject = sqlc.arg('subject')::text
  AND unassigned_at IS NULL
ORDER BY assigned_at DESC
LIMIT 1;

-- name: DeletePlanSubjectAssignment :execrows
UPDATE plan_subject_assignments
SET unassigned_at = sqlc.arg('unassigned_at'),
	updated_at = sqlc.arg('updated_at')
WHERE workspace_id = sqlc.arg('workspace_id')::text
	AND subject = sqlc.arg('subject')::text
	AND unassigned_at IS NULL;

-- name: GetEntitlementState :one
SELECT workspace_id, subject, meter_name, plan_id, plan_name, period, state, current_value, limit_value, remaining_value, warning_percent, message, evaluated_at, updated_at
FROM entitlement_states
WHERE workspace_id = sqlc.arg('workspace_id')::text
	AND subject = sqlc.arg('subject')::text
	AND meter_name = sqlc.arg('meter_name')::text
	AND plan_id = sqlc.arg('plan_id')::text
	AND period = sqlc.arg('period')::text;

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
WITH next_job AS (
	SELECT workspace_id, subject, meter_name
	FROM entitlement_check_jobs
	WHERE run_after <= sqlc.arg('now')
		AND (locked_until IS NULL OR locked_until < sqlc.arg('now'))
		AND entitlement_check_jobs.attempts < sqlc.arg('max_attempts')::int
	ORDER BY run_after ASC, created_at ASC, workspace_id ASC, subject ASC, meter_name ASC
	FOR UPDATE SKIP LOCKED
	LIMIT 1
)
UPDATE entitlement_check_jobs
SET attempts = entitlement_check_jobs.attempts + 1,
	locked_until = sqlc.arg('locked_until'),
	updated_at = sqlc.arg('now')
WHERE (workspace_id, subject, meter_name) = (SELECT workspace_id, subject, meter_name FROM next_job)
RETURNING workspace_id, subject, meter_name, run_after, locked_until, attempts, created_at, updated_at;

-- name: RequeueEntitlementCheckJob :execrows
UPDATE entitlement_check_jobs
SET run_after = sqlc.arg('run_after'),
	locked_until = NULL,
	updated_at = sqlc.arg('now')
WHERE workspace_id = sqlc.arg('workspace_id')::text
	AND subject = sqlc.arg('subject')::text
	AND meter_name = sqlc.arg('meter_name')::text;

-- name: DeleteEntitlementCheckJob :execrows
DELETE FROM entitlement_check_jobs
WHERE workspace_id = sqlc.arg('workspace_id')::text
	AND subject = sqlc.arg('subject')::text
	AND meter_name = sqlc.arg('meter_name')::text;

-- name: ListEntitlementStates :many
SELECT workspace_id, subject, meter_name, plan_id, plan_name, period, state, current_value, limit_value, remaining_value, warning_percent, message, evaluated_at, updated_at
FROM entitlement_states
WHERE workspace_id = sqlc.arg('workspace_id')::text
	AND (sqlc.narg('subject')::text IS NULL OR subject = sqlc.narg('subject')::text)
	AND (sqlc.narg('meter_name')::text IS NULL OR meter_name = sqlc.narg('meter_name')::text)
	AND (sqlc.narg('state')::text IS NULL OR state = sqlc.narg('state')::text)
ORDER BY updated_at DESC, subject ASC, meter_name ASC, plan_id ASC, period ASC
LIMIT sqlc.arg('limit')::int;

-- name: ListEntitlementEvents :many
SELECT id, workspace_id, subject, meter_name, plan_id, plan_name, period, previous_state, state, type,
	current_value, limit_value, remaining_value, warning_percent, message, created_at
FROM entitlement_events
WHERE workspace_id = sqlc.arg('workspace_id')::text
	AND (sqlc.narg('subject')::text IS NULL OR subject = sqlc.narg('subject')::text)
	AND (sqlc.narg('meter_name')::text IS NULL OR meter_name = sqlc.narg('meter_name')::text)
	AND (sqlc.narg('plan_id')::text IS NULL OR plan_id = sqlc.narg('plan_id')::text)
	AND (sqlc.narg('state')::text IS NULL OR state = sqlc.narg('state')::text)
	AND (sqlc.narg('type')::text IS NULL OR type = sqlc.narg('type')::text)
	AND (sqlc.narg('cursor_created_at')::text IS NULL
		OR (created_at < sqlc.narg('cursor_created_at')::text
			OR (created_at = sqlc.narg('cursor_created_at')::text AND id < sqlc.narg('cursor_id')::text)))
ORDER BY created_at DESC, id DESC
LIMIT sqlc.arg('limit')::int;

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
	quantity_min = LEAST(entitlement_usage_counters.quantity_min, excluded.quantity_min),
	quantity_max = GREATEST(entitlement_usage_counters.quantity_max, excluded.quantity_max),
	first_quantity = CASE
		WHEN excluded.first_event_time < entitlement_usage_counters.first_event_time THEN excluded.first_quantity
		ELSE entitlement_usage_counters.first_quantity
	END,
	first_event_time = LEAST(entitlement_usage_counters.first_event_time, excluded.first_event_time),
	last_quantity = CASE
		WHEN excluded.last_event_time >= entitlement_usage_counters.last_event_time THEN excluded.last_quantity
		ELSE entitlement_usage_counters.last_quantity
	END,
	last_event_time = GREATEST(entitlement_usage_counters.last_event_time, excluded.last_event_time),
	updated_at = excluded.updated_at;

-- name: GetEntitlementUsageCounter :one
SELECT workspace_id, subject, meter_name, period, period_start, period_end,
	event_count, quantity_sum, quantity_min, quantity_max,
	first_quantity, first_event_time, last_quantity, last_event_time, updated_at
FROM entitlement_usage_counters
WHERE workspace_id = sqlc.arg('workspace_id')::text
	AND subject = sqlc.arg('subject')::text
	AND meter_name = sqlc.arg('meter_name')::text
	AND period = sqlc.arg('period')::text
	AND period_start = sqlc.arg('period_start')::text;
