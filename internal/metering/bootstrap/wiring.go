package bootstrap

import (
	"context"

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
	authorizer, err := appauth.NewCasbinAuthorizer()
	if err != nil {
		return nil, err
	}
	meterService := appmeter.NewService(repos.meter, repos.usage)
	savedQueryService := appsavedquery.NewService(repos.savedQuery)
	subjectService := appsubject.NewService(repos.usage)
	usageService := appusage.NewService(repos.meter, repos.usage, repos.transactor)
	alertService := appalert.NewService(repos.alert, repos.meter, repos.usage, repos.transactor)
	entitlementService := appentitlement.NewService(repos.entitlement, repos.meter, repos.usage, repos.transactor)
	consumptionService := appconsumption.NewService(repos.consumption, usageService, entitlementService, repos.transactor)
	systemService := appsystem.NewService(repos.system, repos.transactor, appsystem.ServiceOptions{
		ReconciliationStaleAfter: cfg.ReconciliationStaleAfter,
		WorkerEnabled:            map[string]bool{"export": true, "alert": true, "entitlement": true, "retention": cfg.RetentionPruneEnabled, "reconciliation": cfg.ReconciliationEnabled},
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
	}, nil
}

func RegisterRoutes(ctx context.Context, router chi.Router, cfg config.Config) (*App, error) {
	app, err := NewApp(ctx, cfg)
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
				Alerts:            app.AlertService,
				Entitlements:      app.EntitlementService,
				Consumption:       app.ConsumptionService,
				ExportStoragePath: cfg.ExportStoragePath,
				ExportStore:       exportStore,
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

func repositories(ctx context.Context, cfg config.Config) (repositorySet, error) {
	switch cfg.DBDriver {
	case "postgres":
		store, err := postgres.NewStore(ctx, cfg.PostgresDSN, cfg.DBPool)
		if err != nil {
			return repositorySet{}, err
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
		}, nil
	}
}

func readiness(checker readinessChecker) func(context.Context) error {
	return checker.Ping
}
