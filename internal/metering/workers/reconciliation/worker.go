package reconciliation

import (
	"context"
	"log"
	"sync"
	"time"

	appsystem "github.com/ssubedir/open-spanner/internal/metering/app/system"
)

type Service interface {
	ClaimScheduledReconciliation(context.Context, time.Time, time.Time) (appsystem.ReconciliationClaim, bool, error)
	RunScheduledReconciliation(context.Context, appsystem.ReconciliationClaim, time.Duration, appsystem.ReconciliationQuery) (appsystem.ReconciliationRun, bool, error)
	FailScheduledReconciliation(context.Context, appsystem.ReconciliationClaim, time.Time, error) error
	ClaimReconciliationNotification(context.Context, time.Time, time.Time) (appsystem.ReconciliationNotification, bool, error)
	CompleteReconciliationNotification(context.Context, appsystem.ReconciliationNotification) error
	RetryReconciliationNotification(context.Context, appsystem.ReconciliationNotification, time.Time, int, error) error
}

type Notifier interface {
	Notify(context.Context, appsystem.ReconciliationNotification) error
}

type Logger func(format string, args ...any)

type Options struct {
	PollInterval     time.Duration
	ScheduleInterval time.Duration
	LockTTL          time.Duration
	Timeout          time.Duration
	RetryAfter       time.Duration
	Limit            int
	LookbackHours    int
	MaxAttempts      int
	Notifier         Notifier
	Logger           Logger
}

type Worker struct {
	service Service
	options Options
}

func NewWorker(service Service, options Options) *Worker {
	if options.Logger == nil {
		options.Logger = log.Printf
	}
	return &Worker{service: service, options: options}
}

func (w *Worker) ProcessOnce(ctx context.Context) (bool, error) {
	delivered, err := w.ProcessDeliveryOnce(ctx)
	if err != nil {
		return delivered, err
	}
	now := time.Now().UTC()
	claim, ok, err := w.service.ClaimScheduledReconciliation(ctx, now, now.Add(w.options.LockTTL))
	if err != nil || !ok {
		return delivered || ok, err
	}
	run, notify, err := w.service.RunScheduledReconciliation(ctx, claim, w.options.ScheduleInterval, appsystem.ReconciliationQuery{Limit: w.options.Limit, LookbackHours: w.options.LookbackHours})
	if err != nil {
		failCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
		defer cancel()
		if failErr := w.service.FailScheduledReconciliation(failCtx, claim, time.Now().UTC().Add(w.options.RetryAfter), err); failErr != nil {
			w.options.Logger("reconciliation failure audit failed: workspace=%s error=%v", claim.WorkspaceID, failErr)
		}
		return true, err
	}
	_ = notify
	w.options.Logger("reconciliation completed: workspace=%s status=%s issues=%d duration=%s", claim.WorkspaceID, run.Status, run.IssueCount, run.Duration.Round(time.Millisecond))
	return true, nil
}

func (w *Worker) ProcessDeliveryOnce(ctx context.Context) (bool, error) {
	if w.options.Notifier == nil {
		return false, nil
	}
	now := time.Now().UTC()
	notification, ok, err := w.service.ClaimReconciliationNotification(ctx, now, now.Add(w.options.LockTTL))
	if err != nil || !ok {
		return ok, err
	}
	if err := w.options.Notifier.Notify(ctx, notification); err != nil {
		delay := deliveryRetryDelay(w.options.RetryAfter, notification.Attempts)
		if retryErr := w.service.RetryReconciliationNotification(ctx, notification, time.Now().UTC().Add(delay), w.options.MaxAttempts, err); retryErr != nil {
			return true, retryErr
		}
		w.options.Logger("reconciliation notification failed: id=%s type=%s attempt=%d error=%v", notification.ID, notification.EventType, notification.Attempts+1, err)
		return true, nil
	}
	if err := w.service.CompleteReconciliationNotification(ctx, notification); err != nil {
		return true, err
	}
	w.options.Logger("reconciliation notification delivered: id=%s type=%s workspace=%s", notification.ID, notification.EventType, notification.WorkspaceID)
	return true, nil
}

func deliveryRetryDelay(base time.Duration, attempts int) time.Duration {
	if base <= 0 {
		base = time.Minute
	}
	for range attempts {
		if base >= time.Hour/2 {
			return time.Hour
		}
		base *= 2
	}
	return base
}

func (w *Worker) Start(ctx context.Context) func() {
	workerCtx, cancel := context.WithCancel(ctx)
	done := make(chan struct{})
	go func() {
		defer close(done)
		w.run(workerCtx)
	}()
	var once sync.Once
	return func() { once.Do(func() { cancel(); <-done }) }
}

func (w *Worker) run(ctx context.Context) {
	if w.service == nil || w.options.PollInterval <= 0 {
		return
	}
	ticker := time.NewTicker(w.options.PollInterval)
	defer ticker.Stop()
	w.options.Logger("reconciliation worker started: poll=%s schedule=%s", w.options.PollInterval, w.options.ScheduleInterval)
	defer w.options.Logger("reconciliation worker stopped")
	for {
		runCtx, cancel := context.WithTimeout(ctx, w.options.Timeout)
		_, err := w.ProcessOnce(runCtx)
		cancel()
		if err != nil && ctx.Err() == nil {
			w.options.Logger("reconciliation failed: error=%v", err)
		}
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}
