package postgres

import (
	"context"
	"database/sql"
	"errors"
	"time"

	appauth "github.com/ssubedir/open-spanner/internal/auth"
	"github.com/ssubedir/open-spanner/internal/metering/adapters/postgres/postgresdb"
	"github.com/ssubedir/open-spanner/internal/metering/domain"
)

type ConsumptionRepository struct {
	queries *postgresdb.Queries
	now     func() time.Time
}

func NewConsumptionRepository(store *Store) *ConsumptionRepository {
	return &ConsumptionRepository{
		queries: postgresdb.New(store),
		now:     func() time.Time { return time.Now().UTC() },
	}
}

func (r *ConsumptionRepository) Find(ctx context.Context, idempotencyKey string) ([]byte, error) {
	workspaceID, err := appauth.RequireWorkspaceID(ctx)
	if err != nil {
		return nil, err
	}
	response, err := r.queries.FindConsumptionDecision(ctx, postgresdb.FindConsumptionDecisionParams{
		WorkspaceID: workspaceID, IdempotencyKey: idempotencyKey,
	})
	if errors.Is(err, sql.ErrNoRows) {
		return nil, domain.ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return response, nil
}

func (r *ConsumptionRepository) Save(ctx context.Context, idempotencyKey string, snapshot []byte) ([]byte, bool, error) {
	workspaceID, err := appauth.RequireWorkspaceID(ctx)
	if err != nil {
		return nil, false, err
	}
	rows, err := r.queries.SaveConsumptionDecision(ctx, postgresdb.SaveConsumptionDecisionParams{
		WorkspaceID: workspaceID, IdempotencyKey: idempotencyKey, Response: snapshot, CreatedAt: r.now(),
	})
	if err != nil {
		return nil, false, err
	}
	stored, err := r.Find(ctx, idempotencyKey)
	return stored, rows == 1, err
}
