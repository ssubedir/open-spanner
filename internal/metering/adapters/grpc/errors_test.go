package grpcadapter

import (
	"errors"
	"testing"
	"time"

	"github.com/ssubedir/open-spanner/internal/metering/domain"
	"google.golang.org/genproto/googleapis/rpc/errdetails"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type retryableTestError struct {
	delay time.Duration
}

func (e retryableTestError) Error() string             { return domain.ErrRateLimited.Error() }
func (e retryableTestError) Unwrap() error             { return domain.ErrRateLimited }
func (e retryableTestError) RetryAfter() time.Duration { return e.delay }

func TestServiceErrorReturnsResourceExhaustedWithRetryInfo(t *testing.T) {
	delay := 17 * time.Second
	err := serviceError(errors.Join(errors.New("ingestion denied"), retryableTestError{delay: delay}))
	st := status.Convert(err)
	if st.Code() != codes.ResourceExhausted {
		t.Fatalf("code = %s, want %s", st.Code(), codes.ResourceExhausted)
	}
	for _, detail := range st.Details() {
		if retry, ok := detail.(*errdetails.RetryInfo); ok {
			if got := retry.RetryDelay.AsDuration(); got != delay {
				t.Fatalf("retry delay = %s, want %s", got, delay)
			}
			return
		}
	}
	t.Fatal("ResourceExhausted error did not include google.rpc.RetryInfo")
}
