package bootstrap

import (
	"context"

	"github.com/go-chi/chi/v5"

	"github.com/ssubedir/open-spanner/internal/config"
	httpmeter "github.com/ssubedir/open-spanner/internal/metering/adapters/http/meter"
	httpsubject "github.com/ssubedir/open-spanner/internal/metering/adapters/http/subject"
	httpsystem "github.com/ssubedir/open-spanner/internal/metering/adapters/http/system"
	httpusage "github.com/ssubedir/open-spanner/internal/metering/adapters/http/usage"
	"github.com/ssubedir/open-spanner/internal/metering/adapters/postgres"
	"github.com/ssubedir/open-spanner/internal/metering/adapters/sqlite"
	appmeter "github.com/ssubedir/open-spanner/internal/metering/app/meter"
	appsubject "github.com/ssubedir/open-spanner/internal/metering/app/subject"
	appsystem "github.com/ssubedir/open-spanner/internal/metering/app/system"
	apptransaction "github.com/ssubedir/open-spanner/internal/metering/app/transaction"
	appusage "github.com/ssubedir/open-spanner/internal/metering/app/usage"
	domainmeter "github.com/ssubedir/open-spanner/internal/metering/domain/meter"
	domainusage "github.com/ssubedir/open-spanner/internal/metering/domain/usage"
)

type App struct {
	UsageService appusage.Service
	cleanup      func() error
}

func (a *App) Cleanup() error {
	if a == nil || a.cleanup == nil {
		return nil
	}
	return a.cleanup()
}

func RegisterRoutes(ctx context.Context, router chi.Router, cfg config.Config) (*App, error) {
	meterRepo, usageRepo, transactor, cleanup, err := repositories(ctx, cfg)
	if err != nil {
		return nil, err
	}

	meterService := appmeter.NewService(meterRepo, usageRepo)
	subjectService := appsubject.NewService(usageRepo)
	usageService := appusage.NewService(meterRepo, usageRepo, transactor)
	systemService := appsystem.NewService(meterRepo, usageRepo)

	router.Route("/v1", func(r chi.Router) {
		httpmeter.NewHandler(meterService).RegisterRoutes(r)
		httpsubject.NewHandler(subjectService).RegisterRoutes(r)
		httpusage.NewHandler(usageService).RegisterRoutes(r)
		httpsystem.NewHandler(systemService).RegisterRoutes(r)
	})

	return &App{UsageService: usageService, cleanup: cleanup}, nil
}

func repositories(ctx context.Context, cfg config.Config) (domainmeter.Repository, domainusage.Repository, apptransaction.Transactor, func() error, error) {
	switch cfg.DBDriver {
	case "postgres":
		store, err := postgres.NewStore(ctx, cfg.PostgresDSN, cfg.DBPool)
		if err != nil {
			return nil, nil, nil, nil, err
		}

		return postgres.NewMeterRepository(store), postgres.NewUsageRepository(store), store, store.Close, nil
	default:
		store, err := sqlite.NewStore(ctx, cfg.SQLitePath, cfg.DBPool)
		if err != nil {
			return nil, nil, nil, nil, err
		}

		return sqlite.NewMeterRepository(store), sqlite.NewUsageRepository(store), store, store.Close, nil
	}
}
