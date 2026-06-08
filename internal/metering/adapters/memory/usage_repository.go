package memory

import (
	"context"
	"sort"
	"time"

	domainusage "open-spanner/internal/metering/domain/usage"
)

type UsageRepository struct {
	store *Store
}

func NewUsageRepository(store *Store) *UsageRepository {
	return &UsageRepository{store: store}
}

func (r *UsageRepository) Save(ctx context.Context, event domainusage.Event) (domainusage.Event, error) {
	r.store.mu.Lock()
	defer r.store.mu.Unlock()

	if event.IdempotencyKey() != "" {
		if existing, exists := r.store.idempotencyKeys[event.IdempotencyKey()]; exists {
			return existing, nil
		}
		r.store.idempotencyKeys[event.IdempotencyKey()] = event
	}

	r.store.events = append(r.store.events, event)
	return event, nil
}

func (r *UsageRepository) Query(ctx context.Context, query domainusage.Query) ([]domainusage.Bucket, error) {
	r.store.mu.RLock()
	defer r.store.mu.RUnlock()

	bucketsByStart := map[time.Time]float64{}
	for _, event := range r.store.events {
		if event.Subject() != query.Subject() || event.MeterName() != query.MeterName() {
			continue
		}
		if event.EventTime().Before(query.From()) || !event.EventTime().Before(query.To()) {
			continue
		}

		start := bucketStart(event.EventTime(), query.BucketSize())
		bucketsByStart[start] += event.Quantity()
	}

	buckets := make([]domainusage.Bucket, 0, len(bucketsByStart))
	for start, quantity := range bucketsByStart {
		buckets = append(buckets, domainusage.NewBucket(
			query.Subject(),
			query.MeterName(),
			query.BucketSize(),
			start,
			quantity,
		))
	}

	sort.Slice(buckets, func(i, j int) bool {
		return buckets[i].BucketStart().Before(buckets[j].BucketStart())
	})

	return buckets, nil
}

func bucketStart(t time.Time, size domainusage.BucketSize) time.Time {
	t = t.UTC()
	switch size {
	case domainusage.BucketHour:
		return t.Truncate(time.Hour)
	case domainusage.BucketMonth:
		return time.Date(t.Year(), t.Month(), 1, 0, 0, 0, 0, time.UTC)
	default:
		return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, time.UTC)
	}
}
