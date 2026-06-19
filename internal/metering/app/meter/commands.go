package meter

import (
	"context"

	domainmeter "github.com/ssubedir/open-spanner/internal/metering/domain/meter"
)

type CreateCommand struct {
	Name               string
	Description        string
	Unit               string
	Aggregation        domainmeter.Aggregation
	Dimensions         []domainmeter.Dimension
	EventRetentionDays int
}

func (s *service) Create(ctx context.Context, cmd CreateCommand) (Result, error) {
	meter, err := domainmeter.NewWithDimensions(
		newID(),
		cmd.Name,
		cmd.Description,
		cmd.Unit,
		cmd.Aggregation,
		cmd.Dimensions,
		cmd.EventRetentionDays,
		s.now(),
	)
	if err != nil {
		return Result{}, err
	}

	meter, err = s.repo.Save(ctx, meter)
	if err != nil {
		return Result{}, err
	}

	return resultFromDomain(meter), nil
}
