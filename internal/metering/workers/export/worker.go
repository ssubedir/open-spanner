package export

import (
	"context"
	"errors"
	"io"
	"log"
	"sync"
	"time"

	"github.com/ssubedir/open-spanner/internal/metering/adapters/fileexport"
	appusage "github.com/ssubedir/open-spanner/internal/metering/app/usage"
)

type Service interface {
	ClaimExportJob(ctx context.Context, cmd appusage.ExportJobClaimCommand) (appusage.ExportJobResult, bool, error)
	CompleteExportJob(ctx context.Context, cmd appusage.ExportJobCompleteCommand) (appusage.ExportJobResult, error)
	FailExportJob(ctx context.Context, cmd appusage.ExportJobFailCommand) (appusage.ExportJobResult, error)
	List(ctx context.Context, query appusage.ListQuery) ([]appusage.ListItemResult, error)
}

type Logger func(format string, args ...any)

type Worker struct {
	service     Service
	store       fileexport.Store
	interval    time.Duration
	lockTTL     time.Duration
	timeout     time.Duration
	maxAttempts int
	logger      Logger
}

func NewWorker(service Service, store fileexport.Store, interval time.Duration, lockTTL time.Duration, timeout time.Duration, maxAttempts int, logger Logger) *Worker {
	if logger == nil {
		logger = log.Printf
	}
	return &Worker{
		service:     service,
		store:       store,
		interval:    interval,
		lockTTL:     lockTTL,
		timeout:     timeout,
		maxAttempts: maxAttempts,
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

	w.logger("export worker started: interval=%s lock_ttl=%s timeout=%s max_attempts=%d", w.interval, w.lockTTL, w.timeout, w.maxAttempts)
	defer w.logger("export worker stopped")

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
	for {
		processed, err := w.ProcessOnce(ctx)
		if err != nil {
			w.logger("export job processing failed: error=%v", err)
			return
		}
		if !processed {
			return
		}
	}
}

func (w *Worker) ProcessOnce(ctx context.Context) (bool, error) {
	job, ok, err := w.service.ClaimExportJob(ctx, appusage.ExportJobClaimCommand{
		LockTTL:     w.lockTTL,
		MaxAttempts: w.maxAttempts,
	})
	if err != nil || !ok {
		return ok, err
	}

	startedAt := time.Now()
	jobCtx := ctx
	cancel := func() {}
	if w.timeout > 0 {
		jobCtx, cancel = context.WithTimeout(ctx, w.timeout)
	}
	defer cancel()

	err = w.process(jobCtx, job)
	duration := time.Since(startedAt).Round(time.Millisecond)
	if err == nil {
		w.logger("export job completed: job_id=%s duration=%s", job.ID, duration)
		return true, nil
	}
	if ctx.Err() != nil && errors.Is(err, context.Canceled) {
		w.logger("export job abandoned during shutdown: job_id=%s duration=%s", job.ID, duration)
		return true, nil
	}

	failCtx, failCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer failCancel()
	if _, failErr := w.service.FailExportJob(failCtx, appusage.ExportJobFailCommand{
		ID:           job.ID,
		ErrorMessage: err.Error(),
	}); failErr != nil {
		return true, errors.Join(err, failErr)
	}
	w.logger("export job failed: job_id=%s duration=%s error=%v", job.ID, duration, err)
	return true, nil
}

func (w *Worker) process(ctx context.Context, job appusage.ExportJobResult) error {
	query, err := appusage.ParseExportListQueryJSON(job.QueryJSON)
	if err != nil {
		return err
	}

	buckets, err := w.service.List(ctx, query)
	if err != nil {
		return err
	}

	artifact, err := w.store.Write(ctx, job.ID+".csv", func(writer io.Writer) error {
		return appusage.WriteBucketCSV(writer, query.GroupBy, buckets)
	})
	if err != nil {
		return err
	}

	_, err = w.service.CompleteExportJob(ctx, appusage.ExportJobCompleteCommand{
		ID:           job.ID,
		ArtifactPath: artifact.Name,
		ArtifactSize: artifact.Size,
	})
	return err
}
