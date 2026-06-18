package usage

import (
	"time"

	domainusage "github.com/ssubedir/open-spanner/internal/metering/domain/usage"
)

type Result struct {
	ID             string
	IdempotencyKey string
	Subject        string
	MeterName      string
	Quantity       float64
	EventTime      time.Time
	ReceivedAt     time.Time
	Metadata       map[string]any
}

type ListItemResult struct {
	Subject     string
	MeterName   string
	BucketSize  string
	BucketStart time.Time
	Aggregation string
	Unit        string
	Quantity    float64
	Group       map[string]string
}

type EventListResult struct {
	Items      []Result
	NextCursor string
}

type DimensionValueResult struct {
	Field       string
	Value       string
	UsageEvents int
}

type DimensionValueListResult struct {
	Items []DimensionValueResult
}

type BreakdownResult struct {
	Field       string
	Value       string
	Quantity    float64
	UsageEvents int
	Aggregation string
	Unit        string
}

type BreakdownListResult struct {
	Items []BreakdownResult
}

type PruneRunListResult struct {
	Items      []PruneResult
	NextCursor string
}

type IngestionListResult struct {
	Items      []IngestionResult
	NextCursor string
}

type ExportJobListResult struct {
	Items      []ExportJobResult
	NextCursor string
}

type BulkResult struct {
	Accepted   []Result
	Duplicates []Result
	Failed     []BulkFailureResult
}

func (r BulkResult) Events() []Result {
	events := make([]Result, 0, len(r.Accepted)+len(r.Duplicates))
	events = append(events, r.Accepted...)
	events = append(events, r.Duplicates...)
	return events
}

type BulkFailureResult struct {
	Index   int
	Code    string
	Message string
}

type IngestionResult struct {
	ID         string
	Kind       string
	Accepted   int
	Duplicates int
	Failed     int
	CreatedAt  time.Time
}

type PruneResult struct {
	ID        string
	Deleted   int
	DryRun    bool
	Meters    []PruneMeterResult
	CreatedAt time.Time
}

type PruneMeterResult struct {
	MeterName string
	Before    time.Time
	Deleted   int
}

type ExportJobResult struct {
	ID           string
	Kind         string
	Status       string
	Format       string
	QueryJSON    string
	ErrorMessage string
	Attempts     int
	LockedUntil  time.Time
	ArtifactPath string
	ArtifactSize int64
	CreatedAt    time.Time
	UpdatedAt    time.Time
	CompletedAt  time.Time
}

func eventResultFromDomain(event domainusage.Event) Result {
	return Result{
		ID:             event.ID(),
		IdempotencyKey: event.IdempotencyKey(),
		Subject:        event.Subject(),
		MeterName:      event.MeterName(),
		Quantity:       event.Quantity(),
		EventTime:      event.EventTime(),
		ReceivedAt:     event.ReceivedAt(),
		Metadata:       event.Metadata(),
	}
}

func bucketResultFromDomain(bucket domainusage.Bucket, aggregation string, unit string) ListItemResult {
	return ListItemResult{
		Subject:     bucket.Subject(),
		MeterName:   bucket.MeterName(),
		BucketSize:  string(bucket.BucketSize()),
		BucketStart: bucket.BucketStart(),
		Aggregation: aggregation,
		Unit:        unit,
		Quantity:    bucket.Quantity(),
		Group:       bucket.Group(),
	}
}

func dimensionValueResultFromDomain(value domainusage.DimensionValue) DimensionValueResult {
	return DimensionValueResult{
		Field:       value.Field(),
		Value:       value.Value(),
		UsageEvents: value.UsageEvents(),
	}
}

func breakdownResultFromDomain(item domainusage.BreakdownItem, aggregation string, unit string) BreakdownResult {
	return BreakdownResult{
		Field:       item.Field(),
		Value:       item.Value(),
		Quantity:    item.Quantity(),
		UsageEvents: item.UsageEvents(),
		Aggregation: aggregation,
		Unit:        unit,
	}
}

func bulkResultFromDomain(result domainusage.BulkSaveResult) BulkResult {
	accepted := make([]Result, 0, len(result.Accepted()))
	for _, event := range result.Accepted() {
		accepted = append(accepted, eventResultFromDomain(event))
	}

	duplicates := make([]Result, 0, len(result.Duplicates()))
	for _, event := range result.Duplicates() {
		duplicates = append(duplicates, eventResultFromDomain(event))
	}

	return BulkResult{Accepted: accepted, Duplicates: duplicates}
}

func ingestionResultFromDomain(run domainusage.IngestionRun) IngestionResult {
	return IngestionResult{
		ID:         run.ID(),
		Kind:       string(run.Kind()),
		Accepted:   run.Accepted(),
		Duplicates: run.Duplicates(),
		Failed:     run.Failed(),
		CreatedAt:  run.CreatedAt(),
	}
}

func exportJobResultFromDomain(job domainusage.ExportJob) ExportJobResult {
	return ExportJobResult{
		ID:           job.ID(),
		Kind:         string(job.Kind()),
		Status:       string(job.Status()),
		Format:       string(job.Format()),
		QueryJSON:    job.QueryJSON(),
		ErrorMessage: job.ErrorMessage(),
		Attempts:     job.Attempts(),
		LockedUntil:  job.LockedUntil(),
		ArtifactPath: job.ArtifactPath(),
		ArtifactSize: job.ArtifactSize(),
		CreatedAt:    job.CreatedAt(),
		UpdatedAt:    job.UpdatedAt(),
		CompletedAt:  job.CompletedAt(),
	}
}

func pruneResultFromDomain(run domainusage.PruneRun) PruneResult {
	meters := make([]PruneMeterResult, 0, len(run.Meters()))
	for _, meter := range run.Meters() {
		meters = append(meters, PruneMeterResult{
			MeterName: meter.MeterName(),
			Before:    meter.Before(),
			Deleted:   meter.Deleted(),
		})
	}

	return PruneResult{
		ID:        run.ID(),
		Deleted:   run.Deleted(),
		DryRun:    run.DryRun(),
		Meters:    meters,
		CreatedAt: run.CreatedAt(),
	}
}
