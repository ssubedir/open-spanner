package bootstrap

import (
	"context"

	"github.com/go-chi/chi/v5"

	appauth "github.com/ssubedir/open-spanner/internal/auth"
	"github.com/ssubedir/open-spanner/internal/config"
	httpauth "github.com/ssubedir/open-spanner/internal/metering/adapters/http/auth"
	httpmeter "github.com/ssubedir/open-spanner/internal/metering/adapters/http/meter"
	httpsavedquery "github.com/ssubedir/open-spanner/internal/metering/adapters/http/savedquery"
	httpsubject "github.com/ssubedir/open-spanner/internal/metering/adapters/http/subject"
	httpsystem "github.com/ssubedir/open-spanner/internal/metering/adapters/http/system"
	httpusage "github.com/ssubedir/open-spanner/internal/metering/adapters/http/usage"
	"github.com/ssubedir/open-spanner/internal/metering/adapters/postgres"
	"github.com/ssubedir/open-spanner/internal/metering/adapters/sqlite"
	appmeter "github.com/ssubedir/open-spanner/internal/metering/app/meter"
	appsavedquery "github.com/ssubedir/open-spanner/internal/metering/app/savedquery"
	appsubject "github.com/ssubedir/open-spanner/internal/metering/app/subject"
	appsystem "github.com/ssubedir/open-spanner/internal/metering/app/system"
	apptransaction "github.com/ssubedir/open-spanner/internal/metering/app/transaction"
	appusage "github.com/ssubedir/open-spanner/internal/metering/app/usage"
	domainmeter "github.com/ssubedir/open-spanner/internal/metering/domain/meter"
	domainusage "github.com/ssubedir/open-spanner/internal/metering/domain/usage"
)

type App struct {
	UsageService      appusage.Service
	authService       appauth.Service
	meterService      appmeter.Service
	savedQueryService appsavedquery.Service
	subjectService    appsubject.Service
	systemService     appsystem.Service
	ready             func(context.Context) error
	cleanup           func() error
}

type readinessChecker interface {
	Ping(ctx context.Context) error
}

type repositorySet struct {
	auth       appauth.Repository
	meter      domainmeter.Repository
	savedQuery appsavedquery.Repository
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

func NewApp(ctx context.Context, cfg config.Config) (*App, error) {
	repos, err := repositories(ctx, cfg)
	if err != nil {
		return nil, err
	}

	authService := appauth.NewService(repos.auth)
	meterService := appmeter.NewService(repos.meter, repos.usage)
	savedQueryService := appsavedquery.NewService(repos.savedQuery)
	subjectService := appsubject.NewService(repos.usage)
	usageService := appusage.NewService(repos.meter, repos.usage, repos.transactor)
	systemService := appsystem.NewService(repos.meter, repos.usage)

	return &App{
		UsageService:      usageService,
		authService:       authService,
		meterService:      meterService,
		savedQueryService: savedQueryService,
		subjectService:    subjectService,
		systemService:     systemService,
		ready:             repos.ready,
		cleanup:           repos.cleanup,
	}, nil
}

func RegisterRoutes(ctx context.Context, router chi.Router, cfg config.Config) (*App, error) {
	app, err := NewApp(ctx, cfg)
	if err != nil {
		return nil, err
	}

	router.Route("/v1", func(r chi.Router) {
		authHandler := httpauth.NewHandler(app.authService)
		authHandler.RegisterRoutes(r)
		r.Group(func(dashboard chi.Router) {
			dashboard.Use(authHandler.RequireSession)
			httpsavedquery.NewHandler(app.savedQueryService).RegisterRoutes(dashboard)
		})
		r.Group(func(protected chi.Router) {
			protected.Use(authHandler.RequireAuth)
			httpmeter.NewHandler(app.meterService).RegisterRoutes(protected)
			httpsubject.NewHandler(app.subjectService).RegisterRoutes(protected)
			httpusage.NewHandler(app.UsageService, cfg.ExportStoragePath).RegisterRoutes(protected)
			httpsystem.NewHandler(app.systemService).RegisterRoutes(protected)
		})
	})

	return app, nil
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
			savedQuery: postgres.NewSavedQueryRepository(store),
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
			savedQuery: sqlite.NewSavedQueryRepository(store),
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
