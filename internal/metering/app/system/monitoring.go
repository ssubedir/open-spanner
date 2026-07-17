package system

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"sort"
	"time"

	"github.com/google/uuid"
	appauth "github.com/ssubedir/open-spanner/internal/auth"
	"github.com/ssubedir/open-spanner/internal/metering/domain"
)

const maxReconciliationRunLimit = 200

type ReconciliationClaim struct {
	WorkspaceID             string
	ClaimToken              string
	LastFingerprint         string
	LastNotifiedFingerprint string
	LastFailureFingerprint  string
}

type MaintenanceLease struct {
	WorkerName  string
	ClaimToken  string
	LockedUntil time.Time
}

type ReconciliationSchedule struct {
	NextRunAt   time.Time
	LockedUntil time.Time
	UpdatedAt   time.Time
}

type ReconciliationHealth struct {
	Status                  string
	NextRunAt               time.Time
	LockedUntil             time.Time
	UpdatedAt               time.Time
	PendingNotifications    int
	DeadLetterNotifications int
}

type ReconciliationNotificationCounts struct {
	Pending    int
	DeadLetter int
}

type ReconciliationNotification struct {
	ID             string
	WorkspaceID    string
	EventType      string
	Fingerprint    string
	Run            ReconciliationRun
	Status         string
	Attempts       int
	TotalAttempts  int
	NextAttemptAt  time.Time
	LockedUntil    time.Time
	LastError      string
	CreatedAt      time.Time
	DeliveredAt    time.Time
	ClaimToken     string
	AttemptHistory []ReconciliationNotificationAttempt
}

type ReconciliationNotificationAttempt struct {
	ID             string
	NotificationID string
	Attempt        int
	Status         string
	Error          string
	CreatedAt      time.Time
}

type ReconciliationRun struct {
	ID               string
	Status           string
	DecisionsChecked int
	CountersChecked  int
	IssueCount       int
	Truncated        bool
	LookbackHours    int
	Duration         time.Duration
	Fingerprint      string
	Issues           []ReconciliationIssue
	Error            string
	CreatedAt        time.Time
}

func (s *service) ClaimScheduledReconciliation(ctx context.Context, now, lockedUntil time.Time) (ReconciliationClaim, bool, error) {
	return s.repo.ClaimReconciliationSchedule(ctx, now.UTC(), lockedUntil.UTC())
}

func (s *service) RunScheduledReconciliation(ctx context.Context, claim ReconciliationClaim, scheduleInterval time.Duration, query ReconciliationQuery) (ReconciliationRun, bool, error) {
	started := time.Now().UTC()
	result, err := s.Reconcile(appauth.WithWorkspaceID(ctx, claim.WorkspaceID), query)
	if err != nil {
		return ReconciliationRun{}, false, err
	}
	run := ReconciliationRun{
		ID: uuid.Must(uuid.NewV7()).String(), Status: result.Status,
		DecisionsChecked: result.DecisionChecked, CountersChecked: result.CountersChecked,
		IssueCount: len(result.Issues), Truncated: result.Truncated, LookbackHours: result.LookbackHours,
		Duration: time.Since(started), Issues: result.Issues, CreatedAt: time.Now().UTC(),
	}
	run.Fingerprint = reconciliationFingerprint(result.Issues)
	if err := s.transactor.WithinTransaction(ctx, func(txCtx context.Context) error {
		if err := s.repo.SaveReconciliationRun(txCtx, claim.WorkspaceID, run); err != nil {
			return err
		}
		if run.Fingerprint != "" && run.Fingerprint != claim.LastFingerprint {
			if err := s.repo.SaveReconciliationNotification(txCtx, notificationFromRun(claim.WorkspaceID, "drift_detected", run.Fingerprint, run)); err != nil {
				return err
			}
		}
		return s.repo.CompleteReconciliationSchedule(txCtx, claim, run.Fingerprint, run.CreatedAt.Add(scheduleInterval))
	}); err != nil {
		return ReconciliationRun{}, false, err
	}
	notify := run.Fingerprint != "" && run.Fingerprint != claim.LastFingerprint
	return run, notify, nil
}

func (s *service) FailScheduledReconciliation(ctx context.Context, claim ReconciliationClaim, retryAt time.Time, runErr error) error {
	now := time.Now().UTC()
	run := ReconciliationRun{ID: uuid.Must(uuid.NewV7()).String(), Status: "failed", Error: runErr.Error(), CreatedAt: now}
	fingerprint := errorFingerprint(runErr)
	return s.transactor.WithinTransaction(ctx, func(txCtx context.Context) error {
		if err := s.repo.SaveReconciliationRun(txCtx, claim.WorkspaceID, run); err != nil {
			return err
		}
		if fingerprint != claim.LastFailureFingerprint {
			if err := s.repo.SaveReconciliationNotification(txCtx, notificationFromRun(claim.WorkspaceID, "scan_failed", fingerprint, run)); err != nil {
				return err
			}
		}
		return s.repo.FailReconciliationSchedule(txCtx, claim, fingerprint, retryAt.UTC())
	})
}

func (s *service) ClaimMaintenanceLease(ctx context.Context, workerName string, now, lockedUntil time.Time) (MaintenanceLease, bool, error) {
	if workerName == "" || !lockedUntil.After(now) {
		return MaintenanceLease{}, false, domain.ErrInvalidInput
	}
	return s.repo.ClaimMaintenanceLease(ctx, workerName, uuid.Must(uuid.NewV7()).String(), now.UTC(), lockedUntil.UTC())
}

func (s *service) ReleaseMaintenanceLease(ctx context.Context, lease MaintenanceLease) error {
	if lease.WorkerName == "" || lease.ClaimToken == "" {
		return domain.ErrInvalidInput
	}
	return s.repo.ReleaseMaintenanceLease(ctx, lease)
}

func (s *service) ClaimReconciliationNotification(ctx context.Context, now, lockedUntil time.Time) (ReconciliationNotification, bool, error) {
	return s.repo.ClaimReconciliationNotification(ctx, now.UTC(), lockedUntil.UTC())
}

func (s *service) CompleteReconciliationNotification(ctx context.Context, notification ReconciliationNotification) error {
	return s.transactor.WithinTransaction(ctx, func(txCtx context.Context) error {
		attempt := ReconciliationNotificationAttempt{ID: uuid.Must(uuid.NewV7()).String(), NotificationID: notification.ID, Attempt: notification.TotalAttempts + 1, Status: "delivered", CreatedAt: time.Now().UTC()}
		if err := s.repo.SaveReconciliationNotificationAttempt(txCtx, attempt); err != nil {
			return err
		}
		if err := s.repo.CompleteReconciliationNotification(txCtx, notification); err != nil {
			return err
		}
		if notification.EventType == "drift_detected" {
			return s.repo.MarkReconciliationNotified(txCtx, notification.WorkspaceID, notification.Fingerprint)
		}
		return nil
	})
}

func (s *service) RetryReconciliationNotification(ctx context.Context, notification ReconciliationNotification, nextAttemptAt time.Time, maxAttempts int, deliveryErr error) error {
	return s.transactor.WithinTransaction(ctx, func(txCtx context.Context) error {
		attempt := ReconciliationNotificationAttempt{ID: uuid.Must(uuid.NewV7()).String(), NotificationID: notification.ID, Attempt: notification.TotalAttempts + 1, Status: "failed", Error: deliveryErr.Error(), CreatedAt: time.Now().UTC()}
		if err := s.repo.SaveReconciliationNotificationAttempt(txCtx, attempt); err != nil {
			return err
		}
		return s.repo.RetryReconciliationNotification(txCtx, notification, nextAttemptAt.UTC(), maxAttempts, deliveryErr)
	})
}

func (s *service) ListReconciliationNotifications(ctx context.Context, limit int) ([]ReconciliationNotification, error) {
	if limit == 0 {
		limit = 50
	}
	if limit < 1 || limit > maxReconciliationRunLimit {
		return nil, errors.Join(domain.ErrInvalidInput, errors.New("reconciliation notification limit must be between 1 and 200"))
	}
	notifications, err := s.repo.ListReconciliationNotifications(ctx, limit)
	if err != nil {
		return nil, err
	}
	for index := range notifications {
		attempts, err := s.repo.ListReconciliationNotificationAttempts(ctx, notifications[index].ID)
		if err != nil {
			return nil, err
		}
		notifications[index].AttemptHistory = attempts
		notifications[index].TotalAttempts = len(attempts)
	}
	return notifications, nil
}

func (s *service) RequeueReconciliationNotification(ctx context.Context, id string) error {
	if id == "" {
		return errors.Join(domain.ErrInvalidInput, errors.New("notification id is required"))
	}
	updated, err := s.repo.RequeueReconciliationNotification(ctx, id, time.Now().UTC())
	if err != nil {
		return err
	}
	if !updated {
		return domain.ErrNotFound
	}
	return nil
}

func (s *service) MarkReconciliationNotified(ctx context.Context, workspaceID, fingerprint string) error {
	return s.repo.MarkReconciliationNotified(ctx, workspaceID, fingerprint)
}

func (s *service) ListReconciliationRuns(ctx context.Context, limit int) ([]ReconciliationRun, error) {
	if limit == 0 {
		limit = 50
	}
	if limit < 1 || limit > maxReconciliationRunLimit {
		return nil, errors.Join(domain.ErrInvalidInput, errors.New("reconciliation run limit must be between 1 and 200"))
	}
	return s.repo.ListReconciliationRuns(ctx, limit)
}

func reconciliationFingerprint(issues []ReconciliationIssue) string {
	if len(issues) == 0 {
		return ""
	}
	encoded := make([]string, 0, len(issues))
	for _, issue := range issues {
		data, _ := json.Marshal([]any{issue.Kind, issue.Subject, issue.MeterName, issue.IdempotencyKey, issue.Period, issue.PeriodStart.UTC(), issue.Expected, issue.Actual})
		encoded = append(encoded, string(data))
	}
	sort.Strings(encoded)
	digest := sha256.New()
	for _, item := range encoded {
		_, _ = digest.Write([]byte(item))
		_, _ = digest.Write([]byte{0})
	}
	return hex.EncodeToString(digest.Sum(nil))
}

func notificationFromRun(workspaceID, eventType, fingerprint string, run ReconciliationRun) ReconciliationNotification {
	return ReconciliationNotification{ID: uuid.Must(uuid.NewV7()).String(), WorkspaceID: workspaceID, EventType: eventType, Fingerprint: fingerprint, Run: run, Status: "pending", NextAttemptAt: run.CreatedAt, CreatedAt: run.CreatedAt}
}

func errorFingerprint(err error) string {
	digest := sha256.Sum256([]byte(err.Error()))
	return hex.EncodeToString(digest[:])
}

func reconciliationHealth(schedule ReconciliationSchedule, exists bool, run ReconciliationRun, counts ReconciliationNotificationCounts, now time.Time, staleAfter time.Duration) ReconciliationHealth {
	health := ReconciliationHealth{Status: "not_started", PendingNotifications: counts.Pending, DeadLetterNotifications: counts.DeadLetter}
	if !exists {
		return health
	}
	health.NextRunAt, health.LockedUntil, health.UpdatedAt = schedule.NextRunAt, schedule.LockedUntil, schedule.UpdatedAt
	if (schedule.LockedUntil.IsZero() || !schedule.LockedUntil.After(now)) && schedule.NextRunAt.Add(staleAfter).Before(now) {
		health.Status = "stale"
		return health
	}
	if counts.DeadLetter > 0 {
		health.Status = "degraded"
		return health
	}
	if run.ID == "" {
		return health
	}
	health.Status = run.Status
	return health
}
