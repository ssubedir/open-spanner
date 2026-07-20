package bootstrap

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/go-chi/chi/v5"

	appauth "github.com/ssubedir/open-spanner/internal/auth"
	"github.com/ssubedir/open-spanner/internal/config"
	"github.com/ssubedir/open-spanner/internal/metering/adapters/fileexport"
	httpalert "github.com/ssubedir/open-spanner/internal/metering/adapters/http/alert"
	httpauth "github.com/ssubedir/open-spanner/internal/metering/adapters/http/auth"
	httpentitlement "github.com/ssubedir/open-spanner/internal/metering/adapters/http/entitlement"
	httpmeter "github.com/ssubedir/open-spanner/internal/metering/adapters/http/meter"
	httpsavedquery "github.com/ssubedir/open-spanner/internal/metering/adapters/http/savedquery"
	httpsubject "github.com/ssubedir/open-spanner/internal/metering/adapters/http/subject"
	httpsystem "github.com/ssubedir/open-spanner/internal/metering/adapters/http/system"
	httpusage "github.com/ssubedir/open-spanner/internal/metering/adapters/http/usage"
	"github.com/ssubedir/open-spanner/internal/metering/adapters/postgres"
	"github.com/ssubedir/open-spanner/internal/metering/adapters/sqlite"
	appalert "github.com/ssubedir/open-spanner/internal/metering/app/alert"
	appconsumption "github.com/ssubedir/open-spanner/internal/metering/app/consumption"
	appentitlement "github.com/ssubedir/open-spanner/internal/metering/app/entitlement"
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
	UsageService       appusage.Service
	AlertService       appalert.Service
	EntitlementService appentitlement.Service
	ConsumptionService appconsumption.Service
	SystemService      appsystem.Service
	AuthService        appauth.Service
	Authorizer         appauth.Authorizer
	meterService       appmeter.Service
	savedQueryService  appsavedquery.Service
	subjectService     appsubject.Service
	ready              func(context.Context) error
	cleanup            func() error
	dbStats            func() sql.DBStats
	systemRepo         appsystem.Repository
}

type readinessChecker interface {
	Ping(ctx context.Context) error
}

type repositorySet struct {
	auth        appauth.Repository
	meter       domainmeter.Repository
	savedQuery  appsavedquery.Repository
	usage       domainusage.Repository
	alert       appalert.Repository
	entitlement appentitlement.Repository
	consumption appconsumption.Repository
	system      appsystem.Repository
	transactor  apptransaction.Transactor
	ready       func(context.Context) error
	cleanup     func() error
	dbStats     func() sql.DBStats
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

func (a *App) DatabaseStats() sql.DBStats {
	if a == nil || a.dbStats == nil {
		return sql.DBStats{}
	}
	return a.dbStats()
}

func (a *App) WorkerTelemetry(ctx context.Context) ([]appsystem.WorkerHeartbeat, []appsystem.WorkerDiagnostics, error) {
	if a == nil || a.systemRepo == nil {
		return nil, nil, nil
	}
	heartbeats, err := a.systemRepo.ListWorkerHeartbeats(ctx)
	if err != nil {
		return nil, nil, err
	}
	diagnostics, err := a.systemRepo.ListWorkerDiagnostics(ctx, time.Now().UTC())
	return heartbeats, diagnostics, err
}

func (a *App) ListWorkspaceIDs(ctx context.Context) ([]string, error) {
	if a == nil || a.systemRepo == nil {
		return nil, errors.New("system repository is not configured")
	}
	lister, ok := a.systemRepo.(interface {
		ListWorkspaceIDs(context.Context) ([]string, error)
	})
	if !ok {
		return nil, errors.New("workspace listing is not supported")
	}
	return lister.ListWorkspaceIDs(ctx)
}

func NewApp(ctx context.Context, cfg config.Config) (*App, error) {
	return NewAppWithMetrics(ctx, cfg, nil)
}

func NewAppWithMetrics(ctx context.Context, cfg config.Config, metrics appusage.IngestionMetrics) (*App, error) {
	repos, err := repositories(ctx, cfg, metrics)
	if err != nil {
		return nil, err
	}

	authService := appauth.NewService(repos.auth)
	authorizer, err := appauth.NewCasbinAuthorizer()
	if err != nil {
		return nil, err
	}
	meterService := appmeter.NewService(repos.meter, repos.usage)
	savedQueryService := appsavedquery.NewService(repos.savedQuery)
	subjectService := appsubject.NewService(repos.usage)
	usageService := appusage.NewService(repos.meter, repos.usage, repos.transactor, appusage.IngestionLimits{
		MaxBatchEvents: max(cfg.IngestionMaxBulkEvents, cfg.IngestionMaxStreamEvents),
		RateEvents:     cfg.IngestionRateLimitEvents, RateWindow: cfg.IngestionRateLimitWindow,
		Metrics: metrics,
	})
	alertService := appalert.NewService(repos.alert, repos.meter, repos.usage, repos.transactor)
	entitlementService := appentitlement.NewService(repos.entitlement, repos.meter, repos.usage, repos.transactor)
	consumptionService := appconsumption.NewService(repos.consumption, usageService, entitlementService, repos.transactor)
	systemService := appsystem.NewService(repos.system, repos.transactor, appsystem.ServiceOptions{
		ReconciliationStaleAfter: cfg.ReconciliationStaleAfter,
		RollupStaleAfter:         2 * cfg.RetentionPruneInterval,
		WorkerEnabled:            map[string]bool{"export": true, "alert": true, "entitlement": true, "usage-outbox": true, "retention": cfg.RetentionPruneEnabled, "history": true, "reconciliation": cfg.ReconciliationEnabled},
	})

	return &App{
		UsageService:       usageService,
		AlertService:       alertService,
		EntitlementService: entitlementService,
		ConsumptionService: consumptionService,
		SystemService:      systemService,
		AuthService:        authService,
		Authorizer:         authorizer,
		meterService:       meterService,
		savedQueryService:  savedQueryService,
		subjectService:     subjectService,
		ready:              repos.ready,
		cleanup:            repos.cleanup,
		dbStats:            repos.dbStats,
		systemRepo:         repos.system,
	}, nil
}

func RegisterRoutes(ctx context.Context, router chi.Router, cfg config.Config) (*App, error) {
	return RegisterRoutesWithMetrics(ctx, router, cfg, nil)
}

func RegisterRoutesWithMetrics(ctx context.Context, router chi.Router, cfg config.Config, metrics appusage.IngestionMetrics) (*App, error) {
	app, err := NewAppWithMetrics(ctx, cfg, metrics)
	if err != nil {
		return nil, err
	}
	exportStore, err := NewExportStore(ctx, cfg)
	if err != nil {
		_ = app.Cleanup()
		return nil, err
	}

	router.Route("/v1", func(r chi.Router) {
		authHandler := httpauth.NewHandler(app.AuthService, httpauth.HandlerOptions{
			OAuth:                cfg.OAuth,
			RegistrationDisabled: !cfg.RegistrationEnabled,
		})
		authHandler.RegisterRoutes(r)
		r.Group(func(dashboard chi.Router) {
			dashboard.Use(authHandler.RequireSession)
			httpsavedquery.NewHandler(app.savedQueryService).RegisterSessionRoutes(dashboard)
		})
		r.Group(func(protected chi.Router) {
			protected.Use(authHandler.RequireAuth)
			httpalert.NewHandler(app.AlertService).RegisterRoutes(protected, app.Authorizer)
			httpentitlement.NewHandler(app.EntitlementService).RegisterRoutes(protected, app.Authorizer)
			httpmeter.NewHandler(app.meterService).RegisterRoutes(protected, app.Authorizer)
			httpsubject.NewHandler(app.subjectService).RegisterRoutes(protected, app.Authorizer)
			httpusage.NewHandler(app.UsageService, httpusage.HandlerOptions{
				Consumption:       app.ConsumptionService,
				ExportStoragePath: cfg.ExportStoragePath,
				ExportStore:       exportStore,
				MaxBodyBytes:      int64(cfg.IngestionMaxBodyBytes),
				MaxBulkEvents:     cfg.IngestionMaxBulkEvents,
			}).RegisterRoutes(protected, app.Authorizer)
			httpsystem.NewHandler(app.SystemService).RegisterRoutes(protected, app.Authorizer)
		})
	})

	return app, nil
}

func NewExportStore(ctx context.Context, cfg config.Config) (fileexport.Store, error) {
	return fileexport.New(ctx, fileexport.Options{
		Driver: cfg.ExportStorageDriver, FilesystemPath: cfg.ExportStoragePath,
		S3Bucket: cfg.ExportS3Bucket, S3Region: cfg.ExportS3Region, S3Endpoint: cfg.ExportS3Endpoint,
		S3AccessKeyID: cfg.ExportS3AccessKeyID, S3SecretAccessKey: cfg.ExportS3SecretAccessKey,
		S3SessionToken: cfg.ExportS3SessionToken, S3Prefix: cfg.ExportS3Prefix, S3ForcePathStyle: cfg.ExportS3ForcePathStyle,
	})
}

func repositories(ctx context.Context, cfg config.Config, metrics appusage.IngestionMetrics) (repositorySet, error) {
	switch cfg.DBDriver {
	case "postgres":
		store, err := postgres.NewStore(ctx, cfg.PostgresDSN, cfg.DBPool)
		if err != nil {
			return repositorySet{}, err
		}
		if transactionMetrics, ok := metrics.(postgres.TransactionRetryMetrics); ok {
			store.SetTransactionRetryMetrics(transactionMetrics)
		}

		return repositorySet{
			auth:        postgres.NewAuthRepository(store),
			meter:       postgres.NewMeterRepository(store),
			savedQuery:  postgres.NewSavedQueryRepository(store),
			usage:       postgres.NewUsageRepository(store),
			alert:       postgres.NewAlertRepository(store),
			entitlement: postgres.NewEntitlementRepository(store),
			consumption: postgres.NewConsumptionRepository(store),
			system:      postgres.NewSystemRepository(store),
			transactor:  store,
			ready:       readiness(store),
			cleanup:     store.Close,
			dbStats:     store.Stats,
		}, nil
	default:
		store, err := sqlite.NewStore(ctx, cfg.SQLitePath, cfg.DBPool)
		if err != nil {
			return repositorySet{}, err
		}

		return repositorySet{
			auth:        sqlite.NewAuthRepository(store),
			meter:       sqlite.NewMeterRepository(store),
			savedQuery:  sqlite.NewSavedQueryRepository(store),
			usage:       sqlite.NewUsageRepository(store),
			alert:       sqlite.NewAlertRepository(store),
			entitlement: sqlite.NewEntitlementRepository(store),
			consumption: sqlite.NewConsumptionRepository(store),
			system:      sqlite.NewSystemRepository(store),
			transactor:  store,
			ready:       readiness(store),
			cleanup:     store.Close,
			dbStats:     store.Stats,
		}, nil
	}
}

func readiness(checker readinessChecker) func(context.Context) error {
	return checker.Ping
}
