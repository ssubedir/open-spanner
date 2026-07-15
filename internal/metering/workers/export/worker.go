package export

import (
	"context"
	"errors"
	"io"
	"log"
	"sync"
	"time"

	appauth "github.com/ssubedir/open-spanner/internal/auth"
	"github.com/ssubedir/open-spanner/internal/metering/adapters/fileexport"
	appusage "github.com/ssubedir/open-spanner/internal/metering/app/usage"
	"github.com/ssubedir/open-spanner/internal/metering/domain"
)

type Service interface {
	ClaimExportJob(ctx context.Context, cmd appusage.ExportJobClaimCommand) (appusage.ExportJobResult, bool, error)
	RenewExportJobLease(ctx context.Context, cmd appusage.ExportJobRenewCommand) error
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
	maxAttempts int
	logger      Logger
}

func NewWorker(service Service, store fileexport.Store, interval time.Duration, lockTTL time.Duration, maxAttempts int, logger Logger) *Worker {
	if logger == nil {
		logger = log.Printf
	}
	return &Worker{
		service:     service,
		store:       store,
		interval:    interval,
		lockTTL:     lockTTL,
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

	w.logger("export worker started: interval=%s lock_ttl=%s max_attempts=%d", w.interval, w.lockTTL, w.maxAttempts)
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
	baseCtx := appauth.WithWorkspaceID(ctx, job.WorkspaceID)
	jobCtx, cancelJob := context.WithCancel(baseCtx)
	leaseCtx, stopLease := context.WithCancel(baseCtx)
	leaseResult := make(chan error, 1)
	go func() { leaseResult <- w.maintainLease(leaseCtx, job, cancelJob) }()

	artifact, err := w.process(jobCtx, job)
	stopLease()
	leaseErr := <-leaseResult
	cancelJob()
	duration := time.Since(startedAt).Round(time.Millisecond)
	if leaseErr != nil {
		if artifact.Name != "" {
			_ = w.store.Remove(artifact.Name)
		}
		w.logger("export job lease lost: job_id=%s duration=%s error=%v", job.ID, duration, leaseErr)
		return true, nil
	}
	if err == nil {
		_, err = w.service.CompleteExportJob(baseCtx, appusage.ExportJobCompleteCommand{ID: job.ID, ClaimToken: job.ClaimToken, ArtifactPath: artifact.Name, ArtifactSize: artifact.Size})
		if errors.Is(err, domain.ErrNotFound) {
			_ = w.store.Remove(artifact.Name)
			w.logger("export job completion fenced because ownership changed: job_id=%s duration=%s", job.ID, duration)
			return true, nil
		}
		if err != nil {
			return true, err
		}
		w.logger("export job completed: job_id=%s duration=%s", job.ID, duration)
		return true, nil
	}
	if ctx.Err() != nil && errors.Is(err, context.Canceled) {
		w.logger("export job abandoned during shutdown: job_id=%s duration=%s", job.ID, duration)
		return true, nil
	}

	failCtx, failCancel := context.WithTimeout(appauth.WithWorkspaceID(context.Background(), job.WorkspaceID), 10*time.Second)
	defer failCancel()
	if _, failErr := w.service.FailExportJob(failCtx, appusage.ExportJobFailCommand{
		ID:           job.ID,
		ClaimToken:   job.ClaimToken,
		ErrorMessage: err.Error(),
	}); failErr != nil {
		if errors.Is(failErr, domain.ErrNotFound) {
			w.logger("export job failure skipped because it is no longer running: job_id=%s duration=%s error=%v", job.ID, duration, err)
			return true, nil
		}
		return true, errors.Join(err, failErr)
	}
	w.logger("export job failed: job_id=%s duration=%s error=%v", job.ID, duration, err)
	return true, nil
}

func (w *Worker) maintainLease(ctx context.Context, job appusage.ExportJobResult, cancelJob context.CancelFunc) error {
	interval := w.lockTTL / 3
	if interval <= 0 {
		interval = time.Millisecond
	}
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
			err := w.service.RenewExportJobLease(ctx, appusage.ExportJobRenewCommand{ID: job.ID, ClaimToken: job.ClaimToken, LockTTL: w.lockTTL})
			if err != nil {
				if ctx.Err() != nil {
					return nil
				}
				cancelJob()
				return err
			}
		}
	}
}

func (w *Worker) process(ctx context.Context, job appusage.ExportJobResult) (fileexport.Artifact, error) {
	query, err := appusage.ParseExportListQueryJSON(job.QueryJSON)
	if err != nil {
		return fileexport.Artifact{}, err
	}

	buckets, err := w.service.List(ctx, query)
	if err != nil {
		return fileexport.Artifact{}, err
	}

	artifact, err := w.store.Write(ctx, job.ID+"-"+job.ClaimToken+".csv", func(writer io.Writer) error {
		return appusage.WriteBucketCSV(writer, query.GroupBy, buckets)
	})
	if err != nil {
		return fileexport.Artifact{}, err
	}
	return artifact, nil
}
