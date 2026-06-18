-- name: SaveAlertRule :exec
INSERT INTO alert_rules (
	id,
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
	trigger_type,
	webhook_url,
	next_evaluate_at,
	created_at,
	updated_at
)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
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
	trigger_type = excluded.trigger_type,
	webhook_url = excluded.webhook_url,
	next_evaluate_at = excluded.next_evaluate_at,
	updated_at = excluded.updated_at;

-- name: ListAlertRules :many
SELECT id, name, meter_name, enabled, subject, metadata, window_seconds, comparator, threshold, evaluation_interval_seconds, group_by, trigger_type, webhook_url, next_evaluate_at, created_at, updated_at
FROM alert_rules
WHERE (CAST(sqlc.narg('id') AS TEXT) IS NULL OR id = CAST(sqlc.narg('id') AS TEXT))
	AND (CAST(sqlc.narg('meter_name') AS TEXT) IS NULL OR meter_name = CAST(sqlc.narg('meter_name') AS TEXT))
	AND (CAST(sqlc.narg('enabled') AS INTEGER) IS NULL OR enabled = CAST(sqlc.narg('enabled') AS INTEGER))
ORDER BY created_at DESC, id DESC
LIMIT sqlc.arg('limit');

-- name: DeleteAlertRule :execrows
DELETE FROM alert_rules
WHERE id = ?;

-- name: SaveAlertState :exec
INSERT INTO alert_states (rule_id, group_key, group_value, status, value, message, evaluated_at, updated_at)
VALUES (?, ?, ?, ?, ?, ?, ?, ?)
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
	AND group_key = sqlc.arg('group_key')
	AND group_value = sqlc.arg('group_value');

-- name: ListAlertStates :many
SELECT rule_id, group_key, group_value, status, value, message, evaluated_at, updated_at
FROM alert_states
WHERE rule_id = sqlc.arg('rule_id')
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
LIMIT sqlc.arg('limit');

-- name: SaveAlertEvent :exec
INSERT INTO alert_events (id, rule_id, group_key, group_value, type, value, message, created_at)
VALUES (?, ?, ?, ?, ?, ?, ?, ?);

-- name: SaveAlertDelivery :exec
INSERT INTO alert_deliveries (id, event_id, trigger_type, status, status_code, error, duration_ms, attempted_at, created_at)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?);

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
WHERE (CAST(sqlc.narg('rule_id') AS TEXT) IS NULL OR alert_events.rule_id = CAST(sqlc.narg('rule_id') AS TEXT))
	AND (CAST(sqlc.narg('cursor_created_at') AS TEXT) IS NULL
		OR (alert_events.created_at < CAST(sqlc.narg('cursor_created_at') AS TEXT)
			OR (alert_events.created_at = CAST(sqlc.narg('cursor_created_at') AS TEXT) AND alert_events.id < CAST(sqlc.narg('cursor_id') AS TEXT))))
ORDER BY alert_events.created_at DESC, alert_events.id DESC
LIMIT sqlc.arg('limit');

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
WHERE enabled = 1
	AND next_evaluate_at <= sqlc.arg('now')
ORDER BY next_evaluate_at ASC, id ASC
LIMIT sqlc.arg('limit')
ON CONFLICT(rule_id) DO UPDATE SET
	run_after = excluded.run_after,
	updated_at = excluded.updated_at
WHERE alert_evaluation_jobs.locked_until IS NULL
	OR alert_evaluation_jobs.locked_until < excluded.updated_at;

-- name: ClaimAlertEvaluationJob :one
UPDATE alert_evaluation_jobs
SET attempts = alert_evaluation_jobs.attempts + 1,
	locked_until = sqlc.arg('locked_until'),
	updated_at = sqlc.arg('now')
WHERE rule_id = (
	SELECT rule_id
	FROM alert_evaluation_jobs
	WHERE run_after <= sqlc.arg('now')
		AND (locked_until IS NULL OR locked_until < sqlc.arg('now'))
		AND alert_evaluation_jobs.attempts < sqlc.arg('max_attempts')
	ORDER BY run_after ASC, created_at ASC, rule_id ASC
	LIMIT 1
)
RETURNING rule_id, run_after, locked_until, attempts, created_at, updated_at;

-- name: RequeueAlertEvaluationJob :execrows
UPDATE alert_evaluation_jobs
SET run_after = sqlc.arg('run_after'),
	locked_until = NULL,
	updated_at = sqlc.arg('now')
WHERE rule_id = sqlc.arg('rule_id');

-- name: DeleteAlertEvaluationJob :execrows
DELETE FROM alert_evaluation_jobs
WHERE rule_id = ?;

-- name: UpdateAlertRuleNextEvaluation :execrows
UPDATE alert_rules
SET next_evaluate_at = sqlc.arg('next_evaluate_at'),
	updated_at = sqlc.arg('updated_at')
WHERE id = sqlc.arg('id');
