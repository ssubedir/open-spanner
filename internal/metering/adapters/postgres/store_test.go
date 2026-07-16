package postgres

import (
	"context"
	"errors"
	"sync"
	"testing"

	"github.com/jackc/pgx/v5/pgconn"
)

func TestRunWithTransactionRetries(t *testing.T) {
	metrics := &transactionRetryRecorder{}
	attempts := 0
	err := runWithTransactionRetries(context.Background(), metrics, func() error {
		attempts++
		if attempts < transactionMaxAttempts {
			return &pgconn.PgError{Code: "40001"}
		}
		return nil
	})
	if err != nil {
		t.Fatalf("retry transaction: %v", err)
	}
	if attempts != transactionMaxAttempts || metrics.retries != transactionMaxAttempts-1 || metrics.exhausted != 0 {
		t.Fatalf("attempts=%d retries=%d exhausted=%d", attempts, metrics.retries, metrics.exhausted)
	}
}

func TestRunWithTransactionRetriesExhausted(t *testing.T) {
	metrics := &transactionRetryRecorder{}
	attempts := 0
	err := runWithTransactionRetries(context.Background(), metrics, func() error {
		attempts++
		return &pgconn.PgError{Code: "40P01"}
	})
	if err == nil {
		t.Fatal("exhausted transaction returned no error")
	}
	if attempts != transactionMaxAttempts || metrics.retries != transactionMaxAttempts-1 || metrics.exhausted != 1 {
		t.Fatalf("attempts=%d retries=%d exhausted=%d", attempts, metrics.retries, metrics.exhausted)
	}
	if metrics.lastReason != "deadlock_detected" {
		t.Fatalf("reason=%q", metrics.lastReason)
	}
}

func TestRunWithTransactionRetriesRejectsUnsafeErrors(t *testing.T) {
	want := &pgconn.PgError{Code: "23505"}
	attempts := 0
	err := runWithTransactionRetries(context.Background(), &transactionRetryRecorder{}, func() error {
		attempts++
		return want
	})
	if !errors.Is(err, want) || attempts != 1 {
		t.Fatalf("error=%v attempts=%d", err, attempts)
	}
}

func TestRunWithTransactionRetriesHonorsCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	attempts := 0
	err := runWithTransactionRetries(ctx, &transactionRetryRecorder{}, func() error {
		attempts++
		cancel()
		return &pgconn.PgError{Code: "40001"}
	})
	if !errors.Is(err, context.Canceled) || attempts != 1 {
		t.Fatalf("error=%v attempts=%d", err, attempts)
	}
}

type transactionRetryRecorder struct {
	mu         sync.Mutex
	retries    int
	exhausted  int
	lastReason string
}

func (r *transactionRetryRecorder) RecordTransactionRetry(_ context.Context, reason string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.retries++
	r.lastReason = reason
}

func (r *transactionRetryRecorder) RecordTransactionRetryExhausted(_ context.Context, reason string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.exhausted++
	r.lastReason = reason
}
