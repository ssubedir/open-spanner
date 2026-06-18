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
	ListBreakdown(ctx context.Context, query BreakdownListQuery) (BreakdownListResult, error)
	ListEvents(ctx context.Context, query EventListQuery) (EventListResult, error)
	PruneEvents(ctx context.Context, cmd PruneCommand) (PruneResult, error)
	ListPruneRuns(ctx context.Context, query PruneRunListQuery) (PruneRunListResult, error)
	RecordIngestion(ctx context.Context, cmd IngestionCommand) (IngestionResult, error)
	ListIngestions(ctx context.Context, query IngestionListQuery) (IngestionListResult, error)
	CreateExportJob(ctx context.Context, cmd ExportJobCreateCommand) (ExportJobResult, error)
	GetExportJob(ctx context.Context, id string) (ExportJobResult, error)
	ListExportJobs(ctx context.Context, query ExportJobListQuery) (ExportJobListResult, error)
	ClaimExportJob(ctx context.Context, cmd ExportJobClaimCommand) (ExportJobResult, bool, error)
	CompleteExportJob(ctx context.Context, cmd ExportJobCompleteCommand) (ExportJobResult, error)
	FailExportJob(ctx context.Context, cmd ExportJobFailCommand) (ExportJobResult, error)
	CancelExportJob(ctx context.Context, cmd ExportJobCancelCommand) (ExportJobResult, error)
	RetryExportJob(ctx context.Context, cmd ExportJobRetryCommand) (ExportJobResult, error)
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
