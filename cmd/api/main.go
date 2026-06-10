package main

import (
	"context"
	"log"
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/ssubedir/open-spanner/internal/config"
	"github.com/ssubedir/open-spanner/internal/metering/bootstrap"
	"github.com/ssubedir/open-spanner/internal/metering/workers/retention"
	serverhttp "github.com/ssubedir/open-spanner/internal/server/http"
	"github.com/ssubedir/open-spanner/internal/ui"
	swaggerdocs "github.com/ssubedir/open-spanner/openapi"
)

// @title Open Spanner API
// @version 0.1.1
// @description Open source metering service for tracking who used what, when, how much, and in what context.
// @BasePath /
// @schemes http
func main() {
	cfg := config.Load()

	router := chi.NewRouter()
	router.Get("/health", health)
	router.Get("/docs", swaggerUI)
	router.Get("/swagger/doc.json", swaggerDoc)
	ui.RegisterRoutes(router)
	app, err := bootstrap.RegisterRoutes(context.Background(), router, cfg)
	if err != nil {
		log.Fatalf("failed to initialize metering: %v", err)
	}

	log.Printf("storage driver: %s", cfg.DBDriver)
	if cfg.DBDriver == "sqlite" {
		log.Printf("sqlite path: %s", cfg.SQLitePath)
	}

	stopRetention := func() {}
	if cfg.RetentionPruneEnabled {
		log.Printf("retention prune worker enabled: interval=%s", cfg.RetentionPruneInterval)
		stopRetention = retention.NewWorker(app.UsageService, cfg.RetentionPruneInterval, log.Printf).Start(context.Background())
	}

	cleanup := func() error {
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

func swaggerDoc(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	_, _ = w.Write([]byte(swaggerdocs.SwaggerInfo.ReadDoc()))
}

func swaggerUI(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = w.Write([]byte(`<!doctype html>
<html lang="en">
  <head>
    <meta charset="utf-8">
    <meta name="viewport" content="width=device-width, initial-scale=1">
    <title>Open Spanner API Docs</title>
    <link rel="stylesheet" href="https://unpkg.com/swagger-ui-dist@5/swagger-ui.css">
    <style>
      body {
        margin: 0;
        background: #f7f8fb;
      }
      .swagger-ui .topbar {
        display: none;
      }
    </style>
  </head>
  <body>
    <div id="swagger-ui"></div>
    <script src="https://unpkg.com/swagger-ui-dist@5/swagger-ui-bundle.js"></script>
    <script>
      window.ui = SwaggerUIBundle({
        url: "/swagger/doc.json",
        dom_id: "#swagger-ui",
        deepLinking: true,
        presets: [SwaggerUIBundle.presets.apis],
        layout: "BaseLayout"
      });
    </script>
  </body>
</html>`))
}
