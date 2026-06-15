package meter

import (
	"context"

	"github.com/ssubedir/open-spanner/internal/metering/domain"
	domainmeter "github.com/ssubedir/open-spanner/internal/metering/domain/meter"
)

type UpdateCommand struct {
	ID                 string
	Description        *string
	Unit               *string
	Aggregation        *domainmeter.Aggregation
	Dimensions         *[]domainmeter.Dimension
	MetadataSchema     *map[string]domainmeter.MetadataType
	EventRetentionDays *int
}

type DeleteCommand struct {
	ID string
}

func (s *service) Update(ctx context.Context, cmd UpdateCommand) (Result, error) {
	meters, err := s.repo.Find(ctx, domainmeter.Query{ID: cmd.ID})
	if err != nil {
		return Result{}, err
	}
	if len(meters) == 0 {
		return Result{}, domain.ErrNotFound
	}

	existing := meters[0]
	description := existing.Description()
	if cmd.Description != nil {
		description = *cmd.Description
	}
	unit := existing.Unit()
	if cmd.Unit != nil {
		unit = *cmd.Unit
	}
	aggregation := existing.Aggregation()
	if cmd.Aggregation != nil {
		aggregation = *cmd.Aggregation
	}
	dimensions := existing.Dimensions()
	metadataSchema := existing.MetadataSchema()
	useDimensions := true
	if cmd.Dimensions != nil {
		dimensions = *cmd.Dimensions
	} else if cmd.MetadataSchema != nil {
		metadataSchema = *cmd.MetadataSchema
		useDimensions = false
	}
	eventRetentionDays := existing.EventRetentionDays()
	if cmd.EventRetentionDays != nil {
		eventRetentionDays = *cmd.EventRetentionDays
	}

	var next domainmeter.Meter
	if useDimensions {
		next, err = domainmeter.NewWithDimensions(
			existing.ID(),
			existing.Name(),
			description,
			unit,
			aggregation,
			dimensions,
			eventRetentionDays,
			existing.CreatedAt(),
		)
	} else {
		next, err = domainmeter.New(
			existing.ID(),
			existing.Name(),
			description,
			unit,
			aggregation,
			metadataSchema,
			eventRetentionDays,
			existing.CreatedAt(),
		)
	}
	if err != nil {
		return Result{}, err
	}

	meter, err := s.repo.Save(ctx, next)
	if err != nil {
		return Result{}, err
	}

	return resultFromDomain(meter), nil
}

func (s *service) Delete(ctx context.Context, cmd DeleteCommand) error {
	return s.repo.Delete(ctx, domainmeter.Query{ID: cmd.ID})
}
