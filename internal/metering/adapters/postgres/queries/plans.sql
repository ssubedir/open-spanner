-- name: SavePlan :exec
INSERT INTO plans (id, workspace_id, name, description, created_at, updated_at)
VALUES ($1, $2, $3, $4, $5, $6)
ON CONFLICT(id) DO UPDATE SET
	name = excluded.name,
	description = excluded.description,
	updated_at = excluded.updated_at;

-- name: ListPlans :many
SELECT id, name, description, created_at, updated_at
FROM plans
WHERE workspace_id = sqlc.arg('workspace_id')::text
	AND (sqlc.narg('id')::text IS NULL OR id = sqlc.narg('id')::text)
	AND (sqlc.narg('name')::text IS NULL OR name = sqlc.narg('name')::text)
ORDER BY name
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
INSERT INTO plan_subject_assignments (workspace_id, subject, plan_id, assigned_at, updated_at)
VALUES ($1, $2, $3, $4, $5)
ON CONFLICT(workspace_id, subject) DO UPDATE SET
	plan_id = excluded.plan_id,
	updated_at = excluded.updated_at;

-- name: ListPlanSubjectAssignments :many
SELECT a.subject, a.plan_id, p.name AS plan_name, a.assigned_at, a.updated_at
FROM plan_subject_assignments a
JOIN plans p ON p.workspace_id = a.workspace_id AND p.id = a.plan_id
WHERE a.workspace_id = sqlc.arg('workspace_id')::text
	AND (sqlc.narg('subject')::text IS NULL OR a.subject = sqlc.narg('subject')::text)
	AND (sqlc.narg('plan_id')::text IS NULL OR a.plan_id = sqlc.narg('plan_id')::text)
ORDER BY a.updated_at DESC, a.subject ASC
LIMIT sqlc.arg('limit')::int;

-- name: DeletePlanSubjectAssignment :execrows
DELETE FROM plan_subject_assignments
WHERE workspace_id = sqlc.arg('workspace_id')::text
	AND subject = sqlc.arg('subject')::text;
