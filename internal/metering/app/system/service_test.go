package system

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/ssubedir/open-spanner/internal/metering/domain"
)

type reconciliationRepository struct {
	heartbeats  []WorkerHeartbeat
	diagnostics []WorkerDiagnostics
	deadLetters []WorkerDeadLetter
	decisions   []DecisionReconciliationRow
	counters    []CounterReconciliationRow
	events      []ReconciliationEvent
	assignments []ReconciliationAssignment
	repairRuns  []CounterRepairResult
	pruneCutoff time.Time
}

func (r *reconciliationRepository) UpsertWorkerHeartbeat(_ context.Context, heartbeat WorkerHeartbeat) error {
	r.heartbeats = append(r.heartbeats, heartbeat)
	return nil
}
func (r *reconciliationRepository) ListWorkerHeartbeats(context.Context) ([]WorkerHeartbeat, error) {
	return r.heartbeats, nil
}
func (r *reconciliationRepository) ListWorkerDiagnostics(context.Context, time.Time) ([]WorkerDiagnostics, error) {
	return r.diagnostics, nil
}
func (r *reconciliationRepository) ListWorkerDeadLetters(context.Context, int) ([]WorkerDeadLetter, error) {
	return r.deadLetters, nil
}
func (r *reconciliationRepository) GetWorkerDeadLetter(_ context.Context, id string) (WorkerDeadLetter, error) {
	for _, item := range r.deadLetters {
		if item.ID == id {
			return item, nil
		}
	}
	return WorkerDeadLetter{}, domain.ErrNotFound
}
func (r *reconciliationRepository) EnqueueWorkerDeadLetter(context.Context, WorkerDeadLetter, time.Time) (bool, error) {
	return true, nil
}
func (r *reconciliationRepository) MarkWorkerDeadLetterRequeued(_ context.Context, id string, at time.Time) (bool, error) {
	for index := range r.deadLetters {
		if r.deadLetters[index].ID == id && r.deadLetters[index].Status == "dead_letter" {
			r.deadLetters[index].Status = "requeued"
			r.deadLetters[index].RequeuedAt = at
			return true, nil
		}
	}
	return false, nil
}

func TestWorkerHealthDegradesForOldBacklogAndFailures(t *testing.T) {
	now := time.Date(2026, 7, 15, 12, 0, 0, 0, time.UTC)
	heartbeats := []WorkerHeartbeat{
		{Name: "export", StartedAt: now.Add(-time.Hour), LastHeartbeatAt: now.Add(-time.Second)},
		{Name: "alert", StartedAt: now.Add(-time.Hour), LastHeartbeatAt: now.Add(-time.Second)},
	}
	diagnostics := []WorkerDiagnostics{
		{Name: "export", PendingJobs: 2, OldestPendingAt: now.Add(-6 * time.Minute)},
		{Name: "alert", FailedJobs: 1, LastFailureAt: now.Add(-time.Minute)},
	}

	result := workerHealth(map[string]bool{}, heartbeats, diagnostics, now, 30*time.Second, 5*time.Minute)
	if result[0].Status != "degraded" || result[0].PendingJobs != 2 {
		t.Fatalf("expected export backlog to be degraded, got %+v", result[0])
	}
	if result[1].Status != "degraded" || result[1].FailedJobs != 1 {
		t.Fatalf("expected alert failures to be degraded, got %+v", result[1])
	}
}

func TestWorkerHealthStaleTakesPriorityOverDiagnostics(t *testing.T) {
	now := time.Date(2026, 7, 15, 12, 0, 0, 0, time.UTC)
	result := workerHealth(map[string]bool{}, []WorkerHeartbeat{{Name: "export", LastHeartbeatAt: now.Add(-time.Minute)}}, []WorkerDiagnostics{{Name: "export", FailedJobs: 1}}, now, 30*time.Second, 5*time.Minute)
	if result[0].Status != "stale" {
		t.Fatalf("expected stale status, got %q", result[0].Status)
	}
}

func TestRetryWorkerDeadLetterPreservesAuditAndRejectsDuplicate(t *testing.T) {
	repo := reconciliationRepository{deadLetters: []WorkerDeadLetter{{ID: "00000000-0000-4000-8000-000000000001", WorkerName: "alert", Status: "dead_letter"}}}
	service := NewService(&repo, directTransactor{})

	if err := service.RetryWorkerDeadLetter(context.Background(), "00000000-0000-4000-8000-000000000001"); err != nil {
		t.Fatalf("retry worker dead letter: %v", err)
	}
	if repo.deadLetters[0].Status != "requeued" || repo.deadLetters[0].RequeuedAt.IsZero() {
		t.Fatalf("dead-letter audit = %+v, want requeued timestamp", repo.deadLetters[0])
	}
	if err := service.RetryWorkerDeadLetter(context.Background(), "00000000-0000-4000-8000-000000000001"); !errors.Is(err, domain.ErrConflict) {
		t.Fatalf("second retry error = %v, want conflict", err)
	}
}

func (r *reconciliationRepository) ClaimReconciliationSchedule(context.Context, time.Time, time.Time) (ReconciliationClaim, bool, error) {
	return ReconciliationClaim{}, false, nil
}
func (r *reconciliationRepository) SaveReconciliationRun(context.Context, string, ReconciliationRun) error {
	return nil
}
func (r *reconciliationRepository) CompleteReconciliationSchedule(context.Context, string, string, time.Time) error {
	return nil
}
func (r *reconciliationRepository) FailReconciliationSchedule(context.Context, string, string, time.Time) error {
	return nil
}
func (r *reconciliationRepository) GetReconciliationSchedule(context.Context) (ReconciliationSchedule, bool, error) {
	return ReconciliationSchedule{}, false, nil
}
func (r *reconciliationRepository) SaveReconciliationNotification(context.Context, ReconciliationNotification) error {
	return nil
}
func (r *reconciliationRepository) ClaimReconciliationNotification(context.Context, time.Time, time.Time) (ReconciliationNotification, bool, error) {
	return ReconciliationNotification{}, false, nil
}
func (r *reconciliationRepository) CompleteReconciliationNotification(context.Context, ReconciliationNotification) error {
	return nil
}
func (r *reconciliationRepository) RetryReconciliationNotification(context.Context, ReconciliationNotification, time.Time, int, error) error {
	return nil
}
func (r *reconciliationRepository) ListReconciliationNotifications(context.Context, int) ([]ReconciliationNotification, error) {
	return nil, nil
}
func (r *reconciliationRepository) CountReconciliationNotifications(context.Context) (ReconciliationNotificationCounts, error) {
	return ReconciliationNotificationCounts{}, nil
}
func (r *reconciliationRepository) RequeueReconciliationNotification(context.Context, string, time.Time) (bool, error) {
	return false, nil
}
func (r *reconciliationRepository) SaveReconciliationNotificationAttempt(context.Context, ReconciliationNotificationAttempt) error {
	return nil
}
func (r *reconciliationRepository) ListReconciliationNotificationAttempts(context.Context, string) ([]ReconciliationNotificationAttempt, error) {
	return nil, nil
}
func (r *reconciliationRepository) MarkReconciliationNotified(context.Context, string, string) error {
	return nil
}
func (r *reconciliationRepository) ListReconciliationRuns(context.Context, int) ([]ReconciliationRun, error) {
	return nil, nil
}

func TestReconciliationFingerprintIsStable(t *testing.T) {
	first := ReconciliationIssue{Kind: "counter_quantity_sum_mismatch", Subject: "customer", MeterName: "requests", Expected: "4", Actual: "3"}
	second := ReconciliationIssue{Kind: "accepted_decision_missing_event", IdempotencyKey: "decision"}
	left := reconciliationFingerprint([]ReconciliationIssue{first, second})
	right := reconciliationFingerprint([]ReconciliationIssue{second, first})
	if left == "" || left != right {
		t.Fatalf("fingerprints left=%q right=%q", left, right)
	}
	second.IdempotencyKey = "changed"
	if reconciliationFingerprint([]ReconciliationIssue{first, second}) == left {
		t.Fatal("fingerprint did not change with issue identity")
	}
}

func TestReconciliationHealthDetectsStaleSchedule(t *testing.T) {
	now := time.Now().UTC()
	health := reconciliationHealth(ReconciliationSchedule{NextRunAt: now.Add(-31 * time.Minute), UpdatedAt: now.Add(-time.Hour)}, true, ReconciliationRun{ID: "run", Status: "healthy"}, ReconciliationNotificationCounts{}, now, 30*time.Minute)
	if health.Status != "stale" {
		t.Fatalf("health = %#v, want stale", health)
	}
	health = reconciliationHealth(ReconciliationSchedule{NextRunAt: now.Add(-31 * time.Minute), LockedUntil: now.Add(time.Minute)}, true, ReconciliationRun{ID: "run", Status: "healthy"}, ReconciliationNotificationCounts{}, now, 30*time.Minute)
	if health.Status != "healthy" {
		t.Fatalf("locked health = %#v, want healthy", health)
	}
}

func TestReconciliationHealthDetectsDeadLetters(t *testing.T) {
	now := time.Now().UTC()
	health := reconciliationHealth(ReconciliationSchedule{NextRunAt: now.Add(time.Minute)}, true, ReconciliationRun{ID: "run", Status: "healthy"}, ReconciliationNotificationCounts{DeadLetter: 1}, now, 30*time.Minute)
	if health.Status != "degraded" || health.DeadLetterNotifications != 1 {
		t.Fatalf("health = %#v, want degraded", health)
	}
}

func TestRollupHealthDetectsCoverageAndIntegrityIssues(t *testing.T) {
	now := time.Date(2026, 7, 15, 12, 30, 0, 0, time.UTC)
	health := rollupHealth([]RollupMeterCoverage{
		{MeterName: "healthy", RetentionDays: 1, FinalizedThrough: time.Date(2026, 7, 14, 12, 0, 0, 0, time.UTC), SourceEvents: 10, RollupRows: 3, LastRunAt: now.Add(-time.Hour)},
		{MeterName: "missing", RetentionDays: 7},
		{MeterName: "corrupt", RetentionDays: 30, FinalizedThrough: now.AddDate(0, 0, -30).Truncate(time.Hour), InvalidRollupRows: 1},
	}, now, 2*time.Hour)
	if health.Status != "degraded" || health.Meters != 3 || health.HealthyMeters != 1 || health.Issues != 2 {
		t.Fatalf("rollup health = %#v", health)
	}
	if health.Items[1].Status != "not_started" || health.Items[2].Status != "degraded" {
		t.Fatalf("rollup items = %#v", health.Items)
	}
}

func TestRollupHealthReportsStaleCoverage(t *testing.T) {
	now := time.Date(2026, 7, 15, 12, 30, 0, 0, time.UTC)
	health := rollupHealth([]RollupMeterCoverage{{
		MeterName: "stale", RetentionDays: 1, FinalizedThrough: time.Date(2026, 7, 14, 8, 0, 0, 0, time.UTC),
	}}, now, 2*time.Hour)
	if health.Status != "stale" || len(health.Items) != 1 || health.Items[0].Status != "stale" {
		t.Fatalf("stale rollup health = %#v", health)
	}
}

type directTransactor struct{}

func (directTransactor) WithinTransaction(ctx context.Context, fn func(context.Context) error) error {
	return fn(ctx)
}

func (r reconciliationRepository) FindStats(context.Context) (StatsResult, error) {
	return StatsResult{}, nil
}
func (r reconciliationRepository) ListDecisionReconciliationRows(context.Context, time.Time, int) ([]DecisionReconciliationRow, error) {
	return r.decisions, nil
}
func (r reconciliationRepository) ListActiveEntitlementCounters(context.Context, time.Time, int) ([]CounterReconciliationRow, error) {
	return r.counters, nil
}
func (r reconciliationRepository) ListCounterReconciliationEvents(context.Context, CounterReconciliationRow) ([]ReconciliationEvent, error) {
	return r.events, nil
}
func (r reconciliationRepository) ListCounterReconciliationAssignments(context.Context, CounterReconciliationRow) ([]ReconciliationAssignment, error) {
	return r.assignments, nil
}
func (r *reconciliationRepository) GetEntitlementCounterForRepair(context.Context, CounterRepairTarget) (CounterReconciliationRow, error) {
	return r.counters[0], nil
}
func (r *reconciliationRepository) UpdateEntitlementCounterForRepair(_ context.Context, counter CounterReconciliationRow, expected time.Time, replacement CounterSnapshot, updated time.Time) (bool, error) {
	if !counter.UpdatedAt.Equal(expected) {
		return false, nil
	}
	r.counters[0].EventCount, r.counters[0].QuantitySum = replacement.EventCount, replacement.QuantitySum
	r.counters[0].QuantityMin, r.counters[0].QuantityMax = replacement.QuantityMin, replacement.QuantityMax
	r.counters[0].FirstQuantity, r.counters[0].FirstEventTime = replacement.FirstQuantity, replacement.FirstEventTime
	r.counters[0].LastQuantity, r.counters[0].LastEventTime = replacement.LastQuantity, replacement.LastEventTime
	r.counters[0].UpdatedAt = updated
	return true, nil
}
func (r *reconciliationRepository) SaveQuotaCounterRepairRun(_ context.Context, run CounterRepairResult) error {
	r.repairRuns = append(r.repairRuns, run)
	return nil
}
func (r *reconciliationRepository) ListQuotaCounterRepairRuns(context.Context, int) ([]CounterRepairResult, error) {
	return r.repairRuns, nil
}
func (r *reconciliationRepository) FindLatestMeterPruneCutoff(context.Context, string) (time.Time, error) {
	return r.pruneCutoff, nil
}

func TestReconcileHealthy(t *testing.T) {
	anchor := time.Now().UTC().Add(-time.Hour)
	eventTime := anchor.Add(10 * time.Minute)
	repo := reconciliationRepository{
		decisions:   []DecisionReconciliationRow{{IdempotencyKey: "consume-1", Subject: "org_1", MeterName: "api", Accepted: true, EventID: "event-1", EventSubject: "org_1", EventMeterName: "api"}},
		counters:    []CounterReconciliationRow{{Subject: "org_1", MeterName: "api", Period: "day", PeriodStart: anchor, PeriodEnd: anchor.AddDate(0, 0, 1), EventCount: 1, QuantitySum: 2, QuantityMin: 2, QuantityMax: 2, EventRetentionDays: 90}},
		events:      []ReconciliationEvent{{ID: "event-1", Quantity: 2, EventTime: eventTime}},
		assignments: []ReconciliationAssignment{{ID: "assignment-1", AssignedAt: anchor, PeriodAnchorAt: anchor}},
	}

	result, err := NewService(&repo, directTransactor{}).Reconcile(context.Background(), ReconciliationQuery{})
	if err != nil {
		t.Fatal(err)
	}
	if result.Status != "healthy" || len(result.Issues) != 0 || result.DecisionChecked != 1 || result.CountersChecked != 1 {
		t.Fatalf("reconciliation = %#v", result)
	}
}

func TestReconcileReportsDecisionAndCounterDrift(t *testing.T) {
	anchor := time.Now().UTC().Add(-time.Hour)
	repo := reconciliationRepository{
		decisions:   []DecisionReconciliationRow{{IdempotencyKey: "consume-1", Subject: "org_1", MeterName: "api", Accepted: true}},
		counters:    []CounterReconciliationRow{{Subject: "org_1", MeterName: "api", Period: "day", PeriodStart: anchor, PeriodEnd: anchor.AddDate(0, 0, 1), EventCount: 2, QuantitySum: 4, QuantityMin: 2, QuantityMax: 2, EventRetentionDays: 90}},
		assignments: []ReconciliationAssignment{{ID: "assignment-1", AssignedAt: anchor, PeriodAnchorAt: anchor}},
	}

	result, err := NewService(&repo, directTransactor{}).Reconcile(context.Background(), ReconciliationQuery{Limit: 10, LookbackHours: 12})
	if err != nil {
		t.Fatal(err)
	}
	if result.Status != "drift_detected" || len(result.Issues) != 3 {
		t.Fatalf("reconciliation = %#v", result)
	}
	if result.Issues[0].Kind != "accepted_decision_missing_event" || result.Issues[1].Kind != "counter_event_count_mismatch" || result.Issues[2].Kind != "counter_quantity_sum_mismatch" {
		t.Fatalf("issues = %#v", result.Issues)
	}
}

func TestRepairCounterRequiresPreviewVersionAndAuditsApply(t *testing.T) {
	anchor := time.Now().UTC().Add(-time.Hour).Truncate(time.Second)
	updatedAt := anchor.Add(30 * time.Minute)
	repo := reconciliationRepository{
		counters: []CounterReconciliationRow{{
			Subject: "org_1", MeterName: "api", Period: "day", PeriodStart: anchor, PeriodEnd: anchor.AddDate(0, 0, 1),
			EventCount: 9, QuantitySum: 9, QuantityMin: 9, QuantityMax: 9, UpdatedAt: updatedAt, EventRetentionDays: 90,
		}},
		events:      []ReconciliationEvent{{ID: "event-1", Quantity: 2, EventTime: anchor.Add(10 * time.Minute), ReceivedAt: anchor.Add(11 * time.Minute)}},
		assignments: []ReconciliationAssignment{{ID: "assignment-1", AssignedAt: anchor, PeriodAnchorAt: anchor}},
	}
	service := NewService(&repo, directTransactor{})
	target := CounterRepairTarget{Subject: "org_1", MeterName: "api", Period: "day", PeriodStart: anchor}
	preview, err := service.RepairCounter(context.Background(), RepairCounterCommand{CounterRepairTarget: target, DryRun: true})
	if err != nil {
		t.Fatal(err)
	}
	if !preview.DryRun || preview.Applied || preview.Before.EventCount != 9 || preview.After.EventCount != 1 || len(repo.repairRuns) != 1 {
		t.Fatalf("preview = %#v runs=%#v", preview, repo.repairRuns)
	}
	applied, err := service.RepairCounter(context.Background(), RepairCounterCommand{CounterRepairTarget: target, ExpectedUpdatedAt: preview.CounterUpdatedAt})
	if err != nil {
		t.Fatal(err)
	}
	if !applied.Applied || applied.DryRun || repo.counters[0].EventCount != 1 || repo.counters[0].QuantitySum != 2 || len(repo.repairRuns) != 2 {
		t.Fatalf("applied = %#v counter=%#v runs=%#v", applied, repo.counters[0], repo.repairRuns)
	}
	if _, err := service.RepairCounter(context.Background(), RepairCounterCommand{CounterRepairTarget: target, ExpectedUpdatedAt: preview.CounterUpdatedAt}); !errors.Is(err, domain.ErrConflict) {
		t.Fatalf("stale apply error = %v, want conflict", err)
	}
	repo.pruneCutoff = anchor.Add(time.Minute)
	if _, err := service.RepairCounter(context.Background(), RepairCounterCommand{CounterRepairTarget: target, DryRun: true}); !errors.Is(err, domain.ErrInvalidInput) {
		t.Fatalf("incomplete source preview error = %v, want invalid input", err)
	}
}
