package sqlite

import (
	"context"
	"testing"

	"github.com/ssubedir/open-spanner/internal/config"
	"github.com/ssubedir/open-spanner/internal/metering/adapters/repositorytest"
	apptransaction "github.com/ssubedir/open-spanner/internal/metering/app/transaction"
	domainmeter "github.com/ssubedir/open-spanner/internal/metering/domain/meter"
	domainusage "github.com/ssubedir/open-spanner/internal/metering/domain/usage"
)

func TestRepositoryContract(t *testing.T) {
	repositorytest.Run(t, func(t *testing.T, ctx context.Context) (domainmeter.Repository, domainusage.Repository, apptransaction.Transactor) {
		t.Helper()

		store, err := NewStore(ctx, ":memory:", config.DBPoolConfig{MaxOpenConns: 1})
		if err != nil {
			t.Fatalf("new sqlite store: %v", err)
		}
		t.Cleanup(func() {
			if err := store.Close(); err != nil {
				t.Fatalf("close sqlite store: %v", err)
			}
		})

		return NewMeterRepository(store), NewUsageRepository(store), store
	})
}
