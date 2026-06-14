package sqlite

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/ssubedir/open-spanner/internal/config"
	"github.com/ssubedir/open-spanner/internal/metering/domain"
	domainmeter "github.com/ssubedir/open-spanner/internal/metering/domain/meter"
	domainusage "github.com/ssubedir/open-spanner/internal/metering/domain/usage"
)

func TestMeterRepositorySaveAndFind(t *testing.T) {
	ctx := context.Background()
	store := newTestStore(t, ctx)
	repo := NewMeterRepository(store)
	createdAt := time.Date(2026, 6, 8, 12, 0, 0, 0, time.UTC)
	meter := newTestMeter(t, "meter-1", "api_calls", createdAt)

	saved, err := repo.Save(ctx, meter)
	if err != nil {
		t.Fatalf("save meter: %v", err)
	}
	if saved.ID() != "meter-1" {
		t.Fatalf("saved meter id = %q, want meter-1", saved.ID())
	}

	byID, err := repo.Find(ctx, domainmeter.Query{ID: "meter-1"})
	if err != nil {
		t.Fatalf("find meter by id: %v", err)
	}
	if len(byID) != 1 || byID[0].Name() != "api_calls" {
		t.Fatalf("find by id returned %#v", byID)
	}

	byName, err := repo.Find(ctx, domainmeter.Query{Name: "api_calls"})
	if err != nil {
		t.Fatalf("find meter by name: %v", err)
	}
	if len(byName) != 1 || byName[0].ID() != "meter-1" {
		t.Fatalf("find by name returned %#v", byName)
	}
}

func TestMeterRepositoryDuplicateNameReturnsConflict(t *testing.T) {
	ctx := context.Background()
	store := newTestStore(t, ctx)
	repo := NewMeterRepository(store)

	if _, err := repo.Save(ctx, newTestMeter(t, "meter-1", "api_calls", time.Now())); err != nil {
		t.Fatalf("save first meter: %v", err)
	}

	_, err := repo.Save(ctx, newTestMeter(t, "meter-2", "api_calls", time.Now()))
	if !errors.Is(err, domain.ErrConflict) {
		t.Fatalf("duplicate meter error = %v, want ErrConflict", err)
	}
}

func TestMeterRepositoryFindLimitsResults(t *testing.T) {
	ctx := context.Background()
	store := newTestStore(t, ctx)
	repo := NewMeterRepository(store)

	for i, name := range []string{"api_calls", "tokens", "storage"} {
		if _, err := repo.Save(ctx, newTestMeter(t, "meter-"+string(rune('a'+i)), name, time.Now())); err != nil {
			t.Fatalf("save meter %s: %v", name, err)
		}
	}

	meters, err := repo.Find(ctx, domainmeter.Query{Limit: 2})
	if err != nil {
		t.Fatalf("find meters: %v", err)
	}
	if len(meters) != 2 {
		t.Fatalf("meter count = %d, want 2", len(meters))
	}
}

func TestMeterRepositoryUpdateAndDelete(t *testing.T) {
	ctx := context.Background()
	store := newTestStore(t, ctx)
	repo := NewMeterRepository(store)
	meter := newTestMeter(t, "meter-1", "api_calls", time.Now())

	if _, err := repo.Save(ctx, meter); err != nil {
		t.Fatalf("save meter: %v", err)
	}
	if _, err := repo.Save(ctx, meter.WithDescription("updated")); err != nil {
		t.Fatalf("update meter: %v", err)
	}

	found, err := repo.Find(ctx, domainmeter.Query{ID: "meter-1"})
	if err != nil {
		t.Fatalf("find meter: %v", err)
	}
	if len(found) != 1 || found[0].Description() != "updated" {
		t.Fatalf("found meter = %#v", found)
	}

	if err := repo.Delete(ctx, domainmeter.Query{ID: "meter-1"}); err != nil {
		t.Fatalf("delete meter: %v", err)
	}
	found, err = repo.Find(ctx, domainmeter.Query{ID: "meter-1"})
	if err != nil {
		t.Fatalf("find deleted meter: %v", err)
	}
	if len(found) != 0 {
		t.Fatalf("deleted meter still found: %#v", found)
	}
}

func TestUsageRepositorySaveIdempotencyAndQuery(t *testing.T) {
	ctx := context.Background()
	store := newTestStore(t, ctx)
	meterRepo := NewMeterRepository(store)
	usageRepo := NewUsageRepository(store)

	if _, err := meterRepo.Save(ctx, newTestMeter(t, "meter-1", "api_calls", time.Now())); err != nil {
		t.Fatalf("save meter: %v", err)
	}

	eventTime := time.Date(2026, 6, 8, 14, 15, 0, 0, time.UTC)
	first := newTestEvent(t, "event-1", "idem-1", "org_123", "api_calls", 3, eventTime)
	saved, err := usageRepo.Save(ctx, first)
	if err != nil {
		t.Fatalf("save usage event: %v", err)
	}

	duplicate := newTestEvent(t, "event-2", "idem-1", "org_123", "api_calls", 9, eventTime)
	replayed, err := usageRepo.Save(ctx, duplicate)
	if err != nil {
		t.Fatalf("save duplicate idempotency event: %v", err)
	}
	if replayed.ID() != saved.ID() || replayed.Quantity() != saved.Quantity() {
		t.Fatalf("replayed event = %s/%v, want %s/%v", replayed.ID(), replayed.Quantity(), saved.ID(), saved.Quantity())
	}

	later := newTestEvent(t, "event-3", "", "org_123", "api_calls", 2, eventTime.Add(2*time.Hour))
	if _, err := usageRepo.Save(ctx, later); err != nil {
		t.Fatalf("save later usage event: %v", err)
	}

	query, err := domainusage.NewQuery(
		"org_123",
		"api_calls",
		time.Date(2026, 6, 8, 0, 0, 0, 0, time.UTC),
		time.Date(2026, 6, 9, 0, 0, 0, 0, time.UTC),
		domainusage.BucketDay,
		domainmeter.AggregationSum,
		nil,
		"",
		0,
	)
	if err != nil {
		t.Fatalf("new usage query: %v", err)
	}

	buckets, err := usageRepo.Query(ctx, query)
	if err != nil {
		t.Fatalf("query usage: %v", err)
	}
	if len(buckets) != 1 {
		t.Fatalf("bucket count = %d, want 1", len(buckets))
	}
	if buckets[0].Quantity() != 5 {
		t.Fatalf("bucket quantity = %v, want 5", buckets[0].Quantity())
	}
}

func TestMeterRepositoryDeleteWithUsageReturnsConflict(t *testing.T) {
	ctx := context.Background()
	store := newTestStore(t, ctx)
	meterRepo := NewMeterRepository(store)
	usageRepo := NewUsageRepository(store)

	if _, err := meterRepo.Save(ctx, newTestMeter(t, "meter-1", "api_calls", time.Now())); err != nil {
		t.Fatalf("save meter: %v", err)
	}
	event := newTestEvent(t, "event-1", "", "org_123", "api_calls", 1, time.Now())
	if _, err := usageRepo.Save(ctx, event); err != nil {
		t.Fatalf("save usage: %v", err)
	}

	err := meterRepo.Delete(ctx, domainmeter.Query{ID: "meter-1"})
	if !errors.Is(err, domain.ErrConflict) {
		t.Fatalf("delete meter with usage error = %v, want ErrConflict", err)
	}
}

func TestUsageRepositoryQueryFiltersMetadata(t *testing.T) {
	ctx := context.Background()
	store := newTestStore(t, ctx)
	meterRepo := NewMeterRepository(store)
	usageRepo := NewUsageRepository(store)

	if _, err := meterRepo.Save(ctx, newTestMeter(t, "meter-1", "api_calls", time.Now())); err != nil {
		t.Fatalf("save meter: %v", err)
	}

	east := newTestEvent(t, "event-1", "", "org_123", "api_calls", 3, time.Date(2026, 6, 8, 10, 0, 0, 0, time.UTC))
	eastMetadata := east.Metadata()
	eastMetadata["region"] = "us-east-1"
	eastMetadata["retry"] = true
	if _, err := usageRepo.Save(ctx, east); err != nil {
		t.Fatalf("save east event: %v", err)
	}

	west := newTestEvent(t, "event-2", "", "org_123", "api_calls", 7, time.Date(2026, 6, 8, 11, 0, 0, 0, time.UTC))
	westMetadata := west.Metadata()
	westMetadata["region"] = "us-west-2"
	westMetadata["retry"] = false
	if _, err := usageRepo.Save(ctx, west); err != nil {
		t.Fatalf("save west event: %v", err)
	}

	query, err := domainusage.NewQuery(
		"org_123",
		"api_calls",
		time.Date(2026, 6, 8, 0, 0, 0, 0, time.UTC),
		time.Date(2026, 6, 9, 0, 0, 0, 0, time.UTC),
		domainusage.BucketDay,
		domainmeter.AggregationSum,
		map[string]string{"region": "us-east-1", "retry": "true"},
		"",
		0,
	)
	if err != nil {
		t.Fatalf("new usage query: %v", err)
	}

	buckets, err := usageRepo.Query(ctx, query)
	if err != nil {
		t.Fatalf("query usage: %v", err)
	}
	if len(buckets) != 1 || buckets[0].Quantity() != 3 {
		t.Fatalf("buckets = %#v, want one filtered bucket with quantity 3", buckets)
	}
}

func TestUsageRepositorySaveBulkReplaysIdempotencyKey(t *testing.T) {
	ctx := context.Background()
	store := newTestStore(t, ctx)
	meterRepo := NewMeterRepository(store)
	usageRepo := NewUsageRepository(store)

	if _, err := meterRepo.Save(ctx, newTestMeter(t, "meter-1", "api_calls", time.Now())); err != nil {
		t.Fatalf("save meter: %v", err)
	}

	events := []domainusage.Event{
		newTestEvent(t, "event-1", "usage-1", "org_123", "api_calls", 2, time.Date(2026, 6, 8, 10, 0, 0, 0, time.UTC)),
		newTestEvent(t, "event-2", "usage-2", "org_123", "api_calls", 3, time.Date(2026, 6, 8, 11, 0, 0, 0, time.UTC)),
	}
	first, err := usageRepo.SaveBulk(ctx, "batch-1", events)
	if err != nil {
		t.Fatalf("save first bulk: %v", err)
	}

	replayEvents := []domainusage.Event{
		newTestEvent(t, "event-3", "usage-3", "org_123", "api_calls", 100, time.Date(2026, 6, 8, 12, 0, 0, 0, time.UTC)),
	}
	second, err := usageRepo.SaveBulk(ctx, "batch-1", replayEvents)
	if err != nil {
		t.Fatalf("replay bulk: %v", err)
	}
	firstAccepted := first.Accepted()
	secondAccepted := second.Accepted()
	if len(secondAccepted) != len(firstAccepted) || secondAccepted[0].ID() != firstAccepted[0].ID() || secondAccepted[1].ID() != firstAccepted[1].ID() || len(second.Duplicates()) != 0 {
		t.Fatalf("replayed bulk = %#v, want original %#v", second, first)
	}

	query, err := domainusage.NewQuery(
		"org_123",
		"api_calls",
		time.Date(2026, 6, 8, 0, 0, 0, 0, time.UTC),
		time.Date(2026, 6, 9, 0, 0, 0, 0, time.UTC),
		domainusage.BucketDay,
		domainmeter.AggregationSum,
		nil,
		"",
		0,
	)
	if err != nil {
		t.Fatalf("new usage query: %v", err)
	}

	buckets, err := usageRepo.Query(ctx, query)
	if err != nil {
		t.Fatalf("query usage: %v", err)
	}
	if len(buckets) != 1 || buckets[0].Quantity() != 5 {
		t.Fatalf("buckets = %#v, want one bucket with quantity 5", buckets)
	}
}

func TestUsageRepositorySaveBulkReportsDuplicateEventKeys(t *testing.T) {
	ctx := context.Background()
	store := newTestStore(t, ctx)
	meterRepo := NewMeterRepository(store)
	usageRepo := NewUsageRepository(store)

	if _, err := meterRepo.Save(ctx, newTestMeter(t, "meter-1", "api_calls", time.Now())); err != nil {
		t.Fatalf("save meter: %v", err)
	}

	first, err := usageRepo.SaveBulk(ctx, "", []domainusage.Event{
		newTestEvent(t, "event-1", "usage-1", "org_123", "api_calls", 2, time.Date(2026, 6, 8, 10, 0, 0, 0, time.UTC)),
	})
	if err != nil {
		t.Fatalf("save first bulk: %v", err)
	}

	second, err := usageRepo.SaveBulk(ctx, "", []domainusage.Event{
		newTestEvent(t, "event-2", "usage-1", "org_123", "api_calls", 100, time.Date(2026, 6, 8, 11, 0, 0, 0, time.UTC)),
		newTestEvent(t, "event-3", "usage-2", "org_123", "api_calls", 3, time.Date(2026, 6, 8, 12, 0, 0, 0, time.UTC)),
	})
	if err != nil {
		t.Fatalf("save duplicate bulk: %v", err)
	}

	accepted := second.Accepted()
	duplicates := second.Duplicates()
	firstAccepted := first.Accepted()
	if len(accepted) != 1 || len(duplicates) != 1 {
		t.Fatalf("second bulk result = %#v", second)
	}
	if duplicates[0].ID() != firstAccepted[0].ID() || duplicates[0].Quantity() != 2 {
		t.Fatalf("duplicate = %#v, want original %#v", duplicates[0], firstAccepted[0])
	}
}

func TestUsageRepositoryHourlyAndMonthlyBuckets(t *testing.T) {
	ctx := context.Background()
	store := newTestStore(t, ctx)
	meterRepo := NewMeterRepository(store)
	usageRepo := NewUsageRepository(store)

	if _, err := meterRepo.Save(ctx, newTestMeter(t, "meter-1", "tokens", time.Now())); err != nil {
		t.Fatalf("save meter: %v", err)
	}

	times := []time.Time{
		time.Date(2026, 6, 8, 10, 10, 0, 0, time.UTC),
		time.Date(2026, 6, 8, 10, 40, 0, 0, time.UTC),
		time.Date(2026, 7, 1, 9, 0, 0, 0, time.UTC),
	}
	for i, eventTime := range times {
		event := newTestEvent(t, "event-"+string(rune('a'+i)), "", "org_123", "tokens", 1, eventTime)
		if _, err := usageRepo.Save(ctx, event); err != nil {
			t.Fatalf("save event %d: %v", i, err)
		}
	}

	hourlyQuery, err := domainusage.NewQuery("org_123", "tokens", times[0].Add(-time.Hour), times[1].Add(time.Hour), domainusage.BucketHour, domainmeter.AggregationSum, nil, "", 0)
	if err != nil {
		t.Fatalf("new hourly query: %v", err)
	}
	hourly, err := usageRepo.Query(ctx, hourlyQuery)
	if err != nil {
		t.Fatalf("query hourly: %v", err)
	}
	if len(hourly) != 1 || hourly[0].Quantity() != 2 {
		t.Fatalf("hourly buckets = %#v, want one bucket with quantity 2", hourly)
	}

	monthlyQuery, err := domainusage.NewQuery("org_123", "tokens", time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC), time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC), domainusage.BucketMonth, domainmeter.AggregationSum, nil, "", 0)
	if err != nil {
		t.Fatalf("new monthly query: %v", err)
	}
	monthly, err := usageRepo.Query(ctx, monthlyQuery)
	if err != nil {
		t.Fatalf("query monthly: %v", err)
	}
	if len(monthly) != 2 {
		t.Fatalf("monthly bucket count = %d, want 2", len(monthly))
	}
}

func TestUsageRepositoryQueryLimitsBuckets(t *testing.T) {
	ctx := context.Background()
	store := newTestStore(t, ctx)
	meterRepo := NewMeterRepository(store)
	usageRepo := NewUsageRepository(store)

	if _, err := meterRepo.Save(ctx, newTestMeter(t, "meter-1", "api_calls", time.Now())); err != nil {
		t.Fatalf("save meter: %v", err)
	}

	for day := 1; day <= 3; day++ {
		event := newTestEvent(t, "event-"+string(rune('a'+day)), "", "org_123", "api_calls", 1, time.Date(2026, 6, day, 10, 0, 0, 0, time.UTC))
		if _, err := usageRepo.Save(ctx, event); err != nil {
			t.Fatalf("save event %d: %v", day, err)
		}
	}

	query, err := domainusage.NewQuery(
		"org_123",
		"api_calls",
		time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC),
		time.Date(2026, 6, 4, 0, 0, 0, 0, time.UTC),
		domainusage.BucketDay,
		domainmeter.AggregationSum,
		nil,
		"",
		2,
	)
	if err != nil {
		t.Fatalf("new usage query: %v", err)
	}

	buckets, err := usageRepo.Query(ctx, query)
	if err != nil {
		t.Fatalf("query usage: %v", err)
	}
	if len(buckets) != 2 {
		t.Fatalf("bucket count = %d, want 2", len(buckets))
	}
}

func TestUsageRepositoryQueryGroupsBuckets(t *testing.T) {
	ctx := context.Background()
	store := newTestStore(t, ctx)
	meterRepo := NewMeterRepository(store)
	usageRepo := NewUsageRepository(store)

	if _, err := meterRepo.Save(ctx, newTestMeter(t, "meter-1", "api_calls", time.Now())); err != nil {
		t.Fatalf("save meter: %v", err)
	}

	inputs := []struct {
		id       string
		region   string
		quantity float64
	}{
		{"event-1", "us-east-1", 2},
		{"event-2", "us-west-2", 3},
		{"event-3", "us-east-1", 5},
	}
	for _, input := range inputs {
		event := newTestEvent(t, input.id, "", "org_123", "api_calls", input.quantity, time.Date(2026, 6, 8, 10, 0, 0, 0, time.UTC))
		event.Metadata()["region"] = input.region
		if _, err := usageRepo.Save(ctx, event); err != nil {
			t.Fatalf("save event %s: %v", input.id, err)
		}
	}

	query, err := domainusage.NewQuery(
		"org_123",
		"api_calls",
		time.Date(2026, 6, 8, 0, 0, 0, 0, time.UTC),
		time.Date(2026, 6, 9, 0, 0, 0, 0, time.UTC),
		domainusage.BucketDay,
		domainmeter.AggregationSum,
		nil,
		"region",
		0,
	)
	if err != nil {
		t.Fatalf("new usage query: %v", err)
	}

	buckets, err := usageRepo.Query(ctx, query)
	if err != nil {
		t.Fatalf("query usage: %v", err)
	}
	if len(buckets) != 2 {
		t.Fatalf("bucket count = %d, want 2", len(buckets))
	}
	if buckets[0].Group()["region"] != "us-east-1" || buckets[0].Quantity() != 7 {
		t.Fatalf("first grouped bucket = %#v", buckets[0])
	}
	if buckets[1].Group()["region"] != "us-west-2" || buckets[1].Quantity() != 3 {
		t.Fatalf("second grouped bucket = %#v", buckets[1])
	}
}

func TestUsageRepositoryQueryAppliesMetadataFilters(t *testing.T) {
	ctx := context.Background()
	store := newTestStore(t, ctx)
	meterRepo := NewMeterRepository(store)
	usageRepo := NewUsageRepository(store)

	if _, err := meterRepo.Save(ctx, newTestMeter(t, "meter-1", "api_calls", time.Now())); err != nil {
		t.Fatalf("save meter: %v", err)
	}

	inputs := []struct {
		id       string
		region   string
		quantity float64
	}{
		{"event-1", "us-east-1", 2},
		{"event-2", "us-west-2", 3},
		{"event-3", "us-east-1", 5},
	}
	for _, input := range inputs {
		event := newTestEvent(t, input.id, "", "org_123", "api_calls", input.quantity, time.Date(2026, 6, 8, 10, 0, 0, 0, time.UTC))
		event.Metadata()["region"] = input.region
		if _, err := usageRepo.Save(ctx, event); err != nil {
			t.Fatalf("save event %s: %v", input.id, err)
		}
	}

	query, err := domainusage.NewQuery(
		"org_123",
		"api_calls",
		time.Date(2026, 6, 8, 0, 0, 0, 0, time.UTC),
		time.Date(2026, 6, 9, 0, 0, 0, 0, time.UTC),
		domainusage.BucketDay,
		domainmeter.AggregationSum,
		map[string]string{"region": "us-east-1"},
		"region",
		0,
	)
	if err != nil {
		t.Fatalf("new usage query: %v", err)
	}

	buckets, err := usageRepo.Query(ctx, query)
	if err != nil {
		t.Fatalf("query usage: %v", err)
	}
	if len(buckets) != 1 || buckets[0].Quantity() != 7 || buckets[0].Group()["region"] != "us-east-1" {
		t.Fatalf("filtered buckets = %#v", buckets)
	}
}

func TestUsageRepositoryAggregationModes(t *testing.T) {
	ctx := context.Background()
	store := newTestStore(t, ctx)
	meterRepo := NewMeterRepository(store)
	usageRepo := NewUsageRepository(store)

	if _, err := meterRepo.Save(ctx, newTestMeter(t, "meter-1", "readings", time.Now())); err != nil {
		t.Fatalf("save meter: %v", err)
	}

	events := []struct {
		id       string
		quantity float64
		at       time.Time
	}{
		{"event-1", 10, time.Date(2026, 6, 8, 10, 10, 0, 0, time.UTC)},
		{"event-2", 4, time.Date(2026, 6, 8, 10, 20, 0, 0, time.UTC)},
		{"event-3", 16, time.Date(2026, 6, 8, 10, 30, 0, 0, time.UTC)},
	}
	for _, input := range events {
		event := newTestEvent(t, input.id, "", "org_123", "readings", input.quantity, input.at)
		if _, err := usageRepo.Save(ctx, event); err != nil {
			t.Fatalf("save event %s: %v", input.id, err)
		}
	}

	cases := []struct {
		name        string
		aggregation domainmeter.Aggregation
		want        float64
	}{
		{"sum", domainmeter.AggregationSum, 30},
		{"count", domainmeter.AggregationCount, 3},
		{"avg", domainmeter.AggregationAverage, 10},
		{"min", domainmeter.AggregationMinimum, 4},
		{"max", domainmeter.AggregationMaximum, 16},
		{"first", domainmeter.AggregationFirst, 10},
		{"last", domainmeter.AggregationLast, 16},
		{"rate", domainmeter.AggregationRate, float64(3) / 3600},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			query, err := domainusage.NewQuery(
				"org_123",
				"readings",
				time.Date(2026, 6, 8, 10, 0, 0, 0, time.UTC),
				time.Date(2026, 6, 8, 11, 0, 0, 0, time.UTC),
				domainusage.BucketHour,
				tc.aggregation,
				nil,
				"",
				0,
			)
			if err != nil {
				t.Fatalf("new usage query: %v", err)
			}

			buckets, err := usageRepo.Query(ctx, query)
			if err != nil {
				t.Fatalf("query usage: %v", err)
			}
			if len(buckets) != 1 {
				t.Fatalf("bucket count = %d, want 1", len(buckets))
			}
			if buckets[0].Quantity() != tc.want {
				t.Fatalf("bucket quantity = %v, want %v", buckets[0].Quantity(), tc.want)
			}
		})
	}
}

func TestStoreTracksMigrations(t *testing.T) {
	ctx := context.Background()
	store := newTestStore(t, ctx)

	var version int
	var dirty bool
	err := store.db.QueryRowContext(ctx, `
SELECT version, dirty
FROM schema_migrations
LIMIT 1
`).Scan(&version, &dirty)
	if err != nil {
		t.Fatalf("query schema migration version: %v", err)
	}

	if version != 2 || dirty {
		t.Fatalf("schema migration version = %d dirty=%v, want version 2 dirty=false", version, dirty)
	}
}

func TestStoreCreatesUsagePerformanceIndexes(t *testing.T) {
	ctx := context.Background()
	store := newTestStore(t, ctx)

	want := map[string]bool{
		"idx_usage_events_event_time_id":               false,
		"idx_usage_events_subject_event_time_id":       false,
		"idx_usage_events_meter_event_time_id":         false,
		"idx_usage_events_subject_meter_event_time_id": false,
	}

	rows, err := store.db.QueryContext(ctx, `
SELECT name
FROM sqlite_master
WHERE type = 'index'
	AND tbl_name = 'usage_events'
`)
	if err != nil {
		t.Fatalf("query sqlite indexes: %v", err)
	}
	defer rows.Close()

	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			t.Fatalf("scan sqlite index: %v", err)
		}
		if _, ok := want[name]; ok {
			want[name] = true
		}
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("sqlite index rows: %v", err)
	}

	for name, found := range want {
		if !found {
			t.Fatalf("missing sqlite usage_events index %s", name)
		}
	}
}

func TestStoreAppliesPoolConfig(t *testing.T) {
	ctx := context.Background()
	store, err := NewStore(ctx, ":memory:", config.DBPoolConfig{MaxOpenConns: 1})
	if err != nil {
		t.Fatalf("new sqlite store: %v", err)
	}
	defer store.Close()

	stats := store.db.Stats()
	if stats.MaxOpenConnections != 1 {
		t.Fatalf("max open connections = %d, want 1", stats.MaxOpenConnections)
	}
}

func newTestStore(t *testing.T, ctx context.Context) *Store {
	t.Helper()

	store, err := NewStore(ctx, ":memory:")
	if err != nil {
		t.Fatalf("new sqlite store: %v", err)
	}
	t.Cleanup(func() {
		if err := store.Close(); err != nil {
			t.Fatalf("close sqlite store: %v", err)
		}
	})

	return store
}

func newTestMeter(t *testing.T, id string, name string, createdAt time.Time) domainmeter.Meter {
	t.Helper()

	meter, err := domainmeter.New(id, name, "test meter", "count", domainmeter.AggregationSum, map[string]domainmeter.MetadataType{}, 0, createdAt)
	if err != nil {
		t.Fatalf("new meter: %v", err)
	}

	return meter
}

func newTestEvent(t *testing.T, id string, idempotencyKey string, subject string, meterName string, quantity float64, eventTime time.Time) domainusage.Event {
	t.Helper()

	event, err := domainusage.NewEvent(
		id,
		idempotencyKey,
		subject,
		meterName,
		quantity,
		eventTime,
		eventTime.Add(time.Second),
		map[string]any{"source": "test"},
	)
	if err != nil {
		t.Fatalf("new usage event: %v", err)
	}

	return event
}
