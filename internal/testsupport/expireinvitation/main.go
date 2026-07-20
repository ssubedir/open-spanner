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
	invitationID := flag.String("invitation-id", "", "workspace invitation ID")
	flag.Parse()

	if strings.TrimSpace(*invitationID) == "" {
		fail("invitation id is required")
	}

	databaseDriver, dataSource := "sqlite", *sqlitePath
	statement := "UPDATE auth_workspace_invitations SET expires_at = ? WHERE id = ?"
	if strings.EqualFold(*driver, "postgres") {
		databaseDriver, dataSource = "pgx", *postgresDSN
		statement = "UPDATE auth_workspace_invitations SET expires_at = $1 WHERE id = $2"
	}

	database, err := sql.Open(databaseDriver, dataSource)
	if err != nil {
		fail("open database: %v", err)
	}
	defer database.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	result, err := database.ExecContext(ctx, statement, time.Date(2000, time.January, 1, 0, 0, 0, 0, time.UTC).Format(time.RFC3339), *invitationID)
	if err != nil {
		fail("expire invitation fixture: %v", err)
	}
	rows, err := result.RowsAffected()
	if err != nil {
		fail("read affected invitation rows: %v", err)
	}
	if rows != 1 {
		fail("expire invitation fixture affected %d rows, want 1", rows)
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
