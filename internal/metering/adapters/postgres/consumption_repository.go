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

func (r *ConsumptionRepository) CountExpired(ctx context.Context, before time.Time) (int, error) {
	workspaceID, err := appauth.RequireWorkspaceID(ctx)
	if err != nil {
		return 0, err
	}
	count, err := r.queries.CountExpiredConsumptionDecisions(ctx, postgresdb.CountExpiredConsumptionDecisionsParams{WorkspaceID: workspaceID, Before: before})
	return int(count), err
}

func (r *ConsumptionRepository) PruneExpired(ctx context.Context, before time.Time) (int, error) {
	workspaceID, err := appauth.RequireWorkspaceID(ctx)
	if err != nil {
		return 0, err
	}
	count, err := r.queries.PruneExpiredConsumptionDecisions(ctx, postgresdb.PruneExpiredConsumptionDecisionsParams{WorkspaceID: workspaceID, Before: before})
	return int(count), err
}

func (r *ConsumptionRepository) SavePruneRun(ctx context.Context, id string, before time.Time, dryRun bool, deleted int, createdAt time.Time) error {
	workspaceID, err := appauth.RequireWorkspaceID(ctx)
	if err != nil {
		return err
	}
	return r.queries.SaveConsumptionDecisionPruneRun(ctx, postgresdb.SaveConsumptionDecisionPruneRunParams{
		ID: id, WorkspaceID: workspaceID, Before: before, DryRun: dryRun, Deleted: int64(deleted), CreatedAt: createdAt,
	})
}
