-- name: SavePlan :exec
INSERT INTO plans (id, workspace_id, name, description, created_at, updated_at)
VALUES (?, ?, ?, ?, ?, ?)
ON CONFLICT(id) DO UPDATE SET
	name = excluded.name,
	description = excluded.description,
	updated_at = excluded.updated_at;

-- name: ListPlans :many
SELECT id, name, description, created_at, updated_at
FROM plans
WHERE workspace_id = sqlc.arg('workspace_id')
	AND (sqlc.narg('id') IS NULL OR id = sqlc.narg('id'))
	AND (sqlc.narg('name') IS NULL OR name = sqlc.narg('name'))
ORDER BY name
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
INSERT INTO plan_subject_assignments (workspace_id, subject, plan_id, assigned_at, updated_at)
VALUES (?, ?, ?, ?, ?)
ON CONFLICT(workspace_id, subject) DO UPDATE SET
	plan_id = excluded.plan_id,
	updated_at = excluded.updated_at;

-- name: ListPlanSubjectAssignments :many
SELECT a.subject, a.plan_id, p.name AS plan_name, a.assigned_at, a.updated_at
FROM plan_subject_assignments a
JOIN plans p ON p.workspace_id = a.workspace_id AND p.id = a.plan_id
WHERE a.workspace_id = sqlc.arg('workspace_id')
	AND (sqlc.narg('subject') IS NULL OR a.subject = sqlc.narg('subject'))
	AND (sqlc.narg('plan_id') IS NULL OR a.plan_id = sqlc.narg('plan_id'))
ORDER BY a.updated_at DESC, a.subject ASC
LIMIT sqlc.arg('limit');

-- name: DeletePlanSubjectAssignment :execrows
DELETE FROM plan_subject_assignments
WHERE workspace_id = sqlc.arg('workspace_id')
	AND subject = sqlc.arg('subject');

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
