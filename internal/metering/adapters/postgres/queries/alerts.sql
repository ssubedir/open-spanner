-- name: SaveAlertRule :exec
INSERT INTO alert_rules (
	id,
	workspace_id,
	name,
	meter_name,
	enabled,
	subject,
	metadata,
	window_seconds,
	comparator,
	threshold,
	evaluation_interval_seconds,
	group_by,
	destination_id,
	next_evaluate_at,
	created_at,
	updated_at
)
VALUES (
	sqlc.arg('id'),
	sqlc.arg('workspace_id'),
	sqlc.arg('name'),
	sqlc.arg('meter_name'),
	sqlc.arg('enabled'),
	sqlc.arg('subject'),
	sqlc.arg('metadata')::jsonb,
	sqlc.arg('window_seconds'),
	sqlc.arg('comparator'),
	sqlc.arg('threshold'),
	sqlc.arg('evaluation_interval_seconds'),
	sqlc.arg('group_by'),
	sqlc.arg('destination_id'),
	sqlc.arg('next_evaluate_at'),
	sqlc.arg('created_at'),
	sqlc.arg('updated_at')
)
ON CONFLICT(id) DO UPDATE SET
	name = excluded.name,
	meter_name = excluded.meter_name,
	enabled = excluded.enabled,
	subject = excluded.subject,
	metadata = excluded.metadata,
	window_seconds = excluded.window_seconds,
	comparator = excluded.comparator,
	threshold = excluded.threshold,
	evaluation_interval_seconds = excluded.evaluation_interval_seconds,
	group_by = excluded.group_by,
	destination_id = excluded.destination_id,
	next_evaluate_at = excluded.next_evaluate_at,
	updated_at = excluded.updated_at;

-- name: ListAlertRules :many
SELECT id, name, meter_name, enabled, subject, metadata, window_seconds, comparator, threshold, evaluation_interval_seconds, group_by, destination_id, next_evaluate_at, created_at, updated_at
FROM alert_rules
WHERE workspace_id = sqlc.arg('workspace_id')::text
	AND (sqlc.narg('id')::text IS NULL OR id = sqlc.narg('id')::text)
	AND (sqlc.narg('meter_name')::text IS NULL OR meter_name = sqlc.narg('meter_name')::text)
	AND (sqlc.narg('enabled')::boolean IS NULL OR enabled = sqlc.narg('enabled')::boolean)
	AND (sqlc.narg('destination_id')::text IS NULL OR destination_id = sqlc.narg('destination_id')::text)
ORDER BY created_at DESC, id DESC
LIMIT sqlc.arg('limit')::int;

-- name: SaveAlertDestination :exec
INSERT INTO alert_destinations (id, workspace_id, name, type, enabled, webhook_url, webhook_secret, created_at, updated_at)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
ON CONFLICT(id) DO UPDATE SET
	name = excluded.name,
	type = excluded.type,
	enabled = excluded.enabled,
	webhook_url = excluded.webhook_url,
	webhook_secret = excluded.webhook_secret,
	updated_at = excluded.updated_at;

-- name: ListAlertDestinations :many
SELECT id, name, type, enabled, webhook_url, webhook_secret, created_at, updated_at
FROM alert_destinations
WHERE workspace_id = sqlc.arg('workspace_id')::text
	AND (sqlc.narg('id')::text IS NULL OR id = sqlc.narg('id')::text)
	AND (sqlc.narg('type')::text IS NULL OR type = sqlc.narg('type')::text)
	AND (sqlc.narg('enabled')::boolean IS NULL OR enabled = sqlc.narg('enabled')::boolean)
ORDER BY created_at DESC, id DESC
LIMIT sqlc.arg('limit')::int;

-- name: DeleteAlertDestination :execrows
DELETE FROM alert_destinations
WHERE workspace_id = sqlc.arg('workspace_id')::text
	AND id = sqlc.arg('id')::text;

-- name: DeleteAlertRule :execrows
DELETE FROM alert_rules
WHERE workspace_id = sqlc.arg('workspace_id')::text
	AND id = sqlc.arg('id')::text;

-- name: SaveAlertState :exec
INSERT INTO alert_states (rule_id, group_key, group_value, status, value, message, evaluated_at, updated_at)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
ON CONFLICT(rule_id, group_key, group_value) DO UPDATE SET
	status = excluded.status,
	value = excluded.value,
	message = excluded.message,
	evaluated_at = excluded.evaluated_at,
	updated_at = excluded.updated_at;

-- name: FindAlertState :one
SELECT rule_id, group_key, group_value, status, value, message, evaluated_at, updated_at
FROM alert_states
WHERE rule_id = sqlc.arg('rule_id')
	AND EXISTS (
		SELECT 1
		FROM alert_rules
		WHERE alert_rules.id = alert_states.rule_id
			AND alert_rules.workspace_id = sqlc.arg('workspace_id')::text
	)
	AND group_key = sqlc.arg('group_key')
	AND group_value = sqlc.arg('group_value');

-- name: ListAlertStates :many
SELECT rule_id, group_key, group_value, status, value, message, evaluated_at, updated_at
FROM alert_states
WHERE rule_id = sqlc.arg('rule_id')
	AND EXISTS (
		SELECT 1
		FROM alert_rules
		WHERE alert_rules.id = alert_states.rule_id
			AND alert_rules.workspace_id = sqlc.arg('workspace_id')::text
	)
ORDER BY
	CASE status
		WHEN 'alerting' THEN 0
		WHEN 'error' THEN 1
		WHEN 'no_data' THEN 2
		ELSE 3
	END,
	updated_at DESC,
	group_key ASC,
	group_value ASC
LIMIT sqlc.arg('limit')::int;

-- name: SaveAlertEvent :exec
INSERT INTO alert_events (id, rule_id, group_key, group_value, type, value, message, created_at)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8);

-- name: SaveAlertDelivery :exec
INSERT INTO alert_deliveries (id, event_id, trigger_type, status, status_code, error, duration_ms, attempted_at, created_at)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9);

-- name: ListAlertEvents :many
SELECT
	alert_events.id,
	alert_events.rule_id,
	alert_events.group_key,
	alert_events.group_value,
	alert_events.type,
	alert_events.value,
	alert_events.message,
	alert_events.created_at,
	delivery.id AS delivery_id,
	delivery.trigger_type AS delivery_trigger_type,
	delivery.status AS delivery_status,
	delivery.status_code AS delivery_status_code,
	delivery.error AS delivery_error,
	delivery.duration_ms AS delivery_duration_ms,
	delivery.attempted_at AS delivery_attempted_at,
	delivery.created_at AS delivery_created_at
FROM alert_events
LEFT JOIN alert_deliveries AS delivery
	ON delivery.id = (
		SELECT id
		FROM alert_deliveries
		WHERE event_id = alert_events.id
		ORDER BY attempted_at DESC, id DESC
		LIMIT 1
	)
WHERE EXISTS (
		SELECT 1
		FROM alert_rules
		WHERE alert_rules.id = alert_events.rule_id
			AND alert_rules.workspace_id = sqlc.arg('workspace_id')::text
	)
	AND (sqlc.narg('rule_id')::text IS NULL OR alert_events.rule_id = sqlc.narg('rule_id')::text)
	AND (sqlc.narg('cursor_created_at')::text IS NULL
		OR (alert_events.created_at < sqlc.narg('cursor_created_at')::text
			OR (alert_events.created_at = sqlc.narg('cursor_created_at')::text AND alert_events.id < sqlc.narg('cursor_id')::text)))
ORDER BY alert_events.created_at DESC, alert_events.id DESC
LIMIT sqlc.arg('limit')::int;

-- name: EnqueueAlertEvaluationJob :exec
INSERT INTO alert_evaluation_jobs (rule_id, run_after, locked_until, attempts, created_at, updated_at)
VALUES (sqlc.arg('rule_id'), sqlc.arg('run_after'), NULL, 0, sqlc.arg('now'), sqlc.arg('now'))
ON CONFLICT(rule_id) DO UPDATE SET
	run_after = CASE
		WHEN alert_evaluation_jobs.run_after > excluded.run_after THEN excluded.run_after
		ELSE alert_evaluation_jobs.run_after
	END,
	updated_at = excluded.updated_at
WHERE alert_evaluation_jobs.locked_until IS NULL
	OR alert_evaluation_jobs.locked_until < excluded.updated_at;

-- name: EnqueueDueAlertEvaluationJobs :execrows
INSERT INTO alert_evaluation_jobs (rule_id, run_after, locked_until, attempts, created_at, updated_at)
SELECT id, sqlc.arg('run_after'), NULL, 0, sqlc.arg('now'), sqlc.arg('now')
FROM alert_rules
WHERE enabled = TRUE
	AND next_evaluate_at <= sqlc.arg('now')
ORDER BY next_evaluate_at ASC, id ASC
LIMIT sqlc.arg('limit')::int
ON CONFLICT(rule_id) DO UPDATE SET
	run_after = excluded.run_after,
	updated_at = excluded.updated_at
WHERE alert_evaluation_jobs.locked_until IS NULL
	OR alert_evaluation_jobs.locked_until < excluded.updated_at;

-- name: ClaimAlertEvaluationJob :one
WITH next_job AS (
	SELECT rule_id
	FROM alert_evaluation_jobs
	WHERE run_after <= sqlc.arg('now')
		AND (locked_until IS NULL OR locked_until < sqlc.arg('now'))
		AND alert_evaluation_jobs.attempts < sqlc.arg('max_attempts')::int
	ORDER BY run_after ASC, created_at ASC, rule_id ASC
	FOR UPDATE SKIP LOCKED
	LIMIT 1
)
UPDATE alert_evaluation_jobs
SET attempts = alert_evaluation_jobs.attempts + 1,
	locked_until = sqlc.arg('locked_until'),
	updated_at = sqlc.arg('now')
WHERE rule_id = (SELECT rule_id FROM next_job)
RETURNING rule_id, run_after, locked_until, attempts, created_at, updated_at;

-- name: FindWorkspaceIDForAlertRule :one
SELECT workspace_id
FROM alert_rules
WHERE id = $1;

-- name: RequeueAlertEvaluationJob :execrows
UPDATE alert_evaluation_jobs
SET run_after = sqlc.arg('run_after'),
	locked_until = NULL,
	updated_at = sqlc.arg('now')
WHERE rule_id = sqlc.arg('rule_id');

-- name: DeleteAlertEvaluationJob :execrows
DELETE FROM alert_evaluation_jobs
WHERE rule_id = $1;

-- name: UpdateAlertRuleNextEvaluation :execrows
UPDATE alert_rules
SET next_evaluate_at = sqlc.arg('next_evaluate_at'),
	updated_at = sqlc.arg('updated_at')
WHERE id = sqlc.arg('id');
