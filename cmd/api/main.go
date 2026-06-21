package main

import (
	"context"
	"errors"
	"log"
	"net"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/ssubedir/open-spanner/internal/config"
	grpcadapter "github.com/ssubedir/open-spanner/internal/metering/adapters/grpc"
	"github.com/ssubedir/open-spanner/internal/metering/bootstrap"
	"github.com/ssubedir/open-spanner/internal/metering/workers/retention"
	serverhttp "github.com/ssubedir/open-spanner/internal/server/http"
	"github.com/ssubedir/open-spanner/internal/ui"
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

	router := chi.NewRouter()
	router.Get("/health", health)
	ui.RegisterRoutes(router)
	app, err := bootstrap.RegisterRoutes(context.Background(), router, cfg)
	if err != nil {
		log.Fatalf("failed to initialize metering: %v", err)
	}
	router.Get("/ready", ready(app))

	log.Printf("storage driver: %s", cfg.DBDriver)
	if cfg.DBDriver == "sqlite" {
		log.Printf("sqlite path: %s", cfg.SQLitePath)
	}

	grpcListener, err := net.Listen("tcp", cfg.GRPCAddr)
	if err != nil {
		log.Fatalf("failed to listen for grpc: %v", err)
	}
	grpcServer := grpcadapter.NewServer(app.UsageService, app.AlertService, app.AuthService, app.Authorizer)
	go func() {
		log.Printf("grpc listening on %s", cfg.GRPCAddr)
		if err := grpcServer.Serve(grpcListener); err != nil && !errors.Is(err, grpc.ErrServerStopped) {
			log.Printf("grpc server stopped: %v", err)
		}
	}()

	stopRetention := func() {}
	if cfg.RetentionPruneEnabled {
		log.Printf("retention prune worker enabled: interval=%s timeout=%s", cfg.RetentionPruneInterval, cfg.RetentionPruneTimeout)
		stopRetention = retention.NewWorker(app.UsageService, cfg.RetentionPruneInterval, cfg.RetentionPruneTimeout, log.Printf).Start(context.Background())
	}

	cleanup := func() error {
		grpcServer.GracefulStop()
		stopRetention()
		return app.Cleanup()
	}

	server := serverhttp.New(cfg.HTTPAddr, router, cleanup)
	if err := server.Run(context.Background()); err != nil {
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
func ready(checker readyChecker) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
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
