package system

import (
	"context"
	"testing"
	"time"
)

type reconciliationRepository struct {
	decisions   []DecisionReconciliationRow
	counters    []CounterReconciliationRow
	events      []ReconciliationEvent
	assignments []ReconciliationAssignment
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

func TestReconcileHealthy(t *testing.T) {
	anchor := time.Now().UTC().Add(-time.Hour)
	eventTime := anchor.Add(10 * time.Minute)
	repo := reconciliationRepository{
		decisions:   []DecisionReconciliationRow{{IdempotencyKey: "consume-1", Subject: "org_1", MeterName: "api", Accepted: true, EventID: "event-1", EventSubject: "org_1", EventMeterName: "api"}},
		counters:    []CounterReconciliationRow{{Subject: "org_1", MeterName: "api", Period: "day", PeriodStart: anchor, PeriodEnd: anchor.AddDate(0, 0, 1), EventCount: 1, QuantitySum: 2, QuantityMin: 2, QuantityMax: 2}},
		events:      []ReconciliationEvent{{ID: "event-1", Quantity: 2, EventTime: eventTime}},
		assignments: []ReconciliationAssignment{{ID: "assignment-1", AssignedAt: anchor, PeriodAnchorAt: anchor}},
	}

	result, err := NewService(repo).Reconcile(context.Background(), ReconciliationQuery{})
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
		counters:    []CounterReconciliationRow{{Subject: "org_1", MeterName: "api", Period: "day", PeriodStart: anchor, PeriodEnd: anchor.AddDate(0, 0, 1), EventCount: 2, QuantitySum: 4, QuantityMin: 2, QuantityMax: 2}},
		assignments: []ReconciliationAssignment{{ID: "assignment-1", AssignedAt: anchor, PeriodAnchorAt: anchor}},
	}

	result, err := NewService(repo).Reconcile(context.Background(), ReconciliationQuery{Limit: 10, LookbackHours: 12})
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
