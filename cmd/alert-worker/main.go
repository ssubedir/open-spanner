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
	alertworker "github.com/ssubedir/open-spanner/internal/metering/workers/alert"
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
	probe, err := workerhealth.Start(cfg.AlertWorkerHealthAddr, app, log.Printf)
	if err != nil {
		log.Fatalf("failed to start worker health server: %v", err)
	}
	log.Printf("worker health listening on %s", cfg.AlertWorkerHealthAddr)

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
	stopHeartbeat := heartbeat.Start(ctx, app.SystemService, "alert", log.Printf)
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
