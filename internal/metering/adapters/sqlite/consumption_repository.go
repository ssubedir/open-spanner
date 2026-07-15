package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"time"

	appauth "github.com/ssubedir/open-spanner/internal/auth"
	"github.com/ssubedir/open-spanner/internal/metering/adapters/sqlite/sqlitedb"
	"github.com/ssubedir/open-spanner/internal/metering/domain"
)

type ConsumptionRepository struct {
	queries *sqlitedb.Queries
	now     func() time.Time
}

func NewConsumptionRepository(store *Store) *ConsumptionRepository {
	return &ConsumptionRepository{
		queries: sqlitedb.New(store),
		now:     func() time.Time { return time.Now().UTC() },
	}
}

func (r *ConsumptionRepository) Find(ctx context.Context, idempotencyKey string) ([]byte, error) {
	workspaceID, err := appauth.RequireWorkspaceID(ctx)
	if err != nil {
		return nil, err
	}
	response, err := r.queries.FindConsumptionDecision(ctx, sqlitedb.FindConsumptionDecisionParams{
		WorkspaceID: workspaceID, IdempotencyKey: idempotencyKey,
	})
	if errors.Is(err, sql.ErrNoRows) {
		return nil, domain.ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return []byte(response), nil
}

func (r *ConsumptionRepository) Save(ctx context.Context, idempotencyKey string, snapshot []byte) ([]byte, bool, error) {
	workspaceID, err := appauth.RequireWorkspaceID(ctx)
	if err != nil {
		return nil, false, err
	}
	rows, err := r.queries.SaveConsumptionDecision(ctx, sqlitedb.SaveConsumptionDecisionParams{
		WorkspaceID: workspaceID, IdempotencyKey: idempotencyKey, Response: string(snapshot), CreatedAt: r.now().Format(time.RFC3339Nano),
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
	count, err := r.queries.CountExpiredConsumptionDecisions(ctx, sqlitedb.CountExpiredConsumptionDecisionsParams{WorkspaceID: workspaceID, Before: before.Format(time.RFC3339Nano)})
	return int(count), err
}

func (r *ConsumptionRepository) PruneExpired(ctx context.Context, before time.Time) (int, error) {
	workspaceID, err := appauth.RequireWorkspaceID(ctx)
	if err != nil {
		return 0, err
	}
	count, err := r.queries.PruneExpiredConsumptionDecisions(ctx, sqlitedb.PruneExpiredConsumptionDecisionsParams{WorkspaceID: workspaceID, Before: before.Format(time.RFC3339Nano)})
	return int(count), err
}

func (r *ConsumptionRepository) SavePruneRun(ctx context.Context, id string, before time.Time, dryRun bool, deleted int, createdAt time.Time) error {
	workspaceID, err := appauth.RequireWorkspaceID(ctx)
	if err != nil {
		return err
	}
	return r.queries.SaveConsumptionDecisionPruneRun(ctx, sqlitedb.SaveConsumptionDecisionPruneRunParams{
		ID: id, WorkspaceID: workspaceID, Before: before.Format(time.RFC3339Nano), DryRun: boolInt64(dryRun), Deleted: int64(deleted), CreatedAt: createdAt.Format(time.RFC3339Nano),
	})
}

func boolInt64(value bool) int64 {
	if value {
		return 1
	}
	return 0
}
