package postgres

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"testing"
	"time"

	"github.com/ssubedir/open-spanner/internal/config"
)

func TestBindArg(t *testing.T) {
	args := []any{}

	first := bindArg(&args, "org_123")
	second := bindArg(&args, "api_requests")

	if first != "$1" {
		t.Fatalf("first bind arg = %q, want $1", first)
	}
	if second != "$2" {
		t.Fatalf("second bind arg = %q, want $2", second)
	}
	if len(args) != 2 {
		t.Fatalf("args length = %d, want 2", len(args))
	}
}

func TestPostgresJSONPath(t *testing.T) {
	got := postgresJSONPath("region.zone")
	want := "'{region,zone}'"
	if got != want {
		t.Fatalf("postgresJSONPath() = %q, want %q", got, want)
	}
}

func TestApplyPoolConfig(t *testing.T) {
	db := sql.OpenDB(noopConnector{})
	defer db.Close()

	applyPoolConfig(db, config.DBPoolConfig{
		MaxOpenConns:    7,
		MaxIdleConns:    3,
		ConnMaxLifetime: time.Minute,
		ConnMaxIdleTime: time.Second,
	})

	stats := db.Stats()
	if stats.MaxOpenConnections != 7 {
		t.Fatalf("max open connections = %d, want 7", stats.MaxOpenConnections)
	}
}

type noopConnector struct{}

func (noopConnector) Connect(context.Context) (driver.Conn, error) {
	return nil, errors.New("noop connector does not connect")
}

func (noopConnector) Driver() driver.Driver {
	return noopDriver{}
}

type noopDriver struct{}

func (noopDriver) Open(string) (driver.Conn, error) {
	return nil, errors.New("noop driver does not connect")
}
