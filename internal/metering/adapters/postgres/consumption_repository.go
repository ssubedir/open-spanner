package postgres

import (
	"context"
	"database/sql"
	"errors"
	"time"

	appauth "github.com/ssubedir/open-spanner/internal/auth"
	"github.com/ssubedir/open-spanner/internal/metering/adapters/postgres/postgresdb"
	"github.com/ssubedir/open-spanner/internal/metering/domain"
	domainconsumption "github.com/ssubedir/open-spanner/internal/metering/domain/consumption"
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

func (r *ConsumptionRepository) Find(ctx context.Context, idempotencyKey string) (domainconsumption.Decision, error) {
	workspaceID, err := appauth.RequireWorkspaceID(ctx)
	if err != nil {
		return domainconsumption.Decision{}, err
	}
	response, err := r.queries.FindConsumptionDecision(ctx, postgresdb.FindConsumptionDecisionParams{
		WorkspaceID: workspaceID, IdempotencyKey: idempotencyKey,
	})
	if errors.Is(err, sql.ErrNoRows) {
		return domainconsumption.Decision{}, domain.ErrNotFound
	}
	if err != nil {
		return domainconsumption.Decision{}, err
	}
	return domainconsumption.FromSnapshot(idempotencyKey, response.Response, response.CreatedAt)
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
	rows, err := r.queries.SaveConsumptionDecision(ctx, postgresdb.SaveConsumptionDecisionParams{
		WorkspaceID: workspaceID, IdempotencyKey: idempotencyKey, Response: snapshot, Subject: indexed.Subject,
		MeterName: indexed.MeterName, Accepted: indexed.Accepted, EvaluationFailed: indexed.EvaluationFailed,
		Enforcement: indexed.Enforcement, State: indexed.State, CreatedAt: createdAt,
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
	rows, err := r.queries.ListConsumptionDecisions(ctx, postgresdb.ListConsumptionDecisionsParams{
		WorkspaceID: workspaceID, Subject: query.Subject, MeterName: query.MeterName,
		Accepted: nullableBool(query.Accepted), EvaluationFailed: nullableBool(query.EvaluationFailed),
		Enforcement: query.Enforcement, State: query.State, CursorCreatedAt: sql.NullTime{Time: query.CursorCreatedAt, Valid: !query.CursorCreatedAt.IsZero()},
		CursorID: query.CursorID, Limit: int32(query.Limit),
	})
	if err != nil {
		return nil, err
	}
	result := make([]domainconsumption.Decision, 0, len(rows))
	for _, row := range rows {
		result = append(result, domainconsumption.Decision{IdempotencyKey: row.IdempotencyKey, Snapshot: row.Response, Subject: row.Subject, MeterName: row.MeterName, Accepted: row.Accepted, EvaluationFailed: row.EvaluationFailed, Enforcement: row.Enforcement, State: row.State, CreatedAt: row.CreatedAt})
	}
	return result, nil
}

func nullableBool(value *bool) sql.NullBool {
	if value == nil {
		return sql.NullBool{}
	}
	return sql.NullBool{Bool: *value, Valid: true}
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
