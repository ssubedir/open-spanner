package bootstrap

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"

	postgresadapter "github.com/ssubedir/open-spanner/internal/metering/adapters/postgres"
	sqliteadapter "github.com/ssubedir/open-spanner/internal/metering/adapters/sqlite"
	appsystem "github.com/ssubedir/open-spanner/internal/metering/app/system"
)

type historyStore interface {
	ExecContext(context.Context, string, ...any) (sql.Result, error)
	QueryRowContext(context.Context, string, ...any) *sql.Row
	WithinTransaction(context.Context, func(context.Context) error) error
	Close() error
}

func TestIntegrationSQLiteOperationalHistoryCleanup(t *testing.T) {
	store, err := sqliteadapter.NewStore(context.Background(), ":memory:")
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })
	runOperationalHistoryCleanup(t, store, sqliteadapter.NewSystemRepository(store))
}

func TestIntegrationPostgresOperationalHistoryCleanup(t *testing.T) {
	dsn := os.Getenv("OPEN_SPANNER_TEST_POSTGRES_DSN")
	if dsn == "" {
		t.Skip("set OPEN_SPANNER_TEST_POSTGRES_DSN to run Postgres bootstrap integration tests")
	}
	store, err := postgresadapter.NewStore(context.Background(), dsn)
	if err != nil {
		t.Fatalf("open postgres: %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })
	runOperationalHistoryCleanup(t, store, postgresadapter.NewSystemRepository(store))
}

func runOperationalHistoryCleanup(t *testing.T, store historyStore, repo appsystem.Repository) {
	t.Helper()
	ctx := context.Background()
	suffix := uuid.NewString()
	workspaceID := "history-" + suffix
	old := time.Now().UTC().Add(-60 * 24 * time.Hour).Format(time.RFC3339Nano)
	recent := time.Now().UTC().Add(-time.Hour).Format(time.RFC3339Nano)
	cutoff := time.Now().UTC().Add(-30 * 24 * time.Hour)

	mustHistoryExec(t, store, fmt.Sprintf(`INSERT INTO auth_workspaces (id, name, created_at) VALUES ('%s', 'History Test', '%s')`, workspaceID, old))
	t.Cleanup(func() {
		_, _ = store.ExecContext(context.Background(), fmt.Sprintf(`DELETE FROM auth_workspaces WHERE id = '%s'`, workspaceID))
	})
	for index, createdAt := range []string{old, old, recent} {
		mustHistoryExec(t, store, fmt.Sprintf(`INSERT INTO usage_ingestions (id, workspace_id, kind, accepted, duplicates, failed, created_at) VALUES ('audit-%d-%s', '%s', 'single', 1, 0, 0, '%s')`, index, suffix, workspaceID, createdAt))
	}
	mustHistoryExec(t, store, fmt.Sprintf(`INSERT INTO usage_export_jobs (id, workspace_id, kind, status, format, query_json, created_at, updated_at, completed_at, expired_at) VALUES
		('expired-%s', '%s', 'usage', 'completed', 'csv', '{}', '%s', '%s', '%s', '%s'),
		('failed-%s', '%s', 'usage', 'failed', 'csv', '{}', '%s', '%s', '%s', NULL),
		('recent-%s', '%s', 'usage', 'completed', 'csv', '{}', '%s', '%s', '%s', '%s')`,
		suffix, workspaceID, old, old, old, old,
		suffix, workspaceID, old, old, old,
		suffix, workspaceID, recent, recent, recent, recent))
	mustHistoryExec(t, store, fmt.Sprintf(`INSERT INTO reconciliation_notifications (id, workspace_id, event_type, fingerprint, payload, status, attempts, next_attempt_at, last_error, created_at, delivered_at) VALUES
		('delivered-%s', '%s', 'drift_detected', 'one', '{}', 'delivered', 1, '%s', '', '%s', '%s'),
		('dead-%s', '%s', 'scan_failed', 'two', '{}', 'dead_letter', 1, '%s', 'failed', '%s', NULL)`, suffix, workspaceID, old, old, old, suffix, workspaceID, old, old))
	mustHistoryExec(t, store, fmt.Sprintf(`INSERT INTO reconciliation_notification_attempts (id, notification_id, attempt, status, error, created_at) VALUES ('attempt-%s', 'delivered-%s', 1, 'delivered', '', '%s')`, suffix, suffix, old))

	service := appsystem.NewService(repo, store)
	first, err := service.PruneOperationalHistory(ctx, cutoff, 1)
	if err != nil {
		t.Fatalf("first cleanup: %v", err)
	}
	if first.IngestionAudits != 1 || first.ExportJobs != 1 || first.ReconciliationNotifications != 1 {
		t.Fatalf("first cleanup = %+v", first)
	}
	second, err := service.PruneOperationalHistory(ctx, cutoff, 1)
	if err != nil {
		t.Fatalf("second cleanup: %v", err)
	}
	if second.IngestionAudits != 1 || second.ExportJobs != 0 || second.ReconciliationNotifications != 0 {
		t.Fatalf("second cleanup = %+v", second)
	}

	assertHistoryCount(t, store, fmt.Sprintf(`SELECT COUNT(*) FROM usage_ingestions WHERE workspace_id = '%s'`, workspaceID), 1)
	assertHistoryCount(t, store, fmt.Sprintf(`SELECT COUNT(*) FROM usage_export_jobs WHERE workspace_id = '%s'`, workspaceID), 2)
	assertHistoryCount(t, store, fmt.Sprintf(`SELECT COUNT(*) FROM usage_export_jobs WHERE workspace_id = '%s' AND status = 'failed'`, workspaceID), 1)
	assertHistoryCount(t, store, fmt.Sprintf(`SELECT COUNT(*) FROM reconciliation_notifications WHERE workspace_id = '%s' AND status = 'dead_letter'`, workspaceID), 1)
	assertHistoryCount(t, store, fmt.Sprintf(`SELECT COUNT(*) FROM reconciliation_notification_attempts WHERE notification_id = 'delivered-%s'`, suffix), 0)
}

func mustHistoryExec(t *testing.T, store historyStore, query string) {
	t.Helper()
	if _, err := store.ExecContext(context.Background(), query); err != nil {
		t.Fatalf("exec history fixture: %v\n%s", err, query)
	}
}

func assertHistoryCount(t *testing.T, store historyStore, query string, want int) {
	t.Helper()
	var count int
	if err := store.QueryRowContext(context.Background(), query).Scan(&count); err != nil {
		t.Fatalf("count history rows: %v", err)
	}
	if count != want {
		t.Fatalf("history count=%d, want %d: %s", count, want, query)
	}
}
