package meter

import (
	"context"
	"fmt"

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

	if err := s.validateDimensionUpdate(ctx, existing, next); err != nil {
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

func (s *service) validateDimensionUpdate(ctx context.Context, existing domainmeter.Meter, next domainmeter.Meter) error {
	hasUsage, err := s.meterHasUsage(ctx, existing.Name())
	if err != nil {
		return err
	}
	if !hasUsage {
		return nil
	}

	existingByName := dimensionsByName(existing.Dimensions())
	nextByName := dimensionsByName(next.Dimensions())
	for name, current := range existingByName {
		updated, exists := nextByName[name]
		if !exists {
			return fmt.Errorf("%w: dimension %q cannot be removed after usage has been recorded", domain.ErrConflict, name)
		}
		if updated.Type() != current.Type() {
			return fmt.Errorf("%w: dimension %q type cannot change after usage has been recorded", domain.ErrConflict, name)
		}
		if !current.RequiresValue() && updated.RequiresValue() {
			return fmt.Errorf("%w: dimension %q cannot become required after usage has been recorded", domain.ErrConflict, name)
		}
	}
	for name, dimension := range nextByName {
		if _, exists := existingByName[name]; !exists && dimension.RequiresValue() {
			return fmt.Errorf("%w: required dimension %q cannot be added after usage has been recorded", domain.ErrConflict, name)
		}
	}

	return nil
}

func (s *service) meterHasUsage(ctx context.Context, meterName string) (bool, error) {
	if s.usageRepo == nil {
		return false, nil
	}

	stats, err := s.usageRepo.FindMeterStats(ctx)
	if err != nil {
		return false, err
	}
	for _, stat := range stats {
		if stat.MeterName() == meterName && stat.UsageEvents() > 0 {
			return true, nil
		}
	}
	return false, nil
}

func dimensionsByName(dimensions []domainmeter.Dimension) map[string]domainmeter.Dimension {
	byName := make(map[string]domainmeter.Dimension, len(dimensions))
	for _, dimension := range dimensions {
		byName[dimension.Name()] = dimension
	}
	return byName
}
