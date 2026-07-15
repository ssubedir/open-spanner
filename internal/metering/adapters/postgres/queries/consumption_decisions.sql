-- name: SaveConsumptionDecision :execrows
INSERT INTO consumption_decisions (workspace_id, idempotency_key, response, created_at)
VALUES (sqlc.arg('workspace_id'), sqlc.arg('idempotency_key'), sqlc.arg('response')::jsonb, sqlc.arg('created_at'))
ON CONFLICT DO NOTHING;

-- name: FindConsumptionDecision :one
SELECT response
FROM consumption_decisions
WHERE workspace_id = sqlc.arg('workspace_id')
	AND idempotency_key = sqlc.arg('idempotency_key');
