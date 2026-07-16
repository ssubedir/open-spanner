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
	return s.create(ctx, "", cmd)
}

func (s *service) CreateIngestion(ctx context.Context, kind string, cmd CreateCommand) (Result, error) {
	return s.create(ctx, kind, cmd)
}

func (s *service) create(ctx context.Context, kind string, cmd CreateCommand) (Result, error) {
	if err := s.checkIngestionCapacity(ctx, 1); err != nil {
		return Result{}, err
	}
	event, err := s.newEvent(ctx, cmd, map[string]domainmeter.Meter{})
	if err != nil {
		return Result{}, err
	}

	requestedEventID := event.ID()
	var run domainusage.IngestionRun
	if kind == "" {
		event, err = s.usageRepo.Save(ctx, event)
	} else {
		err = s.transactor.WithinTransaction(ctx, func(txCtx context.Context) error {
			var saveErr error
			event, saveErr = s.usageRepo.Save(txCtx, event)
			if saveErr != nil {
				return saveErr
			}
			replayed := event.ID() != requestedEventID
			run, saveErr = s.saveIngestionRun(txCtx, IngestionCommand{
				Kind: kind, Accepted: boolInt(!replayed), Duplicates: boolInt(replayed),
			})
			return saveErr
		})
		if err == nil {
			s.recordIngestionMetrics(ctx, run)
		}
	}
	if err != nil {
		return Result{}, err
	}

	result := eventResultFromDomain(event)
	result.Replayed = event.ID() != requestedEventID
	return result, nil
}

func (s *service) GetByIdempotencyKey(ctx context.Context, idempotencyKey string) (Result, error) {
	idempotencyKey = strings.TrimSpace(idempotencyKey)
	if idempotencyKey == "" {
		return Result{}, fmt.Errorf("%w: idempotency key is required", domain.ErrInvalidInput)
	}
	event, err := s.usageRepo.FindEventByIdempotencyKey(ctx, idempotencyKey)
	if err != nil {
		return Result{}, err
	}
	return eventResultFromDomain(event), nil
}

func (s *service) CreateBulk(ctx context.Context, idempotencyKey string, commands []CreateCommand) (BulkResult, error) {
	return s.createBulk(ctx, "", idempotencyKey, commands, 0)
}

func (s *service) CreateBulkIngestion(ctx context.Context, kind string, idempotencyKey string, commands []CreateCommand, initialFailures int) (BulkResult, error) {
	return s.createBulk(ctx, kind, idempotencyKey, commands, initialFailures)
}

func (s *service) createBulk(ctx context.Context, kind string, idempotencyKey string, commands []CreateCommand, initialFailures int) (BulkResult, error) {
	idempotencyKey = strings.TrimSpace(idempotencyKey)

	if len(commands) == 0 {
		return BulkResult{}, fmt.Errorf("%w: at least one usage event is required", domain.ErrInvalidInput)
	}
	if len(commands) > s.limits.MaxBatchEvents {
		return BulkResult{}, fmt.Errorf("%w: bulk usage event limit is %d", domain.ErrInvalidInput, s.limits.MaxBatchEvents)
	}
	if err := s.checkIngestionCapacity(ctx, len(commands)); err != nil {
		return BulkResult{}, err
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
		result := BulkResult{Failed: failures}
		if kind == "" {
			return result, nil
		}
		run, err := s.saveIngestionRun(ctx, IngestionCommand{Kind: kind, Failed: initialFailures + len(failures)})
		if err != nil {
			return BulkResult{}, err
		}
		s.recordIngestionMetrics(ctx, run)
		return result, nil
	}

	var saved domainusage.BulkSaveResult
	var run domainusage.IngestionRun
	var err error
	if kind == "" {
		saved, err = s.usageRepo.SaveBulk(ctx, idempotencyKey, events)
	} else {
		err = s.transactor.WithinTransaction(ctx, func(txCtx context.Context) error {
			var saveErr error
			saved, saveErr = s.usageRepo.SaveBulk(txCtx, idempotencyKey, events)
			if saveErr != nil {
				return saveErr
			}
			accepted, duplicates := len(saved.Accepted()), len(saved.Duplicates())
			if saved.Replayed() {
				accepted, duplicates = 0, len(saved.Events())
			}
			run, saveErr = s.saveIngestionRun(txCtx, IngestionCommand{
				Kind: kind, Accepted: accepted, Duplicates: duplicates, Failed: initialFailures + len(failures),
			})
			return saveErr
		})
		if err == nil {
			s.recordIngestionMetrics(ctx, run)
		}
	}
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
	run, err := s.saveIngestionRun(ctx, cmd)
	if err != nil {
		return IngestionResult{}, err
	}
	s.recordIngestionMetrics(ctx, run)
	return ingestionResultFromDomain(run), nil
}

func (s *service) saveIngestionRun(ctx context.Context, cmd IngestionCommand) (domainusage.IngestionRun, error) {
	run, err := domainusage.NewIngestionRun(
		newID(),
		domainusage.IngestionKind(cmd.Kind),
		cmd.Accepted,
		cmd.Duplicates,
		cmd.Failed,
		s.now(),
	)
	if err != nil {
		return domainusage.IngestionRun{}, err
	}

	run, err = s.usageRepo.SaveIngestionRun(ctx, run)
	if err != nil {
		return domainusage.IngestionRun{}, err
	}
	return run, nil
}

func (s *service) recordIngestionMetrics(ctx context.Context, run domainusage.IngestionRun) {
	if s.metrics != nil {
		kind := string(run.Kind())
		s.metrics.RecordIngestion(ctx, kind, "accepted", run.Accepted())
		s.metrics.RecordIngestion(ctx, kind, "duplicate", run.Duplicates())
		s.metrics.RecordIngestion(ctx, kind, "rejected", run.Failed())
	}
}

func boolInt(value bool) int {
	if value {
		return 1
	}
	return 0
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
