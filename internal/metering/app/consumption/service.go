package consumption

import (
	"context"
	"encoding/json"
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
	errFailOpen = errors.New("quota evaluation failed open")
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

type Repository interface {
	Find(ctx context.Context, idempotencyKey string) ([]byte, error)
	Save(ctx context.Context, idempotencyKey string, snapshot []byte) (stored []byte, created bool, err error)
}

type UsageService interface {
	Create(ctx context.Context, cmd appusage.CreateCommand) (appusage.Result, error)
	GetByIdempotencyKey(ctx context.Context, idempotencyKey string) (appusage.Result, error)
}

type EntitlementService interface {
	AssessConsumption(ctx context.Context, cmd appentitlement.ConsumptionAssessmentCommand) (appentitlement.ConsumptionAssessment, error)
}

type service struct {
	repo         Repository
	usage        UsageService
	entitlements EntitlementService
	transactor   apptransaction.Transactor
}

func NewService(repo Repository, usage UsageService, entitlements EntitlementService, transactor apptransaction.Transactor) Service {
	if repo == nil || usage == nil || entitlements == nil || transactor == nil {
		panic("consumption service requires repository, usage, entitlement, and transaction services")
	}
	return &service{repo: repo, usage: usage, entitlements: entitlements, transactor: transactor}
}

func (s *service) Consume(ctx context.Context, cmd Command) (Result, error) {
	cmd.IdempotencyKey = strings.TrimSpace(cmd.IdempotencyKey)
	if cmd.IdempotencyKey == "" {
		return Result{}, fmt.Errorf("%w: idempotency key is required", domain.ErrInvalidInput)
	}
	if existing, err := s.find(ctx, cmd.IdempotencyKey); err == nil {
		existing.Replayed = true
		return existing, nil
	} else if !errors.Is(err, domain.ErrNotFound) {
		return Result{}, err
	}

	var result Result
	err := s.transactor.WithinTransaction(ctx, func(txCtx context.Context) error {
		assessment, err := s.entitlements.AssessConsumption(txCtx, appentitlement.ConsumptionAssessmentCommand{
			Subject: cmd.Subject, Meter: cmd.MeterName, Quantity: cmd.Quantity, EventTime: cmd.EventTime,
		})
		result.Quota = assessment
		if err != nil {
			if assessment.FailurePolicy == appentitlement.FailurePolicyFailClosed {
				result.EvaluationFailed = true
				return s.persist(txCtx, cmd.IdempotencyKey, &result)
			}
			return errors.Join(errFailOpen, err)
		}
		if existing, findErr := s.find(txCtx, cmd.IdempotencyKey); findErr == nil {
			result = existing
			result.Replayed = true
			return nil
		} else if !errors.Is(findErr, domain.ErrNotFound) {
			return findErr
		}

		existing, findErr := s.usage.GetByIdempotencyKey(txCtx, cmd.IdempotencyKey)
		if findErr == nil {
			result.Accepted = true
			result.Event = existing
			return s.persist(txCtx, cmd.IdempotencyKey, &result)
		}
		if !errors.Is(findErr, domain.ErrNotFound) {
			return findErr
		}
		if assessment.Enforcement == appentitlement.EnforcementHard && !assessment.Allowed {
			return s.persist(txCtx, cmd.IdempotencyKey, &result)
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
		return s.persist(txCtx, cmd.IdempotencyKey, &result)
	})
	if errors.Is(err, errFailOpen) {
		result.EvaluationFailed = true
		saveErr := s.transactor.WithinTransaction(ctx, func(txCtx context.Context) error {
			if existing, findErr := s.find(txCtx, cmd.IdempotencyKey); findErr == nil {
				result = existing
				result.Replayed = true
				return nil
			} else if !errors.Is(findErr, domain.ErrNotFound) {
				return findErr
			}
			event, createErr := s.usage.Create(txCtx, appusage.CreateCommand{
				IdempotencyKey: cmd.IdempotencyKey,
				Subject:        cmd.Subject,
				MeterName:      cmd.MeterName,
				Quantity:       cmd.Quantity,
				EventTime:      cmd.EventTime,
				Metadata:       cmd.Metadata,
			})
			if createErr != nil {
				return createErr
			}
			result.Accepted = true
			result.Event = event
			return s.persist(txCtx, cmd.IdempotencyKey, &result)
		})
		if saveErr != nil {
			return Result{}, saveErr
		}
		return result, nil
	}
	if err != nil {
		return Result{}, err
	}
	return result, nil
}

func (s *service) persist(ctx context.Context, idempotencyKey string, result *Result) error {
	result.Replayed = false
	snapshot, err := json.Marshal(*result)
	if err != nil {
		return err
	}
	storedSnapshot, created, err := s.repo.Save(ctx, idempotencyKey, snapshot)
	if err != nil {
		return err
	}
	var stored Result
	if err := json.Unmarshal(storedSnapshot, &stored); err != nil {
		return err
	}
	stored.Replayed = !created
	*result = stored
	return nil
}

func (s *service) find(ctx context.Context, idempotencyKey string) (Result, error) {
	snapshot, err := s.repo.Find(ctx, idempotencyKey)
	if err != nil {
		return Result{}, err
	}
	var result Result
	if err := json.Unmarshal(snapshot, &result); err != nil {
		return Result{}, err
	}
	return result, nil
}
