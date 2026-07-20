package entitlement

import (
	"context"
	"errors"
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/ssubedir/open-spanner/internal/metering/domain"
	domainmeter "github.com/ssubedir/open-spanner/internal/metering/domain/meter"
	domainusage "github.com/ssubedir/open-spanner/internal/metering/domain/usage"
)

func (s *service) AssessConsumption(ctx context.Context, cmd ConsumptionAssessmentCommand) (ConsumptionAssessment, error) {
	subject, err := domainusage.NormalizeSubject(cmd.Subject)
	if err != nil {
		return ConsumptionAssessment{}, err
	}
	meterName := strings.TrimSpace(cmd.Meter)
	if meterName == "" {
		return ConsumptionAssessment{}, fmt.Errorf("%w: meter is required", domain.ErrInvalidInput)
	}
	if !isFinitePositive(cmd.Quantity) {
		return ConsumptionAssessment{}, fmt.Errorf("%w: quantity must be greater than zero", domain.ErrInvalidInput)
	}
	at := cmd.EventTime.UTC()
	if at.IsZero() {
		at = s.now()
	}

	base := ConsumptionAssessment{
		Allowed:       true,
		Subject:       subject,
		MeterName:     meterName,
		Quantity:      cmd.Quantity,
		Enforcement:   EnforcementAdvisory,
		FailurePolicy: FailurePolicyFailOpen,
	}
	assignment, err := s.repo.LockEffectiveSubjectAssignment(ctx, subject, at)
	if errors.Is(err, domain.ErrNotFound) {
		base.State = StateNoPlan
		base.Message = "subject is not assigned to a plan; usage is accepted in advisory mode"
		return base, nil
	}
	if err != nil {
		return base, err
	}
	plan, err := s.findPlan(ctx, assignment.PlanID)
	if err != nil {
		return base, err
	}
	limit, ok := plan.limitForMeter(meterName)
	if !ok {
		base.State = StateNotInPlan
		base.PlanID = plan.Plan.ID
		base.PlanName = plan.Plan.Name
		base.Message = "meter is not included in the subject's plan; usage is accepted in advisory mode"
		return base, nil
	}
	base.Enforcement = limit.Enforcement
	base.FailurePolicy = limit.FailurePolicy

	meters, err := s.meterRepo.Find(ctx, domainmeter.Query{Name: meterName, Limit: 1})
	if err != nil {
		return base, err
	}
	if len(meters) == 0 {
		return base, domain.ErrNotFound
	}
	meter := meters[0]
	from, to := periodWindow(at, assignment.PeriodAnchorAt, limit.Period)
	counter, err := s.repo.GetEntitlementUsageCounter(ctx, CounterQuery{
		Subject: subject, MeterName: meterName, Period: limit.Period, From: from,
	})
	if err != nil && !errors.Is(err, domain.ErrNotFound) {
		return base, err
	}

	current := counterValue(counter, meter.Aggregation(), to.Sub(from).Seconds())
	projected := projectedCounterValue(counter, meter.Aggregation(), cmd.Quantity, at, to.Sub(from).Seconds())
	remaining := math.Max(limit.Limit-projected, 0)
	overage := math.Max(projected-limit.Limit, 0)
	percent := 0.0
	if limit.Limit > 0 {
		percent = projected / limit.Limit * 100
	}
	state := overageState(percent, projected, limit.Limit, limit.WarningPercent)
	allowed := projected <= limit.Limit
	message := "quota is available"
	retryAfter := int64(0)
	if !allowed {
		message = "quota would be exceeded"
		retryAfter = quotaRetryAfterSeconds(s.now(), to)
	} else if state == StateExceeded {
		message = "quota is available; quota limit reached"
	} else if state == StateWarning {
		message = "quota is available; warning threshold reached"
	}

	return ConsumptionAssessment{
		Allowed: allowed, State: state, Subject: subject, MeterName: meterName, Quantity: cmd.Quantity,
		Current: current, Projected: projected, Limit: limit.Limit, Remaining: remaining, Overage: overage,
		PlanID: plan.Plan.ID, PlanName: plan.Plan.Name, Period: limit.Period, From: from, To: to,
		PeriodResetAt: to, RetryAfterSeconds: retryAfter, Enforcement: limit.Enforcement,
		FailurePolicy: limit.FailurePolicy, Message: message,
	}, nil
}

func projectedCounterValue(counter EntitlementUsageCounter, aggregation domainmeter.Aggregation, quantity float64, eventTime time.Time, durationSeconds float64) float64 {
	switch aggregation {
	case domainmeter.AggregationCount:
		return float64(counter.EventCount + 1)
	case domainmeter.AggregationAverage:
		return (counter.QuantitySum + quantity) / float64(counter.EventCount+1)
	case domainmeter.AggregationMinimum:
		if counter.EventCount == 0 {
			return quantity
		}
		return math.Min(counter.QuantityMin, quantity)
	case domainmeter.AggregationMaximum:
		if counter.EventCount == 0 {
			return quantity
		}
		return math.Max(counter.QuantityMax, quantity)
	case domainmeter.AggregationFirst:
		if counter.EventCount == 0 || eventTime.Before(counter.FirstEventTime) {
			return quantity
		}
		return counter.FirstQuantity
	case domainmeter.AggregationLast:
		if counter.EventCount == 0 || !eventTime.Before(counter.LastEventTime) {
			return quantity
		}
		return counter.LastQuantity
	case domainmeter.AggregationRate:
		if durationSeconds <= 0 {
			return 0
		}
		return float64(counter.EventCount+1) / durationSeconds
	default:
		return counter.QuantitySum + quantity
	}
}
