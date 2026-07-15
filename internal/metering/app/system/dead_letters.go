package system

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/ssubedir/open-spanner/internal/metering/domain"
)

type WorkerDeadLetter struct {
	ID         string
	WorkerName string
	JobKey     string
	RuleID     string
	Subject    string
	MeterName  string
	Attempts   int
	LastError  string
	Status     string
	CreatedAt  time.Time
	RequeuedAt time.Time
}

func (s *service) ListWorkerDeadLetters(ctx context.Context, limit int) ([]WorkerDeadLetter, error) {
	if limit == 0 {
		limit = 50
	}
	if limit < 1 || limit > 200 {
		return nil, domain.ErrInvalidInput
	}
	return s.repo.ListWorkerDeadLetters(ctx, limit)
}

func (s *service) RetryWorkerDeadLetter(ctx context.Context, id string) error {
	id = strings.TrimSpace(id)
	publicID, err := uuid.Parse(id)
	if err != nil {
		return domain.ErrInvalidInput
	}
	id = publicID.String()
	deadLetter, err := s.repo.GetWorkerDeadLetter(ctx, id)
	if err != nil {
		return err
	}
	if deadLetter.Status != "dead_letter" {
		return errors.Join(domain.ErrConflict, errors.New("worker dead letter was already requeued"))
	}
	now := time.Now().UTC()
	return s.transactor.WithinTransaction(ctx, func(txCtx context.Context) error {
		enqueued, err := s.repo.EnqueueWorkerDeadLetter(txCtx, deadLetter, now)
		if err != nil {
			return err
		}
		if !enqueued {
			return errors.Join(domain.ErrConflict, errors.New("worker dead letter is no longer retryable"))
		}
		updated, err := s.repo.MarkWorkerDeadLetterRequeued(txCtx, id, now)
		if err != nil {
			return err
		}
		if !updated {
			return errors.Join(domain.ErrConflict, errors.New("worker dead letter was already requeued"))
		}
		return nil
	})
}
