package meter

import (
	"fmt"
	"sort"
	"strings"

	"github.com/ssubedir/open-spanner/internal/metering/domain"
)

type Dimension struct {
	name         string
	displayName  string
	description  string
	metadataType MetadataType
	required     bool
}

func NewDimension(name string, metadataType MetadataType, displayName string, description string, required bool) (Dimension, error) {
	name = strings.TrimSpace(name)
	displayName = strings.TrimSpace(displayName)
	description = strings.TrimSpace(description)

	if name == "" {
		return Dimension{}, fmt.Errorf("%w: dimension name is required", domain.ErrInvalidInput)
	}
	if !isSupportedMetadataType(metadataType) {
		return Dimension{}, fmt.Errorf("%w: unsupported dimension type %q", domain.ErrInvalidInput, metadataType)
	}
	if displayName == "" {
		displayName = humanizeDimensionName(name)
	}

	return Dimension{
		name:         name,
		displayName:  displayName,
		description:  description,
		metadataType: metadataType,
		required:     required,
	}, nil
}

func DimensionsFromMetadataSchema(schema map[string]MetadataType) ([]Dimension, error) {
	dimensions := make([]Dimension, 0, len(schema))
	for key, metadataType := range schema {
		dimension, err := NewDimension(key, metadataType, "", "", true)
		if err != nil {
			return nil, err
		}
		dimensions = append(dimensions, dimension)
	}
	sort.Slice(dimensions, func(i, j int) bool {
		return dimensions[i].Name() < dimensions[j].Name()
	})
	return dimensions, nil
}

func normalizeDimensions(dimensions []Dimension) ([]Dimension, map[string]MetadataType, error) {
	normalized := make([]Dimension, 0, len(dimensions))
	schema := make(map[string]MetadataType, len(dimensions))
	seen := map[string]struct{}{}

	for _, dimension := range dimensions {
		normalizedDimension, err := NewDimension(
			dimension.Name(),
			dimension.Type(),
			dimension.DisplayName(),
			dimension.Description(),
			dimension.Required(),
		)
		if err != nil {
			return nil, nil, err
		}
		if _, exists := seen[normalizedDimension.Name()]; exists {
			return nil, nil, fmt.Errorf("%w: dimension %q is already defined", domain.ErrInvalidInput, normalizedDimension.Name())
		}
		seen[normalizedDimension.Name()] = struct{}{}
		normalized = append(normalized, normalizedDimension)
		schema[normalizedDimension.Name()] = normalizedDimension.Type()
	}

	return normalized, schema, nil
}

func humanizeDimensionName(name string) string {
	parts := strings.FieldsFunc(name, func(r rune) bool {
		return r == '.' || r == '_' || r == '-'
	})
	for i, part := range parts {
		if part == "" {
			continue
		}
		parts[i] = strings.ToUpper(part[:1]) + part[1:]
	}
	return strings.Join(parts, " ")
}

func (d Dimension) Name() string {
	return d.name
}

func (d Dimension) DisplayName() string {
	return d.displayName
}

func (d Dimension) Description() string {
	return d.description
}

func (d Dimension) Type() MetadataType {
	return d.metadataType
}

func (d Dimension) Required() bool {
	return d.required
}
