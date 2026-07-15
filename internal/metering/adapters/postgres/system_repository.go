package postgres

import (
	"context"
	"database/sql"
	"errors"
	"time"

	appauth "github.com/ssubedir/open-spanner/internal/auth"
	"github.com/ssubedir/open-spanner/internal/metering/adapters/postgres/postgresdb"
	appsystem "github.com/ssubedir/open-spanner/internal/metering/app/system"
	domainusage "github.com/ssubedir/open-spanner/internal/metering/domain/usage"
)

type SystemRepository struct {
	queries *postgresdb.Queries
}

func NewSystemRepository(store *Store) *SystemRepository {
	return &SystemRepository{queries: postgresdb.New(store)}
}

func (r *SystemRepository) FindStats(ctx context.Context) (appsystem.StatsResult, error) {
	workspaceID, err := appauth.RequireWorkspaceID(ctx)
	if err != nil {
		return appsystem.StatsResult{}, err
	}

	now := formatTime(time.Now().UTC())
	if err := queriesFor(ctx, r.queries).EnsureWorkspaceStats(ctx, postgresdb.EnsureWorkspaceStatsParams{
		WorkspaceID: workspaceID,
		UpdatedAt:   now,
	}); err != nil {
		return appsystem.StatsResult{}, err
	}

	stats, err := queriesFor(ctx, r.queries).GetWorkspaceStats(ctx, workspaceID)
	if err != nil {
		return appsystem.StatsResult{}, err
	}

	result := appsystem.StatsResult{
		Meters:      int(stats.Meters),
		UsageEvents: int(stats.UsageEvents),
		PruneRuns:   int(stats.PruneRuns),
	}
	decisionCount, err := r.queries.CountConsumptionDecisions(ctx, workspaceID)
	if err != nil {
		return appsystem.StatsResult{}, err
	}
	decisionRunCount, err := r.queries.CountConsumptionDecisionPruneRuns(ctx, workspaceID)
	if err != nil {
		return appsystem.StatsResult{}, err
	}
	result.ConsumptionDecisions = int(decisionCount)
	result.DecisionPruneRuns = int(decisionRunCount)
	decisionRun, err := r.queries.FindLatestConsumptionDecisionPruneRun(ctx, workspaceID)
	if err == nil {
		result.LastDecisionPruneRun = appsystem.LastDecisionPruneRunResult{ID: decisionRun.ID, Before: decisionRun.Before, Deleted: int(decisionRun.Deleted), DryRun: decisionRun.DryRun, CreatedAt: decisionRun.CreatedAt}
	} else if !errors.Is(err, sql.ErrNoRows) {
		return appsystem.StatsResult{}, err
	}

	runs, err := queriesFor(ctx, r.queries).ListUsagePruneRuns(ctx, postgresdb.ListUsagePruneRunsParams{
		WorkspaceID: workspaceID,
		Limit:       1,
	})
	if err != nil {
		return appsystem.StatsResult{}, err
	}
	if len(runs) > 0 {
		run, err := pruneRunFromFields(runs[0].ID, runs[0].DryRun, runs[0].Deleted, runs[0].Meters, runs[0].CreatedAt)
		if err != nil {
			return appsystem.StatsResult{}, err
		}
		result.LastPruneRun = lastPruneRunFromDomain(run)
	}

	return result, nil
}

func (r *SystemRepository) ListDecisionReconciliationRows(ctx context.Context, since time.Time, limit int) ([]appsystem.DecisionReconciliationRow, error) {
	workspaceID, err := appauth.RequireWorkspaceID(ctx)
	if err != nil {
		return nil, err
	}
	rows, err := queriesFor(ctx, r.queries).ListDecisionReconciliationRows(ctx, postgresdb.ListDecisionReconciliationRowsParams{WorkspaceID: workspaceID, Since: since, Limit: int32(limit)})
	if err != nil {
		return nil, err
	}
	result := make([]appsystem.DecisionReconciliationRow, 0, len(rows))
	for _, row := range rows {
		result = append(result, appsystem.DecisionReconciliationRow{
			IdempotencyKey: row.IdempotencyKey, Subject: row.Subject, MeterName: row.MeterName,
			Accepted: row.Accepted, CreatedAt: row.CreatedAt, EventID: row.EventID.String,
			EventSubject: row.EventSubject.String, EventMeterName: row.EventMeterName.String,
		})
	}
	return result, nil
}

func (r *SystemRepository) ListActiveEntitlementCounters(ctx context.Context, now time.Time, limit int) ([]appsystem.CounterReconciliationRow, error) {
	workspaceID, err := appauth.RequireWorkspaceID(ctx)
	if err != nil {
		return nil, err
	}
	rows, err := queriesFor(ctx, r.queries).ListActiveEntitlementCounters(ctx, postgresdb.ListActiveEntitlementCountersParams{WorkspaceID: workspaceID, Now: now, Limit: int32(limit)})
	if err != nil {
		return nil, err
	}
	result := make([]appsystem.CounterReconciliationRow, 0, len(rows))
	for _, row := range rows {
		start, startErr := parseEntitlementTime(row.PeriodStart)
		end, endErr := parseEntitlementTime(row.PeriodEnd)
		updated, updatedErr := parseEntitlementTime(row.UpdatedAt)
		if err := errors.Join(startErr, endErr, updatedErr); err != nil {
			return nil, err
		}
		result = append(result, appsystem.CounterReconciliationRow{
			Subject: row.Subject, MeterName: row.MeterName, Period: row.Period, PeriodStart: start, PeriodEnd: end,
			EventCount: row.EventCount, QuantitySum: row.QuantitySum, QuantityMin: row.QuantityMin, QuantityMax: row.QuantityMax, UpdatedAt: updated,
		})
	}
	return result, nil
}

func (r *SystemRepository) ListCounterReconciliationEvents(ctx context.Context, counter appsystem.CounterReconciliationRow) ([]appsystem.ReconciliationEvent, error) {
	workspaceID, err := appauth.RequireWorkspaceID(ctx)
	if err != nil {
		return nil, err
	}
	rows, err := queriesFor(ctx, r.queries).ListCounterReconciliationEvents(ctx, postgresdb.ListCounterReconciliationEventsParams{
		WorkspaceID: workspaceID, Subject: counter.Subject, MeterName: counter.MeterName,
		PeriodStart: counter.PeriodStart, PeriodEnd: counter.PeriodEnd,
	})
	if err != nil {
		return nil, err
	}
	result := make([]appsystem.ReconciliationEvent, 0, len(rows))
	for _, row := range rows {
		eventTime, err := parseEntitlementTime(row.EventTime)
		if err != nil {
			return nil, err
		}
		result = append(result, appsystem.ReconciliationEvent{ID: row.ID, Quantity: row.Quantity, EventTime: eventTime})
	}
	return result, nil
}

func (r *SystemRepository) ListCounterReconciliationAssignments(ctx context.Context, counter appsystem.CounterReconciliationRow) ([]appsystem.ReconciliationAssignment, error) {
	workspaceID, err := appauth.RequireWorkspaceID(ctx)
	if err != nil {
		return nil, err
	}
	rows, err := queriesFor(ctx, r.queries).ListCounterReconciliationAssignments(ctx, postgresdb.ListCounterReconciliationAssignmentsParams{
		WorkspaceID: workspaceID, Subject: counter.Subject, WindowStart: counter.PeriodStart, WindowEnd: counter.PeriodEnd,
	})
	if err != nil {
		return nil, err
	}
	result := make([]appsystem.ReconciliationAssignment, 0, len(rows))
	for _, row := range rows {
		assigned, assignedErr := parseEntitlementTime(row.AssignedAt)
		anchor, anchorErr := parseEntitlementTime(row.PeriodAnchorAt)
		unassigned, unassignedErr := parseNullableEntitlementTime(row.UnassignedAt)
		if err := errors.Join(assignedErr, anchorErr, unassignedErr); err != nil {
			return nil, err
		}
		result = append(result, appsystem.ReconciliationAssignment{ID: row.ID, AssignedAt: assigned, PeriodAnchorAt: anchor, UnassignedAt: unassigned})
	}
	return result, nil
}

func lastPruneRunFromDomain(run domainusage.PruneRun) appsystem.LastPruneRunResult {
	return appsystem.LastPruneRunResult{
		ID:        run.ID(),
		Deleted:   run.Deleted(),
		DryRun:    run.DryRun(),
		CreatedAt: run.CreatedAt(),
	}
}
