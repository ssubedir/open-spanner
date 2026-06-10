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
	if cfg.RetentionPruneInterval != time.Hour {
		t.Fatalf("retention interval = %s, want 1h", cfg.RetentionPruneInterval)
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

func clearEnv(t *testing.T) {
	t.Helper()

	for _, key := range []string{
		"OPEN_SPANNER_HTTP_ADDR",
		"OPEN_SPANNER_DB_DRIVER",
		"OPEN_SPANNER_SQLITE_PATH",
		"OPEN_SPANNER_POSTGRES_DSN",
		"OPEN_SPANNER_DB_MAX_OPEN_CONNS",
		"OPEN_SPANNER_DB_MAX_IDLE_CONNS",
		"OPEN_SPANNER_DB_CONN_MAX_LIFETIME",
		"OPEN_SPANNER_DB_CONN_MAX_IDLE_TIME",
		"OPEN_SPANNER_RETENTION_PRUNE_ENABLED",
		"OPEN_SPANNER_RETENTION_PRUNE_INTERVAL",
	} {
		t.Setenv(key, "")
	}
}
