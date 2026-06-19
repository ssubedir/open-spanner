package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/ssubedir/open-spanner/internal/config"
	"github.com/ssubedir/open-spanner/internal/metering/bootstrap"
	alertworker "github.com/ssubedir/open-spanner/internal/metering/workers/alert"
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

	worker := alertworker.NewWorker(
		app.AlertService,
		cfg.AlertWorkerInterval,
		cfg.AlertWorkerLockTTL,
		cfg.AlertWorkerTimeout,
		cfg.AlertWorkerRetryAfter,
		cfg.AlertWorkerMaxAttempts,
		cfg.AlertWorkerBatchSize,
		log.Printf,
	)
	stopWorker := worker.Start(ctx)

	<-ctx.Done()
	stopWorker()
}
