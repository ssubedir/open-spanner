package outbox

import (
	"context"
	"errors"
	"log"
	"sync"
	"time"

	appauth "github.com/ssubedir/open-spanner/internal/auth"
	appalert "github.com/ssubedir/open-spanner/internal/metering/app/alert"
	appentitlement "github.com/ssubedir/open-spanner/internal/metering/app/entitlement"
	appusage "github.com/ssubedir/open-spanner/internal/metering/app/usage"
	"github.com/ssubedir/open-spanner/internal/metering/domain"
	domainusage "github.com/ssubedir/open-spanner/internal/metering/domain/usage"
)

type Service interface {
	ClaimOutbox(context.Context, appusage.OutboxClaimCommand) (domainusage.OutboxMessage, bool, error)
	CompleteOutbox(context.Context, appusage.OutboxCompleteCommand) error
	FailOutbox(context.Context, appusage.OutboxFailCommand) error
}

type AlertEnqueuer interface {
	EnqueueForUsageEvents(context.Context, []appalert.UsageEvent) error
}

type EntitlementEnqueuer interface {
	EnqueueForUsageEvents(context.Context, []appentitlement.UsageEvent) error
}

type Options struct {
	PollInterval time.Duration
	LockTTL      time.Duration
	Timeout      time.Duration
	RetryAfter   time.Duration
	MaxAttempts  int
	BatchSize    int
	Logger       func(string, ...any)
}

type Worker struct {
	service      Service
	alerts       AlertEnqueuer
	entitlements EntitlementEnqueuer
	options      Options
}

func NewWorker(service Service, alerts AlertEnqueuer, entitlements EntitlementEnqueuer, options Options) *Worker {
	if options.Logger == nil {
		options.Logger = log.Printf
	}
	return &Worker{service: service, alerts: alerts, entitlements: entitlements, options: options}
}

func (w *Worker) ProcessOnce(ctx context.Context) (bool, error) {
	message, ok, err := w.service.ClaimOutbox(ctx, appusage.OutboxClaimCommand{LockTTL: w.options.LockTTL, MaxAttempts: w.options.MaxAttempts})
	if err != nil || !ok {
		return ok, err
	}
	baseCtx := appauth.WithWorkspaceID(ctx, message.WorkspaceID)
	jobCtx := baseCtx
	cancel := func() {}
	if w.options.Timeout > 0 {
		jobCtx, cancel = context.WithTimeout(baseCtx, w.options.Timeout)
	}
	defer cancel()

	err = w.alerts.EnqueueForUsageEvents(jobCtx, []appalert.UsageEvent{{Subject: message.Subject, Meter: message.MeterName, Metadata: message.Metadata}})
	if err == nil {
		err = w.entitlements.EnqueueForUsageEvents(jobCtx, []appentitlement.UsageEvent{{Subject: message.Subject, Meter: message.MeterName, Quantity: message.Quantity}})
	}
	if err == nil {
		err = w.service.CompleteOutbox(baseCtx, appusage.OutboxCompleteCommand{ID: message.ID, ClaimToken: message.ClaimToken})
		if errors.Is(err, domain.ErrNotFound) {
			return true, nil
		}
		return true, err
	}
	if ctx.Err() != nil && errors.Is(err, context.Canceled) {
		return true, nil
	}

	failCtx, failCancel := context.WithTimeout(appauth.WithWorkspaceID(context.Background(), message.WorkspaceID), 10*time.Second)
	defer failCancel()
	failErr := w.service.FailOutbox(failCtx, appusage.OutboxFailCommand{ID: message.ID, ClaimToken: message.ClaimToken, RetryAfter: retryDelay(w.options.RetryAfter, message.Attempts), MaxAttempts: w.options.MaxAttempts, Error: err.Error()})
	if errors.Is(failErr, domain.ErrNotFound) {
		return true, nil
	}
	if failErr != nil {
		return true, errors.Join(err, failErr)
	}
	return true, nil
}

func retryDelay(base time.Duration, attempts int) time.Duration {
	if base <= 0 {
		base = time.Second
	}
	for i := 1; i < attempts; i++ {
		if base >= time.Hour/2 {
			return time.Hour
		}
		base *= 2
	}
	return min(base, time.Hour)
}

func (w *Worker) Start(ctx context.Context) func() {
	workerCtx, cancel := context.WithCancel(ctx)
	done := make(chan struct{})
	go func() { defer close(done); w.run(workerCtx) }()
	var once sync.Once
	return func() { once.Do(func() { cancel(); <-done }) }
}

func (w *Worker) run(ctx context.Context) {
	if w.service == nil || w.alerts == nil || w.entitlements == nil || w.options.PollInterval <= 0 {
		return
	}
	ticker := time.NewTicker(w.options.PollInterval)
	defer ticker.Stop()
	w.options.Logger("usage outbox worker started: poll=%s lock_ttl=%s batch_size=%d", w.options.PollInterval, w.options.LockTTL, w.options.BatchSize)
	defer w.options.Logger("usage outbox worker stopped")
	for {
		for processed := 0; processed < w.options.BatchSize; processed++ {
			ok, err := w.ProcessOnce(ctx)
			if err != nil {
				w.options.Logger("usage outbox dispatch failed: %v", err)
				break
			}
			if !ok {
				break
			}
		}
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}
