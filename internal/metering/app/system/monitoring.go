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
	LastFingerprint         string
	LastNotifiedFingerprint string
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
		return s.repo.CompleteReconciliationSchedule(txCtx, claim.WorkspaceID, run.Fingerprint, run.CreatedAt.Add(scheduleInterval))
	}); err != nil {
		return ReconciliationRun{}, false, err
	}
	notify := run.Fingerprint != "" && run.Fingerprint != claim.LastNotifiedFingerprint
	return run, notify, nil
}

func (s *service) FailScheduledReconciliation(ctx context.Context, claim ReconciliationClaim, retryAt time.Time, runErr error) error {
	now := time.Now().UTC()
	run := ReconciliationRun{ID: uuid.Must(uuid.NewV7()).String(), Status: "failed", Error: runErr.Error(), CreatedAt: now}
	return s.transactor.WithinTransaction(ctx, func(txCtx context.Context) error {
		if err := s.repo.SaveReconciliationRun(txCtx, claim.WorkspaceID, run); err != nil {
			return err
		}
		return s.repo.FailReconciliationSchedule(txCtx, claim.WorkspaceID, retryAt.UTC())
	})
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
