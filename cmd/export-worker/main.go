package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/ssubedir/open-spanner/internal/config"
	"github.com/ssubedir/open-spanner/internal/metering/bootstrap"
	exportworker "github.com/ssubedir/open-spanner/internal/metering/workers/export"
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

	worker := exportworker.NewWorker(
		app.UsageService,
		store,
		cfg.ExportWorkerInterval,
		cfg.ExportWorkerLockTTL,
		cfg.ExportWorkerMaxAttempts,
		log.Printf,
	).WithCleanup(cfg.ExportRetention, cfg.ExportCleanupInterval, cfg.ExportCleanupBatchSize)
	heartbeat.Start(ctx, app.SystemService, "export", log.Printf)
	stopWorker := worker.Start(ctx)

	<-ctx.Done()
	stopWorker()
}
