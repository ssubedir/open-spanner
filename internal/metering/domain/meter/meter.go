package meter

import (
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/ssubedir/open-spanner/internal/metering/domain"
)

type Aggregation string
type MetadataType string

const (
	AggregationSum     Aggregation = "sum"
	AggregationCount   Aggregation = "count"
	AggregationAverage Aggregation = "avg"
	AggregationMinimum Aggregation = "min"
	AggregationMaximum Aggregation = "max"
	AggregationFirst   Aggregation = "first"
	AggregationLast    Aggregation = "last"
	AggregationRate    Aggregation = "rate"
)

const (
	MetadataString  MetadataType = "string"
	MetadataNumber  MetadataType = "number"
	MetadataBoolean MetadataType = "boolean"
)

const (
	DefaultEventRetentionDays = 90
	MaxEventRetentionDays     = 3650
)

type Meter struct {
	id                 string
	name               string
	description        string
	unit               string
	aggregation        Aggregation
	metadataSchema     map[string]MetadataType
	dimensions         []Dimension
	eventRetentionDays int
	createdAt          time.Time
}

func New(id, name, description, unit string, aggregation Aggregation, metadataSchema map[string]MetadataType, eventRetentionDays int, createdAt time.Time) (Meter, error) {
	dimensions, err := DimensionsFromMetadataSchema(metadataSchema)
	if err != nil {
		return Meter{}, err
	}
	return NewWithDimensions(id, name, description, unit, aggregation, dimensions, eventRetentionDays, createdAt)
}

func NewWithDimensions(id, name, description, unit string, aggregation Aggregation, dimensions []Dimension, eventRetentionDays int, createdAt time.Time) (Meter, error) {
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
	if !IsSupportedAggregation(aggregation) {
		return Meter{}, fmt.Errorf("%w: unsupported aggregation %q", domain.ErrInvalidInput, aggregation)
	}
	dimensions, metadataSchema, err := normalizeDimensions(dimensions)
	if err != nil {
		return Meter{}, err
	}
	eventRetentionDays, err = normalizeEventRetentionDays(eventRetentionDays)
	if err != nil {
		return Meter{}, err
	}
	if createdAt.IsZero() {
		return Meter{}, fmt.Errorf("%w: created at is required", domain.ErrInvalidInput)
	}

	return Meter{
		id:                 id,
		name:               name,
		description:        description,
		unit:               unit,
		aggregation:        aggregation,
		metadataSchema:     metadataSchema,
		dimensions:         dimensions,
		eventRetentionDays: eventRetentionDays,
		createdAt:          createdAt.UTC(),
	}, nil
}

func IsSupportedAggregation(aggregation Aggregation) bool {
	switch aggregation {
	case AggregationSum,
		AggregationCount,
		AggregationAverage,
		AggregationMinimum,
		AggregationMaximum,
		AggregationFirst,
		AggregationLast,
		AggregationRate:
		return true
	default:
		return false
	}
}

func normalizeMetadataSchema(schema map[string]MetadataType) (map[string]MetadataType, error) {
	normalized := map[string]MetadataType{}
	for key, value := range schema {
		key = strings.TrimSpace(key)
		if key == "" {
			return nil, fmt.Errorf("%w: metadata schema key is required", domain.ErrInvalidInput)
		}
		if !isSupportedMetadataType(value) {
			return nil, fmt.Errorf("%w: unsupported metadata type %q", domain.ErrInvalidInput, value)
		}
		normalized[key] = value
	}
	return normalized, nil
}

func isSupportedMetadataType(value MetadataType) bool {
	switch value {
	case MetadataString, MetadataNumber, MetadataBoolean:
		return true
	default:
		return false
	}
}

func normalizeEventRetentionDays(days int) (int, error) {
	if days == 0 {
		return DefaultEventRetentionDays, nil
	}
	if days < 0 {
		return 0, fmt.Errorf("%w: event retention days must be greater than zero", domain.ErrInvalidInput)
	}
	if days > MaxEventRetentionDays {
		return 0, fmt.Errorf("%w: event retention days cannot exceed %d", domain.ErrInvalidInput, MaxEventRetentionDays)
	}
	return days, nil
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

func (m Meter) MetadataSchema() map[string]MetadataType {
	schema := make(map[string]MetadataType, len(m.metadataSchema))
	for key, value := range m.metadataSchema {
		schema[key] = value
	}
	return schema
}

func (m Meter) Dimensions() []Dimension {
	dimensions := make([]Dimension, len(m.dimensions))
	copy(dimensions, m.dimensions)
	return dimensions
}

func (m Meter) EventRetentionDays() int {
	return m.eventRetentionDays
}

func (m Meter) ValidateMetadata(metadata map[string]any) error {
	if metadata == nil {
		metadata = map[string]any{}
	}
	for key := range metadata {
		if _, exists := m.metadataSchema[key]; !exists {
			return fmt.Errorf("%w: metadata key %q is not allowed", domain.ErrInvalidInput, key)
		}
	}
	for _, dimension := range m.dimensions {
		key := dimension.Name()
		expected := dimension.Type()
		value, exists := metadata[key]
		if !exists {
			if dimension.Required() {
				return fmt.Errorf("%w: metadata key %q is required", domain.ErrInvalidInput, key)
			}
			continue
		}
		if !metadataValueMatches(value, expected) {
			return fmt.Errorf("%w: metadata key %q must be %s", domain.ErrInvalidInput, key, expected)
		}
	}
	return nil
}

func metadataValueMatches(value any, expected MetadataType) bool {
	switch expected {
	case MetadataString:
		_, ok := value.(string)
		return ok
	case MetadataBoolean:
		_, ok := value.(bool)
		return ok
	case MetadataNumber:
		switch typed := value.(type) {
		case float64:
			return !math.IsNaN(typed) && !math.IsInf(typed, 0)
		case float32:
			return !math.IsNaN(float64(typed)) && !math.IsInf(float64(typed), 0)
		case int, int8, int16, int32, int64, uint, uint8, uint16, uint32, uint64:
			return true
		default:
			return false
		}
	default:
		return false
	}
}

func (m Meter) CreatedAt() time.Time {
	return m.createdAt
}

func (m Meter) WithDescription(description string) Meter {
	m.description = strings.TrimSpace(description)
	return m
}
