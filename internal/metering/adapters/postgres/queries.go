package postgres

import (
	"context"

	"github.com/ssubedir/open-spanner/internal/metering/adapters/postgres/postgresdb"
)

func queriesFor(ctx context.Context, queries *postgresdb.Queries) *postgresdb.Queries {
	if tx, ok := txFromContext(ctx); ok {
		return queries.WithTx(tx)
	}
	return queries
}
