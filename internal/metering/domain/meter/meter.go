package meter

import (
	"fmt"
	"strings"
	"time"

	"open-spanner/internal/metering/domain"
)

type Aggregation string

const (
	AggregationSum Aggregation = "sum"
)

type Meter struct {
	id          string
	name        string
	description string
	unit        string
	aggregation Aggregation
	createdAt   time.Time
}

func New(id, name, description, unit string, aggregation Aggregation, createdAt time.Time) (Meter, error) {
	name = strings.TrimSpace(name)
	unit = strings.TrimSpace(unit)
	description = strings.TrimSpace(description)

	if strings.TrimSpace(id) == "" {
		return Meter{}, fmt.Errorf("%w: meter id is required", domain.ErrInvalidInput)
	}
	if name == "" {
		return Meter{}, fmt.Errorf("%w: meter name is required", domain.ErrInvalidInput)
	}
	if unit == "" {
		return Meter{}, fmt.Errorf("%w: meter unit is required", domain.ErrInvalidInput)
	}
	if aggregation == "" {
		aggregation = AggregationSum
	}
	if aggregation != AggregationSum {
		return Meter{}, fmt.Errorf("%w: unsupported aggregation %q", domain.ErrInvalidInput, aggregation)
	}
	if createdAt.IsZero() {
		return Meter{}, fmt.Errorf("%w: created at is required", domain.ErrInvalidInput)
	}

	return Meter{
		id:          id,
		name:        name,
		description: description,
		unit:        unit,
		aggregation: aggregation,
		createdAt:   createdAt.UTC(),
	}, nil
}

func (m Meter) ID() string {
	return m.id
}

func (m Meter) Name() string {
	return m.name
}

func (m Meter) Description() string {
	return m.description
}

func (m Meter) Unit() string {
	return m.unit
}

func (m Meter) Aggregation() Aggregation {
	return m.aggregation
}

func (m Meter) CreatedAt() time.Time {
	return m.createdAt
}
