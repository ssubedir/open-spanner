package meter

import (
	"time"

	domainmeter "github.com/ssubedir/open-spanner/internal/metering/domain/meter"
)

type Result struct {
	ID                 string
	Name               string
	Description        string
	Unit               string
	Aggregation        string
	Dimensions         []DimensionResult
	EventRetentionDays int
	CreatedAt          time.Time
}

type DimensionResult struct {
	Name        string
	DisplayName string
	Description string
	Type        string
	Required    bool
	Deprecated  bool
}

type StatsResult struct {
	MeterName          string
	UsageEvents        int
	LastEventAt        time.Time
	EventRetentionDays int
}

type ListResult struct {
	Items      []Result
	NextCursor string
}

type StatsListResult struct {
	Items      []StatsResult
	NextCursor string
}

func resultFromDomain(meter domainmeter.Meter) Result {
	dimensions := make([]DimensionResult, 0, len(meter.Dimensions()))
	for _, dimension := range meter.Dimensions() {
		dimensions = append(dimensions, DimensionResult{
			Name:        dimension.Name(),
			DisplayName: dimension.DisplayName(),
			Description: dimension.Description(),
			Type:        string(dimension.Type()),
			Required:    dimension.Required(),
			Deprecated:  dimension.Deprecated(),
		})
	}

	return Result{
		ID:                 meter.ID(),
		Name:               meter.Name(),
		Description:        meter.Description(),
		Unit:               meter.Unit(),
		Aggregation:        string(meter.Aggregation()),
		Dimensions:         dimensions,
		EventRetentionDays: meter.EventRetentionDays(),
		CreatedAt:          meter.CreatedAt(),
	}
}
