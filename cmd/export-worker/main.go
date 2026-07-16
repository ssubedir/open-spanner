package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/ssubedir/open-spanner/internal/config"
	"github.com/ssubedir/open-spanner/internal/metering/bootstrap"
	exportworker "github.com/ssubedir/open-spanner/internal/metering/workers/export"
	workerhealth "github.com/ssubedir/open-spanner/internal/metering/workers/health"
	"github.com/ssubedir/open-spanner/internal/metering/workers/heartbeat"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("invalid config: %v", err)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	app, err := bootstrap.NewApp(ctx, cfg)
	if err != nil {
		log.Fatalf("failed to initialize metering: %v", err)
	}
	defer func() {
		if err := app.Cleanup(); err != nil {
			log.Printf("cleanup failed: %v", err)
		}
	}()

	log.Printf("storage driver: %s", cfg.DBDriver)
	log.Printf("export storage driver: %s", cfg.ExportStorageDriver)
	store, err := bootstrap.NewExportStore(ctx, cfg)
	if err != nil {
		log.Fatalf("failed to initialize export storage: %v", err)
	}
	probe, err := workerhealth.Start(cfg.ExportWorkerHealthAddr, app, log.Printf)
	if err != nil {
		log.Fatalf("failed to start worker health server: %v", err)
	}
	log.Printf("worker health listening on %s", cfg.ExportWorkerHealthAddr)

	worker := exportworker.NewWorker(
		app.UsageService,
		store,
		cfg.ExportWorkerInterval,
		cfg.ExportWorkerLockTTL,
		cfg.ExportWorkerMaxAttempts,
		log.Printf,
	).WithCleanup(cfg.ExportRetention, cfg.ExportCleanupInterval, cfg.ExportCleanupBatchSize)
	stopHeartbeat := heartbeat.Start(ctx, app.SystemService, "export", log.Printf)
	stopWorker := worker.Start(ctx)

	<-ctx.Done()
	probe.BeginDrain()
	stopWorker()
	stopHeartbeat()
	shutdownCtx, cancelShutdown := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancelShutdown()
	if err := probe.Shutdown(shutdownCtx); err != nil {
		log.Printf("worker health shutdown failed: %v", err)
	}
}
