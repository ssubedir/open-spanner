package stream

import (
	"errors"
	"testing"
	"time"

	"google.golang.org/genproto/googleapis/rpc/errdetails"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/durationpb"
)

func TestRetryDelayHonorsRetryInfo(t *testing.T) {
	want := 7 * time.Second
	withDetails, err := status.New(codes.ResourceExhausted, "limited").WithDetails(&errdetails.RetryInfo{RetryDelay: durationpb.New(want)})
	if err != nil {
		t.Fatal(err)
	}
	if got := retryDelay(withDetails.Err(), DefaultRetryPolicy(), 1); got != want {
		t.Fatalf("retry delay = %s, want %s", got, want)
	}
}

func TestRetryDelayUsesCappedBackoff(t *testing.T) {
	policy := RetryPolicy{MaxAttempts: 5, InitialBackoff: time.Second, MaxBackoff: 3 * time.Second}
	err := status.Error(codes.Unavailable, "offline")
	if got := retryDelay(err, policy, 4); got != 3*time.Second {
		t.Fatalf("retry delay = %s, want 3s", got)
	}
}

func TestRetryableErrorRejectsPermanentFailures(t *testing.T) {
	if retryableError(status.Error(codes.InvalidArgument, "bad input")) {
		t.Fatal("invalid argument should not be retried")
	}
	if retryableError(errors.New("local validation")) {
		t.Fatal("local error should not be retried")
	}
}
