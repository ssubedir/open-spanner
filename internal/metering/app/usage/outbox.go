package usage

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/ssubedir/open-spanner/internal/metering/domain"
	domainusage "github.com/ssubedir/open-spanner/internal/metering/domain/usage"
)

type OutboxClaimCommand struct {
	LockTTL     time.Duration
	MaxAttempts int
}

type OutboxCompleteCommand struct {
	ID         string
	ClaimToken string
}

type OutboxFailCommand struct {
	ID          string
	ClaimToken  string
	RetryAfter  time.Duration
	MaxAttempts int
	Error       string
}

func (s *service) ClaimOutbox(ctx context.Context, cmd OutboxClaimCommand) (domainusage.OutboxMessage, bool, error) {
	if cmd.LockTTL <= 0 || cmd.MaxAttempts <= 0 {
		return domainusage.OutboxMessage{}, false, domain.ErrInvalidInput
	}
	now := s.now()
	message, err := s.usageRepo.ClaimOutbox(ctx, now, now.Add(cmd.LockTTL), newID(), cmd.MaxAttempts)
	if errors.Is(err, domain.ErrNotFound) {
		return domainusage.OutboxMessage{}, false, nil
	}
	return message, err == nil, err
}

func (s *service) CompleteOutbox(ctx context.Context, cmd OutboxCompleteCommand) error {
	if strings.TrimSpace(cmd.ID) == "" || strings.TrimSpace(cmd.ClaimToken) == "" {
		return domain.ErrInvalidInput
	}
	return s.usageRepo.CompleteOutbox(ctx, cmd.ID, cmd.ClaimToken, s.now())
}

func (s *service) FailOutbox(ctx context.Context, cmd OutboxFailCommand) error {
	if strings.TrimSpace(cmd.ID) == "" || strings.TrimSpace(cmd.ClaimToken) == "" || cmd.RetryAfter <= 0 || cmd.MaxAttempts <= 0 || strings.TrimSpace(cmd.Error) == "" {
		return fmt.Errorf("%w: invalid outbox failure", domain.ErrInvalidInput)
	}
	now := s.now()
	return s.usageRepo.RetryOutbox(ctx, cmd.ID, cmd.ClaimToken, now.Add(cmd.RetryAfter), cmd.MaxAttempts, cmd.Error, now)
}
