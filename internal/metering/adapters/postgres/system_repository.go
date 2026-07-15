package postgres

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"time"

	appauth "github.com/ssubedir/open-spanner/internal/auth"
	"github.com/ssubedir/open-spanner/internal/metering/adapters/postgres/postgresdb"
	appsystem "github.com/ssubedir/open-spanner/internal/metering/app/system"
	"github.com/ssubedir/open-spanner/internal/metering/domain"
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
			EventRetentionDays: int(row.EventRetentionDays),
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
		receivedAt, err := parseEntitlementTime(row.ReceivedAt)
		if err != nil {
			return nil, err
		}
		result = append(result, appsystem.ReconciliationEvent{ID: row.ID, Quantity: row.Quantity, EventTime: eventTime, ReceivedAt: receivedAt})
	}
	return result, nil
}

func (r *SystemRepository) GetEntitlementCounterForRepair(ctx context.Context, target appsystem.CounterRepairTarget) (appsystem.CounterReconciliationRow, error) {
	workspaceID, err := appauth.RequireWorkspaceID(ctx)
	if err != nil {
		return appsystem.CounterReconciliationRow{}, err
	}
	row, err := queriesFor(ctx, r.queries).GetEntitlementCounterForRepair(ctx, postgresdb.GetEntitlementCounterForRepairParams{
		WorkspaceID: workspaceID, Subject: target.Subject, MeterName: target.MeterName, Period: target.Period, PeriodStart: formatTime(target.PeriodStart),
	})
	if errors.Is(err, sql.ErrNoRows) {
		return appsystem.CounterReconciliationRow{}, domain.ErrNotFound
	}
	if err != nil {
		return appsystem.CounterReconciliationRow{}, err
	}
	start, e1 := parseEntitlementTime(row.PeriodStart)
	end, e2 := parseEntitlementTime(row.PeriodEnd)
	first, e3 := parseEntitlementTime(row.FirstEventTime)
	last, e4 := parseEntitlementTime(row.LastEventTime)
	updated, e5 := parseEntitlementTime(row.UpdatedAt)
	if err := errors.Join(e1, e2, e3, e4, e5); err != nil {
		return appsystem.CounterReconciliationRow{}, err
	}
	return appsystem.CounterReconciliationRow{
		Subject: row.Subject, MeterName: row.MeterName, Period: row.Period, PeriodStart: start, PeriodEnd: end,
		EventCount: row.EventCount, QuantitySum: row.QuantitySum, QuantityMin: row.QuantityMin, QuantityMax: row.QuantityMax,
		FirstQuantity: row.FirstQuantity, FirstEventTime: first, LastQuantity: row.LastQuantity, LastEventTime: last,
		UpdatedAt: updated, EventRetentionDays: int(row.EventRetentionDays),
	}, nil
}

func (r *SystemRepository) UpdateEntitlementCounterForRepair(ctx context.Context, counter appsystem.CounterReconciliationRow, expectedUpdatedAt time.Time, replacement appsystem.CounterSnapshot, updatedAt time.Time) (bool, error) {
	workspaceID, err := appauth.RequireWorkspaceID(ctx)
	if err != nil {
		return false, err
	}
	if replacement.EventCount == 0 {
		rows, err := queriesFor(ctx, r.queries).DeleteEntitlementCounterForRepair(ctx, postgresdb.DeleteEntitlementCounterForRepairParams{
			WorkspaceID: workspaceID, Subject: counter.Subject, MeterName: counter.MeterName, Period: counter.Period,
			PeriodStart: formatTime(counter.PeriodStart), ExpectedUpdatedAt: formatTime(expectedUpdatedAt),
		})
		return rows == 1, err
	}
	rows, err := queriesFor(ctx, r.queries).UpdateEntitlementCounterForRepair(ctx, postgresdb.UpdateEntitlementCounterForRepairParams{
		EventCount: replacement.EventCount, QuantitySum: replacement.QuantitySum, QuantityMin: replacement.QuantityMin, QuantityMax: replacement.QuantityMax,
		FirstQuantity: replacement.FirstQuantity, FirstEventTime: formatTime(replacement.FirstEventTime), LastQuantity: replacement.LastQuantity, LastEventTime: formatTime(replacement.LastEventTime),
		UpdatedAt: formatTime(updatedAt), WorkspaceID: workspaceID, Subject: counter.Subject, MeterName: counter.MeterName,
		Period: counter.Period, PeriodStart: formatTime(counter.PeriodStart), ExpectedUpdatedAt: formatTime(expectedUpdatedAt),
	})
	return rows == 1, err
}

func (r *SystemRepository) SaveQuotaCounterRepairRun(ctx context.Context, run appsystem.CounterRepairResult) error {
	workspaceID, err := appauth.RequireWorkspaceID(ctx)
	if err != nil {
		return err
	}
	before, err := json.Marshal(run.Before)
	if err != nil {
		return err
	}
	after, err := json.Marshal(run.After)
	if err != nil {
		return err
	}
	return queriesFor(ctx, r.queries).SaveQuotaCounterRepairRun(ctx, postgresdb.SaveQuotaCounterRepairRunParams{
		ID: run.ID, WorkspaceID: workspaceID, Subject: run.Subject, MeterName: run.MeterName, Period: run.Period,
		PeriodStart: formatTime(run.PeriodStart), PeriodEnd: formatTime(run.PeriodEnd), DryRun: run.DryRun, Applied: run.Applied,
		BeforeSnapshot: before, AfterSnapshot: after, CounterUpdatedAt: formatTime(run.CounterUpdatedAt), CreatedAt: run.CreatedAt,
	})
}

func (r *SystemRepository) ListQuotaCounterRepairRuns(ctx context.Context, limit int) ([]appsystem.CounterRepairResult, error) {
	workspaceID, err := appauth.RequireWorkspaceID(ctx)
	if err != nil {
		return nil, err
	}
	rows, err := queriesFor(ctx, r.queries).ListQuotaCounterRepairRuns(ctx, postgresdb.ListQuotaCounterRepairRunsParams{WorkspaceID: workspaceID, Limit: int32(limit)})
	if err != nil {
		return nil, err
	}
	result := make([]appsystem.CounterRepairResult, 0, len(rows))
	for _, row := range rows {
		start, e1 := parseEntitlementTime(row.PeriodStart)
		end, e2 := parseEntitlementTime(row.PeriodEnd)
		counterUpdated, e3 := parseEntitlementTime(row.CounterUpdatedAt)
		if err := errors.Join(e1, e2, e3); err != nil {
			return nil, err
		}
		var before, after appsystem.CounterSnapshot
		if err := errors.Join(json.Unmarshal(row.BeforeSnapshot, &before), json.Unmarshal(row.AfterSnapshot, &after)); err != nil {
			return nil, err
		}
		result = append(result, appsystem.CounterRepairResult{ID: row.ID, CounterRepairTarget: appsystem.CounterRepairTarget{Subject: row.Subject, MeterName: row.MeterName, Period: row.Period, PeriodStart: start}, PeriodEnd: end, DryRun: row.DryRun, Applied: row.Applied, Before: before, After: after, CounterUpdatedAt: counterUpdated, CreatedAt: row.CreatedAt})
	}
	return result, nil
}

func (r *SystemRepository) FindLatestMeterPruneCutoff(ctx context.Context, meterName string) (time.Time, error) {
	workspaceID, err := appauth.RequireWorkspaceID(ctx)
	if err != nil {
		return time.Time{}, err
	}
	var latest time.Time
	var cursorCreatedAt, cursorID sql.NullString
	for {
		rows, err := queriesFor(ctx, r.queries).ListUsagePruneRuns(ctx, postgresdb.ListUsagePruneRunsParams{
			WorkspaceID: workspaceID, CursorCreatedAt: cursorCreatedAt, CursorID: cursorID, Limit: 200,
		})
		if err != nil {
			return time.Time{}, err
		}
		for _, row := range rows {
			run, err := pruneRunFromFields(row.ID, row.DryRun, row.Deleted, row.Meters, row.CreatedAt)
			if err != nil {
				return time.Time{}, err
			}
			if run.DryRun() {
				continue
			}
			for _, meter := range run.Meters() {
				if meter.MeterName() == meterName && meter.Deleted() > 0 && meter.Before().After(latest) {
					latest = meter.Before()
				}
			}
		}
		if len(rows) < 200 {
			break
		}
		last := rows[len(rows)-1]
		cursorCreatedAt, cursorID = sql.NullString{String: last.CreatedAt, Valid: true}, sql.NullString{String: last.ID, Valid: true}
	}
	return latest, nil
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
