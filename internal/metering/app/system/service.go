package system

import (
	"context"
	"fmt"
	"math"
	"time"

	apptransaction "github.com/ssubedir/open-spanner/internal/metering/app/transaction"
	"github.com/ssubedir/open-spanner/internal/metering/domain"
)

type Service interface {
	Stats(ctx context.Context) (StatsResult, error)
	Reconcile(ctx context.Context, query ReconciliationQuery) (ReconciliationResult, error)
	RepairCounter(ctx context.Context, cmd RepairCounterCommand) (CounterRepairResult, error)
	ListCounterRepairRuns(ctx context.Context, limit int) ([]CounterRepairResult, error)
}

type service struct {
	repo       Repository
	transactor apptransaction.Transactor
}

type Repository interface {
	FindStats(ctx context.Context) (StatsResult, error)
	ListDecisionReconciliationRows(ctx context.Context, since time.Time, limit int) ([]DecisionReconciliationRow, error)
	ListActiveEntitlementCounters(ctx context.Context, now time.Time, limit int) ([]CounterReconciliationRow, error)
	ListCounterReconciliationEvents(ctx context.Context, counter CounterReconciliationRow) ([]ReconciliationEvent, error)
	ListCounterReconciliationAssignments(ctx context.Context, counter CounterReconciliationRow) ([]ReconciliationAssignment, error)
	GetEntitlementCounterForRepair(ctx context.Context, target CounterRepairTarget) (CounterReconciliationRow, error)
	UpdateEntitlementCounterForRepair(ctx context.Context, counter CounterReconciliationRow, expectedUpdatedAt time.Time, replacement CounterSnapshot, updatedAt time.Time) (bool, error)
	SaveQuotaCounterRepairRun(ctx context.Context, run CounterRepairResult) error
	ListQuotaCounterRepairRuns(ctx context.Context, limit int) ([]CounterRepairResult, error)
	FindLatestMeterPruneCutoff(ctx context.Context, meterName string) (time.Time, error)
}

const (
	defaultReconciliationLimit         = 100
	maxReconciliationLimit             = 500
	defaultReconciliationLookbackHours = 24
	maxReconciliationLookbackHours     = 720
)

type ReconciliationQuery struct {
	Limit         int
	LookbackHours int
}

type ReconciliationResult struct {
	Status          string
	DecisionChecked int
	CountersChecked int
	Issues          []ReconciliationIssue
	Truncated       bool
	LookbackHours   int
	CheckedAt       time.Time
}

type ReconciliationIssue struct {
	Kind           string
	Severity       string
	Subject        string
	MeterName      string
	IdempotencyKey string
	Period         string
	PeriodStart    time.Time
	Expected       string
	Actual         string
	Message        string
}

type DecisionReconciliationRow struct {
	IdempotencyKey string
	Subject        string
	MeterName      string
	Accepted       bool
	CreatedAt      time.Time
	EventID        string
	EventSubject   string
	EventMeterName string
}

type CounterReconciliationRow struct {
	Subject            string
	MeterName          string
	Period             string
	PeriodStart        time.Time
	PeriodEnd          time.Time
	EventCount         int64
	QuantitySum        float64
	QuantityMin        float64
	QuantityMax        float64
	UpdatedAt          time.Time
	EventRetentionDays int
	FirstQuantity      float64
	FirstEventTime     time.Time
	LastQuantity       float64
	LastEventTime      time.Time
}

type ReconciliationEvent struct {
	ID         string
	Quantity   float64
	EventTime  time.Time
	ReceivedAt time.Time
}

type ReconciliationAssignment struct {
	ID             string
	AssignedAt     time.Time
	PeriodAnchorAt time.Time
	UnassignedAt   time.Time
}

type StatsResult struct {
	Meters               int
	UsageEvents          int
	PruneRuns            int
	LastPruneRun         LastPruneRunResult
	ConsumptionDecisions int
	DecisionPruneRuns    int
	LastDecisionPruneRun LastDecisionPruneRunResult
}

type LastDecisionPruneRunResult struct {
	ID        string
	Before    time.Time
	Deleted   int
	DryRun    bool
	CreatedAt time.Time
}

type LastPruneRunResult struct {
	ID        string
	Deleted   int
	DryRun    bool
	CreatedAt time.Time
}

func NewService(repo Repository, transactor apptransaction.Transactor) Service {
	if repo == nil || transactor == nil {
		panic("system service requires repository and transactor")
	}
	return &service{repo: repo, transactor: transactor}
}

func (s *service) Stats(ctx context.Context) (StatsResult, error) {
	return s.repo.FindStats(ctx)
}

func (s *service) Reconcile(ctx context.Context, query ReconciliationQuery) (ReconciliationResult, error) {
	limit := query.Limit
	if limit == 0 {
		limit = defaultReconciliationLimit
	}
	if limit < 1 || limit > maxReconciliationLimit {
		return ReconciliationResult{}, fmt.Errorf("%w: limit must be between 1 and %d", domain.ErrInvalidInput, maxReconciliationLimit)
	}
	lookback := query.LookbackHours
	if lookback == 0 {
		lookback = defaultReconciliationLookbackHours
	}
	if lookback < 1 || lookback > maxReconciliationLookbackHours {
		return ReconciliationResult{}, fmt.Errorf("%w: lookback_hours must be between 1 and %d", domain.ErrInvalidInput, maxReconciliationLookbackHours)
	}

	now := time.Now().UTC()
	result := ReconciliationResult{Status: "healthy", Issues: []ReconciliationIssue{}, LookbackHours: lookback, CheckedAt: now}
	decisions, err := s.repo.ListDecisionReconciliationRows(ctx, now.Add(-time.Duration(lookback)*time.Hour), limit+1)
	if err != nil {
		return ReconciliationResult{}, err
	}
	if len(decisions) > limit {
		result.Truncated = true
		decisions = decisions[:limit]
	}
	result.DecisionChecked = len(decisions)
	for _, row := range decisions {
		result.Issues = append(result.Issues, decisionIssues(row)...)
	}

	counters, err := s.repo.ListActiveEntitlementCounters(ctx, now, limit+1)
	if err != nil {
		return ReconciliationResult{}, err
	}
	if len(counters) > limit {
		result.Truncated = true
		counters = counters[:limit]
	}
	pruneCutoffs := make(map[string]time.Time)
	for _, counter := range counters {
		pruneCutoff, ok := pruneCutoffs[counter.MeterName]
		if !ok {
			pruneCutoff, err = s.repo.FindLatestMeterPruneCutoff(ctx, counter.MeterName)
			if err != nil {
				return ReconciliationResult{}, err
			}
			pruneCutoffs[counter.MeterName] = pruneCutoff
		}
		if !counterSourceComplete(counter, now, pruneCutoff) {
			continue
		}
		result.CountersChecked++
		issues, err := s.reconcileCounter(ctx, counter)
		if err != nil {
			return ReconciliationResult{}, err
		}
		result.Issues = append(result.Issues, issues...)
	}
	if len(result.Issues) > 0 {
		result.Status = "drift_detected"
	}
	return result, nil
}

func decisionIssues(row DecisionReconciliationRow) []ReconciliationIssue {
	base := ReconciliationIssue{Severity: "critical", Subject: row.Subject, MeterName: row.MeterName, IdempotencyKey: row.IdempotencyKey}
	if row.Accepted && row.EventID == "" {
		base.Kind, base.Expected, base.Actual = "accepted_decision_missing_event", "usage event", "missing"
		base.Message = "An accepted consumption decision has no matching usage event."
		return []ReconciliationIssue{base}
	}
	if !row.Accepted && row.EventID != "" {
		base.Kind, base.Expected, base.Actual = "rejected_decision_has_event", "no usage event", row.EventID
		base.Message = "A rejected consumption decision has a matching usage event."
		return []ReconciliationIssue{base}
	}
	if row.EventID != "" && (row.Subject != row.EventSubject || row.MeterName != row.EventMeterName) {
		base.Kind = "decision_event_identity_mismatch"
		base.Expected = row.Subject + "/" + row.MeterName
		base.Actual = row.EventSubject + "/" + row.EventMeterName
		base.Message = "The decision and usage event disagree on subject or meter."
		return []ReconciliationIssue{base}
	}
	return nil
}

func (s *service) reconcileCounter(ctx context.Context, counter CounterReconciliationRow) ([]ReconciliationIssue, error) {
	matched, err := s.matchedCounterEvents(ctx, counter)
	if err != nil {
		return nil, err
	}
	expectedCount := int64(len(matched))
	expectedSum, expectedMin, expectedMax := counterAggregates(matched)
	base := ReconciliationIssue{Severity: "critical", Subject: counter.Subject, MeterName: counter.MeterName, Period: counter.Period, PeriodStart: counter.PeriodStart}
	issues := make([]ReconciliationIssue, 0, 4)
	if counter.EventCount != expectedCount {
		issue := base
		issue.Kind, issue.Expected, issue.Actual = "counter_event_count_mismatch", fmt.Sprint(expectedCount), fmt.Sprint(counter.EventCount)
		issue.Message = "The active quota counter event count differs from source usage."
		issues = append(issues, issue)
	}
	if !floatEqual(counter.QuantitySum, expectedSum) {
		issues = append(issues, counterValueIssue(base, "counter_quantity_sum_mismatch", expectedSum, counter.QuantitySum, "sum"))
	}
	if expectedCount > 0 && !floatEqual(counter.QuantityMin, expectedMin) {
		issues = append(issues, counterValueIssue(base, "counter_quantity_min_mismatch", expectedMin, counter.QuantityMin, "minimum"))
	}
	if expectedCount > 0 && !floatEqual(counter.QuantityMax, expectedMax) {
		issues = append(issues, counterValueIssue(base, "counter_quantity_max_mismatch", expectedMax, counter.QuantityMax, "maximum"))
	}
	return issues, nil
}

func (s *service) matchedCounterEvents(ctx context.Context, counter CounterReconciliationRow) ([]ReconciliationEvent, error) {
	events, err := s.repo.ListCounterReconciliationEvents(ctx, counter)
	if err != nil {
		return nil, err
	}
	assignments, err := s.repo.ListCounterReconciliationAssignments(ctx, counter)
	if err != nil {
		return nil, err
	}
	matched := make([]ReconciliationEvent, 0, len(events))
	for _, event := range events {
		assignment, ok := effectiveAssignment(assignments, event.EventTime)
		if !ok {
			continue
		}
		from, _ := counterWindow(event.EventTime, assignment.PeriodAnchorAt, counter.Period)
		if from.Equal(counter.PeriodStart) {
			matched = append(matched, event)
		}
	}
	return matched, nil
}

func counterSourceComplete(counter CounterReconciliationRow, now, pruneCutoff time.Time) bool {
	retentionComplete := counter.EventRetentionDays > 0 && !counter.PeriodStart.Before(now.AddDate(0, 0, -counter.EventRetentionDays))
	pruneComplete := pruneCutoff.IsZero() || !counter.PeriodStart.Before(pruneCutoff)
	return retentionComplete && pruneComplete
}

func effectiveAssignment(assignments []ReconciliationAssignment, at time.Time) (ReconciliationAssignment, bool) {
	for _, assignment := range assignments {
		if !assignment.AssignedAt.After(at) && (assignment.UnassignedAt.IsZero() || assignment.UnassignedAt.After(at)) {
			return assignment, true
		}
	}
	return ReconciliationAssignment{}, false
}

func counterWindow(at, anchor time.Time, period string) (time.Time, time.Time) {
	from, to := anchor.UTC(), addCounterPeriod(anchor.UTC(), period)
	for !at.UTC().Before(to) {
		from, to = to, addCounterPeriod(to, period)
	}
	return from, to
}

func addCounterPeriod(from time.Time, period string) time.Time {
	switch period {
	case "day":
		return from.AddDate(0, 0, 1)
	case "week":
		return from.AddDate(0, 0, 7)
	case "year":
		return from.AddDate(1, 0, 0)
	default:
		return from.AddDate(0, 1, 0)
	}
}

func counterAggregates(events []ReconciliationEvent) (sum, min, max float64) {
	if len(events) == 0 {
		return 0, 0, 0
	}
	min, max = events[0].Quantity, events[0].Quantity
	for _, event := range events {
		sum += event.Quantity
		min = math.Min(min, event.Quantity)
		max = math.Max(max, event.Quantity)
	}
	return sum, min, max
}

func counterValueIssue(base ReconciliationIssue, kind string, expected, actual float64, field string) ReconciliationIssue {
	base.Kind, base.Expected, base.Actual = kind, fmt.Sprintf("%.10g", expected), fmt.Sprintf("%.10g", actual)
	base.Message = "The active quota counter " + field + " differs from source usage."
	return base
}

func floatEqual(a, b float64) bool {
	return math.Abs(a-b) <= 1e-9*math.Max(1, math.Max(math.Abs(a), math.Abs(b)))
}
