package history

import (
	"context"
	"log"
	"sync"
	"time"

	appsystem "github.com/ssubedir/open-spanner/internal/metering/app/system"
)

type Service interface {
	PruneOperationalHistory(context.Context, time.Time, int) (appsystem.OperationalHistoryPruneResult, error)
}

type Metrics interface {
	RecordOperationalHistoryCleanup(context.Context, int)
	RecordOperationalHistoryCleanupFailure(context.Context)
}

type Logger func(string, ...any)

type Worker struct {
	service   Service
	retention time.Duration
	interval  time.Duration
	timeout   time.Duration
	batchSize int
	metrics   Metrics
	logger    Logger
}

func NewWorker(service Service, retention, interval, timeout time.Duration, batchSize int, metrics Metrics, logger Logger) *Worker {
	if logger == nil {
		logger = log.Printf
	}
	return &Worker{service: service, retention: retention, interval: interval, timeout: timeout, batchSize: batchSize, metrics: metrics, logger: logger}
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
	if w.service == nil || w.retention <= 0 || w.interval <= 0 || w.batchSize <= 0 {
		return
	}
	ticker := time.NewTicker(w.interval)
	defer ticker.Stop()
	w.logger("operational history worker started: retention=%s interval=%s timeout=%s batch_size=%d", w.retention, w.interval, w.timeout, w.batchSize)
	defer w.logger("operational history worker stopped")

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if _, err := w.ProcessOnce(ctx); err != nil && ctx.Err() == nil {
				w.logger("operational history cleanup failed: %v", err)
			}
		}
	}
}

func (w *Worker) ProcessOnce(ctx context.Context) (appsystem.OperationalHistoryPruneResult, error) {
	runCtx := ctx
	cancel := func() {}
	if w.timeout > 0 {
		runCtx, cancel = context.WithTimeout(ctx, w.timeout)
	}
	defer cancel()
	startedAt := time.Now()
	result, err := w.service.PruneOperationalHistory(runCtx, time.Now().UTC().Add(-w.retention), w.batchSize)
	if err != nil {
		if w.metrics != nil {
			w.metrics.RecordOperationalHistoryCleanupFailure(runCtx)
		}
		return appsystem.OperationalHistoryPruneResult{}, err
	}
	if w.metrics != nil {
		w.metrics.RecordOperationalHistoryCleanup(runCtx, result.Total())
	}
	w.logger("operational history cleanup completed: duration=%s rows_deleted=%d ingestion_audits=%d export_jobs=%d alert_jobs=%d reconciliation_notifications=%d", time.Since(startedAt).Round(time.Millisecond), result.Total(), result.IngestionAudits, result.ExportJobs, result.AlertDeliveryJobs, result.ReconciliationNotifications)
	return result, nil
}
