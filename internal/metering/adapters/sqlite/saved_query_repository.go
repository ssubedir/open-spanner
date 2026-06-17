package sqlite

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"time"

	"github.com/ssubedir/open-spanner/internal/metering/adapters/sqlite/sqlitedb"
	appsavedquery "github.com/ssubedir/open-spanner/internal/metering/app/savedquery"
	"github.com/ssubedir/open-spanner/internal/metering/domain"
)

type SavedQueryRepository struct {
	queries *sqlitedb.Queries
}

func NewSavedQueryRepository(store *Store) *SavedQueryRepository {
	return &SavedQueryRepository{queries: sqlitedb.New(store)}
}

func (r *SavedQueryRepository) Save(ctx context.Context, query appsavedquery.SavedQuery) (appsavedquery.SavedQuery, error) {
	groupBy, err := json.Marshal(query.GroupBy)
	if err != nil {
		return appsavedquery.SavedQuery{}, err
	}

	err = queriesFor(ctx, r.queries).SaveSavedQuery(ctx, sqlitedb.SaveSavedQueryParams{
		ID:          query.ID,
		UserID:      query.UserID,
		Name:        query.Name,
		QueryJson:   string(query.Query),
		GroupBy:     string(groupBy),
		BucketSize:  query.BucketSize,
		ResultLimit: int64(query.Limit),
		Pinned:      int64(boolInt(query.Pinned)),
		Position:    int64(query.Position),
		CreatedAt:   formatTime(query.CreatedAt),
		UpdatedAt:   formatTime(query.UpdatedAt),
	})
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
		row, err := queriesFor(ctx, r.queries).FindSavedQueryByID(ctx, sqlitedb.FindSavedQueryByIDParams{
			UserID: query.UserID,
			ID:     query.ID,
		})
		saved, err := savedQueryFromFields(row.ID, row.UserID, row.Name, row.QueryJson, row.GroupBy, row.BucketSize, row.ResultLimit, row.Pinned, row.Position, row.CreatedAt, row.UpdatedAt, err)
		if err != nil {
			if errors.Is(err, domain.ErrNotFound) {
				return []appsavedquery.SavedQuery{}, nil
			}
			return nil, err
		}
		return []appsavedquery.SavedQuery{saved}, nil
	}

	rows, err := queriesFor(ctx, r.queries).ListSavedQueries(ctx, query.UserID)
	if err != nil {
		return nil, err
	}

	queries := make([]appsavedquery.SavedQuery, 0, len(rows))
	for _, row := range rows {
		saved, err := savedQueryFromFields(row.ID, row.UserID, row.Name, row.QueryJson, row.GroupBy, row.BucketSize, row.ResultLimit, row.Pinned, row.Position, row.CreatedAt, row.UpdatedAt, nil)
		if err != nil {
			return nil, err
		}
		queries = append(queries, saved)
	}
	return queries, nil
}

func (r *SavedQueryRepository) Delete(ctx context.Context, userID string, id string) error {
	rows, err := queriesFor(ctx, r.queries).DeleteSavedQuery(ctx, sqlitedb.DeleteSavedQueryParams{UserID: userID, ID: id})
	if err != nil {
		return err
	}
	if rows == 0 {
		return domain.ErrNotFound
	}
	return nil
}

func savedQueryFromFields(id string, userID string, name string, queryJSON string, groupByJSON string, bucketSize string, resultLimit int64, pinned int64, position int64, createdAt string, updatedAt string, err error) (appsavedquery.SavedQuery, error) {
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return appsavedquery.SavedQuery{}, domain.ErrNotFound
		}
		return appsavedquery.SavedQuery{}, err
	}

	query := appsavedquery.SavedQuery{
		ID:         id,
		UserID:     userID,
		Name:       name,
		Query:      json.RawMessage(queryJSON),
		BucketSize: bucketSize,
		Limit:      int(resultLimit),
		Pinned:     pinned != 0,
		Position:   int(position),
	}
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
