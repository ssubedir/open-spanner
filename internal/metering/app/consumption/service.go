package consumption

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	appentitlement "github.com/ssubedir/open-spanner/internal/metering/app/entitlement"
	apptransaction "github.com/ssubedir/open-spanner/internal/metering/app/transaction"
	appusage "github.com/ssubedir/open-spanner/internal/metering/app/usage"
	"github.com/ssubedir/open-spanner/internal/metering/domain"
)

var (
	errHardLimitRejected = errors.New("hard quota limit rejected consumption")
	errFailOpen          = errors.New("quota evaluation failed open")
)

type Service interface {
	Consume(ctx context.Context, cmd Command) (Result, error)
}

type Command struct {
	IdempotencyKey string
	Subject        string
	MeterName      string
	Quantity       float64
	EventTime      time.Time
	Metadata       map[string]any
}

type Result struct {
	Accepted         bool
	Replayed         bool
	EvaluationFailed bool
	Event            appusage.Result
	Quota            appentitlement.ConsumptionAssessment
}

type service struct {
	usage        appusage.Service
	entitlements appentitlement.Service
	transactor   apptransaction.Transactor
}

func NewService(usage appusage.Service, entitlements appentitlement.Service, transactor apptransaction.Transactor) Service {
	if usage == nil || entitlements == nil || transactor == nil {
		panic("consumption service requires usage, entitlement, and transaction services")
	}
	return &service{usage: usage, entitlements: entitlements, transactor: transactor}
}

func (s *service) Consume(ctx context.Context, cmd Command) (Result, error) {
	cmd.IdempotencyKey = strings.TrimSpace(cmd.IdempotencyKey)
	if cmd.IdempotencyKey == "" {
		return Result{}, fmt.Errorf("%w: idempotency key is required", domain.ErrInvalidInput)
	}

	var result Result
	err := s.transactor.WithinTransaction(ctx, func(txCtx context.Context) error {
		assessment, err := s.entitlements.AssessConsumption(txCtx, appentitlement.ConsumptionAssessmentCommand{
			Subject: cmd.Subject, Meter: cmd.MeterName, Quantity: cmd.Quantity, EventTime: cmd.EventTime,
		})
		result.Quota = assessment
		if err != nil {
			if assessment.FailurePolicy == appentitlement.FailurePolicyFailClosed {
				return err
			}
			return errors.Join(errFailOpen, err)
		}

		existing, findErr := s.usage.GetByIdempotencyKey(txCtx, cmd.IdempotencyKey)
		if findErr == nil {
			result.Accepted = true
			result.Replayed = true
			result.Event = existing
			return nil
		}
		if !errors.Is(findErr, domain.ErrNotFound) {
			return findErr
		}
		if assessment.Enforcement == appentitlement.EnforcementHard && !assessment.Allowed {
			return errHardLimitRejected
		}

		event, err := s.usage.Create(txCtx, appusage.CreateCommand{
			IdempotencyKey: cmd.IdempotencyKey,
			Subject:        cmd.Subject,
			MeterName:      cmd.MeterName,
			Quantity:       cmd.Quantity,
			EventTime:      cmd.EventTime,
			Metadata:       cmd.Metadata,
		})
		if err != nil {
			return err
		}
		result.Accepted = true
		result.Event = event
		return nil
	})
	if errors.Is(err, errHardLimitRejected) {
		return result, nil
	}
	if errors.Is(err, errFailOpen) {
		event, saveErr := s.usage.Create(ctx, appusage.CreateCommand{
			IdempotencyKey: cmd.IdempotencyKey,
			Subject:        cmd.Subject,
			MeterName:      cmd.MeterName,
			Quantity:       cmd.Quantity,
			EventTime:      cmd.EventTime,
			Metadata:       cmd.Metadata,
		})
		if saveErr != nil {
			return Result{}, saveErr
		}
		result.Accepted = true
		result.EvaluationFailed = true
		result.Event = event
		return result, nil
	}
	if err != nil {
		return Result{}, err
	}
	return result, nil
}
