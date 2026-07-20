package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"time"

	appauth "github.com/ssubedir/open-spanner/internal/auth"
	"github.com/ssubedir/open-spanner/internal/metering/adapters/sqlite/sqlitedb"
	"github.com/ssubedir/open-spanner/internal/metering/domain"
	domainconsumption "github.com/ssubedir/open-spanner/internal/metering/domain/consumption"
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

func (r *ConsumptionRepository) Find(ctx context.Context, idempotencyKey string) (domainconsumption.Decision, error) {
	workspaceID, err := appauth.RequireWorkspaceID(ctx)
	if err != nil {
		return domainconsumption.Decision{}, err
	}
	response, err := r.queries.FindConsumptionDecision(ctx, sqlitedb.FindConsumptionDecisionParams{
		WorkspaceID: workspaceID, IdempotencyKey: idempotencyKey,
	})
	if errors.Is(err, sql.ErrNoRows) {
		return domainconsumption.Decision{}, domain.ErrNotFound
	}
	if err != nil {
		return domainconsumption.Decision{}, err
	}
	createdAt, err := time.Parse(time.RFC3339Nano, response.CreatedAt)
	if err != nil {
		return domainconsumption.Decision{}, err
	}
	return domainconsumption.FromSnapshot(idempotencyKey, []byte(response.Response), createdAt)
}

func (r *ConsumptionRepository) Save(ctx context.Context, idempotencyKey string, snapshot []byte) (domainconsumption.Decision, bool, error) {
	workspaceID, err := appauth.RequireWorkspaceID(ctx)
	if err != nil {
		return domainconsumption.Decision{}, false, err
	}
	createdAt := r.now()
	indexed, err := domainconsumption.FromSnapshot(idempotencyKey, snapshot, createdAt)
	if err != nil {
		return domainconsumption.Decision{}, false, err
	}
	rows, err := r.queries.SaveConsumptionDecision(ctx, sqlitedb.SaveConsumptionDecisionParams{
		WorkspaceID: workspaceID, IdempotencyKey: idempotencyKey, Response: string(snapshot), Subject: indexed.Subject,
		MeterName: indexed.MeterName, Accepted: boolInt64(indexed.Accepted), EvaluationFailed: boolInt64(indexed.EvaluationFailed),
		Enforcement: indexed.Enforcement, State: indexed.State, CreatedAt: createdAt.Format(time.RFC3339Nano),
	})
	if err != nil {
		return domainconsumption.Decision{}, false, err
	}
	stored, err := r.Find(ctx, idempotencyKey)
	return stored, rows == 1, err
}

func (r *ConsumptionRepository) List(ctx context.Context, query domainconsumption.Query) ([]domainconsumption.Decision, error) {
	workspaceID, err := appauth.RequireWorkspaceID(ctx)
	if err != nil {
		return nil, err
	}
	rows, err := r.queries.ListConsumptionDecisions(ctx, sqlitedb.ListConsumptionDecisionsParams{
		WorkspaceID: workspaceID, Subject: query.Subject, MeterName: query.MeterName,
		Accepted: nullableSQLiteBool(query.Accepted), EvaluationFailed: nullableSQLiteBool(query.EvaluationFailed),
		Enforcement: query.Enforcement, State: query.State, CursorCreatedAt: nullableSQLiteTime(query.CursorCreatedAt), CursorID: query.CursorID, Limit: int64(query.Limit),
	})
	if err != nil {
		return nil, err
	}
	result := make([]domainconsumption.Decision, 0, len(rows))
	for _, row := range rows {
		createdAt, err := time.Parse(time.RFC3339Nano, row.CreatedAt)
		if err != nil {
			return nil, err
		}
		result = append(result, domainconsumption.Decision{IdempotencyKey: row.IdempotencyKey, Snapshot: []byte(row.Response), Subject: row.Subject, MeterName: row.MeterName, Accepted: row.Accepted != 0, EvaluationFailed: row.EvaluationFailed != 0, Enforcement: row.Enforcement, State: row.State, CreatedAt: createdAt})
	}
	return result, nil
}

func nullableSQLiteBool(value *bool) any {
	if value == nil {
		return nil
	}
	return boolInt64(*value)
}

func nullableSQLiteTime(value time.Time) any {
	if value.IsZero() {
		return nil
	}
	return value.UTC().Format(time.RFC3339Nano)
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
