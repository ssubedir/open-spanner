package usage

import (
	"context"

	domainmeter "github.com/ssubedir/open-spanner/internal/metering/domain/meter"
	domainusage "github.com/ssubedir/open-spanner/internal/metering/domain/usage"
)

func (s *service) PruneEvents(ctx context.Context, cmd PruneCommand) (PruneResult, error) {
	meters, err := s.meterRepo.Find(ctx, domainmeter.Query{Limit: domainmeter.MaxLimit})
	if err != nil {
		return PruneResult{}, err
	}

	now := s.now()
	result := PruneResult{DryRun: cmd.DryRun, Meters: make([]PruneMeterResult, 0, len(meters))}
	runMeters := make([]domainusage.PruneRunMeter, 0, len(meters))
	for _, meter := range meters {
		before := now.AddDate(0, 0, -meter.EventRetentionDays())
		query, err := domainusage.NewPruneQuery(meter.Name(), before)
		if err != nil {
			return PruneResult{}, err
		}

		deleted, err := s.prunableEventCount(ctx, query, cmd.DryRun)
		if err != nil {
			return PruneResult{}, err
		}

		result.Deleted += deleted
		result.Meters = append(result.Meters, PruneMeterResult{
			MeterName: meter.Name(),
			Before:    before,
			Deleted:   deleted,
		})
		runMeter, err := domainusage.NewPruneRunMeter(meter.Name(), before, deleted)
		if err != nil {
			return PruneResult{}, err
		}
		runMeters = append(runMeters, runMeter)
	}

	run, err := domainusage.NewPruneRun(newID(), cmd.DryRun, result.Deleted, runMeters, now)
	if err != nil {
		return PruneResult{}, err
	}
	run, err = s.usageRepo.SavePruneRun(ctx, run)
	if err != nil {
		return PruneResult{}, err
	}

	return pruneResultFromDomain(run), nil
}

func (s *service) prunableEventCount(ctx context.Context, query domainusage.PruneQuery, dryRun bool) (int, error) {
	if dryRun {
		return s.usageRepo.CountPrunableEvents(ctx, query)
	}
	return s.usageRepo.PruneEvents(ctx, query)
}
