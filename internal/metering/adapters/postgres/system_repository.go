package postgres

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"

	appauth "github.com/ssubedir/open-spanner/internal/auth"
	"github.com/ssubedir/open-spanner/internal/metering/adapters/postgres/postgresdb"
	appsystem "github.com/ssubedir/open-spanner/internal/metering/app/system"
	"github.com/ssubedir/open-spanner/internal/metering/domain"
	domainusage "github.com/ssubedir/open-spanner/internal/metering/domain/usage"
)

type SystemRepository struct {
	store   *Store
	queries *postgresdb.Queries
}

func NewSystemRepository(store *Store) *SystemRepository {
	return &SystemRepository{store: store, queries: postgresdb.New(store)}
}

func (r *SystemRepository) ListWorkspaceIDs(ctx context.Context) ([]string, error) {
	rows, err := r.store.QueryContext(ctx, `SELECT id FROM auth_workspaces ORDER BY id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	workspaceIDs := []string{}
	for rows.Next() {
		var workspaceID string
		if err := rows.Scan(&workspaceID); err != nil {
			return nil, err
		}
		workspaceIDs = append(workspaceIDs, workspaceID)
	}
	return workspaceIDs, rows.Err()
}

func (r *SystemRepository) UpsertWorkerHeartbeat(ctx context.Context, heartbeat appsystem.WorkerHeartbeat) error {
	queries := queriesFor(ctx, r.queries)
	if err := queries.DeleteExpiredWorkerHeartbeats(ctx, heartbeat.LastHeartbeatAt.Add(-24*time.Hour)); err != nil {
		return err
	}
	return queries.UpsertWorkerHeartbeat(ctx, postgresdb.UpsertWorkerHeartbeatParams{WorkerName: heartbeat.Name, InstanceID: heartbeat.InstanceID, StartedAt: heartbeat.StartedAt, LastHeartbeatAt: heartbeat.LastHeartbeatAt})
}

func (r *SystemRepository) DeleteWorkerHeartbeat(ctx context.Context, workerName, instanceID string) error {
	return queriesFor(ctx, r.queries).DeleteWorkerHeartbeat(ctx, postgresdb.DeleteWorkerHeartbeatParams{WorkerName: workerName, InstanceID: instanceID})
}

func (r *SystemRepository) ListWorkerHeartbeats(ctx context.Context) ([]appsystem.WorkerHeartbeat, error) {
	rows, err := queriesFor(ctx, r.queries).ListWorkerHeartbeats(ctx)
	if err != nil {
		return nil, err
	}
	items := make([]appsystem.WorkerHeartbeat, 0, len(rows))
	for _, row := range rows {
		items = append(items, appsystem.WorkerHeartbeat{Name: row.WorkerName, InstanceID: row.InstanceID, StartedAt: row.StartedAt, LastHeartbeatAt: row.LastHeartbeatAt})
	}
	return items, nil
}

func (r *SystemRepository) ListWorkerDiagnostics(ctx context.Context, now time.Time) ([]appsystem.WorkerDiagnostics, error) {
	workspaceID, _ := appauth.WorkspaceIDFromContext(ctx)
	rows, err := queriesFor(ctx, r.queries).ListWorkerDiagnostics(ctx, postgresdb.ListWorkerDiagnosticsParams{Now: now, WorkspaceID: workspaceID})
	if err != nil {
		return nil, err
	}
	items := make([]appsystem.WorkerDiagnostics, 0, len(rows))
	for _, row := range rows {
		oldest, err := diagnosticTime(row.OldestPendingAt)
		if err != nil {
			return nil, err
		}
		lastSuccess, err := diagnosticTime(row.LastSuccessAt)
		if err != nil {
			return nil, err
		}
		lastFailure, err := diagnosticTime(row.LastFailureAt)
		if err != nil {
			return nil, err
		}
		items = append(items, appsystem.WorkerDiagnostics{
			Name: row.WorkerName, PendingJobs: int(row.PendingJobs), RunningJobs: int(row.RunningJobs), FailedJobs: int(row.FailedJobs),
			OldestPendingAt: oldest, LastSuccessAt: lastSuccess, LastFailureAt: lastFailure,
		})
	}
	return items, nil
}

func (r *SystemRepository) ListWorkerDeadLetters(ctx context.Context, limit int) ([]appsystem.WorkerDeadLetter, error) {
	workspaceID, err := appauth.RequireWorkspaceID(ctx)
	if err != nil {
		return nil, err
	}
	rows, err := queriesFor(ctx, r.queries).ListWorkerDeadLetters(ctx, postgresdb.ListWorkerDeadLettersParams{WorkspaceID: workspaceID, Limit: int32(limit)})
	if err != nil {
		return nil, err
	}
	items := make([]appsystem.WorkerDeadLetter, 0, len(rows))
	for _, row := range rows {
		items = append(items, postgresWorkerDeadLetter(row.PublicID.String(), row.WorkerName, row.JobKey, row.RuleID, row.Subject, row.MeterName, row.Attempts, row.LastError, row.Status, row.CreatedAt, row.RequeuedAt))
	}
	return items, nil
}

func (r *SystemRepository) GetWorkerDeadLetter(ctx context.Context, id string) (appsystem.WorkerDeadLetter, error) {
	workspaceID, err := appauth.RequireWorkspaceID(ctx)
	if err != nil {
		return appsystem.WorkerDeadLetter{}, err
	}
	publicID, err := uuid.Parse(id)
	if err != nil {
		return appsystem.WorkerDeadLetter{}, domain.ErrInvalidInput
	}
	row, err := queriesFor(ctx, r.queries).GetWorkerDeadLetter(ctx, postgresdb.GetWorkerDeadLetterParams{WorkspaceID: workspaceID, PublicID: publicID})
	if errors.Is(err, sql.ErrNoRows) {
		return appsystem.WorkerDeadLetter{}, domain.ErrNotFound
	}
	if err != nil {
		return appsystem.WorkerDeadLetter{}, err
	}
	return postgresWorkerDeadLetter(row.PublicID.String(), row.WorkerName, row.JobKey, row.RuleID, row.Subject, row.MeterName, row.Attempts, row.LastError, row.Status, row.CreatedAt, row.RequeuedAt), nil
}

func (r *SystemRepository) EnqueueWorkerDeadLetter(ctx context.Context, deadLetter appsystem.WorkerDeadLetter, now time.Time) (bool, error) {
	workspaceID, err := appauth.RequireWorkspaceID(ctx)
	if err != nil {
		return false, err
	}
	var rows int64
	publicID, err := uuid.Parse(deadLetter.ID)
	if err != nil {
		return false, domain.ErrInvalidInput
	}
	switch deadLetter.WorkerName {
	case "alert":
		rows, err = queriesFor(ctx, r.queries).EnqueueAlertWorkerDeadLetter(ctx, postgresdb.EnqueueAlertWorkerDeadLetterParams{Now: formatTime(now), WorkspaceID: workspaceID, PublicID: publicID})
	case "entitlement":
		rows, err = queriesFor(ctx, r.queries).EnqueueEntitlementWorkerDeadLetter(ctx, postgresdb.EnqueueEntitlementWorkerDeadLetterParams{Now: formatTime(now), WorkspaceID: workspaceID, PublicID: publicID})
	default:
		return false, domain.ErrInvalidInput
	}
	return rows == 1, err
}

func (r *SystemRepository) MarkWorkerDeadLetterRequeued(ctx context.Context, id string, now time.Time) (bool, error) {
	workspaceID, err := appauth.RequireWorkspaceID(ctx)
	if err != nil {
		return false, err
	}
	publicID, err := uuid.Parse(id)
	if err != nil {
		return false, domain.ErrInvalidInput
	}
	rows, err := queriesFor(ctx, r.queries).MarkWorkerDeadLetterRequeued(ctx, postgresdb.MarkWorkerDeadLetterRequeuedParams{Now: now, WorkspaceID: workspaceID, PublicID: publicID})
	return rows == 1, err
}

func postgresWorkerDeadLetter(id, workerName, jobKey, ruleID, subject, meterName string, attempts int32, lastError, status string, createdAt time.Time, requeuedAt sql.NullTime) appsystem.WorkerDeadLetter {
	return appsystem.WorkerDeadLetter{ID: id, WorkerName: workerName, JobKey: jobKey, RuleID: ruleID, Subject: subject, MeterName: meterName, Attempts: int(attempts), LastError: lastError, Status: status, CreatedAt: createdAt, RequeuedAt: requeuedAt.Time}
}

func diagnosticTime(value any) (time.Time, error) {
	var raw string
	switch typed := value.(type) {
	case string:
		raw = typed
	case []byte:
		raw = string(typed)
	case nil:
		return time.Time{}, nil
	case time.Time:
		return typed.UTC(), nil
	}
	if raw == "" {
		return time.Time{}, nil
	}
	for _, layout := range []string{time.RFC3339Nano, "2006-01-02 15:04:05.999999999Z07:00", "2006-01-02 15:04:05.999999999Z07"} {
		parsed, err := time.Parse(layout, raw)
		if err == nil {
			return parsed.UTC(), nil
		}
	}
	return time.Time{}, fmt.Errorf("parse diagnostic timestamp %q", raw)
}

func (r *SystemRepository) ClaimReconciliationSchedule(ctx context.Context, now, lockedUntil time.Time) (appsystem.ReconciliationClaim, bool, error) {
	if err := queriesFor(ctx, r.queries).EnsureReconciliationSchedules(ctx, now); err != nil {
		return appsystem.ReconciliationClaim{}, false, err
	}
	row, err := queriesFor(ctx, r.queries).ClaimReconciliationSchedule(ctx, postgresdb.ClaimReconciliationScheduleParams{Now: now, LockedUntil: lockedUntil})
	if errors.Is(err, sql.ErrNoRows) {
		return appsystem.ReconciliationClaim{}, false, nil
	}
	if err != nil {
		return appsystem.ReconciliationClaim{}, false, err
	}
	return appsystem.ReconciliationClaim{WorkspaceID: row.WorkspaceID, LastFingerprint: row.LastFingerprint, LastNotifiedFingerprint: row.LastNotifiedFingerprint, LastFailureFingerprint: row.LastFailureFingerprint}, true, nil
}

func (r *SystemRepository) SaveReconciliationRun(ctx context.Context, workspaceID string, run appsystem.ReconciliationRun) error {
	issues, err := json.Marshal(run.Issues)
	if err != nil {
		return err
	}
	return queriesFor(ctx, r.queries).SaveReconciliationRun(ctx, postgresdb.SaveReconciliationRunParams{
		ID: run.ID, WorkspaceID: workspaceID, Status: run.Status, DecisionsChecked: int32(run.DecisionsChecked),
		CountersChecked: int32(run.CountersChecked), IssueCount: int32(run.IssueCount), Truncated: run.Truncated,
		LookbackHours: int32(run.LookbackHours), DurationMs: run.Duration.Milliseconds(), Fingerprint: run.Fingerprint,
		Issues: issues, Error: run.Error, CreatedAt: run.CreatedAt,
	})
}

func (r *SystemRepository) CompleteReconciliationSchedule(ctx context.Context, workspaceID, fingerprint string, nextRunAt time.Time) error {
	return queriesFor(ctx, r.queries).CompleteReconciliationSchedule(ctx, postgresdb.CompleteReconciliationScheduleParams{WorkspaceID: workspaceID, Fingerprint: fingerprint, NextRunAt: nextRunAt, UpdatedAt: time.Now().UTC()})
}

func (r *SystemRepository) FailReconciliationSchedule(ctx context.Context, workspaceID, failureFingerprint string, nextRunAt time.Time) error {
	return queriesFor(ctx, r.queries).FailReconciliationSchedule(ctx, postgresdb.FailReconciliationScheduleParams{WorkspaceID: workspaceID, FailureFingerprint: failureFingerprint, NextRunAt: nextRunAt, UpdatedAt: time.Now().UTC()})
}

func (r *SystemRepository) GetReconciliationSchedule(ctx context.Context) (appsystem.ReconciliationSchedule, bool, error) {
	workspaceID, err := appauth.RequireWorkspaceID(ctx)
	if err != nil {
		return appsystem.ReconciliationSchedule{}, false, err
	}
	row, err := queriesFor(ctx, r.queries).GetReconciliationSchedule(ctx, workspaceID)
	if errors.Is(err, sql.ErrNoRows) {
		return appsystem.ReconciliationSchedule{}, false, nil
	}
	if err != nil {
		return appsystem.ReconciliationSchedule{}, false, err
	}
	return appsystem.ReconciliationSchedule{NextRunAt: row.NextRunAt, LockedUntil: row.LockedUntil.Time, UpdatedAt: row.UpdatedAt}, true, nil
}

func (r *SystemRepository) SaveReconciliationNotification(ctx context.Context, notification appsystem.ReconciliationNotification) error {
	payload, err := json.Marshal(notification.Run)
	if err != nil {
		return err
	}
	return queriesFor(ctx, r.queries).SaveReconciliationNotification(ctx, postgresdb.SaveReconciliationNotificationParams{ID: notification.ID, WorkspaceID: notification.WorkspaceID, EventType: notification.EventType, Fingerprint: notification.Fingerprint, Payload: payload, NextAttemptAt: notification.NextAttemptAt, CreatedAt: notification.CreatedAt})
}

func (r *SystemRepository) ClaimReconciliationNotification(ctx context.Context, now, lockedUntil time.Time) (appsystem.ReconciliationNotification, bool, error) {
	row, err := queriesFor(ctx, r.queries).ClaimReconciliationNotification(ctx, postgresdb.ClaimReconciliationNotificationParams{Now: now, LockedUntil: lockedUntil})
	if errors.Is(err, sql.ErrNoRows) {
		return appsystem.ReconciliationNotification{}, false, nil
	}
	if err != nil {
		return appsystem.ReconciliationNotification{}, false, err
	}
	var run appsystem.ReconciliationRun
	if err := json.Unmarshal(row.Payload, &run); err != nil {
		return appsystem.ReconciliationNotification{}, false, err
	}
	return appsystem.ReconciliationNotification{ID: row.ID, WorkspaceID: row.WorkspaceID, EventType: row.EventType, Fingerprint: row.Fingerprint, Run: run, Status: row.Status, Attempts: int(row.Attempts), TotalAttempts: int(row.TotalAttempts), NextAttemptAt: row.NextAttemptAt, LastError: row.LastError, CreatedAt: row.CreatedAt, DeliveredAt: row.DeliveredAt.Time}, true, nil
}

func (r *SystemRepository) CompleteReconciliationNotification(ctx context.Context, notification appsystem.ReconciliationNotification) error {
	return queriesFor(ctx, r.queries).CompleteReconciliationNotification(ctx, postgresdb.CompleteReconciliationNotificationParams{ID: notification.ID, DeliveredAt: time.Now().UTC()})
}

func (r *SystemRepository) RetryReconciliationNotification(ctx context.Context, notification appsystem.ReconciliationNotification, nextAttemptAt time.Time, maxAttempts int, deliveryErr error) error {
	return queriesFor(ctx, r.queries).RetryReconciliationNotification(ctx, postgresdb.RetryReconciliationNotificationParams{ID: notification.ID, MaxAttempts: int32(maxAttempts), NextAttemptAt: nextAttemptAt, LastError: deliveryErr.Error()})
}

func (r *SystemRepository) ListReconciliationNotifications(ctx context.Context, limit int) ([]appsystem.ReconciliationNotification, error) {
	workspaceID, err := appauth.RequireWorkspaceID(ctx)
	if err != nil {
		return nil, err
	}
	rows, err := queriesFor(ctx, r.queries).ListReconciliationNotifications(ctx, postgresdb.ListReconciliationNotificationsParams{WorkspaceID: workspaceID, Limit: int32(limit)})
	if err != nil {
		return nil, err
	}
	result := make([]appsystem.ReconciliationNotification, 0, len(rows))
	for _, row := range rows {
		var run appsystem.ReconciliationRun
		if err := json.Unmarshal(row.Payload, &run); err != nil {
			return nil, err
		}
		result = append(result, appsystem.ReconciliationNotification{ID: row.ID, WorkspaceID: workspaceID, EventType: row.EventType, Fingerprint: row.Fingerprint, Run: run, Status: row.Status, Attempts: int(row.Attempts), NextAttemptAt: row.NextAttemptAt, LockedUntil: row.LockedUntil.Time, LastError: row.LastError, CreatedAt: row.CreatedAt, DeliveredAt: row.DeliveredAt.Time})
	}
	return result, nil
}

func (r *SystemRepository) CountReconciliationNotifications(ctx context.Context) (appsystem.ReconciliationNotificationCounts, error) {
	workspaceID, err := appauth.RequireWorkspaceID(ctx)
	if err != nil {
		return appsystem.ReconciliationNotificationCounts{}, err
	}
	row, err := queriesFor(ctx, r.queries).CountReconciliationNotificationStates(ctx, workspaceID)
	if err != nil {
		return appsystem.ReconciliationNotificationCounts{}, err
	}
	return appsystem.ReconciliationNotificationCounts{Pending: int(row.Pending), DeadLetter: int(row.DeadLetter)}, nil
}

func (r *SystemRepository) RequeueReconciliationNotification(ctx context.Context, id string, nextAttemptAt time.Time) (bool, error) {
	workspaceID, err := appauth.RequireWorkspaceID(ctx)
	if err != nil {
		return false, err
	}
	rows, err := queriesFor(ctx, r.queries).RequeueReconciliationNotification(ctx, postgresdb.RequeueReconciliationNotificationParams{ID: id, WorkspaceID: workspaceID, NextAttemptAt: nextAttemptAt})
	return rows == 1, err
}

func (r *SystemRepository) SaveReconciliationNotificationAttempt(ctx context.Context, attempt appsystem.ReconciliationNotificationAttempt) error {
	return queriesFor(ctx, r.queries).SaveReconciliationNotificationAttempt(ctx, postgresdb.SaveReconciliationNotificationAttemptParams{ID: attempt.ID, NotificationID: attempt.NotificationID, Attempt: int32(attempt.Attempt), Status: attempt.Status, Error: attempt.Error, CreatedAt: attempt.CreatedAt})
}

func (r *SystemRepository) ListReconciliationNotificationAttempts(ctx context.Context, notificationID string) ([]appsystem.ReconciliationNotificationAttempt, error) {
	rows, err := queriesFor(ctx, r.queries).ListReconciliationNotificationAttempts(ctx, notificationID)
	if err != nil {
		return nil, err
	}
	result := make([]appsystem.ReconciliationNotificationAttempt, 0, len(rows))
	for _, row := range rows {
		result = append(result, appsystem.ReconciliationNotificationAttempt{ID: row.ID, NotificationID: notificationID, Attempt: int(row.Attempt), Status: row.Status, Error: row.Error, CreatedAt: row.CreatedAt})
	}
	return result, nil
}

func (r *SystemRepository) MarkReconciliationNotified(ctx context.Context, workspaceID, fingerprint string) error {
	return queriesFor(ctx, r.queries).MarkReconciliationNotified(ctx, postgresdb.MarkReconciliationNotifiedParams{WorkspaceID: workspaceID, Fingerprint: fingerprint, UpdatedAt: time.Now().UTC()})
}

func (r *SystemRepository) ListReconciliationRuns(ctx context.Context, limit int) ([]appsystem.ReconciliationRun, error) {
	workspaceID, err := appauth.RequireWorkspaceID(ctx)
	if err != nil {
		return nil, err
	}
	rows, err := queriesFor(ctx, r.queries).ListReconciliationRuns(ctx, postgresdb.ListReconciliationRunsParams{WorkspaceID: workspaceID, Limit: int32(limit)})
	if err != nil {
		return nil, err
	}
	runs := make([]appsystem.ReconciliationRun, 0, len(rows))
	for _, row := range rows {
		var issues []appsystem.ReconciliationIssue
		if err := json.Unmarshal(row.Issues, &issues); err != nil {
			return nil, err
		}
		runs = append(runs, appsystem.ReconciliationRun{ID: row.ID, Status: row.Status, DecisionsChecked: int(row.DecisionsChecked), CountersChecked: int(row.CountersChecked), IssueCount: int(row.IssueCount), Truncated: row.Truncated, LookbackHours: int(row.LookbackHours), Duration: time.Duration(row.DurationMs) * time.Millisecond, Fingerprint: row.Fingerprint, Issues: issues, Error: row.Error, CreatedAt: row.CreatedAt})
	}
	return runs, nil
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
	cleanupCount, err := queriesFor(ctx, r.queries).CountUsageExportCleanupRuns(ctx, workspaceID)
	if err != nil {
		return appsystem.StatsResult{}, err
	}
	result.ExportCleanupRuns = int(cleanupCount)
	cleanup, err := queriesFor(ctx, r.queries).FindLatestUsageExportCleanupRun(ctx, workspaceID)
	if err == nil {
		expiredBefore, beforeErr := time.Parse(time.RFC3339Nano, cleanup.ExpiredBefore)
		createdAt, createdErr := time.Parse(time.RFC3339Nano, cleanup.CreatedAt)
		if beforeErr != nil || createdErr != nil {
			return appsystem.StatsResult{}, errors.Join(beforeErr, createdErr)
		}
		result.LastExportCleanupRun = appsystem.LastExportCleanupRunResult{ID: cleanup.PublicID.String(), ExpiredBefore: expiredBefore, FilesDeleted: int(cleanup.FilesDeleted), BytesReclaimed: cleanup.BytesReclaimed, Failures: int(cleanup.Failures), CreatedAt: createdAt}
	} else if !errors.Is(err, sql.ErrNoRows) {
		return appsystem.StatsResult{}, err
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
