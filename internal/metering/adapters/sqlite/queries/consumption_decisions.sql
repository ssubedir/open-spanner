-- name: SaveConsumptionDecision :execrows
INSERT INTO consumption_decisions (workspace_id, idempotency_key, response, created_at)
VALUES (sqlc.arg('workspace_id'), sqlc.arg('idempotency_key'), sqlc.arg('response'), sqlc.arg('created_at'))
ON CONFLICT DO NOTHING;

-- name: FindConsumptionDecision :one
SELECT response
FROM consumption_decisions
WHERE workspace_id = sqlc.arg('workspace_id')
	AND idempotency_key = sqlc.arg('idempotency_key');

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
