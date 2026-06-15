package usage

import (
	"context"
	"time"

	"github.com/google/uuid"

	apptransaction "github.com/ssubedir/open-spanner/internal/metering/app/transaction"
	domainmeter "github.com/ssubedir/open-spanner/internal/metering/domain/meter"
	domainusage "github.com/ssubedir/open-spanner/internal/metering/domain/usage"
)

type Service interface {
	Create(ctx context.Context, cmd CreateCommand) (Result, error)
	CreateBulk(ctx context.Context, idempotencyKey string, commands []CreateCommand) (BulkResult, error)
	List(ctx context.Context, query ListQuery) ([]ListItemResult, error)
	ListDimensionValues(ctx context.Context, query DimensionValueListQuery) (DimensionValueListResult, error)
	ListEvents(ctx context.Context, query EventListQuery) (EventListResult, error)
	PruneEvents(ctx context.Context, cmd PruneCommand) (PruneResult, error)
	ListPruneRuns(ctx context.Context, query PruneRunListQuery) (PruneRunListResult, error)
	RecordIngestion(ctx context.Context, cmd IngestionCommand) (IngestionResult, error)
	ListIngestions(ctx context.Context, query IngestionListQuery) (IngestionListResult, error)
}

type service struct {
	meterRepo  domainmeter.Repository
	usageRepo  domainusage.Repository
	transactor apptransaction.Transactor
	now        func() time.Time
}

func NewService(meterRepo domainmeter.Repository, usageRepo domainusage.Repository, transactor apptransaction.Transactor) Service {
	if transactor == nil {
		panic("usage service requires a transactor")
	}

	return &service{
		meterRepo:  meterRepo,
		usageRepo:  usageRepo,
		transactor: transactor,
		now:        func() time.Time { return time.Now().UTC() },
	}
}

func newID() string {
	return uuid.NewString()
}
