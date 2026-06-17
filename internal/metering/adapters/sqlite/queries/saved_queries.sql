-- name: SaveSavedQuery :exec
INSERT INTO usage_saved_queries (id, user_id, name, query_json, group_by, bucket_size, result_limit, pinned, position, created_at, updated_at)
VALUES (
	sqlc.arg('id'),
	sqlc.arg('user_id'),
	sqlc.arg('name'),
	sqlc.arg('query_json'),
	sqlc.arg('group_by'),
	sqlc.arg('bucket_size'),
	sqlc.arg('result_limit'),
	sqlc.arg('pinned'),
	sqlc.arg('position'),
	sqlc.arg('created_at'),
	sqlc.arg('updated_at')
)
ON CONFLICT(id) DO UPDATE SET
	name = excluded.name,
	query_json = excluded.query_json,
	group_by = excluded.group_by,
	bucket_size = excluded.bucket_size,
	result_limit = excluded.result_limit,
	pinned = excluded.pinned,
	position = excluded.position,
	updated_at = excluded.updated_at;

-- name: FindSavedQueryByID :one
SELECT id, user_id, name, query_json, group_by, bucket_size, result_limit, pinned, position, created_at, updated_at
FROM usage_saved_queries
WHERE user_id = ? AND id = ?;

-- name: ListSavedQueries :many
SELECT id, user_id, name, query_json, group_by, bucket_size, result_limit, pinned, position, created_at, updated_at
FROM usage_saved_queries
WHERE user_id = ?
ORDER BY pinned DESC, position ASC, updated_at DESC, id DESC;

-- name: DeleteSavedQuery :execrows
DELETE FROM usage_saved_queries
WHERE user_id = ? AND id = ?;
