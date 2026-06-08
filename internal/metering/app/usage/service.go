package usage

import (
	"context"
	"time"

	"github.com/google/uuid"

	domainmeter "open-spanner/internal/metering/domain/meter"
	domainusage "open-spanner/internal/metering/domain/usage"
)

type Service interface {
	Create(ctx context.Context, cmd CreateCommand) (Result, error)
	List(ctx context.Context, query ListQuery) ([]ListItemResult, error)
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
