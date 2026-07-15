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
	MarkReconciliationNotified(context.Context, string, string) error
}

type Notifier interface {
	Notify(context.Context, string, appsystem.ReconciliationRun) error
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
	now := time.Now().UTC()
	claim, ok, err := w.service.ClaimScheduledReconciliation(ctx, now, now.Add(w.options.LockTTL))
	if err != nil || !ok {
		return ok, err
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
	if notify && w.options.Notifier != nil {
		if err := w.options.Notifier.Notify(ctx, claim.WorkspaceID, run); err != nil {
			return true, err
		}
		if err := w.service.MarkReconciliationNotified(ctx, claim.WorkspaceID, run.Fingerprint); err != nil {
			return true, err
		}
	}
	w.options.Logger("reconciliation completed: workspace=%s status=%s issues=%d duration=%s", claim.WorkspaceID, run.Status, run.IssueCount, run.Duration.Round(time.Millisecond))
	return true, nil
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
