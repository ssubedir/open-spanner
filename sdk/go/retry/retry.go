// Package retry provides opt-in retry handling for generated REST usage calls.
package retry

import (
	"context"
	"errors"
	"math/rand/v2"
	"net"
	"net/http"
	"reflect"
	"strconv"
	"strings"
	"time"

	"github.com/go-openapi/runtime"
)

type Policy struct {
	MaxAttempts    int
	InitialBackoff time.Duration
	MaxBackoff     time.Duration
	Jitter         float64
	OnRetry        func(Event)
}

type Event struct {
	Attempt int
	Delay   time.Duration
	Err     error
}

func DefaultPolicy() Policy {
	return Policy{MaxAttempts: 3, InitialBackoff: 100 * time.Millisecond, MaxBackoff: 5 * time.Second, Jitter: 0.2}
}

// Do retries a generated REST SDK call when it fails because of a transport
// error, HTTP 429, or a temporary 5xx response.
func Do[T any](ctx context.Context, policy Policy, call func() (T, error)) (T, error) {
	policy = normalize(policy)
	var zero T
	for attempt := 1; attempt <= policy.MaxAttempts; attempt++ {
		result, err := call()
		if err == nil {
			return result, nil
		}
		if attempt == policy.MaxAttempts || !retryable(err) || ctx.Err() != nil {
			return zero, err
		}
		delay := delayFor(err, policy, attempt, time.Now())
		if policy.OnRetry != nil {
			policy.OnRetry(Event{Attempt: attempt + 1, Delay: delay, Err: err})
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

func normalize(policy Policy) Policy {
	defaults := DefaultPolicy()
	if policy.MaxAttempts <= 0 {
		policy.MaxAttempts = defaults.MaxAttempts
	}
	if policy.InitialBackoff <= 0 {
		policy.InitialBackoff = defaults.InitialBackoff
	}
	if policy.MaxBackoff <= 0 {
		policy.MaxBackoff = defaults.MaxBackoff
	} else if policy.MaxBackoff < policy.InitialBackoff {
		policy.MaxBackoff = policy.InitialBackoff
	}
	policy.Jitter = min(1, max(0, policy.Jitter))
	return policy
}

func retryable(err error) bool {
	statusCode, _ := responseInfo(err)
	if statusCode == http.StatusTooManyRequests || statusCode == http.StatusInternalServerError || statusCode == http.StatusBadGateway || statusCode == http.StatusServiceUnavailable || statusCode == http.StatusGatewayTimeout {
		return true
	}
	var network net.Error
	return errors.As(err, &network)
}

func delayFor(err error, policy Policy, attempt int, now time.Time) time.Duration {
	_, retryAfter := responseInfo(err)
	if delay := parseRetryAfter(retryAfter, now); delay > 0 {
		return delay
	}
	delay := min(policy.MaxBackoff, policy.InitialBackoff*time.Duration(1<<(attempt-1)))
	if policy.Jitter > 0 {
		delay = time.Duration(float64(delay) * (1 - policy.Jitter + rand.Float64()*2*policy.Jitter))
	}
	return delay
}

func responseInfo(err error) (int, string) {
	var apiError *runtime.APIError
	if errors.As(err, &apiError) {
		if response, ok := apiError.Response.(runtime.ClientResponse); ok {
			return apiError.Code, response.GetHeader("Retry-After")
		}
		return apiError.Code, ""
	}
	var coded interface{ Code() int }
	if errors.As(err, &coded) {
		return coded.Code(), retryAfterField(err)
	}
	return 0, ""
}

func retryAfterField(err error) string {
	value := reflect.ValueOf(err)
	if value.Kind() == reflect.Pointer && !value.IsNil() {
		value = value.Elem()
	}
	if value.Kind() == reflect.Struct {
		field := value.FieldByName("RetryAfter")
		if field.IsValid() && field.Kind() == reflect.String {
			return field.String()
		}
	}
	return ""
}

func parseRetryAfter(value string, now time.Time) time.Duration {
	value = strings.TrimSpace(value)
	if seconds, err := strconv.Atoi(value); err == nil && seconds > 0 {
		return time.Duration(seconds) * time.Second
	}
	if at, err := http.ParseTime(value); err == nil && at.After(now) {
		return at.Sub(now)
	}
	return 0
}
