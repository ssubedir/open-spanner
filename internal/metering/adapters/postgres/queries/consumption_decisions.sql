-- name: SaveConsumptionDecision :execrows
INSERT INTO consumption_decisions (workspace_id, idempotency_key, response, subject, meter_name, accepted, evaluation_failed, enforcement, state, created_at)
VALUES (sqlc.arg('workspace_id'), sqlc.arg('idempotency_key'), sqlc.arg('response')::jsonb, sqlc.arg('subject'), sqlc.arg('meter_name'), sqlc.arg('accepted'), sqlc.arg('evaluation_failed'), sqlc.arg('enforcement'), sqlc.arg('state'), sqlc.arg('created_at'))
ON CONFLICT DO NOTHING;

-- name: FindConsumptionDecision :one
SELECT response, created_at
FROM consumption_decisions
WHERE workspace_id = sqlc.arg('workspace_id')
	AND idempotency_key = sqlc.arg('idempotency_key');

-- name: ListConsumptionDecisions :many
SELECT idempotency_key, response, subject, meter_name, accepted, evaluation_failed, enforcement, state, created_at
FROM consumption_decisions
WHERE workspace_id = sqlc.arg('workspace_id')
	AND (sqlc.arg('subject')::text = '' OR subject = sqlc.arg('subject'))
	AND (sqlc.arg('meter_name')::text = '' OR meter_name = sqlc.arg('meter_name'))
	AND (sqlc.narg('accepted')::boolean IS NULL OR accepted = sqlc.narg('accepted'))
	AND (sqlc.narg('evaluation_failed')::boolean IS NULL OR evaluation_failed = sqlc.narg('evaluation_failed'))
	AND (sqlc.arg('enforcement')::text = '' OR enforcement = sqlc.arg('enforcement'))
	AND (sqlc.arg('state')::text = '' OR state = sqlc.arg('state'))
	AND (sqlc.narg('cursor_created_at')::timestamptz IS NULL OR created_at < sqlc.narg('cursor_created_at')
		OR (created_at = sqlc.narg('cursor_created_at') AND idempotency_key < sqlc.arg('cursor_id')))
ORDER BY created_at DESC, idempotency_key DESC
LIMIT sqlc.arg('limit')::int;

-- name: CountExpiredConsumptionDecisions :one
SELECT COUNT(*) FROM consumption_decisions
WHERE workspace_id = sqlc.arg('workspace_id') AND created_at < sqlc.arg('before');

-- name: PruneExpiredConsumptionDecisions :execrows
DELETE FROM consumption_decisions
WHERE workspace_id = sqlc.arg('workspace_id') AND created_at < sqlc.arg('before');

-- name: SaveConsumptionDecisionPruneRun :exec
INSERT INTO consumption_decision_prune_runs (id, workspace_id, before, dry_run, deleted, created_at)
VALUES (sqlc.arg('id'), sqlc.arg('workspace_id'), sqlc.arg('before'), sqlc.arg('dry_run'), sqlc.arg('deleted'), sqlc.arg('created_at'));

-- name: CountConsumptionDecisions :one
SELECT COUNT(*) FROM consumption_decisions WHERE workspace_id = sqlc.arg('workspace_id');

-- name: CountConsumptionDecisionPruneRuns :one
SELECT COUNT(*) FROM consumption_decision_prune_runs WHERE workspace_id = sqlc.arg('workspace_id');

-- name: FindLatestConsumptionDecisionPruneRun :one
SELECT id, before, dry_run, deleted, created_at
FROM consumption_decision_prune_runs
WHERE workspace_id = sqlc.arg('workspace_id')
ORDER BY created_at DESC, id DESC LIMIT 1;
