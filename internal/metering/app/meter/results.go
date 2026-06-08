package meter

import (
	"time"

	domainmeter "open-spanner/internal/metering/domain/meter"
)

type Result struct {
	ID          string
	Name        string
	Description string
	Unit        string
	Aggregation string
	CreatedAt   time.Time
}

func resultFromDomain(meter domainmeter.Meter) Result {
	return Result{
		ID:          meter.ID(),
		Name:        meter.Name(),
		Description: meter.Description(),
		Unit:        meter.Unit(),
		Aggregation: string(meter.Aggregation()),
		CreatedAt:   meter.CreatedAt(),
	}
}
