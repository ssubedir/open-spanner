package system

import (
	"context"
	"time"

	domainmeter "github.com/ssubedir/open-spanner/internal/metering/domain/meter"
	domainusage "github.com/ssubedir/open-spanner/internal/metering/domain/usage"
)

type Service interface {
	Stats(ctx context.Context) (StatsResult, error)
}

type service struct {
	meterRepo domainmeter.Repository
	usageRepo domainusage.Repository
}

type StatsResult struct {
	Meters       int
	UsageEvents  int
	PruneRuns    int
	LastPruneRun LastPruneRunResult
}

type LastPruneRunResult struct {
	ID        string
	Deleted   int
	DryRun    bool
	CreatedAt time.Time
}

func NewService(meterRepo domainmeter.Repository, usageRepo domainusage.Repository) Service {
	return &service{meterRepo: meterRepo, usageRepo: usageRepo}
}

func (s *service) Stats(ctx context.Context) (StatsResult, error) {
	meters, err := s.meterRepo.Count(ctx)
	if err != nil {
		return StatsResult{}, err
	}
	usageEvents, err := s.usageRepo.CountEvents(ctx)
	if err != nil {
		return StatsResult{}, err
	}
	pruneRuns, err := s.usageRepo.CountPruneRuns(ctx)
	if err != nil {
		return StatsResult{}, err
	}

	result := StatsResult{
		Meters:      meters,
		UsageEvents: usageEvents,
		PruneRuns:   pruneRuns,
	}

	runs, err := s.usageRepo.FindPruneRuns(ctx, domainusage.NewRunQuery(1, time.Time{}, ""))
	if err != nil {
		return StatsResult{}, err
	}
	if len(runs) > 0 {
		result.LastPruneRun = LastPruneRunResult{
			ID:        runs[0].ID(),
			Deleted:   runs[0].Deleted(),
			DryRun:    runs[0].DryRun(),
			CreatedAt: runs[0].CreatedAt(),
		}
	}

	return result, nil
}
