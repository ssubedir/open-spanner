package sqlite

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"time"

	appauth "github.com/ssubedir/open-spanner/internal/auth"
	"github.com/ssubedir/open-spanner/internal/metering/adapters/sqlite/sqlitedb"
	appsystem "github.com/ssubedir/open-spanner/internal/metering/app/system"
	"github.com/ssubedir/open-spanner/internal/metering/domain"
	domainusage "github.com/ssubedir/open-spanner/internal/metering/domain/usage"
)

type SystemRepository struct {
	queries *sqlitedb.Queries
}

func NewSystemRepository(store *Store) *SystemRepository {
	return &SystemRepository{queries: sqlitedb.New(store)}
}

func (r *SystemRepository) UpsertWorkerHeartbeat(ctx context.Context, heartbeat appsystem.WorkerHeartbeat) error {
	return queriesFor(ctx, r.queries).UpsertWorkerHeartbeat(ctx, sqlitedb.UpsertWorkerHeartbeatParams{WorkerName: heartbeat.Name, StartedAt: formatTime(heartbeat.StartedAt), LastHeartbeatAt: formatTime(heartbeat.LastHeartbeatAt)})
}

func (r *SystemRepository) ListWorkerHeartbeats(ctx context.Context) ([]appsystem.WorkerHeartbeat, error) {
	rows, err := queriesFor(ctx, r.queries).ListWorkerHeartbeats(ctx)
	if err != nil {
		return nil, err
	}
	items := make([]appsystem.WorkerHeartbeat, 0, len(rows))
	for _, row := range rows {
		started, err := time.Parse(time.RFC3339Nano, row.StartedAt)
		if err != nil {
			return nil, err
		}
		last, err := time.Parse(time.RFC3339Nano, row.LastHeartbeatAt)
		if err != nil {
			return nil, err
		}
		items = append(items, appsystem.WorkerHeartbeat{Name: row.WorkerName, StartedAt: started, LastHeartbeatAt: last})
	}
	return items, nil
}

func (r *SystemRepository) ListWorkerDiagnostics(ctx context.Context, now time.Time) ([]appsystem.WorkerDiagnostics, error) {
	rows, err := queriesFor(ctx, r.queries).ListWorkerDiagnostics(ctx, sql.NullString{String: formatTime(now), Valid: true})
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
			Name: row.WorkerName, PendingJobs: diagnosticInt(row.PendingJobs), RunningJobs: diagnosticInt(row.RunningJobs), FailedJobs: diagnosticInt(row.FailedJobs),
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
	rows, err := queriesFor(ctx, r.queries).ListWorkerDeadLetters(ctx, sqlitedb.ListWorkerDeadLettersParams{WorkspaceID: workspaceID, Limit: int64(limit)})
	if err != nil {
		return nil, err
	}
	items := make([]appsystem.WorkerDeadLetter, 0, len(rows))
	for _, row := range rows {
		item, err := sqliteWorkerDeadLetter(row.PublicID, row.WorkerName, row.JobKey, row.RuleID, row.Subject, row.MeterName, row.Attempts, row.LastError, row.Status, row.CreatedAt, row.RequeuedAt)
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, nil
}

func (r *SystemRepository) GetWorkerDeadLetter(ctx context.Context, id string) (appsystem.WorkerDeadLetter, error) {
	workspaceID, err := appauth.RequireWorkspaceID(ctx)
	if err != nil {
		return appsystem.WorkerDeadLetter{}, err
	}
	row, err := queriesFor(ctx, r.queries).GetWorkerDeadLetter(ctx, sqlitedb.GetWorkerDeadLetterParams{WorkspaceID: workspaceID, PublicID: id})
	if errors.Is(err, sql.ErrNoRows) {
		return appsystem.WorkerDeadLetter{}, domain.ErrNotFound
	}
	if err != nil {
		return appsystem.WorkerDeadLetter{}, err
	}
	return sqliteWorkerDeadLetter(row.PublicID, row.WorkerName, row.JobKey, row.RuleID, row.Subject, row.MeterName, row.Attempts, row.LastError, row.Status, row.CreatedAt, row.RequeuedAt)
}

func (r *SystemRepository) EnqueueWorkerDeadLetter(ctx context.Context, deadLetter appsystem.WorkerDeadLetter, now time.Time) (bool, error) {
	workspaceID, err := appauth.RequireWorkspaceID(ctx)
	if err != nil {
		return false, err
	}
	var rows int64
	switch deadLetter.WorkerName {
	case "alert":
		rows, err = queriesFor(ctx, r.queries).EnqueueAlertWorkerDeadLetter(ctx, sqlitedb.EnqueueAlertWorkerDeadLetterParams{Now: formatTime(now), WorkspaceID: workspaceID, PublicID: deadLetter.ID})
	case "entitlement":
		rows, err = queriesFor(ctx, r.queries).EnqueueEntitlementWorkerDeadLetter(ctx, sqlitedb.EnqueueEntitlementWorkerDeadLetterParams{Now: formatTime(now), WorkspaceID: workspaceID, PublicID: deadLetter.ID})
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
	rows, err := queriesFor(ctx, r.queries).MarkWorkerDeadLetterRequeued(ctx, sqlitedb.MarkWorkerDeadLetterRequeuedParams{Now: sql.NullString{String: formatTime(now), Valid: true}, WorkspaceID: workspaceID, PublicID: id})
	return rows == 1, err
}

func sqliteWorkerDeadLetter(id, workerName, jobKey, ruleID, subject, meterName string, attempts int64, lastError, status, createdAt string, requeuedAt sql.NullString) (appsystem.WorkerDeadLetter, error) {
	created, err := time.Parse(time.RFC3339Nano, createdAt)
	if err != nil {
		return appsystem.WorkerDeadLetter{}, err
	}
	var requeued time.Time
	if requeuedAt.Valid {
		requeued, err = time.Parse(time.RFC3339Nano, requeuedAt.String)
		if err != nil {
			return appsystem.WorkerDeadLetter{}, err
		}
	}
	return appsystem.WorkerDeadLetter{ID: id, WorkerName: workerName, JobKey: jobKey, RuleID: ruleID, Subject: subject, MeterName: meterName, Attempts: int(attempts), LastError: lastError, Status: status, CreatedAt: created, RequeuedAt: requeued}, nil
}

func diagnosticInt(value any) int {
	switch typed := value.(type) {
	case int64:
		return int(typed)
	case int:
		return typed
	default:
		return 0
	}
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
	parsed, err := time.Parse(time.RFC3339Nano, raw)
	if err != nil {
		return time.Time{}, err
	}
	return parsed.UTC(), nil
}

func (r *SystemRepository) ClaimReconciliationSchedule(ctx context.Context, now, lockedUntil time.Time) (appsystem.ReconciliationClaim, bool, error) {
	formattedNow := formatTime(now)
	if err := queriesFor(ctx, r.queries).EnsureReconciliationSchedules(ctx, formattedNow); err != nil {
		return appsystem.ReconciliationClaim{}, false, err
	}
	row, err := queriesFor(ctx, r.queries).ClaimReconciliationSchedule(ctx, sqlitedb.ClaimReconciliationScheduleParams{Now: formattedNow, LockedUntil: sql.NullString{String: formatTime(lockedUntil), Valid: true}})
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
	return queriesFor(ctx, r.queries).SaveReconciliationRun(ctx, sqlitedb.SaveReconciliationRunParams{
		ID: run.ID, WorkspaceID: workspaceID, Status: run.Status, DecisionsChecked: int64(run.DecisionsChecked),
		CountersChecked: int64(run.CountersChecked), IssueCount: int64(run.IssueCount), Truncated: boolInt64(run.Truncated),
		LookbackHours: int64(run.LookbackHours), DurationMs: run.Duration.Milliseconds(), Fingerprint: run.Fingerprint,
		Issues: string(issues), Error: run.Error, CreatedAt: formatTime(run.CreatedAt),
	})
}

func (r *SystemRepository) CompleteReconciliationSchedule(ctx context.Context, workspaceID, fingerprint string, nextRunAt time.Time) error {
	return queriesFor(ctx, r.queries).CompleteReconciliationSchedule(ctx, sqlitedb.CompleteReconciliationScheduleParams{WorkspaceID: workspaceID, Fingerprint: fingerprint, NextRunAt: formatTime(nextRunAt), UpdatedAt: formatTime(time.Now().UTC())})
}

func (r *SystemRepository) FailReconciliationSchedule(ctx context.Context, workspaceID, failureFingerprint string, nextRunAt time.Time) error {
	return queriesFor(ctx, r.queries).FailReconciliationSchedule(ctx, sqlitedb.FailReconciliationScheduleParams{WorkspaceID: workspaceID, FailureFingerprint: failureFingerprint, NextRunAt: formatTime(nextRunAt), UpdatedAt: formatTime(time.Now().UTC())})
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
	next, e1 := parseEntitlementTime(row.NextRunAt)
	locked, e2 := parseNullableEntitlementTime(row.LockedUntil)
	updated, e3 := parseEntitlementTime(row.UpdatedAt)
	if err := errors.Join(e1, e2, e3); err != nil {
		return appsystem.ReconciliationSchedule{}, false, err
	}
	return appsystem.ReconciliationSchedule{NextRunAt: next, LockedUntil: locked, UpdatedAt: updated}, true, nil
}

func (r *SystemRepository) SaveReconciliationNotification(ctx context.Context, notification appsystem.ReconciliationNotification) error {
	payload, err := json.Marshal(notification.Run)
	if err != nil {
		return err
	}
	return queriesFor(ctx, r.queries).SaveReconciliationNotification(ctx, sqlitedb.SaveReconciliationNotificationParams{ID: notification.ID, WorkspaceID: notification.WorkspaceID, EventType: notification.EventType, Fingerprint: notification.Fingerprint, Payload: string(payload), NextAttemptAt: formatTime(notification.NextAttemptAt), CreatedAt: formatTime(notification.CreatedAt)})
}

func (r *SystemRepository) ClaimReconciliationNotification(ctx context.Context, now, lockedUntil time.Time) (appsystem.ReconciliationNotification, bool, error) {
	row, err := queriesFor(ctx, r.queries).ClaimReconciliationNotification(ctx, sqlitedb.ClaimReconciliationNotificationParams{Now: formatTime(now), LockedUntil: sql.NullString{String: formatTime(lockedUntil), Valid: true}})
	if errors.Is(err, sql.ErrNoRows) {
		return appsystem.ReconciliationNotification{}, false, nil
	}
	if err != nil {
		return appsystem.ReconciliationNotification{}, false, err
	}
	var run appsystem.ReconciliationRun
	next, e1 := parseEntitlementTime(row.NextAttemptAt)
	created, e2 := parseEntitlementTime(row.CreatedAt)
	delivered, e3 := parseNullableEntitlementTime(row.DeliveredAt)
	if err := errors.Join(json.Unmarshal([]byte(row.Payload), &run), e1, e2, e3); err != nil {
		return appsystem.ReconciliationNotification{}, false, err
	}
	return appsystem.ReconciliationNotification{ID: row.ID, WorkspaceID: row.WorkspaceID, EventType: row.EventType, Fingerprint: row.Fingerprint, Run: run, Status: row.Status, Attempts: int(row.Attempts), TotalAttempts: int(row.Count), NextAttemptAt: next, LastError: row.LastError, CreatedAt: created, DeliveredAt: delivered}, true, nil
}

func (r *SystemRepository) CompleteReconciliationNotification(ctx context.Context, notification appsystem.ReconciliationNotification) error {
	return queriesFor(ctx, r.queries).CompleteReconciliationNotification(ctx, sqlitedb.CompleteReconciliationNotificationParams{ID: notification.ID, DeliveredAt: sql.NullString{String: formatTime(time.Now().UTC()), Valid: true}})
}

func (r *SystemRepository) RetryReconciliationNotification(ctx context.Context, notification appsystem.ReconciliationNotification, nextAttemptAt time.Time, maxAttempts int, deliveryErr error) error {
	return queriesFor(ctx, r.queries).RetryReconciliationNotification(ctx, sqlitedb.RetryReconciliationNotificationParams{ID: notification.ID, MaxAttempts: int64(maxAttempts), NextAttemptAt: formatTime(nextAttemptAt), LastError: deliveryErr.Error()})
}

func (r *SystemRepository) ListReconciliationNotifications(ctx context.Context, limit int) ([]appsystem.ReconciliationNotification, error) {
	workspaceID, err := appauth.RequireWorkspaceID(ctx)
	if err != nil {
		return nil, err
	}
	rows, err := queriesFor(ctx, r.queries).ListReconciliationNotifications(ctx, sqlitedb.ListReconciliationNotificationsParams{WorkspaceID: workspaceID, Limit: int64(limit)})
	if err != nil {
		return nil, err
	}
	result := make([]appsystem.ReconciliationNotification, 0, len(rows))
	for _, row := range rows {
		var run appsystem.ReconciliationRun
		next, e1 := parseEntitlementTime(row.NextAttemptAt)
		locked, e2 := parseNullableEntitlementTime(row.LockedUntil)
		created, e3 := parseEntitlementTime(row.CreatedAt)
		delivered, e4 := parseNullableEntitlementTime(row.DeliveredAt)
		if err := errors.Join(json.Unmarshal([]byte(row.Payload), &run), e1, e2, e3, e4); err != nil {
			return nil, err
		}
		result = append(result, appsystem.ReconciliationNotification{ID: row.ID, WorkspaceID: workspaceID, EventType: row.EventType, Fingerprint: row.Fingerprint, Run: run, Status: row.Status, Attempts: int(row.Attempts), NextAttemptAt: next, LockedUntil: locked, LastError: row.LastError, CreatedAt: created, DeliveredAt: delivered})
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
	return appsystem.ReconciliationNotificationCounts{Pending: int(row.Pending.Float64), DeadLetter: int(row.DeadLetter.Float64)}, nil
}

func (r *SystemRepository) RequeueReconciliationNotification(ctx context.Context, id string, nextAttemptAt time.Time) (bool, error) {
	workspaceID, err := appauth.RequireWorkspaceID(ctx)
	if err != nil {
		return false, err
	}
	rows, err := queriesFor(ctx, r.queries).RequeueReconciliationNotification(ctx, sqlitedb.RequeueReconciliationNotificationParams{ID: id, WorkspaceID: workspaceID, NextAttemptAt: formatTime(nextAttemptAt)})
	return rows == 1, err
}

func (r *SystemRepository) SaveReconciliationNotificationAttempt(ctx context.Context, attempt appsystem.ReconciliationNotificationAttempt) error {
	return queriesFor(ctx, r.queries).SaveReconciliationNotificationAttempt(ctx, sqlitedb.SaveReconciliationNotificationAttemptParams{ID: attempt.ID, NotificationID: attempt.NotificationID, Attempt: int64(attempt.Attempt), Status: attempt.Status, Error: attempt.Error, CreatedAt: formatTime(attempt.CreatedAt)})
}

func (r *SystemRepository) ListReconciliationNotificationAttempts(ctx context.Context, notificationID string) ([]appsystem.ReconciliationNotificationAttempt, error) {
	rows, err := queriesFor(ctx, r.queries).ListReconciliationNotificationAttempts(ctx, notificationID)
	if err != nil {
		return nil, err
	}
	result := make([]appsystem.ReconciliationNotificationAttempt, 0, len(rows))
	for _, row := range rows {
		createdAt, err := parseEntitlementTime(row.CreatedAt)
		if err != nil {
			return nil, err
		}
		result = append(result, appsystem.ReconciliationNotificationAttempt{ID: row.ID, NotificationID: notificationID, Attempt: int(row.Attempt), Status: row.Status, Error: row.Error, CreatedAt: createdAt})
	}
	return result, nil
}

func (r *SystemRepository) MarkReconciliationNotified(ctx context.Context, workspaceID, fingerprint string) error {
	return queriesFor(ctx, r.queries).MarkReconciliationNotified(ctx, sqlitedb.MarkReconciliationNotifiedParams{WorkspaceID: workspaceID, Fingerprint: fingerprint, UpdatedAt: formatTime(time.Now().UTC())})
}

func (r *SystemRepository) ListReconciliationRuns(ctx context.Context, limit int) ([]appsystem.ReconciliationRun, error) {
	workspaceID, err := appauth.RequireWorkspaceID(ctx)
	if err != nil {
		return nil, err
	}
	rows, err := queriesFor(ctx, r.queries).ListReconciliationRuns(ctx, sqlitedb.ListReconciliationRunsParams{WorkspaceID: workspaceID, Limit: int64(limit)})
	if err != nil {
		return nil, err
	}
	runs := make([]appsystem.ReconciliationRun, 0, len(rows))
	for _, row := range rows {
		var issues []appsystem.ReconciliationIssue
		createdAt, parseErr := parseEntitlementTime(row.CreatedAt)
		if err := errors.Join(json.Unmarshal([]byte(row.Issues), &issues), parseErr); err != nil {
			return nil, err
		}
		runs = append(runs, appsystem.ReconciliationRun{ID: row.ID, Status: row.Status, DecisionsChecked: int(row.DecisionsChecked), CountersChecked: int(row.CountersChecked), IssueCount: int(row.IssueCount), Truncated: row.Truncated != 0, LookbackHours: int(row.LookbackHours), Duration: time.Duration(row.DurationMs) * time.Millisecond, Fingerprint: row.Fingerprint, Issues: issues, Error: row.Error, CreatedAt: createdAt})
	}
	return runs, nil
}

func (r *SystemRepository) FindStats(ctx context.Context) (appsystem.StatsResult, error) {
	workspaceID, err := appauth.RequireWorkspaceID(ctx)
	if err != nil {
		return appsystem.StatsResult{}, err
	}

	now := formatTime(time.Now().UTC())
	if err := queriesFor(ctx, r.queries).EnsureWorkspaceStats(ctx, sqlitedb.EnsureWorkspaceStatsParams{
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
		before, beforeErr := time.Parse(time.RFC3339Nano, decisionRun.Before)
		createdAt, createdErr := time.Parse(time.RFC3339Nano, decisionRun.CreatedAt)
		if beforeErr != nil || createdErr != nil {
			return appsystem.StatsResult{}, errors.Join(beforeErr, createdErr)
		}
		result.LastDecisionPruneRun = appsystem.LastDecisionPruneRunResult{ID: decisionRun.ID, Before: before, Deleted: int(decisionRun.Deleted), DryRun: decisionRun.DryRun != 0, CreatedAt: createdAt}
	} else if !errors.Is(err, sql.ErrNoRows) {
		return appsystem.StatsResult{}, err
	}

	runs, err := queriesFor(ctx, r.queries).ListUsagePruneRuns(ctx, sqlitedb.ListUsagePruneRunsParams{
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
		result.LastExportCleanupRun = appsystem.LastExportCleanupRunResult{ID: cleanup.PublicID, ExpiredBefore: expiredBefore, FilesDeleted: int(cleanup.FilesDeleted), BytesReclaimed: cleanup.BytesReclaimed, Failures: int(cleanup.Failures), CreatedAt: createdAt}
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
	rows, err := queriesFor(ctx, r.queries).ListDecisionReconciliationRows(ctx, sqlitedb.ListDecisionReconciliationRowsParams{WorkspaceID: workspaceID, Since: formatTime(since), Limit: int64(limit)})
	if err != nil {
		return nil, err
	}
	result := make([]appsystem.DecisionReconciliationRow, 0, len(rows))
	for _, row := range rows {
		createdAt, err := parseEntitlementTime(row.CreatedAt)
		if err != nil {
			return nil, err
		}
		result = append(result, appsystem.DecisionReconciliationRow{
			IdempotencyKey: row.IdempotencyKey, Subject: row.Subject, MeterName: row.MeterName,
			Accepted: row.Accepted != 0, CreatedAt: createdAt, EventID: row.EventID.String,
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
	rows, err := queriesFor(ctx, r.queries).ListActiveEntitlementCounters(ctx, sqlitedb.ListActiveEntitlementCountersParams{WorkspaceID: workspaceID, Now: formatTime(now), Limit: int64(limit)})
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
	rows, err := queriesFor(ctx, r.queries).ListCounterReconciliationEvents(ctx, sqlitedb.ListCounterReconciliationEventsParams{
		WorkspaceID: workspaceID, Subject: counter.Subject, MeterName: counter.MeterName,
		PeriodStart: formatTime(counter.PeriodStart), PeriodEnd: formatTime(counter.PeriodEnd),
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
	row, err := queriesFor(ctx, r.queries).GetEntitlementCounterForRepair(ctx, sqlitedb.GetEntitlementCounterForRepairParams{
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
		rows, err := queriesFor(ctx, r.queries).DeleteEntitlementCounterForRepair(ctx, sqlitedb.DeleteEntitlementCounterForRepairParams{
			WorkspaceID: workspaceID, Subject: counter.Subject, MeterName: counter.MeterName, Period: counter.Period,
			PeriodStart: formatTime(counter.PeriodStart), ExpectedUpdatedAt: formatTime(expectedUpdatedAt),
		})
		return rows == 1, err
	}
	rows, err := queriesFor(ctx, r.queries).UpdateEntitlementCounterForRepair(ctx, sqlitedb.UpdateEntitlementCounterForRepairParams{
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
	return queriesFor(ctx, r.queries).SaveQuotaCounterRepairRun(ctx, sqlitedb.SaveQuotaCounterRepairRunParams{
		ID: run.ID, WorkspaceID: workspaceID, Subject: run.Subject, MeterName: run.MeterName, Period: run.Period,
		PeriodStart: formatTime(run.PeriodStart), PeriodEnd: formatTime(run.PeriodEnd), DryRun: boolInt64(run.DryRun), Applied: boolInt64(run.Applied),
		BeforeSnapshot: string(before), AfterSnapshot: string(after), CounterUpdatedAt: formatTime(run.CounterUpdatedAt), CreatedAt: formatTime(run.CreatedAt),
	})
}

func (r *SystemRepository) ListQuotaCounterRepairRuns(ctx context.Context, limit int) ([]appsystem.CounterRepairResult, error) {
	workspaceID, err := appauth.RequireWorkspaceID(ctx)
	if err != nil {
		return nil, err
	}
	rows, err := queriesFor(ctx, r.queries).ListQuotaCounterRepairRuns(ctx, sqlitedb.ListQuotaCounterRepairRunsParams{WorkspaceID: workspaceID, Limit: int64(limit)})
	if err != nil {
		return nil, err
	}
	result := make([]appsystem.CounterRepairResult, 0, len(rows))
	for _, row := range rows {
		start, e1 := parseEntitlementTime(row.PeriodStart)
		end, e2 := parseEntitlementTime(row.PeriodEnd)
		counterUpdated, e3 := parseEntitlementTime(row.CounterUpdatedAt)
		created, e4 := parseEntitlementTime(row.CreatedAt)
		if err := errors.Join(e1, e2, e3, e4); err != nil {
			return nil, err
		}
		var before, after appsystem.CounterSnapshot
		if err := errors.Join(json.Unmarshal([]byte(row.BeforeSnapshot), &before), json.Unmarshal([]byte(row.AfterSnapshot), &after)); err != nil {
			return nil, err
		}
		result = append(result, appsystem.CounterRepairResult{ID: row.ID, CounterRepairTarget: appsystem.CounterRepairTarget{Subject: row.Subject, MeterName: row.MeterName, Period: row.Period, PeriodStart: start}, PeriodEnd: end, DryRun: row.DryRun != 0, Applied: row.Applied != 0, Before: before, After: after, CounterUpdatedAt: counterUpdated, CreatedAt: created})
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
		rows, err := queriesFor(ctx, r.queries).ListUsagePruneRuns(ctx, sqlitedb.ListUsagePruneRunsParams{
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
	rows, err := queriesFor(ctx, r.queries).ListCounterReconciliationAssignments(ctx, sqlitedb.ListCounterReconciliationAssignmentsParams{
		WorkspaceID: workspaceID, Subject: counter.Subject,
		WindowStart: sql.NullString{String: formatTime(counter.PeriodStart), Valid: true}, WindowEnd: formatTime(counter.PeriodEnd),
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
