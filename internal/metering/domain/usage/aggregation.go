package usage

import (
	"fmt"
	"sort"
	"time"

	domainmeter "github.com/ssubedir/open-spanner/internal/metering/domain/meter"
)

type aggregateState struct {
	bucketStart time.Time
	group       map[string]string
	sum         float64
	count       int
	min         float64
	max         float64
	firstValue  float64
	firstTime   time.Time
	lastValue   float64
	lastTime    time.Time
}

type aggregateKey struct {
	bucketStart time.Time
	groupValue  string
}

func AggregateEvents(query Query, events []Event) []Bucket {
	states := map[aggregateKey]*aggregateState{}

	for _, event := range events {
		if event.Subject() != query.Subject() || event.MeterName() != query.MeterName() {
			continue
		}
		if event.EventTime().Before(query.From()) || !event.EventTime().Before(query.To()) {
			continue
		}
		if !metadataMatches(event.Metadata(), query.Metadata()) {
			continue
		}
		if !query.Filter().Matches(event) {
			continue
		}

		start := BucketStart(event.EventTime(), query.BucketSize())
		group := groupForEvent(query, event)
		key := aggregateKey{bucketStart: start, groupValue: group[query.GroupBy()]}
		state := states[key]
		if state == nil {
			state = &aggregateState{
				bucketStart: start,
				group:       group,
				min:         event.Quantity(),
				max:         event.Quantity(),
				firstValue:  event.Quantity(),
				firstTime:   event.EventTime(),
				lastValue:   event.Quantity(),
				lastTime:    event.EventTime(),
			}
			states[key] = state
		}

		state.sum += event.Quantity()
		state.count++
		if event.Quantity() < state.min {
			state.min = event.Quantity()
		}
		if event.Quantity() > state.max {
			state.max = event.Quantity()
		}
		if event.EventTime().Before(state.firstTime) {
			state.firstTime = event.EventTime()
			state.firstValue = event.Quantity()
		}
		if event.EventTime().After(state.lastTime) || event.EventTime().Equal(state.lastTime) {
			state.lastTime = event.EventTime()
			state.lastValue = event.Quantity()
		}
	}

	keys := make([]aggregateKey, 0, len(states))
	for key := range states {
		keys = append(keys, key)
	}
	sort.Slice(keys, func(i, j int) bool {
		if keys[i].bucketStart.Equal(keys[j].bucketStart) {
			return keys[i].groupValue < keys[j].groupValue
		}
		return keys[i].bucketStart.Before(keys[j].bucketStart)
	})

	buckets := make([]Bucket, 0, len(keys))
	for _, key := range keys {
		state := states[key]
		buckets = append(buckets, NewBucketWithGroup(
			query.Subject(),
			query.MeterName(),
			query.BucketSize(),
			key.bucketStart,
			aggregateQuantity(query, *state),
			state.group,
		))
	}
	if query.Limit() < len(buckets) {
		return buckets[:query.Limit()]
	}

	return buckets
}

func groupForEvent(query Query, event Event) map[string]string {
	if query.GroupBy() == "" {
		return map[string]string{}
	}

	return map[string]string{
		query.GroupBy(): metadataValueString(event.Metadata()[query.GroupBy()]),
	}
}

func metadataMatches(eventMetadata map[string]any, filters map[string]string) bool {
	for key, expected := range filters {
		actual, exists := eventMetadata[key]
		if !exists || metadataValueString(actual) != expected {
			return false
		}
	}
	return true
}

func metadataValueString(value any) string {
	switch typed := value.(type) {
	case string:
		return typed
	case bool:
		return fmt.Sprintf("%t", typed)
	case float64:
		return fmt.Sprintf("%g", typed)
	case float32:
		return fmt.Sprintf("%g", typed)
	case int:
		return fmt.Sprintf("%d", typed)
	case int8:
		return fmt.Sprintf("%d", typed)
	case int16:
		return fmt.Sprintf("%d", typed)
	case int32:
		return fmt.Sprintf("%d", typed)
	case int64:
		return fmt.Sprintf("%d", typed)
	case uint:
		return fmt.Sprintf("%d", typed)
	case uint8:
		return fmt.Sprintf("%d", typed)
	case uint16:
		return fmt.Sprintf("%d", typed)
	case uint32:
		return fmt.Sprintf("%d", typed)
	case uint64:
		return fmt.Sprintf("%d", typed)
	default:
		return fmt.Sprintf("%v", typed)
	}
}

func BucketStart(t time.Time, size BucketSize) time.Time {
	t = t.UTC()
	switch size {
	case BucketHour:
		return t.Truncate(time.Hour)
	case BucketMonth:
		return time.Date(t.Year(), t.Month(), 1, 0, 0, 0, 0, time.UTC)
	default:
		return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, time.UTC)
	}
}

func aggregateQuantity(query Query, state aggregateState) float64 {
	switch query.Aggregation() {
	case domainmeter.AggregationCount:
		return float64(state.count)
	case domainmeter.AggregationAverage:
		return state.sum / float64(state.count)
	case domainmeter.AggregationMinimum:
		return state.min
	case domainmeter.AggregationMaximum:
		return state.max
	case domainmeter.AggregationFirst:
		return state.firstValue
	case domainmeter.AggregationLast:
		return state.lastValue
	case domainmeter.AggregationRate:
		return float64(state.count) / bucketDurationSeconds(state.bucketStart, query.BucketSize())
	default:
		return state.sum
	}
}

func bucketDurationSeconds(bucketStart time.Time, size BucketSize) float64 {
	switch size {
	case BucketHour:
		return float64(time.Hour / time.Second)
	case BucketMonth:
		return bucketStart.AddDate(0, 1, 0).Sub(bucketStart).Seconds()
	default:
		return float64((24 * time.Hour) / time.Second)
	}
}
