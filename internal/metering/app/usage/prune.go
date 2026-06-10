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
	queries := make([]domainusage.PruneQuery, 0, len(meters))
	for _, meter := range meters {
		before := now.AddDate(0, 0, -meter.EventRetentionDays())
		query, err := domainusage.NewPruneQuery(meter.Name(), before)
		if err != nil {
			return PruneResult{}, err
		}
		queries = append(queries, query)
	}

	var run domainusage.PruneRun
	err = s.transactor.WithinTransaction(ctx, func(txCtx context.Context) error {
		result := PruneResult{DryRun: cmd.DryRun, Meters: make([]PruneMeterResult, 0, len(queries))}
		runMeters := make([]domainusage.PruneRunMeter, 0, len(queries))
		for _, query := range queries {
			deleted, err := s.prunableEventCount(txCtx, query, cmd.DryRun)
			if err != nil {
				return err
			}

			result.Deleted += deleted
			result.Meters = append(result.Meters, PruneMeterResult{
				MeterName: query.MeterName(),
				Before:    query.Before(),
				Deleted:   deleted,
			})
			runMeter, err := domainusage.NewPruneRunMeter(query.MeterName(), query.Before(), deleted)
			if err != nil {
				return err
			}
			runMeters = append(runMeters, runMeter)
		}

		var err error
		run, err = domainusage.NewPruneRun(newID(), cmd.DryRun, result.Deleted, runMeters, now)
		if err != nil {
			return err
		}
		run, err = s.usageRepo.SavePruneRun(txCtx, run)
		return err
	})
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
