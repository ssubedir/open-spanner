package config

import (
	"strings"
	"testing"
	"time"
)

func TestLoadDefaultsToSQLite(t *testing.T) {
	clearEnv(t)

	cfg, err := Load()
	if err != nil {
		t.Fatalf("load config: %v", err)
	}

	if cfg.DBDriver != "sqlite" {
		t.Fatalf("db driver = %q, want sqlite", cfg.DBDriver)
	}
	if cfg.SQLitePath != "open-spanner.db" {
		t.Fatalf("sqlite path = %q, want open-spanner.db", cfg.SQLitePath)
	}
	if cfg.GRPCAddr != ":18090" {
		t.Fatalf("grpc addr = %q, want :18090", cfg.GRPCAddr)
	}
	if cfg.ExportStorageDriver != "filesystem" {
		t.Fatalf("export storage driver = %q, want filesystem", cfg.ExportStorageDriver)
	}
	if !cfg.RegistrationEnabled {
		t.Fatal("registration should be enabled by default")
	}
	if cfg.RetentionPruneInterval != time.Hour {
		t.Fatalf("retention interval = %s, want 1h", cfg.RetentionPruneInterval)
	}
	if cfg.RetentionPruneTimeout != 30*time.Minute {
		t.Fatalf("retention timeout = %s, want 30m", cfg.RetentionPruneTimeout)
	}
	if cfg.ConsumptionDecisionRetention != 30*24*time.Hour {
		t.Fatalf("decision retention = %s, want 720h", cfg.ConsumptionDecisionRetention)
	}
	if cfg.AlertWorkerInterval != 5*time.Second {
		t.Fatalf("alert worker interval = %s, want 5s", cfg.AlertWorkerInterval)
	}
	if cfg.AlertWorkerTimeout != time.Minute {
		t.Fatalf("alert worker timeout = %s, want 1m", cfg.AlertWorkerTimeout)
	}
	if cfg.ReconciliationEnabled || cfg.ReconciliationPollInterval != 5*time.Second || cfg.ReconciliationSchedule != 15*time.Minute || cfg.ReconciliationStaleAfter != 30*time.Minute || cfg.ReconciliationLimit != 100 || cfg.ReconciliationLookbackHours != 24 || cfg.ReconciliationMaxAttempts != 5 {
		t.Fatalf("reconciliation defaults = %#v", cfg)
	}
	if cfg.OAuth.GitHub.ClientID != "" || cfg.OAuth.GitHub.ClientSecret != "" || !cfg.OAuth.GitHub.Enabled || cfg.OAuth.GitHub.RedirectURL != "" {
		t.Fatalf("github oauth config = %#v, want empty defaults", cfg.OAuth.GitHub)
	}
	if cfg.OAuth.Google.ClientID != "" || cfg.OAuth.Google.ClientSecret != "" || !cfg.OAuth.Google.Enabled || cfg.OAuth.Google.RedirectURL != "" {
		t.Fatalf("google oauth config = %#v, want empty defaults", cfg.OAuth.Google)
	}
}

func TestLoadPostgresRequiresDSN(t *testing.T) {
	clearEnv(t)
	t.Setenv("OPEN_SPANNER_DB_DRIVER", "postgres")

	_, err := Load()
	if err == nil || !strings.Contains(err.Error(), "OPEN_SPANNER_POSTGRES_DSN") {
		t.Fatalf("load error = %v, want postgres dsn error", err)
	}
}

func TestLoadAcceptsPostgresDSN(t *testing.T) {
	clearEnv(t)
	t.Setenv("OPEN_SPANNER_DB_DRIVER", "postgres")
	t.Setenv("OPEN_SPANNER_POSTGRES_DSN", "postgres://postgres:postgres@localhost:5432/open_spanner?sslmode=disable")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("load config: %v", err)
	}
	if cfg.DBDriver != "postgres" || cfg.PostgresDSN == "" {
		t.Fatalf("config = %#v, want postgres config", cfg)
	}
}

func TestLoadRejectsUnsupportedDriver(t *testing.T) {
	clearEnv(t)
	t.Setenv("OPEN_SPANNER_DB_DRIVER", "memory")

	_, err := Load()
	if err == nil || !strings.Contains(err.Error(), "unsupported OPEN_SPANNER_DB_DRIVER") {
		t.Fatalf("load error = %v, want unsupported driver error", err)
	}
}

func TestLoadAcceptsS3ExportStorage(t *testing.T) {
	clearEnv(t)
	t.Setenv("OPEN_SPANNER_EXPORT_STORAGE_DRIVER", "s3")
	t.Setenv("OPEN_SPANNER_EXPORT_S3_BUCKET", "exports")
	t.Setenv("OPEN_SPANNER_EXPORT_S3_ENDPOINT", "http://localhost:9000")
	t.Setenv("OPEN_SPANNER_EXPORT_S3_ACCESS_KEY_ID", "test")
	t.Setenv("OPEN_SPANNER_EXPORT_S3_SECRET_ACCESS_KEY", "secret")
	cfg, err := Load()
	if err != nil {
		t.Fatalf("load config: %v", err)
	}
	if cfg.ExportStorageDriver != "s3" || cfg.ExportS3Bucket != "exports" || !cfg.ExportS3ForcePathStyle {
		t.Fatalf("S3 config = %#v", cfg)
	}
}

func TestLoadRejectsIncompleteS3ExportStorage(t *testing.T) {
	clearEnv(t)
	t.Setenv("OPEN_SPANNER_EXPORT_STORAGE_DRIVER", "s3")
	if _, err := Load(); err == nil || !strings.Contains(err.Error(), "OPEN_SPANNER_EXPORT_S3_BUCKET") {
		t.Fatalf("load error = %v", err)
	}
	t.Setenv("OPEN_SPANNER_EXPORT_S3_BUCKET", "exports")
	t.Setenv("OPEN_SPANNER_EXPORT_S3_ACCESS_KEY_ID", "test")
	if _, err := Load(); err == nil || !strings.Contains(err.Error(), "must be set together") {
		t.Fatalf("load error = %v", err)
	}
}

func TestLoadRejectsInvalidPoolConfig(t *testing.T) {
	clearEnv(t)
	t.Setenv("OPEN_SPANNER_DB_MAX_OPEN_CONNS", "-1")

	_, err := Load()
	if err == nil || !strings.Contains(err.Error(), "OPEN_SPANNER_DB_MAX_OPEN_CONNS") {
		t.Fatalf("load error = %v, want max open conns error", err)
	}
}

func TestLoadRejectsInvalidRetentionInterval(t *testing.T) {
	clearEnv(t)
	t.Setenv("OPEN_SPANNER_RETENTION_PRUNE_INTERVAL", "0s")

	_, err := Load()
	if err == nil || !strings.Contains(err.Error(), "OPEN_SPANNER_RETENTION_PRUNE_INTERVAL") {
		t.Fatalf("load error = %v, want retention interval error", err)
	}
}

func TestLoadRejectsInvalidRetentionTimeout(t *testing.T) {
	clearEnv(t)
	t.Setenv("OPEN_SPANNER_RETENTION_PRUNE_TIMEOUT", "0s")

	_, err := Load()
	if err == nil || !strings.Contains(err.Error(), "OPEN_SPANNER_RETENTION_PRUNE_TIMEOUT") {
		t.Fatalf("load error = %v, want retention timeout error", err)
	}
}

func TestLoadRejectsInvalidConsumptionDecisionRetention(t *testing.T) {
	clearEnv(t)
	t.Setenv("OPEN_SPANNER_CONSUMPTION_DECISION_RETENTION", "0s")

	_, err := Load()
	if err == nil || !strings.Contains(err.Error(), "OPEN_SPANNER_CONSUMPTION_DECISION_RETENTION") {
		t.Fatalf("load error = %v, want decision retention error", err)
	}
}

func clearEnv(t *testing.T) {
	t.Helper()

	for _, key := range []string{
		"OPEN_SPANNER_HTTP_ADDR",
		"OPEN_SPANNER_GRPC_ADDR",
		"OPEN_SPANNER_REGISTRATION_ENABLED",
		"OPEN_SPANNER_GITHUB_OAUTH_ENABLED",
		"OPEN_SPANNER_GITHUB_OAUTH_CLIENT_ID",
		"OPEN_SPANNER_GITHUB_OAUTH_CLIENT_SECRET",
		"OPEN_SPANNER_GITHUB_OAUTH_REDIRECT_URL",
		"OPEN_SPANNER_GOOGLE_OAUTH_ENABLED",
		"OPEN_SPANNER_GOOGLE_OAUTH_CLIENT_ID",
		"OPEN_SPANNER_GOOGLE_OAUTH_CLIENT_SECRET",
		"OPEN_SPANNER_GOOGLE_OAUTH_REDIRECT_URL",
		"OPEN_SPANNER_DB_DRIVER",
		"OPEN_SPANNER_SQLITE_PATH",
		"OPEN_SPANNER_POSTGRES_DSN",
		"OPEN_SPANNER_DB_MAX_OPEN_CONNS",
		"OPEN_SPANNER_DB_MAX_IDLE_CONNS",
		"OPEN_SPANNER_DB_CONN_MAX_LIFETIME",
		"OPEN_SPANNER_DB_CONN_MAX_IDLE_TIME",
		"OPEN_SPANNER_RETENTION_PRUNE_ENABLED",
		"OPEN_SPANNER_RETENTION_PRUNE_INTERVAL",
		"OPEN_SPANNER_RETENTION_PRUNE_TIMEOUT",
		"OPEN_SPANNER_CONSUMPTION_DECISION_RETENTION",
		"OPEN_SPANNER_EXPORT_STORAGE_PATH",
		"OPEN_SPANNER_EXPORT_STORAGE_DRIVER",
		"OPEN_SPANNER_EXPORT_S3_BUCKET",
		"OPEN_SPANNER_EXPORT_S3_REGION",
		"OPEN_SPANNER_EXPORT_S3_ENDPOINT",
		"OPEN_SPANNER_EXPORT_S3_ACCESS_KEY_ID",
		"OPEN_SPANNER_EXPORT_S3_SECRET_ACCESS_KEY",
		"OPEN_SPANNER_EXPORT_S3_SESSION_TOKEN",
		"OPEN_SPANNER_EXPORT_S3_PREFIX",
		"OPEN_SPANNER_EXPORT_S3_FORCE_PATH_STYLE",
		"OPEN_SPANNER_EXPORT_WORKER_INTERVAL",
		"OPEN_SPANNER_EXPORT_WORKER_LOCK_TTL",
		"OPEN_SPANNER_EXPORT_WORKER_MAX_ATTEMPTS",
		"OPEN_SPANNER_EXPORT_RETENTION",
		"OPEN_SPANNER_EXPORT_CLEANUP_INTERVAL",
		"OPEN_SPANNER_EXPORT_CLEANUP_BATCH_SIZE",
		"OPEN_SPANNER_ALERT_WORKER_INTERVAL",
		"OPEN_SPANNER_ALERT_WORKER_LOCK_TTL",
		"OPEN_SPANNER_ALERT_WORKER_TIMEOUT",
		"OPEN_SPANNER_ALERT_WORKER_RETRY_AFTER",
		"OPEN_SPANNER_ALERT_WORKER_MAX_ATTEMPTS",
		"OPEN_SPANNER_ALERT_WORKER_BATCH_SIZE",
		"OPEN_SPANNER_RECONCILIATION_ENABLED",
		"OPEN_SPANNER_RECONCILIATION_POLL_INTERVAL",
		"OPEN_SPANNER_RECONCILIATION_SCHEDULE",
		"OPEN_SPANNER_RECONCILIATION_LOCK_TTL",
		"OPEN_SPANNER_RECONCILIATION_TIMEOUT",
		"OPEN_SPANNER_RECONCILIATION_RETRY_AFTER",
		"OPEN_SPANNER_RECONCILIATION_STALE_AFTER",
		"OPEN_SPANNER_RECONCILIATION_LIMIT",
		"OPEN_SPANNER_RECONCILIATION_LOOKBACK_HOURS",
		"OPEN_SPANNER_RECONCILIATION_MAX_ATTEMPTS",
		"OPEN_SPANNER_RECONCILIATION_WEBHOOK_URL",
		"OPEN_SPANNER_RECONCILIATION_WEBHOOK_SECRET",
	} {
		t.Setenv(key, "")
	}
}
