package config

import (
	"os"
	"strconv"
	"time"

	"github.com/joho/godotenv"
)

type Config struct {
	HTTPAddr               string
	DBDriver               string
	SQLitePath             string
	PostgresDSN            string
	RetentionPruneEnabled  bool
	RetentionPruneInterval time.Duration
}

func Load() Config {
	_ = godotenv.Load()

	return Config{
		HTTPAddr:               env("OPEN_SPANNER_HTTP_ADDR", ":18081"),
		DBDriver:               env("OPEN_SPANNER_DB_DRIVER", "sqlite"),
		SQLitePath:             env("OPEN_SPANNER_SQLITE_PATH", "open-spanner.db"),
		PostgresDSN:            env("OPEN_SPANNER_POSTGRES_DSN", ""),
		RetentionPruneEnabled:  envBool("OPEN_SPANNER_RETENTION_PRUNE_ENABLED", false),
		RetentionPruneInterval: envDuration("OPEN_SPANNER_RETENTION_PRUNE_INTERVAL", time.Hour),
	}
}

func env(name string, fallback string) string {
	value := os.Getenv(name)
	if value == "" {
		return fallback
	}
	return value
}

func envBool(name string, fallback bool) bool {
	value := os.Getenv(name)
	if value == "" {
		return fallback
	}
	parsed, err := strconv.ParseBool(value)
	if err != nil {
		return fallback
	}
	return parsed
}

func envDuration(name string, fallback time.Duration) time.Duration {
	value := os.Getenv(name)
	if value == "" {
		return fallback
	}
	parsed, err := time.ParseDuration(value)
	if err != nil || parsed <= 0 {
		return fallback
	}
	return parsed
}
