package sqlite

import (
	"context"
	"database/sql"
	"embed"
	"errors"

	"github.com/golang-migrate/migrate/v4"
	migratesqlite "github.com/golang-migrate/migrate/v4/database/sqlite"
	"github.com/golang-migrate/migrate/v4/source/iofs"
	_ "modernc.org/sqlite"
)

//go:embed migrations/*.sql
var migrationFiles embed.FS

type Store struct {
	db *sql.DB
}

func NewStore(ctx context.Context, path string) (*Store, error) {
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, err
	}

	store := &Store{db: db}
	if err := store.configure(ctx); err != nil {
		_ = db.Close()
		return nil, err
	}
	if err := store.migrate(ctx); err != nil {
		_ = db.Close()
		return nil, err
	}
	if err := store.ensureColumns(ctx); err != nil {
		_ = db.Close()
		return nil, err
	}

	return store, nil
}

func (s *Store) ensureColumns(ctx context.Context) error {
	hasMetadataSchema, err := s.hasColumn(ctx, "meters", "metadata_schema")
	if err != nil {
		return err
	}
	if !hasMetadataSchema {
		if _, err := s.db.ExecContext(ctx, `ALTER TABLE meters ADD COLUMN metadata_schema TEXT NOT NULL DEFAULT '{}'`); err != nil {
			return err
		}
	}
	hasEventRetentionDays, err := s.hasColumn(ctx, "meters", "event_retention_days")
	if err != nil {
		return err
	}
	if !hasEventRetentionDays {
		if _, err := s.db.ExecContext(ctx, `ALTER TABLE meters ADD COLUMN event_retention_days INTEGER NOT NULL DEFAULT 90`); err != nil {
			return err
		}
	}
	return nil
}

func (s *Store) hasColumn(ctx context.Context, table string, column string) (bool, error) {
	rows, err := s.db.QueryContext(ctx, "PRAGMA table_info("+table+")")
	if err != nil {
		return false, err
	}
	defer rows.Close()

	for rows.Next() {
		var cid int
		var name string
		var columnType string
		var notNull int
		var defaultValue sql.NullString
		var primaryKey int
		if err := rows.Scan(&cid, &name, &columnType, &notNull, &defaultValue, &primaryKey); err != nil {
			return false, err
		}
		if name == column {
			return true, nil
		}
	}
	return false, rows.Err()
}

func (s *Store) Close() error {
	return s.db.Close()
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
