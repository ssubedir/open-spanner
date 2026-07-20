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
	entitlementworker "github.com/ssubedir/open-spanner/internal/metering/workers/entitlement"
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
	probe, err := workerhealth.Start(cfg.EntitlementWorkerHealthAddr, app, log.Printf)
	if err != nil {
		log.Fatalf("failed to start worker health server: %v", err)
	}
	log.Printf("worker health listening on %s", cfg.EntitlementWorkerHealthAddr)

	worker := entitlementworker.NewWorker(
		app.EntitlementService,
		cfg.EntitlementWorkerInterval,
		cfg.EntitlementWorkerLockTTL,
		cfg.EntitlementWorkerTimeout,
		cfg.EntitlementWorkerRetryAfter,
		cfg.EntitlementWorkerMaxAttempts,
		cfg.EntitlementWorkerBatchSize,
		log.Printf,
	)
	stopHeartbeat := heartbeat.Start(ctx, app.SystemService, "entitlement", log.Printf)
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
