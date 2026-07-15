package system

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/ssubedir/open-spanner/internal/metering/domain"
)

type reconciliationRepository struct {
	decisions   []DecisionReconciliationRow
	counters    []CounterReconciliationRow
	events      []ReconciliationEvent
	assignments []ReconciliationAssignment
	repairRuns  []CounterRepairResult
	pruneCutoff time.Time
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
