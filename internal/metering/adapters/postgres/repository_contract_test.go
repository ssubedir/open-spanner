package postgres

import (
	"context"
	"testing"

	"github.com/ssubedir/open-spanner/internal/metering/adapters/repositorytest"
	apptransaction "github.com/ssubedir/open-spanner/internal/metering/app/transaction"
	domainmeter "github.com/ssubedir/open-spanner/internal/metering/domain/meter"
	domainusage "github.com/ssubedir/open-spanner/internal/metering/domain/usage"
)

func TestIntegrationRepositoryContract(t *testing.T) {
	repositorytest.Run(t, func(t *testing.T, ctx context.Context) (domainmeter.Repository, domainusage.Repository, apptransaction.Transactor) {
		t.Helper()

		store := newIntegrationStore(t, ctx)
		return NewMeterRepository(store), NewUsageRepository(store), store
	})
}
