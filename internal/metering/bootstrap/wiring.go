package bootstrap

import (
	"context"

	"github.com/go-chi/chi/v5"

	appauth "github.com/ssubedir/open-spanner/internal/auth"
	"github.com/ssubedir/open-spanner/internal/config"
	httpauth "github.com/ssubedir/open-spanner/internal/metering/adapters/http/auth"
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
	ready        func(context.Context) error
	cleanup      func() error
}

type readinessChecker interface {
	Ping(ctx context.Context) error
}

type repositorySet struct {
	auth       appauth.Repository
	meter      domainmeter.Repository
	usage      domainusage.Repository
	transactor apptransaction.Transactor
	ready      func(context.Context) error
	cleanup    func() error
}

func (a *App) Ready(ctx context.Context) error {
	if a == nil || a.ready == nil {
		return nil
	}
	return a.ready(ctx)
}

func (a *App) Cleanup() error {
	if a == nil || a.cleanup == nil {
		return nil
	}
	return a.cleanup()
}

func RegisterRoutes(ctx context.Context, router chi.Router, cfg config.Config) (*App, error) {
	repos, err := repositories(ctx, cfg)
	if err != nil {
		return nil, err
	}

	authService := appauth.NewService(repos.auth)
	meterService := appmeter.NewService(repos.meter, repos.usage)
	subjectService := appsubject.NewService(repos.usage)
	usageService := appusage.NewService(repos.meter, repos.usage, repos.transactor)
	systemService := appsystem.NewService(repos.meter, repos.usage)

	router.Route("/v1", func(r chi.Router) {
		authHandler := httpauth.NewHandler(authService)
		authHandler.RegisterRoutes(r)
		r.Group(func(protected chi.Router) {
			protected.Use(authHandler.RequireAuth)
			httpmeter.NewHandler(meterService).RegisterRoutes(protected)
			httpsubject.NewHandler(subjectService).RegisterRoutes(protected)
			httpusage.NewHandler(usageService).RegisterRoutes(protected)
			httpsystem.NewHandler(systemService).RegisterRoutes(protected)
		})
	})

	return &App{UsageService: usageService, ready: repos.ready, cleanup: repos.cleanup}, nil
}

func repositories(ctx context.Context, cfg config.Config) (repositorySet, error) {
	switch cfg.DBDriver {
	case "postgres":
		store, err := postgres.NewStore(ctx, cfg.PostgresDSN, cfg.DBPool)
		if err != nil {
			return repositorySet{}, err
		}

		return repositorySet{
			auth:       postgres.NewAuthRepository(store),
			meter:      postgres.NewMeterRepository(store),
			usage:      postgres.NewUsageRepository(store),
			transactor: store,
			ready:      readiness(store),
			cleanup:    store.Close,
		}, nil
	default:
		store, err := sqlite.NewStore(ctx, cfg.SQLitePath, cfg.DBPool)
		if err != nil {
			return repositorySet{}, err
		}

		return repositorySet{
			auth:       sqlite.NewAuthRepository(store),
			meter:      sqlite.NewMeterRepository(store),
			usage:      sqlite.NewUsageRepository(store),
			transactor: store,
			ready:      readiness(store),
			cleanup:    store.Close,
		}, nil
	}
}

func readiness(checker readinessChecker) func(context.Context) error {
	return checker.Ping
}
