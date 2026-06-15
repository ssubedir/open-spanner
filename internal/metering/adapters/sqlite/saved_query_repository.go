package sqlite

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"time"

	appsavedquery "github.com/ssubedir/open-spanner/internal/metering/app/savedquery"
	"github.com/ssubedir/open-spanner/internal/metering/domain"
)

type SavedQueryRepository struct {
	store *Store
}

func NewSavedQueryRepository(store *Store) *SavedQueryRepository {
	return &SavedQueryRepository{store: store}
}

func (r *SavedQueryRepository) Save(ctx context.Context, query appsavedquery.SavedQuery) (appsavedquery.SavedQuery, error) {
	groupBy, err := json.Marshal(query.GroupBy)
	if err != nil {
		return appsavedquery.SavedQuery{}, err
	}

	_, err = r.store.exec(ctx, `
INSERT INTO usage_saved_queries (id, user_id, name, query_json, group_by, bucket_size, result_limit, created_at, updated_at)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
ON CONFLICT(id) DO UPDATE SET
	name = excluded.name,
	query_json = excluded.query_json,
	group_by = excluded.group_by,
	bucket_size = excluded.bucket_size,
	result_limit = excluded.result_limit,
	updated_at = excluded.updated_at
`, query.ID, query.UserID, query.Name, string(query.Query), string(groupBy), query.BucketSize, query.Limit, formatTime(query.CreatedAt), formatTime(query.UpdatedAt))
	if err != nil {
		if isUniqueConstraint(err) {
			return appsavedquery.SavedQuery{}, errors.Join(domain.ErrConflict, err)
		}
		return appsavedquery.SavedQuery{}, err
	}

	return query, nil
}

func (r *SavedQueryRepository) Find(ctx context.Context, query appsavedquery.FindQuery) ([]appsavedquery.SavedQuery, error) {
	if query.ID != "" {
		saved, err := scanSavedQuery(r.store.queryRow(ctx, `
SELECT id, user_id, name, query_json, group_by, bucket_size, result_limit, created_at, updated_at
FROM usage_saved_queries
WHERE user_id = ? AND id = ?
`, query.UserID, query.ID))
		if err != nil {
			if errors.Is(err, domain.ErrNotFound) {
				return []appsavedquery.SavedQuery{}, nil
			}
			return nil, err
		}
		return []appsavedquery.SavedQuery{saved}, nil
	}

	rows, err := r.store.query(ctx, `
SELECT id, user_id, name, query_json, group_by, bucket_size, result_limit, created_at, updated_at
FROM usage_saved_queries
WHERE user_id = ?
ORDER BY updated_at DESC, id DESC
`, query.UserID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	queries := []appsavedquery.SavedQuery{}
	for rows.Next() {
		saved, err := scanSavedQuery(rows)
		if err != nil {
			return nil, err
		}
		queries = append(queries, saved)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return queries, nil
}

func (r *SavedQueryRepository) Delete(ctx context.Context, userID string, id string) error {
	res, err := r.store.exec(ctx, `
DELETE FROM usage_saved_queries
WHERE user_id = ? AND id = ?
`, userID, id)
	if err != nil {
		return err
	}
	if rows, err := res.RowsAffected(); err == nil && rows == 0 {
		return domain.ErrNotFound
	}
	return nil
}

func scanSavedQuery(scanner interface {
	Scan(dest ...any) error
}) (appsavedquery.SavedQuery, error) {
	var query appsavedquery.SavedQuery
	var queryJSON string
	var groupByJSON string
	var createdAt string
	var updatedAt string
	if err := scanner.Scan(&query.ID, &query.UserID, &query.Name, &queryJSON, &groupByJSON, &query.BucketSize, &query.Limit, &createdAt, &updatedAt); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return appsavedquery.SavedQuery{}, domain.ErrNotFound
		}
		return appsavedquery.SavedQuery{}, err
	}

	query.Query = json.RawMessage(queryJSON)
	if groupByJSON != "" {
		if err := json.Unmarshal([]byte(groupByJSON), &query.GroupBy); err != nil {
			return appsavedquery.SavedQuery{}, err
		}
	}

	parsedCreatedAt, err := time.Parse(time.RFC3339Nano, createdAt)
	if err != nil {
		return appsavedquery.SavedQuery{}, err
	}
	parsedUpdatedAt, err := time.Parse(time.RFC3339Nano, updatedAt)
	if err != nil {
		return appsavedquery.SavedQuery{}, err
	}
	query.CreatedAt = parsedCreatedAt
	query.UpdatedAt = parsedUpdatedAt
	return query, nil
}
