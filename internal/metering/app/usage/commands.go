package usage

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/ssubedir/open-spanner/internal/metering/domain"
	domainmeter "github.com/ssubedir/open-spanner/internal/metering/domain/meter"
	domainusage "github.com/ssubedir/open-spanner/internal/metering/domain/usage"
)

type CreateCommand struct {
	Index          int
	IdempotencyKey string
	Subject        string
	MeterName      string
	Quantity       float64
	EventTime      time.Time
	Metadata       map[string]any
}

type PruneCommand struct {
	DryRun bool
}

type IngestionCommand struct {
	Kind       string
	Accepted   int
	Duplicates int
	Failed     int
}

const MaxBulkEvents = 1000

func (s *service) Create(ctx context.Context, cmd CreateCommand) (Result, error) {
	event, err := s.newEvent(ctx, cmd, map[string]domainmeter.Meter{})
	if err != nil {
		return Result{}, err
	}

	event, err = s.usageRepo.Save(ctx, event)
	if err != nil {
		return Result{}, err
	}

	return eventResultFromDomain(event), nil
}

func (s *service) CreateBulk(ctx context.Context, idempotencyKey string, commands []CreateCommand) (BulkResult, error) {
	idempotencyKey = strings.TrimSpace(idempotencyKey)

	if len(commands) == 0 {
		return BulkResult{}, fmt.Errorf("%w: at least one usage event is required", domain.ErrInvalidInput)
	}
	if len(commands) > MaxBulkEvents {
		return BulkResult{}, fmt.Errorf("%w: bulk usage event limit is %d", domain.ErrInvalidInput, MaxBulkEvents)
	}

	meters := map[string]domainmeter.Meter{}
	events := make([]domainusage.Event, 0, len(commands))
	failures := []BulkFailureResult{}
	for _, cmd := range commands {
		event, err := s.newEvent(ctx, cmd, meters)
		if err != nil {
			if isBulkItemFailure(err) {
				failures = append(failures, bulkFailureFromError(cmd.Index, err))
				continue
			}
			return BulkResult{}, err
		}
		events = append(events, event)
	}

	if len(events) == 0 {
		return BulkResult{Failed: failures}, nil
	}

	saved, err := s.usageRepo.SaveBulk(ctx, idempotencyKey, events)
	if err != nil {
		return BulkResult{}, err
	}

	result := bulkResultFromDomain(saved)
	result.Failed = failures
	return result, nil
}

func isBulkItemFailure(err error) bool {
	return errors.Is(err, domain.ErrInvalidInput) || errors.Is(err, domain.ErrNotFound)
}

func bulkFailureFromError(index int, err error) BulkFailureResult {
	code := "invalid_input"
	if errors.Is(err, domain.ErrNotFound) {
		code = "not_found"
	}

	return BulkFailureResult{
		Index:   index,
		Code:    code,
		Message: err.Error(),
	}
}

func (s *service) RecordIngestion(ctx context.Context, cmd IngestionCommand) (IngestionResult, error) {
	run, err := domainusage.NewIngestionRun(
		newID(),
		domainusage.IngestionKind(cmd.Kind),
		cmd.Accepted,
		cmd.Duplicates,
		cmd.Failed,
		s.now(),
	)
	if err != nil {
		return IngestionResult{}, err
	}

	run, err = s.usageRepo.SaveIngestionRun(ctx, run)
	if err != nil {
		return IngestionResult{}, err
	}

	return ingestionResultFromDomain(run), nil
}

func (s *service) newEvent(ctx context.Context, cmd CreateCommand, meters map[string]domainmeter.Meter) (domainusage.Event, error) {
	if cmd.EventTime.IsZero() {
		cmd.EventTime = s.now()
	}

	meter, exists := meters[cmd.MeterName]
	if !exists {
		found, err := s.meterRepo.Find(ctx, domainmeter.Query{Name: cmd.MeterName})
		if err != nil {
			return domainusage.Event{}, err
		}
		if len(found) == 0 {
			return domainusage.Event{}, domain.ErrNotFound
		}
		meter = found[0]
		meters[cmd.MeterName] = meter
	}
	metadata, err := meter.NormalizeMetadata(cmd.Metadata)
	if err != nil {
		return domainusage.Event{}, err
	}

	event, err := domainusage.NewEvent(
		newID(),
		cmd.IdempotencyKey,
		cmd.Subject,
		meter.Name(),
		cmd.Quantity,
		cmd.EventTime,
		s.now(),
		metadata,
	)
	if err != nil {
		return domainusage.Event{}, err
	}

	return event, nil
}
