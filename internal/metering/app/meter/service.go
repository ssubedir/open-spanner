package meter

import (
	"context"
	"time"

	"github.com/google/uuid"

	domainmeter "open-spanner/internal/metering/domain/meter"
)

type Service interface {
	Create(ctx context.Context, cmd CreateCommand) (Result, error)
	List(ctx context.Context, query ListQuery) ([]Result, error)
	Get(ctx context.Context, query GetQuery) (Result, error)
}

type service struct {
	repo domainmeter.Repository
	now  func() time.Time
}

func NewService(repo domainmeter.Repository) Service {
	return &service{
		repo: repo,
		now:  func() time.Time { return time.Now().UTC() },
	}
}

func newID() string {
	return uuid.NewString()
}
