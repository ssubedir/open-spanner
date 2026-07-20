package retry

import (
	"context"
	"net/http"
	"testing"
	"time"
)

type codedError int

func (e codedError) Error() string { return "request failed" }
func (e codedError) Code() int     { return int(e) }

type guidedError struct{ RetryAfter string }

func (e *guidedError) Error() string { return "limited" }
func (e *guidedError) Code() int     { return 429 }

func TestRetryableHTTPStatuses(t *testing.T) {
	if !retryable(codedError(429)) || !retryable(codedError(503)) {
		t.Fatal("429 and 503 should be retried")
	}
	if retryable(codedError(400)) || retryable(codedError(413)) {
		t.Fatal("permanent client errors should not be retried")
	}
}

func TestParseRetryAfter(t *testing.T) {
	now := time.Date(2026, 7, 16, 0, 0, 0, 0, time.UTC)
	if got := parseRetryAfter("7", now); got != 7*time.Second {
		t.Fatalf("delay = %s, want 7s", got)
	}
	if got := parseRetryAfter(now.Add(4*time.Second).Format(http.TimeFormat), now); got != 4*time.Second {
		t.Fatalf("date delay = %s, want 4s", got)
	}
}

func TestGeneratedErrorRetryAfterField(t *testing.T) {
	policy := Policy{InitialBackoff: time.Second, MaxBackoff: time.Second}
	if got := delayFor(&guidedError{RetryAfter: "9"}, policy, 1, time.Now()); got != 9*time.Second {
		t.Fatalf("delay = %s, want 9s", got)
	}
}

func TestDoRetriesTemporaryFailure(t *testing.T) {
	calls := 0
	policy := Policy{MaxAttempts: 2, InitialBackoff: time.Millisecond, MaxBackoff: time.Millisecond}
	result, err := Do(context.Background(), policy, func() (string, error) {
		calls++
		if calls == 1 {
			return "", codedError(503)
		}
		return "accepted", nil
	})
	if err != nil || result != "accepted" || calls != 2 {
		t.Fatalf("result=%q calls=%d err=%v", result, calls, err)
	}
}
