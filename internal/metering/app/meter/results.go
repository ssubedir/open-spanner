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
	MetadataSchema     map[string]string
	EventRetentionDays int
	CreatedAt          time.Time
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
	metadataSchema := map[string]string{}
	for key, value := range meter.MetadataSchema() {
		metadataSchema[key] = string(value)
	}

	return Result{
		ID:                 meter.ID(),
		Name:               meter.Name(),
		Description:        meter.Description(),
		Unit:               meter.Unit(),
		Aggregation:        string(meter.Aggregation()),
		MetadataSchema:     metadataSchema,
		EventRetentionDays: meter.EventRetentionDays(),
		CreatedAt:          meter.CreatedAt(),
	}
}
