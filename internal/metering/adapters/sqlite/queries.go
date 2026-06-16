package sqlite

import (
	"context"

	"github.com/ssubedir/open-spanner/internal/metering/adapters/sqlite/sqlitedb"
)

func queriesFor(ctx context.Context, queries *sqlitedb.Queries) *sqlitedb.Queries {
	if tx, ok := txFromContext(ctx); ok {
		return queries.WithTx(tx)
	}
	return queries
}
