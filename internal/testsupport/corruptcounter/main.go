package main

import (
	"context"
	"database/sql"
	"flag"
	"fmt"
	"os"
	"strings"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"
	_ "modernc.org/sqlite"
)

func main() {
	driver := flag.String("driver", envOr("OPEN_SPANNER_E2E_DB_DRIVER", "sqlite"), "database driver")
	postgresDSN := flag.String("postgres-dsn", envOr("OPEN_SPANNER_E2E_POSTGRES_DSN", "postgres://postgres:postgres@localhost:5432/open_spanner_e2e?sslmode=disable"), "Postgres DSN")
	sqlitePath := flag.String("sqlite-path", envOr("OPEN_SPANNER_E2E_SQLITE_PATH", ".tmp/e2e-open-spanner.db"), "SQLite path")
	subject := flag.String("subject", "", "counter subject")
	meter := flag.String("meter", "", "counter meter")
	period := flag.String("period", "month", "counter period")
	flag.Parse()

	if strings.TrimSpace(*subject) == "" || strings.TrimSpace(*meter) == "" {
		fail("subject and meter are required")
	}

	updatedAt := time.Now().UTC().Format(time.RFC3339Nano)
	databaseDriver, dataSource, statement := "sqlite", *sqlitePath, `
		UPDATE entitlement_usage_counters
		SET event_count = 0, quantity_sum = 0, updated_at = ?
		WHERE subject = ? AND meter_name = ? AND period = ?`
	if strings.EqualFold(*driver, "postgres") {
		databaseDriver, dataSource = "pgx", *postgresDSN
		statement = `
			UPDATE entitlement_usage_counters
			SET event_count = 0, quantity_sum = 0, updated_at = $1
			WHERE subject = $2 AND meter_name = $3 AND period = $4`
	}

	database, err := sql.Open(databaseDriver, dataSource)
	if err != nil {
		fail("open database: %v", err)
	}
	defer database.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	result, err := database.ExecContext(ctx, statement, updatedAt, *subject, *meter, *period)
	if err != nil {
		fail("corrupt counter fixture: %v", err)
	}
	rows, err := result.RowsAffected()
	if err != nil {
		fail("read affected counter rows: %v", err)
	}
	if rows != 1 {
		fail("corrupt counter fixture affected %d rows, want 1", rows)
	}
}

func envOr(name, fallback string) string {
	if value := strings.TrimSpace(os.Getenv(name)); value != "" {
		return value
	}
	return fallback
}

func fail(format string, args ...any) {
	_, _ = fmt.Fprintf(os.Stderr, format+"\n", args...)
	os.Exit(1)
}
