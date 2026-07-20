package stream

import (
	"context"
	"errors"
	"math/rand/v2"
	"time"

	"google.golang.org/genproto/googleapis/rpc/errdetails"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// RetryPolicy controls opt-in retries for unary ingestion calls.
type RetryPolicy struct {
	MaxAttempts    int
	InitialBackoff time.Duration
	MaxBackoff     time.Duration
	Jitter         float64
	OnRetry        func(RetryEvent)
}

// RetryEvent describes a retry before the client waits and resubmits.
type RetryEvent struct {
	Attempt int
	Delay   time.Duration
	Err     error
}

// DefaultRetryPolicy returns conservative retry defaults. MaxAttempts includes
// the initial request.
func DefaultRetryPolicy() RetryPolicy {
	return RetryPolicy{MaxAttempts: 3, InitialBackoff: 100 * time.Millisecond, MaxBackoff: 5 * time.Second, Jitter: 0.2}
}

func normalizeRetryPolicy(policy RetryPolicy) RetryPolicy {
	defaults := DefaultRetryPolicy()
	if policy.MaxAttempts <= 0 {
		policy.MaxAttempts = defaults.MaxAttempts
	}
	if policy.InitialBackoff <= 0 {
		policy.InitialBackoff = defaults.InitialBackoff
	}
	if policy.MaxBackoff <= 0 {
		policy.MaxBackoff = defaults.MaxBackoff
	}
	if policy.MaxBackoff < policy.InitialBackoff {
		policy.MaxBackoff = policy.InitialBackoff
	}
	if policy.Jitter < 0 {
		policy.Jitter = 0
	}
	if policy.Jitter > 1 {
		policy.Jitter = 1
	}
	return policy
}

func retryUnary[T any](ctx context.Context, policy *RetryPolicy, call func() (T, error)) (T, error) {
	var zero T
	if policy == nil {
		return call()
	}
	for attempt := 1; attempt <= policy.MaxAttempts; attempt++ {
		result, err := call()
		if err == nil {
			return result, nil
		}
		if attempt == policy.MaxAttempts || !retryableError(err) || ctx.Err() != nil {
			return zero, err
		}
		delay := retryDelay(err, *policy, attempt)
		if policy.OnRetry != nil {
			policy.OnRetry(RetryEvent{Attempt: attempt + 1, Delay: delay, Err: err})
		}
		timer := time.NewTimer(delay)
		select {
		case <-ctx.Done():
			timer.Stop()
			return zero, errors.Join(ctx.Err(), err)
		case <-timer.C:
		}
	}
	return zero, nil
}

func retryableError(err error) bool {
	switch status.Code(err) {
	case codes.ResourceExhausted, codes.Unavailable, codes.DeadlineExceeded:
		return true
	default:
		return false
	}
}

func retryDelay(err error, policy RetryPolicy, attempt int) time.Duration {
	if st, ok := status.FromError(err); ok {
		for _, detail := range st.Details() {
			if retry, ok := detail.(*errdetails.RetryInfo); ok && retry.RetryDelay != nil && retry.RetryDelay.AsDuration() > 0 {
				return retry.RetryDelay.AsDuration()
			}
		}
	}
	delay := policy.InitialBackoff
	for step := 1; step < attempt && delay < policy.MaxBackoff; step++ {
		delay *= 2
		if delay > policy.MaxBackoff {
			delay = policy.MaxBackoff
		}
	}
	if policy.Jitter > 0 {
		factor := 1 - policy.Jitter + rand.Float64()*(2*policy.Jitter)
		delay = time.Duration(float64(delay) * factor)
	}
	return delay
}
