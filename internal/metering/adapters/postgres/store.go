package postgres

import (
	"context"
	"database/sql"
	"embed"
	"errors"
	"math/rand/v2"
	"time"

	"github.com/golang-migrate/migrate/v4"
	migratepostgres "github.com/golang-migrate/migrate/v4/database/postgres"
	"github.com/golang-migrate/migrate/v4/source/iofs"
	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/ssubedir/open-spanner/internal/config"
)

//go:embed migrations/*.sql
var migrationFiles embed.FS

type Store struct {
	db                 *sql.DB
	transactionMetrics TransactionRetryMetrics
}

const (
	transactionMaxAttempts = 3
	transactionBaseDelay   = 10 * time.Millisecond
	transactionMaxDelay    = 100 * time.Millisecond
)

// TransactionRetryMetrics records bounded Postgres transaction retry outcomes.
type TransactionRetryMetrics interface {
	RecordTransactionRetry(ctx context.Context, reason string)
	RecordTransactionRetryExhausted(ctx context.Context, reason string)
}

type txContextKey struct{}

func NewStore(ctx context.Context, dsn string, poolConfigs ...config.DBPoolConfig) (*Store, error) {
	db, err := sql.Open("pgx", dsn)
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

func (s *Store) Stats() sql.DBStats { return s.db.Stats() }

// SetTransactionRetryMetrics attaches an optional transaction retry recorder.
// It must be called during application startup before the store is used.
func (s *Store) SetTransactionRetryMetrics(metrics TransactionRetryMetrics) {
	s.transactionMetrics = metrics
}

func (s *Store) WithinTransaction(ctx context.Context, fn func(context.Context) error) error {
	if _, ok := txFromContext(ctx); ok {
		return fn(ctx)
	}

	return runWithTransactionRetries(ctx, s.transactionMetrics, func() error {
		return s.withinTransactionAttempt(ctx, fn)
	})
}

func runWithTransactionRetries(ctx context.Context, metrics TransactionRetryMetrics, fn func() error) error {
	for attempt := 1; attempt <= transactionMaxAttempts; attempt++ {
		err := fn()
		if err == nil {
			return nil
		}

		reason, retryable := transactionRetryReason(err)
		if !retryable {
			return err
		}
		if attempt == transactionMaxAttempts {
			if metrics != nil {
				metrics.RecordTransactionRetryExhausted(ctx, reason)
			}
			return err
		}
		if metrics != nil {
			metrics.RecordTransactionRetry(ctx, reason)
		}
		if err := waitForTransactionRetry(ctx, transactionRetryDelay(attempt)); err != nil {
			return err
		}
	}

	return nil
}

func (s *Store) withinTransactionAttempt(ctx context.Context, fn func(context.Context) error) error {
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

func transactionRetryReason(err error) (string, bool) {
	type sqlStateError interface {
		SQLState() string
	}

	var state sqlStateError
	if !errors.As(err, &state) {
		return "", false
	}
	switch state.SQLState() {
	case "40001":
		return "serialization_failure", true
	case "40P01":
		return "deadlock_detected", true
	default:
		return "", false
	}
}

func transactionRetryDelay(attempt int) time.Duration {
	delay := transactionBaseDelay << (attempt - 1)
	if delay > transactionMaxDelay {
		delay = transactionMaxDelay
	}
	// Use bounded 50-100% jitter so competing transactions do not retry together.
	half := delay / 2
	return half + time.Duration(rand.Int64N(int64(delay-half)+1))
}

func waitForTransactionRetry(ctx context.Context, delay time.Duration) error {
	timer := time.NewTimer(delay)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

func (s *Store) configure(ctx context.Context) error {
	return s.db.PingContext(ctx)
}

func (s *Store) migrate(ctx context.Context) (err error) {
	sourceDriver, err := iofs.New(migrationFiles, "migrations")
	if err != nil {
		return err
	}

	conn, err := s.db.Conn(ctx)
	if err != nil {
		_ = sourceDriver.Close()
		return err
	}

	databaseDriver, err := migratepostgres.WithConnection(ctx, conn, &migratepostgres.Config{})
	if err != nil {
		_ = sourceDriver.Close()
		_ = conn.Close()
		return err
	}

	migration, err := migrate.NewWithInstance("iofs", sourceDriver, "postgres", databaseDriver)
	if err != nil {
		_ = sourceDriver.Close()
		_ = databaseDriver.Close()
		return err
	}
	defer func() {
		sourceErr, databaseErr := migration.Close()
		err = errors.Join(err, sourceErr, databaseErr)
	}()

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
