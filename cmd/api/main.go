package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net"
	"net/http"
	"sync"
	"sync/atomic"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/ssubedir/open-spanner/internal/config"
	grpcadapter "github.com/ssubedir/open-spanner/internal/metering/adapters/grpc"
	"github.com/ssubedir/open-spanner/internal/metering/bootstrap"
	"github.com/ssubedir/open-spanner/internal/metering/workers/heartbeat"
	"github.com/ssubedir/open-spanner/internal/metering/workers/history"
	"github.com/ssubedir/open-spanner/internal/metering/workers/reconciliation"
	"github.com/ssubedir/open-spanner/internal/metering/workers/retention"
	"github.com/ssubedir/open-spanner/internal/observability"
	serverhttp "github.com/ssubedir/open-spanner/internal/server/http"
	"google.golang.org/grpc"
)

// @title Open Spanner API
// @version 0.1.3
// @description Open source metering service for tracking who used what, when, how much, and in what context.
// @BasePath /
// @schemes http
func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("invalid config: %v", err)
	}
	metrics, err := observability.New()
	if err != nil {
		log.Fatalf("failed to initialize telemetry: %v", err)
	}
	runCtx, cancelRun := context.WithCancel(context.Background())
	defer cancelRun()
	draining := &drainState{}

	router := chi.NewRouter()
	router.Use(metrics.HTTPMiddleware)
	router.Get("/health", health)
	router.Handle("/metrics", metrics.Handler())
	app, err := bootstrap.RegisterRoutesWithMetrics(runCtx, router, cfg, metrics)
	if err != nil {
		_ = metrics.Shutdown(context.Background())
		log.Fatalf("failed to initialize metering: %v", err)
	}
	if err := metrics.RegisterDBPool(app.DatabaseStats, cfg.DBDriver); err != nil {
		_ = app.Cleanup()
		_ = metrics.Shutdown(context.Background())
		log.Fatalf("failed to initialize database telemetry: %v", err)
	}
	if err := metrics.RegisterWorkers(func(ctx context.Context) ([]observability.WorkerStats, error) {
		heartbeats, diagnostics, err := app.WorkerTelemetry(ctx)
		if err != nil {
			return nil, err
		}
		items := make(map[string]observability.WorkerStats, len(diagnostics)+len(heartbeats))
		for _, diagnostic := range diagnostics {
			items[diagnostic.Name] = observability.WorkerStats{
				Name: diagnostic.Name, PendingJobs: diagnostic.PendingJobs, RunningJobs: diagnostic.RunningJobs, FailedJobs: diagnostic.FailedJobs,
				OldestPendingAt: diagnostic.OldestPendingAt, LastSuccessAt: diagnostic.LastSuccessAt, LastFailureAt: diagnostic.LastFailureAt,
			}
		}
		for _, heartbeat := range heartbeats {
			item := items[heartbeat.Name]
			item.Name = heartbeat.Name
			item.LastHeartbeatAt = heartbeat.LastHeartbeatAt
			items[heartbeat.Name] = item
		}
		result := make([]observability.WorkerStats, 0, len(items))
		for _, item := range items {
			result = append(result, item)
		}
		return result, nil
	}); err != nil {
		_ = app.Cleanup()
		_ = metrics.Shutdown(context.Background())
		log.Fatalf("failed to initialize worker telemetry: %v", err)
	}
	router.Get("/ready", ready(app, draining))

	log.Printf("storage driver: %s", cfg.DBDriver)
	if cfg.DBDriver == "sqlite" {
		log.Printf("sqlite path: %s", cfg.SQLitePath)
	}

	grpcListener, err := net.Listen("tcp", cfg.GRPCAddr)
	if err != nil {
		log.Fatalf("failed to listen for grpc: %v", err)
	}
	grpcServer := grpcadapter.NewInstrumentedServerWithIngestionLimits(app.UsageService, app.AlertService, app.EntitlementService, app.AuthService, app.Authorizer, grpcadapter.IngestionLimits{MaxBulkEvents: cfg.IngestionMaxBulkEvents, MaxStreamEvents: cfg.IngestionMaxStreamEvents}, metrics, grpc.MaxRecvMsgSize(cfg.IngestionMaxBodyBytes))
	go func() {
		log.Printf("grpc listening on %s", cfg.GRPCAddr)
		if err := grpcServer.Serve(grpcListener); err != nil && !errors.Is(err, grpc.ErrServerStopped) {
			log.Printf("grpc server stopped: %v", err)
			cancelRun()
		}
	}()
	grpcDrain := newGRPCDrainer(grpcServer)

	stopRetention := func() {}
	stopRetentionHeartbeat := func() {}
	if cfg.RetentionPruneEnabled {
		stopRetentionHeartbeat = heartbeat.Start(runCtx, app.SystemService, "retention", log.Printf)
		log.Printf("retention prune worker enabled: interval=%s timeout=%s", cfg.RetentionPruneInterval, cfg.RetentionPruneTimeout)
		stopRetention = retention.NewWorker(app.UsageService, cfg.RetentionPruneInterval, cfg.RetentionPruneTimeout, log.Printf).
			WithDecisionPruner(app.ConsumptionService, cfg.ConsumptionDecisionRetention).
			WithWorkspaceLister(app).
			Start(runCtx)
	}
	stopHistoryHeartbeat := heartbeat.Start(runCtx, app.SystemService, "history", log.Printf)
	log.Printf("operational history worker enabled: retention=%s interval=%s timeout=%s batch_size=%d", cfg.OperationalHistoryRetention, cfg.OperationalHistoryInterval, cfg.OperationalHistoryTimeout, cfg.OperationalHistoryBatchSize)
	stopHistory := history.NewWorker(app.SystemService, cfg.OperationalHistoryRetention, cfg.OperationalHistoryInterval, cfg.OperationalHistoryTimeout, cfg.OperationalHistoryBatchSize, metrics, log.Printf).Start(runCtx)

	stopReconciliation := func() {}
	stopReconciliationHeartbeat := func() {}
	if cfg.ReconciliationEnabled {
		stopReconciliationHeartbeat = heartbeat.Start(runCtx, app.SystemService, "reconciliation", log.Printf)
		var notifier reconciliation.Notifier
		if cfg.ReconciliationWebhookURL != "" {
			notifier = reconciliation.NewWebhookNotifier(cfg.ReconciliationWebhookURL, cfg.ReconciliationWebhookSecret, nil)
		}
		stopReconciliation = reconciliation.NewWorker(app.SystemService, reconciliation.Options{
			PollInterval: cfg.ReconciliationPollInterval, ScheduleInterval: cfg.ReconciliationSchedule,
			LockTTL: cfg.ReconciliationLockTTL, Timeout: cfg.ReconciliationTimeout, RetryAfter: cfg.ReconciliationRetryAfter,
			Limit: cfg.ReconciliationLimit, LookbackHours: cfg.ReconciliationLookbackHours, MaxAttempts: cfg.ReconciliationMaxAttempts, Notifier: notifier, Logger: log.Printf,
		}).Start(runCtx)
	}

	beginDrain := func() {
		draining.Begin()
		cancelRun()
		grpcDrain.Begin()
	}
	cleanup := func() error {
		shutdownCtx, cancelShutdown := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancelShutdown()
		if grpcDrain.Wait(shutdownCtx) {
			log.Printf("grpc graceful shutdown timed out; forced active streams to stop")
		}
		workerErr := stopFunctions(shutdownCtx, stopRetention, stopRetentionHeartbeat, stopHistory, stopHistoryHeartbeat, stopReconciliation, stopReconciliationHeartbeat)
		appErr := app.Cleanup()
		telemetryCtx, cancelTelemetry := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancelTelemetry()
		return errors.Join(workerErr, appErr, metrics.Shutdown(telemetryCtx))
	}

	server := serverhttp.New(cfg.HTTPAddr, router, cleanup, beginDrain)
	if err := server.Run(runCtx); err != nil {
		log.Fatalf("server stopped: %v", err)
	}
}

// health checks whether the API is running.
//
// @Summary Health check
// @ID healthCheck
// @Tags health
// @Success 204
// @Router /health [get]
func health(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusNoContent)
}

type readyChecker interface {
	Ready(ctx context.Context) error
}

// ready checks whether the API can reach its configured storage.
//
// @Summary Readiness check
// @ID readinessCheck
// @Tags health
// @Success 204
// @Failure 503
// @Router /ready [get]
func ready(checker readyChecker, draining *drainState) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if draining != nil && draining.Draining() {
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
		defer cancel()

		if checker == nil {
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		if err := checker.Ready(ctx); err != nil {
			log.Printf("readiness check failed: %v", err)
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}
}

type drainState struct {
	draining atomic.Bool
}

func (s *drainState) Begin() { s.draining.Store(true) }

func (s *drainState) Draining() bool { return s != nil && s.draining.Load() }

type grpcStopper interface {
	GracefulStop()
	Stop()
}

type grpcDrainer struct {
	server grpcStopper
	once   sync.Once
	done   chan struct{}
}

func newGRPCDrainer(server grpcStopper) *grpcDrainer {
	return &grpcDrainer{server: server, done: make(chan struct{})}
}

func (d *grpcDrainer) Begin() {
	d.once.Do(func() {
		go func() {
			defer close(d.done)
			d.server.GracefulStop()
		}()
	})
}

// Wait returns true when active streams exceeded the grace period and were forced closed.
func (d *grpcDrainer) Wait(ctx context.Context) bool {
	d.Begin()
	select {
	case <-d.done:
		return false
	case <-ctx.Done():
		d.server.Stop()
		return true
	}
}

func stopFunctions(ctx context.Context, stops ...func()) error {
	var wg sync.WaitGroup
	for _, stop := range stops {
		if stop == nil {
			continue
		}
		wg.Add(1)
		go func() {
			defer wg.Done()
			stop()
		}()
	}
	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()
	select {
	case <-done:
		return nil
	case <-ctx.Done():
		return fmt.Errorf("worker shutdown: %w", ctx.Err())
	}
}
