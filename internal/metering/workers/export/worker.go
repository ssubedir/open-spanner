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

type CleanupService interface {
	ListExpiredExportJobs(ctx context.Context, expiredBefore time.Time, limit int) ([]appusage.ExportJobResult, error)
	ExpireExportJob(ctx context.Context, id string) (bool, error)
	RecordExportCleanupRun(ctx context.Context, cmd appusage.ExportCleanupRunCommand) (appusage.ExportCleanupRunResult, error)
}

type Logger func(format string, args ...any)

type Worker struct {
	service          Service
	store            fileexport.Store
	interval         time.Duration
	lockTTL          time.Duration
	maxAttempts      int
	retention        time.Duration
	cleanupInterval  time.Duration
	cleanupBatchSize int
	logger           Logger
}

func (w *Worker) WithCleanup(retention, interval time.Duration, batchSize int) *Worker {
	if retention > 0 && interval > 0 && batchSize > 0 {
		w.retention, w.cleanupInterval, w.cleanupBatchSize = retention, interval, batchSize
	}
	return w
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
	var cleanupTicker *time.Ticker
	var cleanupC <-chan time.Time
	if w.retention > 0 {
		cleanupTicker = time.NewTicker(w.cleanupInterval)
		cleanupC = cleanupTicker.C
		defer cleanupTicker.Stop()
		if _, err := w.CleanupOnce(ctx); err != nil {
			w.logger("export artifact cleanup failed: error=%v", err)
		}
	}

	w.logger("export worker started: interval=%s lock_ttl=%s max_attempts=%d", w.interval, w.lockTTL, w.maxAttempts)
	defer w.logger("export worker stopped")

	for {
		w.drain(ctx)

		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		case <-cleanupC:
			if _, err := w.CleanupOnce(ctx); err != nil {
				w.logger("export artifact cleanup failed: error=%v", err)
			}
		}
	}
}

type cleanupMetrics struct {
	files    int
	bytes    int64
	failures int
}

func (w *Worker) CleanupOnce(ctx context.Context) (int, error) {
	if w.service == nil || w.retention <= 0 || w.cleanupBatchSize <= 0 {
		return 0, nil
	}
	expiredBefore := time.Now().UTC().Add(-w.retention)
	service, ok := w.service.(CleanupService)
	if !ok {
		return 0, errors.New("export cleanup service is unavailable")
	}
	jobs, err := service.ListExpiredExportJobs(ctx, expiredBefore, w.cleanupBatchSize)
	if err != nil {
		return 0, err
	}
	metrics := map[string]*cleanupMetrics{}
	expired := 0
	var cleanupErr error
	for _, job := range jobs {
		metric := metrics[job.WorkspaceID]
		if metric == nil {
			metric = &cleanupMetrics{}
			metrics[job.WorkspaceID] = metric
		}
		if err := w.store.Remove(ctx, job.ArtifactPath); err != nil {
			metric.failures++
			cleanupErr = errors.Join(cleanupErr, err)
			continue
		}
		jobCtx := appauth.WithWorkspaceID(ctx, job.WorkspaceID)
		marked, err := service.ExpireExportJob(jobCtx, job.ID)
		if err != nil {
			metric.failures++
			cleanupErr = errors.Join(cleanupErr, err)
			continue
		}
		if !marked {
			continue
		}
		metric.files++
		metric.bytes += job.ArtifactSize
		expired++
	}
	for workspaceID, metric := range metrics {
		_, err := service.RecordExportCleanupRun(appauth.WithWorkspaceID(ctx, workspaceID), appusage.ExportCleanupRunCommand{WorkspaceID: workspaceID, ExpiredBefore: expiredBefore, FilesDeleted: metric.files, BytesReclaimed: metric.bytes, Failures: metric.failures})
		if err != nil {
			cleanupErr = errors.Join(cleanupErr, err)
		}
	}
	if len(jobs) > 0 {
		w.logger("export artifact cleanup finished: candidates=%d expired=%d", len(jobs), expired)
	}
	return expired, cleanupErr
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
			_ = w.store.Remove(baseCtx, artifact.Name)
		}
		w.logger("export job lease lost: job_id=%s duration=%s error=%v", job.ID, duration, leaseErr)
		return true, nil
	}
	if err == nil {
		_, err = w.service.CompleteExportJob(baseCtx, appusage.ExportJobCompleteCommand{ID: job.ID, ClaimToken: job.ClaimToken, ArtifactPath: artifact.Name, ArtifactSize: artifact.Size})
		if errors.Is(err, domain.ErrNotFound) {
			_ = w.store.Remove(baseCtx, artifact.Name)
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
