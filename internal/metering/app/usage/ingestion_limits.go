package usage

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/ssubedir/open-spanner/internal/metering/domain"
)

type ingestionCapacityRepository interface {
	ConsumeIngestionCapacity(ctx context.Context, windowStart time.Time, requested, limit int, updatedAt time.Time) (bool, error)
}

type RateLimitError struct {
	retryAfter time.Duration
}

func (e RateLimitError) Error() string {
	return fmt.Sprintf("%v: retry after %s", domain.ErrRateLimited, e.retryAfter.Round(time.Second))
}
func (e RateLimitError) Unwrap() error             { return domain.ErrRateLimited }
func (e RateLimitError) RetryAfter() time.Duration { return e.retryAfter }

func (s *service) checkIngestionCapacity(ctx context.Context, requested int) error {
	if requested <= 0 || s.limits.RateEvents <= 0 || s.limits.RateWindow <= 0 {
		return nil
	}
	repo, ok := s.usageRepo.(ingestionCapacityRepository)
	if !ok {
		return nil
	}
	now := s.now().UTC()
	windowStart := now.Truncate(s.limits.RateWindow)
	allowed, err := repo.ConsumeIngestionCapacity(ctx, windowStart, requested, s.limits.RateEvents, now)
	if err != nil {
		return err
	}
	if allowed {
		return nil
	}
	if s.metrics != nil {
		s.metrics.RecordIngestion(ctx, "rate_limit", "throttled", requested)
	}
	retryAfter := windowStart.Add(s.limits.RateWindow).Sub(now)
	if retryAfter < time.Second {
		retryAfter = time.Second
	}
	return RateLimitError{retryAfter: retryAfter}
}

func RetryAfter(err error) time.Duration {
	var limited interface{ RetryAfter() time.Duration }
	if errors.As(err, &limited) {
		return limited.RetryAfter()
	}
	return 0
}
