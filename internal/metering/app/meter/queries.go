package meter

import (
	"context"

	"open-spanner/internal/metering/domain"
	domainmeter "open-spanner/internal/metering/domain/meter"
)

type ListQuery struct {
	Name string
}

type GetQuery struct {
	ID string
}

func (s *service) List(ctx context.Context, query ListQuery) ([]Result, error) {
	meters, err := s.repo.Find(ctx, domainmeter.Query{Name: query.Name})
	if err != nil {
		return nil, err
	}

	results := make([]Result, 0, len(meters))
	for _, meter := range meters {
		results = append(results, resultFromDomain(meter))
	}

	return results, nil
}

func (s *service) Get(ctx context.Context, query GetQuery) (Result, error) {
	meters, err := s.repo.Find(ctx, domainmeter.Query{ID: query.ID})
	if err != nil {
		return Result{}, err
	}
	if len(meters) == 0 {
		return Result{}, domain.ErrNotFound
	}

	return resultFromDomain(meters[0]), nil
}
