package consumption

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	appentitlement "github.com/ssubedir/open-spanner/internal/metering/app/entitlement"
	"github.com/ssubedir/open-spanner/internal/metering/app/page"
	apptransaction "github.com/ssubedir/open-spanner/internal/metering/app/transaction"
	appusage "github.com/ssubedir/open-spanner/internal/metering/app/usage"
	"github.com/ssubedir/open-spanner/internal/metering/domain"
	domainconsumption "github.com/ssubedir/open-spanner/internal/metering/domain/consumption"
)

var (
	errFailOpen = errors.New("quota evaluation failed open")
)

type Service interface {
	Consume(ctx context.Context, cmd Command) (Result, error)
	PruneDecisions(ctx context.Context, cmd PruneCommand) (PruneResult, error)
	GetDecision(ctx context.Context, idempotencyKey string) (DecisionResult, error)
	ListDecisions(ctx context.Context, query DecisionListQuery) (DecisionListResult, error)
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
	Find(ctx context.Context, idempotencyKey string) (domainconsumption.Decision, error)
	Save(ctx context.Context, idempotencyKey string, snapshot []byte) (stored domainconsumption.Decision, created bool, err error)
	List(ctx context.Context, query domainconsumption.Query) ([]domainconsumption.Decision, error)
	CountExpired(ctx context.Context, before time.Time) (int, error)
	PruneExpired(ctx context.Context, before time.Time) (int, error)
	SavePruneRun(ctx context.Context, id string, before time.Time, dryRun bool, deleted int, createdAt time.Time) error
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

type PruneCommand struct {
	Before time.Time
	DryRun bool
}

type PruneResult struct {
	ID        string
	Before    time.Time
	Deleted   int
	DryRun    bool
	CreatedAt time.Time
}

type DecisionResult struct {
	IdempotencyKey string
	Result         Result
	CreatedAt      time.Time
}

type DecisionListQuery struct {
	Subject          string
	MeterName        string
	Outcome          string
	EvaluationFailed *bool
	Enforcement      string
	State            string
	Limit            int
	Cursor           string
}

type DecisionListResult struct {
	Items      []DecisionResult
	NextCursor string
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
	stored, created, err := s.repo.Save(ctx, idempotencyKey, snapshot)
	if err != nil {
		return err
	}
	var storedResult Result
	if err := json.Unmarshal(stored.Snapshot, &storedResult); err != nil {
		return err
	}
	storedResult.Replayed = !created
	*result = storedResult
	return nil
}

func (s *service) find(ctx context.Context, idempotencyKey string) (Result, error) {
	stored, err := s.repo.Find(ctx, idempotencyKey)
	if err != nil {
		return Result{}, err
	}
	var result Result
	if err := json.Unmarshal(stored.Snapshot, &result); err != nil {
		return Result{}, err
	}
	return result, nil
}

func (s *service) GetDecision(ctx context.Context, idempotencyKey string) (DecisionResult, error) {
	key := strings.TrimSpace(idempotencyKey)
	if key == "" {
		return DecisionResult{}, fmt.Errorf("%w: idempotency key is required", domain.ErrInvalidInput)
	}
	stored, err := s.repo.Find(ctx, key)
	if err != nil {
		return DecisionResult{}, err
	}
	return decisionResult(stored)
}

func (s *service) ListDecisions(ctx context.Context, query DecisionListQuery) (DecisionListResult, error) {
	cursor, err := page.Decode(query.Cursor)
	if err != nil {
		return DecisionListResult{}, err
	}
	limit := query.Limit
	if limit <= 0 {
		limit = 50
	}
	if limit > 200 {
		limit = 200
	}
	var accepted *bool
	switch strings.ToLower(strings.TrimSpace(query.Outcome)) {
	case "", "all":
	case "accepted":
		v := true
		accepted = &v
	case "rejected":
		v := false
		accepted = &v
	default:
		return DecisionListResult{}, fmt.Errorf("%w: outcome must be accepted or rejected", domain.ErrInvalidInput)
	}
	stored, err := s.repo.List(ctx, domainconsumption.Query{
		Subject: strings.TrimSpace(query.Subject), MeterName: strings.TrimSpace(query.MeterName), Accepted: accepted,
		EvaluationFailed: query.EvaluationFailed, Enforcement: strings.TrimSpace(query.Enforcement), State: strings.TrimSpace(query.State),
		CursorCreatedAt: cursor.Time, CursorID: cursor.ID, Limit: limit + 1,
	})
	if err != nil {
		return DecisionListResult{}, err
	}
	next := ""
	if len(stored) > limit {
		last := stored[limit-1]
		next, err = page.Encode(page.Cursor{Time: last.CreatedAt, ID: last.IdempotencyKey})
		if err != nil {
			return DecisionListResult{}, err
		}
		stored = stored[:limit]
	}
	items := make([]DecisionResult, 0, len(stored))
	for _, decision := range stored {
		item, err := decisionResult(decision)
		if err != nil {
			return DecisionListResult{}, err
		}
		items = append(items, item)
	}
	return DecisionListResult{Items: items, NextCursor: next}, nil
}

func decisionResult(stored domainconsumption.Decision) (DecisionResult, error) {
	var result Result
	if err := json.Unmarshal(stored.Snapshot, &result); err != nil {
		return DecisionResult{}, err
	}
	return DecisionResult{IdempotencyKey: stored.IdempotencyKey, Result: result, CreatedAt: stored.CreatedAt}, nil
}

func (s *service) PruneDecisions(ctx context.Context, cmd PruneCommand) (PruneResult, error) {
	before := cmd.Before.UTC()
	if before.IsZero() {
		return PruneResult{}, fmt.Errorf("%w: decision prune cutoff is required", domain.ErrInvalidInput)
	}
	result := PruneResult{ID: uuid.Must(uuid.NewV7()).String(), Before: before, DryRun: cmd.DryRun, CreatedAt: time.Now().UTC()}
	err := s.transactor.WithinTransaction(ctx, func(txCtx context.Context) error {
		var err error
		if cmd.DryRun {
			result.Deleted, err = s.repo.CountExpired(txCtx, before)
		} else {
			result.Deleted, err = s.repo.PruneExpired(txCtx, before)
		}
		if err != nil {
			return err
		}
		return s.repo.SavePruneRun(txCtx, result.ID, before, cmd.DryRun, result.Deleted, result.CreatedAt)
	})
	return result, err
}
