package entitlement

import (
	"context"
	"errors"
	"log"
	"sync"
	"time"

	appauth "github.com/ssubedir/open-spanner/internal/auth"
	appentitlement "github.com/ssubedir/open-spanner/internal/metering/app/entitlement"
	"github.com/ssubedir/open-spanner/internal/metering/domain"
)

type Service interface {
	ClaimCheckJob(ctx context.Context, cmd appentitlement.ClaimCommand) (appentitlement.CheckJobResult, bool, error)
	Evaluate(ctx context.Context, cmd appentitlement.EvaluateCommand) (appentitlement.EvaluationResult, error)
	CompleteCheckJob(ctx context.Context, cmd appentitlement.CompleteCommand) error
	FailCheckJob(ctx context.Context, cmd appentitlement.FailCommand) error
}

type Logger func(format string, args ...any)

type Worker struct {
	service     Service
	interval    time.Duration
	lockTTL     time.Duration
	timeout     time.Duration
	retryAfter  time.Duration
	maxAttempts int
	batchSize   int
	logger      Logger
}

func NewWorker(service Service, interval time.Duration, lockTTL time.Duration, timeout time.Duration, retryAfter time.Duration, maxAttempts int, batchSize int, logger Logger) *Worker {
	if logger == nil {
		logger = log.Printf
	}
	return &Worker{
		service:     service,
		interval:    interval,
		lockTTL:     lockTTL,
		timeout:     timeout,
		retryAfter:  retryAfter,
		maxAttempts: maxAttempts,
		batchSize:   batchSize,
		logger:      logger,
	}
}

func (w *Worker) Start(ctx context.Context) func() {
	workerCtx, cancel := context.WithCancel(ctx)
	done := make(chan struct{})

	go func() {
		defer close(done)
		w.run(workerCtx)
	}()

	var once sync.Once
	return func() {
		once.Do(func() {
			cancel()
			<-done
		})
	}
}

func (w *Worker) run(ctx context.Context) {
	if w.service == nil || w.interval <= 0 {
		return
	}

	ticker := time.NewTicker(w.interval)
	defer ticker.Stop()

	w.logger("entitlement worker started: interval=%s lock_ttl=%s timeout=%s retry_after=%s max_attempts=%d batch_size=%d", w.interval, w.lockTTL, w.timeout, w.retryAfter, w.maxAttempts, w.batchSize)
	defer w.logger("entitlement worker stopped")

	for {
		w.drain(ctx)

		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}

func (w *Worker) drain(ctx context.Context) {
	for processed := 0; processed < w.batchSize; processed++ {
		ok, err := w.ProcessOnce(ctx)
		if err != nil {
			w.logger("entitlement check processing failed: error=%v", err)
			return
		}
		if !ok {
			return
		}
	}
}

func (w *Worker) ProcessOnce(ctx context.Context) (bool, error) {
	result, ok, err := w.service.ClaimCheckJob(ctx, appentitlement.ClaimCommand{
		LockTTL:     w.lockTTL,
		MaxAttempts: w.maxAttempts,
	})
	if err != nil || !ok {
		return ok, err
	}
	job := result.Job

	startedAt := time.Now()
	baseCtx := appauth.WithWorkspaceID(ctx, job.WorkspaceID)
	jobCtx := baseCtx
	cancel := func() {}
	if w.timeout > 0 {
		jobCtx, cancel = context.WithTimeout(baseCtx, w.timeout)
	}
	defer cancel()

	evaluation, err := w.service.Evaluate(jobCtx, appentitlement.EvaluateCommand{
		Subject: job.Subject,
		Meter:   job.MeterName,
	})
	duration := time.Since(startedAt).Round(time.Millisecond)
	if err == nil {
		if err := w.service.CompleteCheckJob(baseCtx, appentitlement.CompleteCommand{Subject: job.Subject, Meter: job.MeterName}); err != nil && !errors.Is(err, domain.ErrNotFound) {
			return true, err
		}
		w.logSuccess(job, evaluation, duration)
		return true, nil
	}
	if ctx.Err() != nil && errors.Is(err, context.Canceled) {
		w.logger("entitlement check abandoned during shutdown: subject=%s meter=%s duration=%s", job.Subject, job.MeterName, duration)
		return true, nil
	}

	failCtx, failCancel := context.WithTimeout(appauth.WithWorkspaceID(context.Background(), job.WorkspaceID), 10*time.Second)
	defer failCancel()
	if job.Attempts >= w.maxAttempts {
		if completeErr := w.service.CompleteCheckJob(failCtx, appentitlement.CompleteCommand{Subject: job.Subject, Meter: job.MeterName}); completeErr != nil && !errors.Is(completeErr, domain.ErrNotFound) {
			return true, errors.Join(err, completeErr)
		}
		w.logger("entitlement check failed permanently: subject=%s meter=%s attempts=%d duration=%s error=%v", job.Subject, job.MeterName, job.Attempts, duration, err)
		return true, nil
	}
	if failErr := w.service.FailCheckJob(failCtx, appentitlement.FailCommand{
		Subject:    job.Subject,
		Meter:      job.MeterName,
		RetryAfter: w.retryAfter,
		Error:      err.Error(),
	}); failErr != nil && !errors.Is(failErr, domain.ErrNotFound) {
		return true, errors.Join(err, failErr)
	}
	w.logger("entitlement check failed and requeued: subject=%s meter=%s attempts=%d duration=%s error=%v", job.Subject, job.MeterName, job.Attempts, duration, err)
	return true, nil
}

func (w *Worker) logSuccess(job appentitlement.CheckJob, result appentitlement.EvaluationResult, duration time.Duration) {
	if result.Skipped {
		w.logger("entitlement check skipped: subject=%s meter=%s reason=%q duration=%s", job.Subject, job.MeterName, result.Message, duration)
		return
	}
	if result.State == nil {
		w.logger("entitlement check completed: subject=%s meter=%s duration=%s", job.Subject, job.MeterName, duration)
		return
	}
	eventType := ""
	if result.Event != nil {
		eventType = string(result.Event.Type)
	}
	w.logger("entitlement check completed: subject=%s meter=%s state=%s current=%.4f limit=%.4f event=%s duration=%s", job.Subject, job.MeterName, result.State.State, result.State.Current, result.State.Limit, eventType, duration)
}
