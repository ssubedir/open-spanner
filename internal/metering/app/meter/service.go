package meter

import (
	"context"
	"time"

	"github.com/google/uuid"

	domainmeter "github.com/ssubedir/open-spanner/internal/metering/domain/meter"
	domainusage "github.com/ssubedir/open-spanner/internal/metering/domain/usage"
)

type Service interface {
	Create(ctx context.Context, cmd CreateCommand) (Result, error)
	List(ctx context.Context, query ListQuery) (ListResult, error)
	ListStats(ctx context.Context, query StatsListQuery) (StatsListResult, error)
	Get(ctx context.Context, query GetQuery) (Result, error)
	Update(ctx context.Context, cmd UpdateCommand) (Result, error)
	Delete(ctx context.Context, cmd DeleteCommand) error
}

type service struct {
	repo      domainmeter.Repository
	usageRepo domainusage.Repository
	now       func() time.Time
}

func NewService(repo domainmeter.Repository, usageRepos ...domainusage.Repository) Service {
	var usageRepo domainusage.Repository
	if len(usageRepos) > 0 {
		usageRepo = usageRepos[0]
	}
	return &service{
		repo:      repo,
		usageRepo: usageRepo,
		now:       func() time.Time { return time.Now().UTC() },
	}
}

func newID() string {
	return uuid.NewString()
}
