package usage

import (
	"context"
	"time"

	"github.com/google/uuid"

	domainmeter "github.com/ssubedir/open-spanner/internal/metering/domain/meter"
	domainusage "github.com/ssubedir/open-spanner/internal/metering/domain/usage"
)

type Service interface {
	Create(ctx context.Context, cmd CreateCommand) (Result, error)
	CreateBulk(ctx context.Context, idempotencyKey string, commands []CreateCommand) (BulkResult, error)
	List(ctx context.Context, query ListQuery) ([]ListItemResult, error)
	ListEvents(ctx context.Context, query EventListQuery) (EventListResult, error)
	PruneEvents(ctx context.Context, cmd PruneCommand) (PruneResult, error)
	ListPruneRuns(ctx context.Context, query PruneRunListQuery) (PruneRunListResult, error)
	RecordIngestion(ctx context.Context, cmd IngestionCommand) (IngestionResult, error)
	ListIngestions(ctx context.Context, query IngestionListQuery) (IngestionListResult, error)
}

type service struct {
	meterRepo domainmeter.Repository
	usageRepo domainusage.Repository
	now       func() time.Time
}

func NewService(meterRepo domainmeter.Repository, usageRepo domainusage.Repository) Service {
	return &service{
		meterRepo: meterRepo,
		usageRepo: usageRepo,
		now:       func() time.Time { return time.Now().UTC() },
	}
}

func newID() string {
	return uuid.NewString()
}
