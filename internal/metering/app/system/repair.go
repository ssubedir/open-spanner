package system

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/ssubedir/open-spanner/internal/metering/domain"
)

type CounterRepairTarget struct {
	Subject     string
	MeterName   string
	Period      string
	PeriodStart time.Time
}

type RepairCounterCommand struct {
	CounterRepairTarget
	DryRun            bool
	ExpectedUpdatedAt time.Time
}

type CounterSnapshot struct {
	EventCount     int64     `json:"event_count"`
	QuantitySum    float64   `json:"quantity_sum"`
	QuantityMin    float64   `json:"quantity_min"`
	QuantityMax    float64   `json:"quantity_max"`
	FirstQuantity  float64   `json:"first_quantity"`
	FirstEventTime time.Time `json:"first_event_time"`
	LastQuantity   float64   `json:"last_quantity"`
	LastEventTime  time.Time `json:"last_event_time"`
}

type CounterRepairResult struct {
	ID string
	CounterRepairTarget
	PeriodEnd        time.Time
	DryRun           bool
	Applied          bool
	Before           CounterSnapshot
	After            CounterSnapshot
	CounterUpdatedAt time.Time
	CreatedAt        time.Time
}

func (s *service) RepairCounter(ctx context.Context, cmd RepairCounterCommand) (CounterRepairResult, error) {
	cmd.Subject = strings.TrimSpace(cmd.Subject)
	cmd.MeterName = strings.TrimSpace(cmd.MeterName)
	cmd.Period = strings.TrimSpace(cmd.Period)
	if cmd.Subject == "" || cmd.MeterName == "" || cmd.PeriodStart.IsZero() {
		return CounterRepairResult{}, fmt.Errorf("%w: subject, meter, and period_start are required", domain.ErrInvalidInput)
	}
	if !validRepairPeriod(cmd.Period) {
		return CounterRepairResult{}, fmt.Errorf("%w: period must be day, week, month, or year", domain.ErrInvalidInput)
	}
	if !cmd.DryRun && cmd.ExpectedUpdatedAt.IsZero() {
		return CounterRepairResult{}, fmt.Errorf("%w: expected_updated_at is required when applying a repair", domain.ErrInvalidInput)
	}

	var result CounterRepairResult
	err := s.transactor.WithinTransaction(ctx, func(txCtx context.Context) error {
		counter, err := s.repo.GetEntitlementCounterForRepair(txCtx, cmd.CounterRepairTarget)
		if err != nil {
			return err
		}
		now := time.Now().UTC()
		if now.Before(counter.PeriodStart) || !now.Before(counter.PeriodEnd) {
			return fmt.Errorf("%w: only active quota counters can be repaired", domain.ErrInvalidInput)
		}
		pruneCutoff, err := s.repo.FindLatestMeterPruneCutoff(txCtx, counter.MeterName)
		if err != nil {
			return err
		}
		if !counterSourceComplete(counter, now, pruneCutoff) {
			return fmt.Errorf("%w: source usage does not cover the full counter period", domain.ErrInvalidInput)
		}
		if !cmd.DryRun && !cmd.ExpectedUpdatedAt.Equal(counter.UpdatedAt) {
			return fmt.Errorf("%w: quota counter changed after preview", domain.ErrConflict)
		}
		matched, err := s.matchedCounterEvents(txCtx, counter)
		if err != nil {
			return err
		}
		before := snapshotFromCounter(counter)
		after := snapshotFromEvents(matched)
		result = CounterRepairResult{
			ID: uuid.Must(uuid.NewV7()).String(), CounterRepairTarget: cmd.CounterRepairTarget,
			PeriodEnd: counter.PeriodEnd, DryRun: cmd.DryRun, Before: before, After: after,
			CounterUpdatedAt: counter.UpdatedAt, CreatedAt: now,
		}
		if !cmd.DryRun {
			updatedAt := now
			updated, err := s.repo.UpdateEntitlementCounterForRepair(txCtx, counter, cmd.ExpectedUpdatedAt, after, updatedAt)
			if err != nil {
				return err
			}
			if !updated {
				return fmt.Errorf("%w: quota counter changed after preview", domain.ErrConflict)
			}
			result.Applied = true
		}
		return s.repo.SaveQuotaCounterRepairRun(txCtx, result)
	})
	return result, err
}

func (s *service) ListCounterRepairRuns(ctx context.Context, limit int) ([]CounterRepairResult, error) {
	if limit == 0 {
		limit = 50
	}
	if limit < 1 || limit > 200 {
		return nil, fmt.Errorf("%w: limit must be between 1 and 200", domain.ErrInvalidInput)
	}
	return s.repo.ListQuotaCounterRepairRuns(ctx, limit)
}

func snapshotFromCounter(counter CounterReconciliationRow) CounterSnapshot {
	return CounterSnapshot{
		EventCount: counter.EventCount, QuantitySum: counter.QuantitySum, QuantityMin: counter.QuantityMin, QuantityMax: counter.QuantityMax,
		FirstQuantity: counter.FirstQuantity, FirstEventTime: counter.FirstEventTime,
		LastQuantity: counter.LastQuantity, LastEventTime: counter.LastEventTime,
	}
}

func snapshotFromEvents(events []ReconciliationEvent) CounterSnapshot {
	if len(events) == 0 {
		return CounterSnapshot{}
	}
	result := CounterSnapshot{
		QuantityMin: events[0].Quantity, QuantityMax: events[0].Quantity,
		FirstQuantity: events[0].Quantity, FirstEventTime: events[0].EventTime,
		LastQuantity: events[0].Quantity, LastEventTime: events[0].EventTime,
	}
	for _, event := range events {
		result.EventCount++
		result.QuantitySum += event.Quantity
		if event.Quantity < result.QuantityMin {
			result.QuantityMin = event.Quantity
		}
		if event.Quantity > result.QuantityMax {
			result.QuantityMax = event.Quantity
		}
		if event.EventTime.Before(result.FirstEventTime) {
			result.FirstEventTime, result.FirstQuantity = event.EventTime, event.Quantity
		}
		if !event.EventTime.Before(result.LastEventTime) {
			result.LastEventTime, result.LastQuantity = event.EventTime, event.Quantity
		}
	}
	return result
}

func validRepairPeriod(period string) bool {
	switch period {
	case "day", "week", "month", "year":
		return true
	}
	return false
}
