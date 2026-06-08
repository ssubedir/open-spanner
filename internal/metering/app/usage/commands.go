package usage

import (
	"context"
	"time"

	"open-spanner/internal/metering/domain"
	domainmeter "open-spanner/internal/metering/domain/meter"
	domainusage "open-spanner/internal/metering/domain/usage"
)

type CreateCommand struct {
	IdempotencyKey string
	Subject        string
	MeterName      string
	Quantity       float64
	EventTime      time.Time
	Metadata       map[string]any
}

func (s *service) Create(ctx context.Context, cmd CreateCommand) (Result, error) {
	if cmd.EventTime.IsZero() {
		cmd.EventTime = s.now()
	}

	meters, err := s.meterRepo.Find(ctx, domainmeter.Query{Name: cmd.MeterName})
	if err != nil {
		return Result{}, err
	}
	if len(meters) == 0 {
		return Result{}, domain.ErrNotFound
	}
	meter := meters[0]

	event, err := domainusage.NewEvent(
		newID(),
		cmd.IdempotencyKey,
		cmd.Subject,
		meter.Name(),
		cmd.Quantity,
		cmd.EventTime,
		s.now(),
		cmd.Metadata,
	)
	if err != nil {
		return Result{}, err
	}

	event, err = s.usageRepo.Save(ctx, event)
	if err != nil {
		return Result{}, err
	}

	return eventResultFromDomain(event), nil
}
