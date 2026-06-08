package usage

import (
	"time"

	domainusage "open-spanner/internal/metering/domain/usage"
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
	Quantity    float64
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

func bucketResultFromDomain(bucket domainusage.Bucket) ListItemResult {
	return ListItemResult{
		Subject:     bucket.Subject(),
		MeterName:   bucket.MeterName(),
		BucketSize:  string(bucket.BucketSize()),
		BucketStart: bucket.BucketStart(),
		Quantity:    bucket.Quantity(),
	}
}
