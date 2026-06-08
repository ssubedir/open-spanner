package meter

import (
	"context"

	"github.com/ssubedir/open-spanner/internal/metering/app/page"
	"github.com/ssubedir/open-spanner/internal/metering/domain"
	domainmeter "github.com/ssubedir/open-spanner/internal/metering/domain/meter"
)

type ListQuery struct {
	Name   string
	Limit  int
	Cursor string
}

type StatsListQuery struct {
	Limit  int
	Cursor string
}

type GetQuery struct {
	ID string
}

func (s *service) List(ctx context.Context, query ListQuery) (ListResult, error) {
	cursor, err := page.Decode(query.Cursor)
	if err != nil {
		return ListResult{}, err
	}

	limit := domainmeter.NormalizeLimit(query.Limit)
	meters, err := s.repo.Find(ctx, domainmeter.Query{Name: query.Name, Cursor: cursor.Name, Limit: limit + 1})
	if err != nil {
		return ListResult{}, err
	}

	nextCursor := ""
	if len(meters) > limit {
		last := meters[limit-1]
		nextCursor, err = page.Encode(page.Cursor{Name: last.Name()})
		if err != nil {
			return ListResult{}, err
		}
		meters = meters[:limit]
	}

	results := make([]Result, 0, len(meters))
	for _, meter := range meters {
		results = append(results, resultFromDomain(meter))
	}

	return ListResult{Items: results, NextCursor: nextCursor}, nil
}

func (s *service) ListStats(ctx context.Context, query StatsListQuery) (StatsListResult, error) {
	cursor, err := page.Decode(query.Cursor)
	if err != nil {
		return StatsListResult{}, err
	}

	limit := domainmeter.NormalizeLimit(query.Limit)
	meters, err := s.repo.Find(ctx, domainmeter.Query{Cursor: cursor.Name, Limit: limit + 1})
	if err != nil {
		return StatsListResult{}, err
	}

	nextCursor := ""
	if len(meters) > limit {
		last := meters[limit-1]
		nextCursor, err = page.Encode(page.Cursor{Name: last.Name()})
		if err != nil {
			return StatsListResult{}, err
		}
		meters = meters[:limit]
	}

	usageByMeter := map[string]StatsResult{}
	if s.usageRepo != nil {
		stats, err := s.usageRepo.FindMeterStats(ctx)
		if err != nil {
			return StatsListResult{}, err
		}
		for _, stat := range stats {
			usageByMeter[stat.MeterName()] = StatsResult{
				MeterName:   stat.MeterName(),
				UsageEvents: stat.UsageEvents(),
				LastEventAt: stat.LastEventAt(),
			}
		}
	}

	results := make([]StatsResult, 0, len(meters))
	for _, meter := range meters {
		result := usageByMeter[meter.Name()]
		result.MeterName = meter.Name()
		result.EventRetentionDays = meter.EventRetentionDays()
		results = append(results, result)
	}

	return StatsListResult{Items: results, NextCursor: nextCursor}, nil
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
