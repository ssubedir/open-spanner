package sqlite

import (
	"context"
	"database/sql"
	"embed"
	"errors"
	"net/url"
	"strings"

	"github.com/golang-migrate/migrate/v4"
	migratesqlite "github.com/golang-migrate/migrate/v4/database/sqlite"
	"github.com/golang-migrate/migrate/v4/source/iofs"
	"github.com/ssubedir/open-spanner/internal/config"
	_ "modernc.org/sqlite"
)

//go:embed migrations/*.sql
var migrationFiles embed.FS

type Store struct {
	db *sql.DB
}

type txContextKey struct{}

func NewStore(ctx context.Context, path string, poolConfigs ...config.DBPoolConfig) (*Store, error) {
	db, err := sql.Open("sqlite", sqliteDSN(path))
	if err != nil {
		return nil, err
	}
	applyPoolConfig(db, poolConfigs...)

	store := &Store{db: db}
	if err := store.configure(ctx); err != nil {
		_ = db.Close()
		return nil, err
	}
	if err := store.migrate(ctx); err != nil {
		_ = db.Close()
		return nil, err
	}

	return store, nil
}

func sqliteDSN(path string) string {
	values := url.Values{}
	values.Add("_pragma", "busy_timeout=10000")
	values.Add("_pragma", "foreign_keys=ON")
	values.Add("_pragma", "journal_mode=WAL")
	values.Add("_pragma", "synchronous=NORMAL")
	values.Set("_txlock", "immediate")

	separator := "?"
	if strings.Contains(path, "?") {
		separator = "&"
	}

	return path + separator + values.Encode()
}

func applyPoolConfig(db *sql.DB, poolConfigs ...config.DBPoolConfig) {
	if len(poolConfigs) == 0 {
		return
	}

	pool := poolConfigs[0]
	if pool.MaxOpenConns > 0 {
		db.SetMaxOpenConns(pool.MaxOpenConns)
	}
	if pool.MaxIdleConns > 0 {
		db.SetMaxIdleConns(pool.MaxIdleConns)
	}
	if pool.ConnMaxLifetime > 0 {
		db.SetConnMaxLifetime(pool.ConnMaxLifetime)
	}
	if pool.ConnMaxIdleTime > 0 {
		db.SetConnMaxIdleTime(pool.ConnMaxIdleTime)
	}
}

func (s *Store) Close() error {
	return s.db.Close()
}

func (s *Store) Ping(ctx context.Context) error {
	return s.db.PingContext(ctx)
}

func (s *Store) WithinTransaction(ctx context.Context, fn func(context.Context) error) error {
	if _, ok := txFromContext(ctx); ok {
		return fn(ctx)
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	txCtx := context.WithValue(ctx, txContextKey{}, tx)
	if err := fn(txCtx); err != nil {
		return err
	}

	return tx.Commit()
}

func (s *Store) configure(ctx context.Context) error {
	if _, err := s.db.ExecContext(ctx, "PRAGMA foreign_keys = ON"); err != nil {
		return err
	}
	if _, err := s.db.ExecContext(ctx, "PRAGMA busy_timeout = 5000"); err != nil {
		return err
	}
	if _, err := s.db.ExecContext(ctx, "PRAGMA journal_mode = WAL"); err != nil {
		return err
	}
	if _, err := s.db.ExecContext(ctx, "PRAGMA synchronous = NORMAL"); err != nil {
		return err
	}
	return nil
}

func (s *Store) migrate(ctx context.Context) error {
	sourceDriver, err := iofs.New(migrationFiles, "migrations")
	if err != nil {
		return err
	}
	defer sourceDriver.Close()

	databaseDriver, err := migratesqlite.WithInstance(s.db, &migratesqlite.Config{})
	if err != nil {
		return err
	}

	migration, err := migrate.NewWithInstance("iofs", sourceDriver, "sqlite", databaseDriver)
	if err != nil {
		return err
	}

	if err := migration.Up(); err != nil && !errors.Is(err, migrate.ErrNoChange) {
		return err
	}

	return nil
}

func (s *Store) exec(ctx context.Context, query string, args ...any) (sql.Result, error) {
	if tx, ok := txFromContext(ctx); ok {
		return tx.ExecContext(ctx, query, args...)
	}
	return s.db.ExecContext(ctx, query, args...)
}

func (s *Store) ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error) {
	return s.exec(ctx, query, args...)
}

func (s *Store) PrepareContext(ctx context.Context, query string) (*sql.Stmt, error) {
	if tx, ok := txFromContext(ctx); ok {
		return tx.PrepareContext(ctx, query)
	}
	return s.db.PrepareContext(ctx, query)
}

func (s *Store) query(ctx context.Context, query string, args ...any) (*sql.Rows, error) {
	if tx, ok := txFromContext(ctx); ok {
		return tx.QueryContext(ctx, query, args...)
	}
	return s.db.QueryContext(ctx, query, args...)
}

func (s *Store) QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error) {
	return s.query(ctx, query, args...)
}

func (s *Store) queryRow(ctx context.Context, query string, args ...any) *sql.Row {
	if tx, ok := txFromContext(ctx); ok {
		return tx.QueryRowContext(ctx, query, args...)
	}
	return s.db.QueryRowContext(ctx, query, args...)
}

func (s *Store) QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row {
	return s.queryRow(ctx, query, args...)
}

func txFromContext(ctx context.Context) (*sql.Tx, bool) {
	tx, ok := ctx.Value(txContextKey{}).(*sql.Tx)
	return tx, ok
}
