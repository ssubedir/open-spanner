package meter

import (
	"fmt"
	"regexp"
	"sort"
	"strings"

	"github.com/ssubedir/open-spanner/internal/metering/domain"
)

var dimensionNamePattern = regexp.MustCompile(`^[A-Za-z0-9_-]+(\.[A-Za-z0-9_-]+)*$`)

type Dimension struct {
	name         string
	displayName  string
	description  string
	metadataType MetadataType
	required     bool
	deprecated   bool
}

func NewDimension(name string, metadataType MetadataType, displayName string, description string, required bool, deprecated ...bool) (Dimension, error) {
	name = strings.TrimSpace(name)
	displayName = strings.TrimSpace(displayName)
	description = strings.TrimSpace(description)
	isDeprecated := false
	if len(deprecated) > 0 {
		isDeprecated = deprecated[0]
	}

	if name == "" {
		return Dimension{}, fmt.Errorf("%w: dimension name is required", domain.ErrInvalidInput)
	}
	if !dimensionNamePattern.MatchString(name) {
		return Dimension{}, fmt.Errorf("%w: dimension name %q must use letters, numbers, underscores, hyphens, or dots", domain.ErrInvalidInput, name)
	}
	if IsReservedDimensionName(name) {
		return Dimension{}, fmt.Errorf("%w: dimension name %q is reserved", domain.ErrInvalidInput, name)
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
		deprecated:   isDeprecated,
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
			dimension.Deprecated(),
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

func IsReservedDimensionName(name string) bool {
	return strings.TrimSpace(name) == "subject"
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

func (d Dimension) RequiresValue() bool {
	return d.required && !d.deprecated
}

func (d Dimension) Deprecated() bool {
	return d.deprecated
}
