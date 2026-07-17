package config

import (
	"fmt"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/joho/godotenv"
)

type Config struct {
	HTTPAddr                     string
	GRPCAddr                     string
	RegistrationEnabled          bool
	OAuth                        OAuthConfigs
	DBDriver                     string
	SQLitePath                   string
	PostgresDSN                  string
	DBPool                       DBPoolConfig
	IngestionMaxBodyBytes        int
	IngestionMaxBulkEvents       int
	IngestionMaxStreamEvents     int
	IngestionRateLimitEvents     int
	IngestionRateLimitWindow     time.Duration
	UsageWorkerInterval          time.Duration
	UsageWorkerHealthAddr        string
	UsageWorkerLockTTL           time.Duration
	UsageWorkerTimeout           time.Duration
	UsageWorkerRetryAfter        time.Duration
	UsageWorkerMaxAttempts       int
	UsageWorkerBatchSize         int
	ExportStoragePath            string
	ExportStorageDriver          string
	ExportS3Bucket               string
	ExportS3Region               string
	ExportS3Endpoint             string
	ExportS3AccessKeyID          string
	ExportS3SecretAccessKey      string
	ExportS3SessionToken         string
	ExportS3Prefix               string
	ExportS3ForcePathStyle       bool
	ExportWorkerInterval         time.Duration
	ExportWorkerHealthAddr       string
	ExportWorkerLockTTL          time.Duration
	ExportWorkerMaxAttempts      int
	ExportRetention              time.Duration
	ExportCleanupInterval        time.Duration
	ExportCleanupBatchSize       int
	AlertWorkerInterval          time.Duration
	AlertWorkerHealthAddr        string
	AlertWorkerLockTTL           time.Duration
	AlertWorkerTimeout           time.Duration
	AlertWorkerRetryAfter        time.Duration
	AlertWorkerMaxAttempts       int
	AlertWorkerBatchSize         int
	EntitlementWorkerInterval    time.Duration
	EntitlementWorkerHealthAddr  string
	EntitlementWorkerLockTTL     time.Duration
	EntitlementWorkerTimeout     time.Duration
	EntitlementWorkerRetryAfter  time.Duration
	EntitlementWorkerMaxAttempts int
	EntitlementWorkerBatchSize   int
	RetentionPruneEnabled        bool
	RetentionPruneInterval       time.Duration
	RetentionPruneTimeout        time.Duration
	ConsumptionDecisionRetention time.Duration
	OperationalHistoryRetention  time.Duration
	OperationalHistoryInterval   time.Duration
	OperationalHistoryTimeout    time.Duration
	OperationalHistoryBatchSize  int
	ReconciliationEnabled        bool
	ReconciliationPollInterval   time.Duration
	ReconciliationSchedule       time.Duration
	ReconciliationLockTTL        time.Duration
	ReconciliationTimeout        time.Duration
	ReconciliationRetryAfter     time.Duration
	ReconciliationStaleAfter     time.Duration
	ReconciliationLimit          int
	ReconciliationLookbackHours  int
	ReconciliationMaxAttempts    int
	ReconciliationWebhookURL     string
	ReconciliationWebhookSecret  string
}

type OAuthConfig struct {
	ClientID     string
	ClientSecret string
	Enabled      bool
	RedirectURL  string
}

type OAuthConfigs struct {
	GitHub OAuthConfig
	Google OAuthConfig
}

type DBPoolConfig struct {
	MaxOpenConns    int
	MaxIdleConns    int
	ConnMaxLifetime time.Duration
	ConnMaxIdleTime time.Duration
}

func Load() (Config, error) {
	_ = godotenv.Load()
	registrationEnabled, err := envBool("OPEN_SPANNER_REGISTRATION_ENABLED", true)
	if err != nil {
		return Config{}, err
	}
	pool, err := loadDBPoolConfig()
	if err != nil {
		return Config{}, err
	}
	ingestionMaxBodyBytes, err := envInt("OPEN_SPANNER_INGESTION_MAX_BODY_BYTES", 1024*1024)
	if err != nil {
		return Config{}, err
	}
	ingestionMaxBulkEvents, err := envInt("OPEN_SPANNER_INGESTION_MAX_BULK_EVENTS", 1000)
	if err != nil {
		return Config{}, err
	}
	ingestionMaxStreamEvents, err := envInt("OPEN_SPANNER_INGESTION_MAX_STREAM_EVENTS", 1000)
	if err != nil {
		return Config{}, err
	}
	ingestionRateLimitEvents, err := envInt("OPEN_SPANNER_INGESTION_RATE_LIMIT_EVENTS", 10000)
	if err != nil {
		return Config{}, err
	}
	ingestionRateLimitWindow, err := envDuration("OPEN_SPANNER_INGESTION_RATE_LIMIT_WINDOW", time.Minute)
	if err != nil {
		return Config{}, err
	}
	usageWorkerInterval, err := envDuration("OPEN_SPANNER_USAGE_WORKER_INTERVAL", time.Second)
	if err != nil {
		return Config{}, err
	}
	usageWorkerLockTTL, err := envDuration("OPEN_SPANNER_USAGE_WORKER_LOCK_TTL", time.Minute)
	if err != nil {
		return Config{}, err
	}
	usageWorkerTimeout, err := envDuration("OPEN_SPANNER_USAGE_WORKER_TIMEOUT", 30*time.Second)
	if err != nil {
		return Config{}, err
	}
	usageWorkerRetryAfter, err := envDuration("OPEN_SPANNER_USAGE_WORKER_RETRY_AFTER", 5*time.Second)
	if err != nil {
		return Config{}, err
	}
	usageWorkerMaxAttempts, err := envInt("OPEN_SPANNER_USAGE_WORKER_MAX_ATTEMPTS", 10)
	if err != nil {
		return Config{}, err
	}
	usageWorkerBatchSize, err := envInt("OPEN_SPANNER_USAGE_WORKER_BATCH_SIZE", 100)
	if err != nil {
		return Config{}, err
	}
	retentionEnabled, err := envBool("OPEN_SPANNER_RETENTION_PRUNE_ENABLED", false)
	if err != nil {
		return Config{}, err
	}
	retentionInterval, err := envDuration("OPEN_SPANNER_RETENTION_PRUNE_INTERVAL", time.Hour)
	if err != nil {
		return Config{}, err
	}
	retentionTimeout, err := envDuration("OPEN_SPANNER_RETENTION_PRUNE_TIMEOUT", 30*time.Minute)
	if err != nil {
		return Config{}, err
	}
	decisionRetention, err := envDuration("OPEN_SPANNER_CONSUMPTION_DECISION_RETENTION", 30*24*time.Hour)
	if err != nil {
		return Config{}, err
	}
	operationalHistoryRetention, err := envDuration("OPEN_SPANNER_OPERATIONAL_HISTORY_RETENTION", 30*24*time.Hour)
	if err != nil {
		return Config{}, err
	}
	operationalHistoryInterval, err := envDuration("OPEN_SPANNER_OPERATIONAL_HISTORY_INTERVAL", time.Hour)
	if err != nil {
		return Config{}, err
	}
	operationalHistoryTimeout, err := envDuration("OPEN_SPANNER_OPERATIONAL_HISTORY_TIMEOUT", 5*time.Minute)
	if err != nil {
		return Config{}, err
	}
	operationalHistoryBatchSize, err := envInt("OPEN_SPANNER_OPERATIONAL_HISTORY_BATCH_SIZE", 1000)
	if err != nil {
		return Config{}, err
	}
	reconciliationEnabled, err := envBool("OPEN_SPANNER_RECONCILIATION_ENABLED", false)
	if err != nil {
		return Config{}, err
	}
	reconciliationPoll, err := envDuration("OPEN_SPANNER_RECONCILIATION_POLL_INTERVAL", 5*time.Second)
	if err != nil {
		return Config{}, err
	}
	reconciliationSchedule, err := envDuration("OPEN_SPANNER_RECONCILIATION_SCHEDULE", 15*time.Minute)
	if err != nil {
		return Config{}, err
	}
	reconciliationLockTTL, err := envDuration("OPEN_SPANNER_RECONCILIATION_LOCK_TTL", 5*time.Minute)
	if err != nil {
		return Config{}, err
	}
	reconciliationTimeout, err := envDuration("OPEN_SPANNER_RECONCILIATION_TIMEOUT", 2*time.Minute)
	if err != nil {
		return Config{}, err
	}
	reconciliationRetryAfter, err := envDuration("OPEN_SPANNER_RECONCILIATION_RETRY_AFTER", time.Minute)
	if err != nil {
		return Config{}, err
	}
	reconciliationStaleAfter, err := envDuration("OPEN_SPANNER_RECONCILIATION_STALE_AFTER", 30*time.Minute)
	if err != nil {
		return Config{}, err
	}
	reconciliationLimit, err := envInt("OPEN_SPANNER_RECONCILIATION_LIMIT", 100)
	if err != nil {
		return Config{}, err
	}
	reconciliationLookback, err := envInt("OPEN_SPANNER_RECONCILIATION_LOOKBACK_HOURS", 24)
	if err != nil {
		return Config{}, err
	}
	reconciliationMaxAttempts, err := envInt("OPEN_SPANNER_RECONCILIATION_MAX_ATTEMPTS", 5)
	if err != nil {
		return Config{}, err
	}
	exportWorkerInterval, err := envDuration("OPEN_SPANNER_EXPORT_WORKER_INTERVAL", 5*time.Second)
	if err != nil {
		return Config{}, err
	}
	exportS3Endpoint := env("OPEN_SPANNER_EXPORT_S3_ENDPOINT", "")
	exportS3ForcePathStyle, err := envBool("OPEN_SPANNER_EXPORT_S3_FORCE_PATH_STYLE", exportS3Endpoint != "")
	if err != nil {
		return Config{}, err
	}
	exportWorkerLockTTL, err := envDuration("OPEN_SPANNER_EXPORT_WORKER_LOCK_TTL", 5*time.Minute)
	if err != nil {
		return Config{}, err
	}
	exportWorkerMaxAttempts, err := envInt("OPEN_SPANNER_EXPORT_WORKER_MAX_ATTEMPTS", 3)
	if err != nil {
		return Config{}, err
	}
	exportRetention, err := envDuration("OPEN_SPANNER_EXPORT_RETENTION", 7*24*time.Hour)
	if err != nil {
		return Config{}, err
	}
	exportCleanupInterval, err := envDuration("OPEN_SPANNER_EXPORT_CLEANUP_INTERVAL", time.Hour)
	if err != nil {
		return Config{}, err
	}
	exportCleanupBatchSize, err := envInt("OPEN_SPANNER_EXPORT_CLEANUP_BATCH_SIZE", 1000)
	if err != nil {
		return Config{}, err
	}
	alertWorkerInterval, err := envDuration("OPEN_SPANNER_ALERT_WORKER_INTERVAL", 5*time.Second)
	if err != nil {
		return Config{}, err
	}
	alertWorkerLockTTL, err := envDuration("OPEN_SPANNER_ALERT_WORKER_LOCK_TTL", 5*time.Minute)
	if err != nil {
		return Config{}, err
	}
	alertWorkerTimeout, err := envDuration("OPEN_SPANNER_ALERT_WORKER_TIMEOUT", time.Minute)
	if err != nil {
		return Config{}, err
	}
	alertWorkerRetryAfter, err := envDuration("OPEN_SPANNER_ALERT_WORKER_RETRY_AFTER", 30*time.Second)
	if err != nil {
		return Config{}, err
	}
	alertWorkerMaxAttempts, err := envInt("OPEN_SPANNER_ALERT_WORKER_MAX_ATTEMPTS", 3)
	if err != nil {
		return Config{}, err
	}
	alertWorkerBatchSize, err := envInt("OPEN_SPANNER_ALERT_WORKER_BATCH_SIZE", 100)
	if err != nil {
		return Config{}, err
	}
	entitlementWorkerInterval, err := envDuration("OPEN_SPANNER_ENTITLEMENT_WORKER_INTERVAL", 5*time.Second)
	if err != nil {
		return Config{}, err
	}
	entitlementWorkerLockTTL, err := envDuration("OPEN_SPANNER_ENTITLEMENT_WORKER_LOCK_TTL", 5*time.Minute)
	if err != nil {
		return Config{}, err
	}
	entitlementWorkerTimeout, err := envDuration("OPEN_SPANNER_ENTITLEMENT_WORKER_TIMEOUT", time.Minute)
	if err != nil {
		return Config{}, err
	}
	entitlementWorkerRetryAfter, err := envDuration("OPEN_SPANNER_ENTITLEMENT_WORKER_RETRY_AFTER", 30*time.Second)
	if err != nil {
		return Config{}, err
	}
	entitlementWorkerMaxAttempts, err := envInt("OPEN_SPANNER_ENTITLEMENT_WORKER_MAX_ATTEMPTS", 3)
	if err != nil {
		return Config{}, err
	}
	entitlementWorkerBatchSize, err := envInt("OPEN_SPANNER_ENTITLEMENT_WORKER_BATCH_SIZE", 100)
	if err != nil {
		return Config{}, err
	}

	gitHubOAuth, err := loadOAuthConfig("GITHUB")
	if err != nil {
		return Config{}, err
	}
	googleOAuth, err := loadOAuthConfig("GOOGLE")
	if err != nil {
		return Config{}, err
	}

	cfg := Config{
		HTTPAddr:            env("OPEN_SPANNER_HTTP_ADDR", ":18080"),
		GRPCAddr:            env("OPEN_SPANNER_GRPC_ADDR", ":18090"),
		RegistrationEnabled: registrationEnabled,
		OAuth: OAuthConfigs{
			GitHub: gitHubOAuth,
			Google: googleOAuth,
		},
		DBDriver:                     strings.ToLower(env("OPEN_SPANNER_DB_DRIVER", "sqlite")),
		SQLitePath:                   env("OPEN_SPANNER_SQLITE_PATH", "open-spanner.db"),
		PostgresDSN:                  env("OPEN_SPANNER_POSTGRES_DSN", ""),
		DBPool:                       pool,
		IngestionMaxBodyBytes:        ingestionMaxBodyBytes,
		IngestionMaxBulkEvents:       ingestionMaxBulkEvents,
		IngestionMaxStreamEvents:     ingestionMaxStreamEvents,
		IngestionRateLimitEvents:     ingestionRateLimitEvents,
		IngestionRateLimitWindow:     ingestionRateLimitWindow,
		UsageWorkerInterval:          usageWorkerInterval,
		UsageWorkerHealthAddr:        env("OPEN_SPANNER_USAGE_WORKER_HEALTH_ADDR", ":18085"),
		UsageWorkerLockTTL:           usageWorkerLockTTL,
		UsageWorkerTimeout:           usageWorkerTimeout,
		UsageWorkerRetryAfter:        usageWorkerRetryAfter,
		UsageWorkerMaxAttempts:       usageWorkerMaxAttempts,
		UsageWorkerBatchSize:         usageWorkerBatchSize,
		ExportStoragePath:            env("OPEN_SPANNER_EXPORT_STORAGE_PATH", "open-spanner-exports"),
		ExportStorageDriver:          strings.ToLower(env("OPEN_SPANNER_EXPORT_STORAGE_DRIVER", "filesystem")),
		ExportS3Bucket:               env("OPEN_SPANNER_EXPORT_S3_BUCKET", ""),
		ExportS3Region:               env("OPEN_SPANNER_EXPORT_S3_REGION", "us-east-1"),
		ExportS3Endpoint:             exportS3Endpoint,
		ExportS3AccessKeyID:          env("OPEN_SPANNER_EXPORT_S3_ACCESS_KEY_ID", ""),
		ExportS3SecretAccessKey:      env("OPEN_SPANNER_EXPORT_S3_SECRET_ACCESS_KEY", ""),
		ExportS3SessionToken:         env("OPEN_SPANNER_EXPORT_S3_SESSION_TOKEN", ""),
		ExportS3Prefix:               env("OPEN_SPANNER_EXPORT_S3_PREFIX", ""),
		ExportS3ForcePathStyle:       exportS3ForcePathStyle,
		ExportWorkerInterval:         exportWorkerInterval,
		ExportWorkerHealthAddr:       env("OPEN_SPANNER_EXPORT_WORKER_HEALTH_ADDR", ":18082"),
		ExportWorkerLockTTL:          exportWorkerLockTTL,
		ExportWorkerMaxAttempts:      exportWorkerMaxAttempts,
		ExportRetention:              exportRetention,
		ExportCleanupInterval:        exportCleanupInterval,
		ExportCleanupBatchSize:       exportCleanupBatchSize,
		AlertWorkerInterval:          alertWorkerInterval,
		AlertWorkerHealthAddr:        env("OPEN_SPANNER_ALERT_WORKER_HEALTH_ADDR", ":18083"),
		AlertWorkerLockTTL:           alertWorkerLockTTL,
		AlertWorkerTimeout:           alertWorkerTimeout,
		AlertWorkerRetryAfter:        alertWorkerRetryAfter,
		AlertWorkerMaxAttempts:       alertWorkerMaxAttempts,
		AlertWorkerBatchSize:         alertWorkerBatchSize,
		EntitlementWorkerInterval:    entitlementWorkerInterval,
		EntitlementWorkerHealthAddr:  env("OPEN_SPANNER_ENTITLEMENT_WORKER_HEALTH_ADDR", ":18084"),
		EntitlementWorkerLockTTL:     entitlementWorkerLockTTL,
		EntitlementWorkerTimeout:     entitlementWorkerTimeout,
		EntitlementWorkerRetryAfter:  entitlementWorkerRetryAfter,
		EntitlementWorkerMaxAttempts: entitlementWorkerMaxAttempts,
		EntitlementWorkerBatchSize:   entitlementWorkerBatchSize,
		RetentionPruneEnabled:        retentionEnabled,
		RetentionPruneInterval:       retentionInterval,
		RetentionPruneTimeout:        retentionTimeout,
		ConsumptionDecisionRetention: decisionRetention,
		OperationalHistoryRetention:  operationalHistoryRetention,
		OperationalHistoryInterval:   operationalHistoryInterval,
		OperationalHistoryTimeout:    operationalHistoryTimeout,
		OperationalHistoryBatchSize:  operationalHistoryBatchSize,
		ReconciliationEnabled:        reconciliationEnabled,
		ReconciliationPollInterval:   reconciliationPoll,
		ReconciliationSchedule:       reconciliationSchedule,
		ReconciliationLockTTL:        reconciliationLockTTL,
		ReconciliationTimeout:        reconciliationTimeout,
		ReconciliationRetryAfter:     reconciliationRetryAfter,
		ReconciliationStaleAfter:     reconciliationStaleAfter,
		ReconciliationLimit:          reconciliationLimit,
		ReconciliationLookbackHours:  reconciliationLookback,
		ReconciliationMaxAttempts:    reconciliationMaxAttempts,
		ReconciliationWebhookURL:     env("OPEN_SPANNER_RECONCILIATION_WEBHOOK_URL", ""),
		ReconciliationWebhookSecret:  env("OPEN_SPANNER_RECONCILIATION_WEBHOOK_SECRET", ""),
	}

	if err := cfg.Validate(); err != nil {
		return Config{}, err
	}

	return cfg, nil
}

func loadOAuthConfig(provider string) (OAuthConfig, error) {
	prefix := "OPEN_SPANNER_" + strings.ToUpper(provider) + "_OAUTH_"
	enabled, err := envBool(prefix+"ENABLED", true)
	if err != nil {
		return OAuthConfig{}, err
	}
	return OAuthConfig{
		ClientID:     env(prefix+"CLIENT_ID", ""),
		ClientSecret: env(prefix+"CLIENT_SECRET", ""),
		Enabled:      enabled,
		RedirectURL:  env(prefix+"REDIRECT_URL", ""),
	}, nil
}

func (cfg Config) Validate() error {
	if strings.TrimSpace(cfg.HTTPAddr) == "" {
		return fmt.Errorf("OPEN_SPANNER_HTTP_ADDR is required")
	}
	if strings.TrimSpace(cfg.GRPCAddr) == "" {
		return fmt.Errorf("OPEN_SPANNER_GRPC_ADDR is required")
	}
	if strings.TrimSpace(cfg.ExportWorkerHealthAddr) == "" || strings.TrimSpace(cfg.AlertWorkerHealthAddr) == "" || strings.TrimSpace(cfg.EntitlementWorkerHealthAddr) == "" {
		return fmt.Errorf("worker health addresses are required")
	}

	switch cfg.DBDriver {
	case "sqlite":
		if strings.TrimSpace(cfg.SQLitePath) == "" {
			return fmt.Errorf("OPEN_SPANNER_SQLITE_PATH is required when OPEN_SPANNER_DB_DRIVER=sqlite")
		}
	case "postgres":
		if strings.TrimSpace(cfg.PostgresDSN) == "" {
			return fmt.Errorf("OPEN_SPANNER_POSTGRES_DSN is required when OPEN_SPANNER_DB_DRIVER=postgres")
		}
	default:
		return fmt.Errorf("unsupported OPEN_SPANNER_DB_DRIVER %q: expected sqlite or postgres", cfg.DBDriver)
	}

	if cfg.DBPool.MaxOpenConns < 0 {
		return fmt.Errorf("OPEN_SPANNER_DB_MAX_OPEN_CONNS cannot be negative")
	}
	if cfg.DBPool.MaxIdleConns < 0 {
		return fmt.Errorf("OPEN_SPANNER_DB_MAX_IDLE_CONNS cannot be negative")
	}
	if cfg.DBPool.MaxOpenConns > 0 && cfg.DBPool.MaxIdleConns > cfg.DBPool.MaxOpenConns {
		return fmt.Errorf("OPEN_SPANNER_DB_MAX_IDLE_CONNS cannot exceed OPEN_SPANNER_DB_MAX_OPEN_CONNS")
	}
	if cfg.DBPool.ConnMaxLifetime < 0 {
		return fmt.Errorf("OPEN_SPANNER_DB_CONN_MAX_LIFETIME cannot be negative")
	}
	if cfg.DBPool.ConnMaxIdleTime < 0 {
		return fmt.Errorf("OPEN_SPANNER_DB_CONN_MAX_IDLE_TIME cannot be negative")
	}
	if cfg.IngestionMaxBodyBytes <= 0 {
		return fmt.Errorf("OPEN_SPANNER_INGESTION_MAX_BODY_BYTES must be greater than zero")
	}
	if cfg.IngestionMaxBulkEvents <= 0 || cfg.IngestionMaxBulkEvents > 1000 {
		return fmt.Errorf("OPEN_SPANNER_INGESTION_MAX_BULK_EVENTS must be between 1 and 1000")
	}
	if cfg.IngestionMaxStreamEvents <= 0 || cfg.IngestionMaxStreamEvents > 1000 {
		return fmt.Errorf("OPEN_SPANNER_INGESTION_MAX_STREAM_EVENTS must be between 1 and 1000")
	}
	if cfg.IngestionRateLimitEvents <= 0 || cfg.IngestionRateLimitWindow <= 0 {
		return fmt.Errorf("ingestion rate limit and window must be greater than zero")
	}
	switch cfg.ExportStorageDriver {
	case "filesystem":
		if strings.TrimSpace(cfg.ExportStoragePath) == "" {
			return fmt.Errorf("OPEN_SPANNER_EXPORT_STORAGE_PATH is required for filesystem export storage")
		}
	case "s3":
		if strings.TrimSpace(cfg.ExportS3Bucket) == "" {
			return fmt.Errorf("OPEN_SPANNER_EXPORT_S3_BUCKET is required for S3 export storage")
		}
		if strings.TrimSpace(cfg.ExportS3Region) == "" {
			return fmt.Errorf("OPEN_SPANNER_EXPORT_S3_REGION is required for S3 export storage")
		}
		if (cfg.ExportS3AccessKeyID == "") != (cfg.ExportS3SecretAccessKey == "") {
			return fmt.Errorf("OPEN_SPANNER_EXPORT_S3_ACCESS_KEY_ID and OPEN_SPANNER_EXPORT_S3_SECRET_ACCESS_KEY must be set together")
		}
		if cfg.ExportS3SessionToken != "" && cfg.ExportS3AccessKeyID == "" {
			return fmt.Errorf("OPEN_SPANNER_EXPORT_S3_SESSION_TOKEN requires static S3 credentials")
		}
		if cfg.ExportS3Endpoint != "" {
			endpoint, err := url.ParseRequestURI(cfg.ExportS3Endpoint)
			if err != nil || endpoint.Host == "" || (endpoint.Scheme != "http" && endpoint.Scheme != "https") {
				return fmt.Errorf("OPEN_SPANNER_EXPORT_S3_ENDPOINT must be an absolute HTTP(S) URL")
			}
		}
	default:
		return fmt.Errorf("unsupported OPEN_SPANNER_EXPORT_STORAGE_DRIVER %q: expected filesystem or s3", cfg.ExportStorageDriver)
	}
	if cfg.ExportWorkerInterval <= 0 {
		return fmt.Errorf("OPEN_SPANNER_EXPORT_WORKER_INTERVAL must be greater than zero")
	}
	if cfg.ExportWorkerLockTTL <= 0 {
		return fmt.Errorf("OPEN_SPANNER_EXPORT_WORKER_LOCK_TTL must be greater than zero")
	}
	if cfg.ExportWorkerMaxAttempts <= 0 {
		return fmt.Errorf("OPEN_SPANNER_EXPORT_WORKER_MAX_ATTEMPTS must be greater than zero")
	}
	if cfg.ExportRetention <= 0 {
		return fmt.Errorf("OPEN_SPANNER_EXPORT_RETENTION must be greater than zero")
	}
	if cfg.ExportCleanupInterval <= 0 {
		return fmt.Errorf("OPEN_SPANNER_EXPORT_CLEANUP_INTERVAL must be greater than zero")
	}
	if cfg.ExportCleanupBatchSize <= 0 || cfg.ExportCleanupBatchSize > 1000 {
		return fmt.Errorf("OPEN_SPANNER_EXPORT_CLEANUP_BATCH_SIZE must be between 1 and 1000")
	}
	if cfg.AlertWorkerInterval <= 0 {
		return fmt.Errorf("OPEN_SPANNER_ALERT_WORKER_INTERVAL must be greater than zero")
	}
	if cfg.AlertWorkerLockTTL <= 0 {
		return fmt.Errorf("OPEN_SPANNER_ALERT_WORKER_LOCK_TTL must be greater than zero")
	}
	if cfg.AlertWorkerTimeout <= 0 {
		return fmt.Errorf("OPEN_SPANNER_ALERT_WORKER_TIMEOUT must be greater than zero")
	}
	if cfg.AlertWorkerRetryAfter <= 0 {
		return fmt.Errorf("OPEN_SPANNER_ALERT_WORKER_RETRY_AFTER must be greater than zero")
	}
	if cfg.AlertWorkerMaxAttempts <= 0 {
		return fmt.Errorf("OPEN_SPANNER_ALERT_WORKER_MAX_ATTEMPTS must be greater than zero")
	}
	if cfg.AlertWorkerBatchSize <= 0 {
		return fmt.Errorf("OPEN_SPANNER_ALERT_WORKER_BATCH_SIZE must be greater than zero")
	}
	if cfg.UsageWorkerInterval <= 0 || cfg.UsageWorkerLockTTL <= 0 || cfg.UsageWorkerTimeout <= 0 || cfg.UsageWorkerRetryAfter <= 0 {
		return fmt.Errorf("usage worker durations must be greater than zero")
	}
	if cfg.UsageWorkerMaxAttempts <= 0 || cfg.UsageWorkerBatchSize <= 0 {
		return fmt.Errorf("usage worker attempts and batch size must be greater than zero")
	}
	if cfg.EntitlementWorkerInterval <= 0 {
		return fmt.Errorf("OPEN_SPANNER_ENTITLEMENT_WORKER_INTERVAL must be greater than zero")
	}
	if cfg.EntitlementWorkerLockTTL <= 0 {
		return fmt.Errorf("OPEN_SPANNER_ENTITLEMENT_WORKER_LOCK_TTL must be greater than zero")
	}
	if cfg.EntitlementWorkerTimeout <= 0 {
		return fmt.Errorf("OPEN_SPANNER_ENTITLEMENT_WORKER_TIMEOUT must be greater than zero")
	}
	if cfg.EntitlementWorkerRetryAfter <= 0 {
		return fmt.Errorf("OPEN_SPANNER_ENTITLEMENT_WORKER_RETRY_AFTER must be greater than zero")
	}
	if cfg.EntitlementWorkerMaxAttempts <= 0 {
		return fmt.Errorf("OPEN_SPANNER_ENTITLEMENT_WORKER_MAX_ATTEMPTS must be greater than zero")
	}
	if cfg.EntitlementWorkerBatchSize <= 0 {
		return fmt.Errorf("OPEN_SPANNER_ENTITLEMENT_WORKER_BATCH_SIZE must be greater than zero")
	}
	if cfg.RetentionPruneInterval <= 0 {
		return fmt.Errorf("OPEN_SPANNER_RETENTION_PRUNE_INTERVAL must be greater than zero")
	}
	if cfg.RetentionPruneTimeout <= 0 {
		return fmt.Errorf("OPEN_SPANNER_RETENTION_PRUNE_TIMEOUT must be greater than zero")
	}
	if cfg.ConsumptionDecisionRetention <= 0 {
		return fmt.Errorf("OPEN_SPANNER_CONSUMPTION_DECISION_RETENTION must be greater than zero")
	}
	if cfg.OperationalHistoryRetention <= 0 || cfg.OperationalHistoryInterval <= 0 || cfg.OperationalHistoryTimeout <= 0 {
		return fmt.Errorf("operational history retention, interval, and timeout must be greater than zero")
	}
	if cfg.OperationalHistoryBatchSize < 1 || cfg.OperationalHistoryBatchSize > 10000 {
		return fmt.Errorf("OPEN_SPANNER_OPERATIONAL_HISTORY_BATCH_SIZE must be between 1 and 10000")
	}
	if cfg.ReconciliationPollInterval <= 0 || cfg.ReconciliationSchedule <= 0 || cfg.ReconciliationLockTTL <= 0 || cfg.ReconciliationTimeout <= 0 || cfg.ReconciliationRetryAfter <= 0 || cfg.ReconciliationStaleAfter <= 0 {
		return fmt.Errorf("reconciliation durations must be greater than zero")
	}
	if cfg.ReconciliationLimit < 1 || cfg.ReconciliationLimit > 500 {
		return fmt.Errorf("OPEN_SPANNER_RECONCILIATION_LIMIT must be between 1 and 500")
	}
	if cfg.ReconciliationLookbackHours < 1 || cfg.ReconciliationLookbackHours > 720 {
		return fmt.Errorf("OPEN_SPANNER_RECONCILIATION_LOOKBACK_HOURS must be between 1 and 720")
	}
	if cfg.ReconciliationMaxAttempts < 1 {
		return fmt.Errorf("OPEN_SPANNER_RECONCILIATION_MAX_ATTEMPTS must be greater than zero")
	}
	if cfg.ReconciliationWebhookURL != "" {
		webhookURL, err := url.ParseRequestURI(cfg.ReconciliationWebhookURL)
		if err != nil || webhookURL.Host == "" || (webhookURL.Scheme != "http" && webhookURL.Scheme != "https") {
			return fmt.Errorf("OPEN_SPANNER_RECONCILIATION_WEBHOOK_URL must be an absolute HTTP(S) URL")
		}
	}

	return nil
}

func loadDBPoolConfig() (DBPoolConfig, error) {
	maxOpenConns, err := envInt("OPEN_SPANNER_DB_MAX_OPEN_CONNS", 0)
	if err != nil {
		return DBPoolConfig{}, err
	}
	maxIdleConns, err := envInt("OPEN_SPANNER_DB_MAX_IDLE_CONNS", 0)
	if err != nil {
		return DBPoolConfig{}, err
	}
	connMaxLifetime, err := envDurationAllowZero("OPEN_SPANNER_DB_CONN_MAX_LIFETIME", 0)
	if err != nil {
		return DBPoolConfig{}, err
	}
	connMaxIdleTime, err := envDurationAllowZero("OPEN_SPANNER_DB_CONN_MAX_IDLE_TIME", 0)
	if err != nil {
		return DBPoolConfig{}, err
	}

	return DBPoolConfig{
		MaxOpenConns:    maxOpenConns,
		MaxIdleConns:    maxIdleConns,
		ConnMaxLifetime: connMaxLifetime,
		ConnMaxIdleTime: connMaxIdleTime,
	}, nil
}

func env(name string, fallback string) string {
	value := os.Getenv(name)
	if value == "" {
		return fallback
	}
	return value
}

func envBool(name string, fallback bool) (bool, error) {
	value := os.Getenv(name)
	if value == "" {
		return fallback, nil
	}
	parsed, err := strconv.ParseBool(value)
	if err != nil {
		return false, fmt.Errorf("%s must be a boolean: %w", name, err)
	}
	return parsed, nil
}

func envInt(name string, fallback int) (int, error) {
	value := os.Getenv(name)
	if value == "" {
		return fallback, nil
	}
	parsed, err := strconv.Atoi(value)
	if err != nil {
		return 0, fmt.Errorf("%s must be an integer: %w", name, err)
	}
	if parsed < 0 {
		return 0, fmt.Errorf("%s cannot be negative", name)
	}
	return parsed, nil
}

func envDuration(name string, fallback time.Duration) (time.Duration, error) {
	value := os.Getenv(name)
	if value == "" {
		return fallback, nil
	}
	parsed, err := time.ParseDuration(value)
	if err != nil {
		return 0, fmt.Errorf("%s must be a duration: %w", name, err)
	}
	if parsed <= 0 {
		return 0, fmt.Errorf("%s must be greater than zero", name)
	}
	return parsed, nil
}

func envDurationAllowZero(name string, fallback time.Duration) (time.Duration, error) {
	value := os.Getenv(name)
	if value == "" {
		return fallback, nil
	}
	parsed, err := time.ParseDuration(value)
	if err != nil {
		return 0, fmt.Errorf("%s must be a duration: %w", name, err)
	}
	if parsed < 0 {
		return 0, fmt.Errorf("%s cannot be negative", name)
	}
	return parsed, nil
}
