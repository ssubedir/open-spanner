package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/joho/godotenv"
)

type Config struct {
	HTTPAddr                string
	DBDriver                string
	SQLitePath              string
	PostgresDSN             string
	DBPool                  DBPoolConfig
	ExportStoragePath       string
	ExportWorkerInterval    time.Duration
	ExportWorkerLockTTL     time.Duration
	ExportWorkerTimeout     time.Duration
	ExportWorkerMaxAttempts int
	AlertWorkerInterval     time.Duration
	AlertWorkerLockTTL      time.Duration
	AlertWorkerTimeout      time.Duration
	AlertWorkerRetryAfter   time.Duration
	AlertWorkerMaxAttempts  int
	AlertWorkerBatchSize    int
	RetentionPruneEnabled   bool
	RetentionPruneInterval  time.Duration
	RetentionPruneTimeout   time.Duration
}

type DBPoolConfig struct {
	MaxOpenConns    int
	MaxIdleConns    int
	ConnMaxLifetime time.Duration
	ConnMaxIdleTime time.Duration
}

func Load() (Config, error) {
	_ = godotenv.Load()

	pool, err := loadDBPoolConfig()
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
	exportWorkerInterval, err := envDuration("OPEN_SPANNER_EXPORT_WORKER_INTERVAL", 5*time.Second)
	if err != nil {
		return Config{}, err
	}
	exportWorkerLockTTL, err := envDuration("OPEN_SPANNER_EXPORT_WORKER_LOCK_TTL", 5*time.Minute)
	if err != nil {
		return Config{}, err
	}
	exportWorkerTimeout, err := envDuration("OPEN_SPANNER_EXPORT_WORKER_TIMEOUT", 10*time.Minute)
	if err != nil {
		return Config{}, err
	}
	exportWorkerMaxAttempts, err := envInt("OPEN_SPANNER_EXPORT_WORKER_MAX_ATTEMPTS", 3)
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

	cfg := Config{
		HTTPAddr:                env("OPEN_SPANNER_HTTP_ADDR", ":18081"),
		DBDriver:                strings.ToLower(env("OPEN_SPANNER_DB_DRIVER", "sqlite")),
		SQLitePath:              env("OPEN_SPANNER_SQLITE_PATH", "open-spanner.db"),
		PostgresDSN:             env("OPEN_SPANNER_POSTGRES_DSN", ""),
		DBPool:                  pool,
		ExportStoragePath:       env("OPEN_SPANNER_EXPORT_STORAGE_PATH", "open-spanner-exports"),
		ExportWorkerInterval:    exportWorkerInterval,
		ExportWorkerLockTTL:     exportWorkerLockTTL,
		ExportWorkerTimeout:     exportWorkerTimeout,
		ExportWorkerMaxAttempts: exportWorkerMaxAttempts,
		AlertWorkerInterval:     alertWorkerInterval,
		AlertWorkerLockTTL:      alertWorkerLockTTL,
		AlertWorkerTimeout:      alertWorkerTimeout,
		AlertWorkerRetryAfter:   alertWorkerRetryAfter,
		AlertWorkerMaxAttempts:  alertWorkerMaxAttempts,
		AlertWorkerBatchSize:    alertWorkerBatchSize,
		RetentionPruneEnabled:   retentionEnabled,
		RetentionPruneInterval:  retentionInterval,
		RetentionPruneTimeout:   retentionTimeout,
	}

	if err := cfg.Validate(); err != nil {
		return Config{}, err
	}

	return cfg, nil
}

func (cfg Config) Validate() error {
	if strings.TrimSpace(cfg.HTTPAddr) == "" {
		return fmt.Errorf("OPEN_SPANNER_HTTP_ADDR is required")
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
	if cfg.DBPool.ConnMaxLifetime < 0 {
		return fmt.Errorf("OPEN_SPANNER_DB_CONN_MAX_LIFETIME cannot be negative")
	}
	if cfg.DBPool.ConnMaxIdleTime < 0 {
		return fmt.Errorf("OPEN_SPANNER_DB_CONN_MAX_IDLE_TIME cannot be negative")
	}
	if strings.TrimSpace(cfg.ExportStoragePath) == "" {
		return fmt.Errorf("OPEN_SPANNER_EXPORT_STORAGE_PATH is required")
	}
	if cfg.ExportWorkerInterval <= 0 {
		return fmt.Errorf("OPEN_SPANNER_EXPORT_WORKER_INTERVAL must be greater than zero")
	}
	if cfg.ExportWorkerLockTTL <= 0 {
		return fmt.Errorf("OPEN_SPANNER_EXPORT_WORKER_LOCK_TTL must be greater than zero")
	}
	if cfg.ExportWorkerTimeout <= 0 {
		return fmt.Errorf("OPEN_SPANNER_EXPORT_WORKER_TIMEOUT must be greater than zero")
	}
	if cfg.ExportWorkerMaxAttempts <= 0 {
		return fmt.Errorf("OPEN_SPANNER_EXPORT_WORKER_MAX_ATTEMPTS must be greater than zero")
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
	if cfg.RetentionPruneInterval <= 0 {
		return fmt.Errorf("OPEN_SPANNER_RETENTION_PRUNE_INTERVAL must be greater than zero")
	}
	if cfg.RetentionPruneTimeout <= 0 {
		return fmt.Errorf("OPEN_SPANNER_RETENTION_PRUNE_TIMEOUT must be greater than zero")
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
