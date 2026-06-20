package system

import (
	"context"
	"sync"
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
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	var (
		firstErr error
		once     sync.Once
		result   StatsResult
		runs     []domainusage.PruneRun
		wg       sync.WaitGroup
	)

	run := func(fn func() error) {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := fn(); err != nil {
				once.Do(func() {
					firstErr = err
					cancel()
				})
			}
		}()
	}

	run(func() error {
		meters, err := s.meterRepo.Count(ctx)
		result.Meters = meters
		return err
	})
	run(func() error {
		usageEvents, err := s.usageRepo.CountEvents(ctx)
		result.UsageEvents = usageEvents
		return err
	})
	run(func() error {
		pruneRuns, err := s.usageRepo.CountPruneRuns(ctx)
		result.PruneRuns = pruneRuns
		return err
	})
	run(func() error {
		var err error
		runs, err = s.usageRepo.FindPruneRuns(ctx, domainusage.NewRunQuery(1, time.Time{}, ""))
		return err
	})

	wg.Wait()
	if firstErr != nil {
		return StatsResult{}, firstErr
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
